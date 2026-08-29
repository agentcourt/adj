#!/usr/bin/env -S uv run --no-cache --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import csv
import datetime as dt
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]

STAGES = ["inventory", "screen", "eval", "filter", "genes", "pca", "clusters", "aggregate", "pool"]


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def timestamp() -> str:
    return dt.datetime.now(dt.UTC).strftime("%Y%m%dT%H%M%SZ")


def resolve_path(path_text: str) -> Path:
    path = Path(path_text)
    if path.is_absolute():
        return path
    return ROOT / path


def display_path(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text())


def load_jsonl(path: Path) -> list[dict[str, Any]]:
    with path.open() as handle:
        return [json.loads(line) for line in handle if line.strip()]


def write_json(path: Path, payload: dict[str, Any]) -> None:
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False, sort_keys=True) + "\n")


def write_jsonl(path: Path, rows: list[dict[str, Any]]) -> None:
    with path.open("w") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")


def line_count(path: Path) -> int:
    try:
        with path.open() as handle:
            return sum(1 for line in handle if line.strip())
    except FileNotFoundError:
        return 0


def event(kind: str, **data: Any) -> None:
    print(json.dumps({"at": utc_now(), "kind": kind, **data}, ensure_ascii=False, sort_keys=True), flush=True)


def command_env() -> dict[str, str]:
    return os.environ.copy()


def run_command(
    cmd: list[str],
    *,
    cwd: Path,
    stage: str,
    stdout_path: Path | None = None,
) -> None:
    event("command_started", stage=stage, cmd=cmd)

    stdout_handle = None
    if stdout_path is not None:
        stdout_path.parent.mkdir(parents=True, exist_ok=True)
        stdout_handle = stdout_path.open("w")

    try:
        process = subprocess.Popen(
            cmd,
            cwd=cwd,
            env=command_env(),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )
        assert process.stdout is not None
        for line in process.stdout:
            print(line, end="")
            if stdout_handle is not None:
                stdout_handle.write(line)
        code = process.wait()
    finally:
        if stdout_handle is not None:
            stdout_handle.close()

    if code != 0:
        raise RuntimeError(f"{stage} command failed with exit code {code}: {' '.join(cmd)}")
    event("command_finished", stage=stage, exit_code=code)


def run_inventory(args: argparse.Namespace, run_dir: Path) -> Path:
    out_dir = run_dir / "inventory"
    cmd = [
        "uv",
        "run",
        "--no-cache",
        "--script",
        "tools/model_inventory.py",
        "--out-root",
        display_path(run_dir),
        "--run-id",
        "inventory",
        "--sample-seed",
        str(args.root_seed),
        "--request-timeout",
        str(args.inventory_request_timeout),
        "--retries",
        str(args.inventory_retries),
    ]
    if args.model_id:
        for model_id in args.model_id:
            cmd.extend(["--model-id", model_id])
    elif not args.full_catalog:
        cmd.extend(["--sample-models", str(args.root_count)])
    if args.inventory_sleep:
        cmd.extend(["--sleep", str(args.inventory_sleep)])
    run_command(cmd, cwd=ROOT, stage="inventory")
    return out_dir


def partition_variants_by_model(variants: list[dict[str, Any]], process_count: int) -> list[list[dict[str, Any]]]:
    groups: dict[str, list[dict[str, Any]]] = {}
    for variant in variants:
        model_id = str(variant.get("openrouter_model_id") or "")
        groups.setdefault(model_id, []).append(variant)
    partitions: list[list[dict[str, Any]]] = [
        [] for _ in range(min(process_count, len(groups)))
    ]
    for group in sorted(groups.values(), key=len, reverse=True):
        min(partitions, key=len).extend(group)
    return partitions


def eval_command(args: argparse.Namespace, variants_path: Path, out_dir: Path) -> list[str]:
    cmd = [
        "uv",
        "run",
        "--no-cache",
        "--script",
        "tools/run_variant_batch.py",
        "--variants",
        display_path(variants_path),
        "--out",
        display_path(out_dir),
        "--questions",
        args.questions,
        "--prompt",
        args.prompt,
        "--trials",
        str(args.eval_trials),
        "--timeout",
        str(args.timeout),
        "--tool-mode",
        "function",
        "--no-progress-timeout",
        str(args.eval_no_progress_timeout),
    ]
    if args.eval_variant_timeout is not None:
        cmd.extend(["--variant-timeout", str(args.eval_variant_timeout)])
    return cmd


def terminate_process(process: subprocess.Popen) -> None:
    if process.poll() is not None:
        return
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    try:
        process.wait(timeout=10)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            return
        process.wait()


def run_processes(stage: str, commands: list[tuple[str, list[str]]]) -> None:
    processes: list[tuple[str, list[str], subprocess.Popen]] = []
    try:
        for label, command in commands:
            event("command_started", stage=f"{stage}:{label}", cmd=command)
            process = subprocess.Popen(
                command,
                cwd=ROOT,
                env=command_env(),
                start_new_session=True,
            )
            processes.append((label, command, process))
        pending = list(processes)
        while pending:
            for entry in list(pending):
                label, command, process = entry
                code = process.poll()
                if code is None:
                    continue
                pending.remove(entry)
                event("command_finished", stage=f"{stage}:{label}", exit_code=code)
                if code != 0:
                    for _, _, other in pending:
                        terminate_process(other)
                    raise RuntimeError(
                        f"{stage}:{label} command failed with exit code {code}: {' '.join(command)}"
                    )
            if pending:
                time.sleep(1)
    except BaseException:
        for _, _, process in processes:
            terminate_process(process)
        raise


def combine_eval_partitions(out_dir: Path, partition_dirs: list[Path]) -> None:
    rows: list[dict[str, Any]] = []
    summaries: list[dict[str, Any]] = []
    for partition_dir in partition_dirs:
        rows.extend(load_eval_summary(partition_dir / "variant_summary.csv"))
        summaries.append(load_json(partition_dir / "summary.json"))
    rows.sort(key=lambda row: row_index(row, 0))
    if rows:
        fieldnames = sorted({key for row in rows for key in row})
        with (out_dir / "variant_summary.csv").open("w", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=fieldnames, extrasaction="ignore")
            writer.writeheader()
            writer.writerows(rows)
    summary = {
        "finished_at": utc_now(),
        "run_dir": display_path(out_dir),
        "partition_count": len(partition_dirs),
        "total_variants": sum(int(item["total_variants"]) for item in summaries),
        "completed_variants": sum(int(item["completed_variants"]) for item in summaries),
        "succeeded": sum(int(item["succeeded"]) for item in summaries),
        "failed": sum(int(item["failed"]) for item in summaries),
        "timed_out": sum(int(item["timed_out"]) for item in summaries),
        "score_failed": sum(int(item["score_failed"]) for item in summaries),
        "command_failed": sum(int(item["command_failed"]) for item in summaries),
        "observed_cost": sum(float(item.get("observed_cost") or 0) for item in summaries),
        "cost_observation_count": sum(int(item.get("cost_observation_count") or 0) for item in summaries),
        "partition_summaries": [display_path(path / "summary.json") for path in partition_dirs],
        "summary_csv": display_path(out_dir / "variant_summary.csv"),
    }
    write_json(out_dir / "summary.json", summary)
    event("eval_finished", **summary)


def screen_command(args: argparse.Namespace, variants_path: Path, out_dir: Path) -> list[str]:
    return [
        "uv",
        "run",
        "--no-cache",
        "--script",
        "tools/run_model_screen.py",
        "--variants",
        display_path(variants_path),
        "--out",
        display_path(out_dir),
        "--screen-command",
        args.screen_command,
        "--direct-timeout",
        str(args.screen_direct_timeout),
        "--pi-timeout",
        str(args.screen_pi_timeout),
        "--max-attempts",
        str(args.screen_max_attempts),
        "--podman-command",
        args.podman_command,
        "--pi-image",
        args.pi_image,
        "--pi-mcp-adapter",
        args.pi_mcp_adapter,
        "--pi-mcp-host",
        args.pi_mcp_host,
    ]


def combine_screen_partitions(out_dir: Path, partition_dirs: list[Path]) -> None:
    results: list[dict[str, Any]] = []
    accepted: list[dict[str, Any]] = []
    rejected: list[dict[str, Any]] = []
    summaries: list[dict[str, Any]] = []
    for partition_dir in partition_dirs:
        results.extend(load_jsonl(partition_dir / "results.jsonl"))
        accepted.extend(load_jsonl(partition_dir / "endpoint_variants.jsonl"))
        rejected.extend(load_jsonl(partition_dir / "rejected_variants.jsonl"))
        summaries.append(load_json(partition_dir / "summary.json"))
    results.sort(key=lambda row: row_index(row, 0))
    accepted.sort(key=lambda row: row_index(row, 0))
    rejected.sort(key=lambda row: row_index(row, 0))
    write_jsonl(out_dir / "results.jsonl", results)
    write_jsonl(out_dir / "endpoint_variants.jsonl", accepted)
    write_jsonl(out_dir / "rejected_variants.jsonl", rejected)
    summary = {
        "created_at": utc_now(),
        "partition_count": len(partition_dirs),
        "total_configurations": sum(int(item["total_configurations"]) for item in summaries),
        "accepted": len(accepted),
        "rejected": len(rejected),
        "observed_cost_usd": sum(float(item.get("observed_cost_usd") or 0) for item in summaries),
        "cost_observation_count": sum(int(item.get("cost_observation_count") or 0) for item in summaries),
        "partition_summaries": [display_path(path / "summary.json") for path in partition_dirs],
        "outputs": {
            "accepted_variants": display_path(out_dir / "endpoint_variants.jsonl"),
            "rejected_variants": display_path(out_dir / "rejected_variants.jsonl"),
            "results": display_path(out_dir / "results.jsonl"),
        },
    }
    write_json(out_dir / "summary.json", summary)
    event("screen_finished", **summary)


def run_screen(args: argparse.Namespace, run_dir: Path, inventory_dir: Path) -> Path:
    out_dir = run_dir / "screen"
    variants_path = inventory_dir / "endpoint_variants.jsonl"
    if args.screen_processes == 1:
        run_command(screen_command(args, variants_path, out_dir), cwd=ROOT, stage="screen")
        if line_count(out_dir / "endpoint_variants.jsonl") == 0:
            raise RuntimeError("screen accepted zero model configurations")
        return out_dir

    variants = load_jsonl(variants_path)
    partitions = partition_variants_by_model(variants, args.screen_processes)
    inputs_dir = out_dir / "partition-inputs"
    inputs_dir.mkdir(parents=True, exist_ok=False)
    partition_root = out_dir / "partitions"
    partition_root.mkdir()
    commands: list[tuple[str, list[str]]] = []
    partition_dirs: list[Path] = []
    for index, rows in enumerate(partitions, 1):
        label = f"partition-{index:02d}"
        partition_input = inputs_dir / f"{label}.jsonl"
        partition_dir = partition_root / label
        write_jsonl(partition_input, rows)
        partition_dirs.append(partition_dir)
        commands.append((label, screen_command(args, partition_input, partition_dir)))
    run_processes("screen", commands)
    combine_screen_partitions(out_dir, partition_dirs)
    if line_count(out_dir / "endpoint_variants.jsonl") == 0:
        raise RuntimeError("screen accepted zero model configurations")
    return out_dir


def run_eval(args: argparse.Namespace, run_dir: Path, screen_dir: Path) -> Path:
    out_dir = run_dir / "eval"
    variants_path = screen_dir / "endpoint_variants.jsonl"
    if args.eval_processes == 1:
        run_command(eval_command(args, variants_path, out_dir), cwd=ROOT, stage="eval")
        return out_dir

    variants = load_jsonl(variants_path)
    partitions = partition_variants_by_model(variants, args.eval_processes)
    inputs_dir = out_dir / "partition-inputs"
    inputs_dir.mkdir(parents=True, exist_ok=False)
    partition_root = out_dir / "partitions"
    partition_root.mkdir()
    commands: list[tuple[str, list[str]]] = []
    partition_dirs: list[Path] = []
    for index, rows in enumerate(partitions, 1):
        label = f"partition-{index:02d}"
        partition_input = inputs_dir / f"{label}.jsonl"
        partition_dir = partition_root / label
        write_jsonl(partition_input, rows)
        partition_dirs.append(partition_dir)
        commands.append((label, eval_command(args, partition_input, partition_dir)))
    run_processes("eval", commands)
    combine_eval_partitions(out_dir, partition_dirs)
    return out_dir


def row_index(row: dict[str, Any], fallback: int) -> int:
    value = row.get("combined_index") or row.get("index") or fallback
    return int(value)


def int_field(row: dict[str, Any], key: str) -> int:
    value = row.get(key)
    return int(value) if value not in (None, "") else 0


def float_field(row: dict[str, Any], key: str) -> float | None:
    value = row.get(key)
    if value in (None, ""):
        return None
    return float(value)


def load_eval_summary(path: Path) -> list[dict[str, Any]]:
    if path.suffix == ".csv":
        with path.open(newline="") as handle:
            return [dict(row) for row in csv.DictReader(handle)]
    return load_jsonl(path)


def filter_variants(
    args: argparse.Namespace,
    run_dir: Path,
    variant_path: Path,
    eval_summary_path: Path,
) -> Path:
    out_dir = run_dir / "filtered"
    summary_path = out_dir / "summary.json"
    out_dir.mkdir(parents=True, exist_ok=True)

    variants = load_jsonl(variant_path)
    summaries = load_eval_summary(eval_summary_path)
    summary_by_index = {row_index(row, position): row for position, row in enumerate(summaries, start=1)}
    if args.filter_deliberation_score_gt is None:
        score_operator = ">="
        score_threshold = args.filter_min_deliberation_score
    else:
        score_operator = ">"
        score_threshold = args.filter_deliberation_score_gt

    survivor_variants: list[dict[str, Any]] = []
    removed_variants: list[dict[str, Any]] = []
    for position, variant in enumerate(variants, start=1):
        index = row_index(variant, position)
        eval_row = summary_by_index.get(index)
        if eval_row is None:
            raise RuntimeError(f"missing eval summary for variant index {index}")
        run_exit_code = int_field(eval_row, "run_exit_code")
        if run_exit_code != 0:
            variant_status = eval_row.get("variant_status")
            removed_variants.append(
                {
                    "combined_index": index,
                    "openrouter_model_id": variant.get("openrouter_model_id"),
                    "provider_name": variant.get("provider_name"),
                    "endpoint_tag": variant.get("endpoint_tag"),
                    "quantization": variant.get("quantization"),
                    "reason": "precheck_rejected" if variant_status == "precheck_rejected" else "run_exit_code",
                    "run_exit_code": run_exit_code,
                    "variant_status": variant_status,
                    "precheck_reasons": eval_row.get("precheck_reasons"),
                    "timeout_kind": eval_row.get("timeout_kind"),
                }
            )
            continue
        provider_errors = int_field(eval_row, "provider_error_count")
        score = float_field(eval_row, "deliberation_score")
        if provider_errors != args.filter_provider_error_count:
            removed_variants.append(
                {
                    "combined_index": index,
                    "openrouter_model_id": variant.get("openrouter_model_id"),
                    "provider_name": variant.get("provider_name"),
                    "endpoint_tag": variant.get("endpoint_tag"),
                    "quantization": variant.get("quantization"),
                    "reason": "provider_error_count",
                    "provider_error_count": provider_errors,
                }
            )
            continue
        score_rejected = score is None
        if score is not None:
            if score_operator == ">":
                score_rejected = score <= score_threshold
            else:
                score_rejected = score < score_threshold
        if score_rejected:
            removed_variants.append(
                {
                    "combined_index": index,
                    "openrouter_model_id": variant.get("openrouter_model_id"),
                    "provider_name": variant.get("provider_name"),
                    "endpoint_tag": variant.get("endpoint_tag"),
                    "quantization": variant.get("quantization"),
                    "reason": "deliberation_score",
                    "deliberation_score": score,
                }
            )
            continue

        survivor = dict(variant)
        survivor["combined_index"] = index
        survivor["filter_provider_error_count"] = provider_errors
        survivor["filter_deliberation_score"] = score
        survivor_variants.append(survivor)

    if not survivor_variants:
        raise RuntimeError("filter produced zero survivor variants")

    write_jsonl(out_dir / "endpoint_variants.jsonl", survivor_variants)
    write_jsonl(out_dir / "removed_variants.jsonl", removed_variants)

    fields = [
        "combined_index",
        "openrouter_model_id",
        "provider_name",
        "endpoint_tag",
        "quantization",
        "endpoint_variant_id",
        "filter_provider_error_count",
        "filter_deliberation_score",
    ]
    with (out_dir / "endpoint_variants.csv").open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fields, extrasaction="ignore")
        writer.writeheader()
        writer.writerows(survivor_variants)

    summary = {
        "created_at": utc_now(),
        "source_variant_file": display_path(variant_path),
        "source_eval_summary_file": display_path(eval_summary_path),
        "filter_criteria": {
            "provider_error_count": args.filter_provider_error_count,
            "deliberation_score_operator": score_operator,
            "deliberation_score_threshold": score_threshold,
        },
        "total_variants": len(variants),
        "survivor_count": len(survivor_variants),
        "survivor_combined_indexes": [row["combined_index"] for row in survivor_variants],
        "removed_count": len(removed_variants),
        "removed_variant_indexes": [row["combined_index"] for row in removed_variants],
        "outputs": [
            "endpoint_variants.jsonl",
            "endpoint_variants.csv",
            "removed_variants.jsonl",
            "summary.json",
        ],
    }
    write_json(summary_path, summary)
    event("filter_finished", survivor_count=len(survivor_variants), output=display_path(out_dir))
    return out_dir


def selected_gene_indexes(args: argparse.Namespace) -> list[int]:
    if args.gene_index:
        seen: set[int] = set()
        indexes: list[int] = []
        for value in args.gene_index:
            if value not in seen:
                indexes.append(value)
                seen.add(value)
        return indexes
    return list(range(args.gene_count))


def validate_pool_capacity(args: argparse.Namespace, filtered_dir: Path) -> None:
    if not args.one_per_model:
        return
    variants = load_jsonl(filtered_dir / "endpoint_variants.jsonl")
    model_ids = [str(row.get("openrouter_model_id") or "").strip() for row in variants]
    if any(not model_id for model_id in model_ids):
        raise RuntimeError("--one-per-model requires openrouter_model_id in every filtered variant")
    unique_model_count = len(set(model_ids))
    if args.pool_size > unique_model_count:
        raise RuntimeError(
            f"--pool-size={args.pool_size} exceeds {unique_model_count} filtered unique models with --one-per-model"
        )
    event("pool_capacity_checked", pool_size=args.pool_size, unique_model_count=unique_model_count)


def run_genes(args: argparse.Namespace, run_dir: Path, filtered_dir: Path) -> dict[int, Path]:
    out: dict[int, Path] = {}
    commands: list[tuple[str, list[str]]] = []
    for gene_index in selected_gene_indexes(args):
        gene_dir = run_dir / "genes" / f"gene-{gene_index}"
        inference_dir = gene_dir / "inference"
        out[gene_index] = inference_dir
        cmd = [
            "uv",
            "run",
            "--no-cache",
            "--script",
            "tools/run_first_gene_inference_embeddings.py",
            "--variants",
            display_path(filtered_dir / "endpoint_variants.jsonl"),
            "--genes",
            args.genes,
            "--persona",
            args.persona,
            "--persona-record-path",
            args.persona_record_path,
            "--samples",
            str(args.samples_per_gene),
            "--gene-index",
            str(gene_index),
            "--out",
            display_path(inference_dir),
            "--timeout",
            str(args.timeout),
            "--embedding-model",
            args.embedding_model,
            "--temperature",
            str(args.temperature),
            "--top-p",
            str(args.top_p),
            "--max-tokens",
            str(args.max_tokens),
            "--completion-attempts",
            str(args.completion_attempts),
            "--retry-sleep",
            str(args.retry_sleep),
            "--allow-record-errors",
        ]
        commands.append((str(gene_index), cmd))
    for start in range(0, len(commands), args.gene_processes):
        run_processes("genes", commands[start : start + args.gene_processes])
    return out


def gene_record_failure(gene_index: int, record: dict[str, Any]) -> dict[str, Any] | None:
    status = str(record.get("status") or "")
    embedding = record.get("embedding")
    if status == "ok" and isinstance(embedding, list) and embedding:
        return None
    metadata = record.get("metadata") if isinstance(record.get("metadata"), dict) else {}
    return {
        "gene_index": gene_index,
        "sample_index": record.get("sample_index"),
        "status": status or "missing_status",
        "error_type": metadata.get("error_type") or metadata.get("embedding_error_type"),
        "error_message": metadata.get("error_message") or metadata.get("embedding_error_message"),
    }


def filter_gene_records(
    args: argparse.Namespace,
    run_dir: Path,
    filtered_dir: Path,
    gene_dirs: dict[int, Path],
) -> tuple[Path, dict[int, Path], int]:
    variants = load_jsonl(filtered_dir / "endpoint_variants.jsonl")
    variants_by_index = {row_index(row, 0): row for row in variants}
    if len(variants_by_index) != len(variants):
        raise RuntimeError("filtered variants contain duplicate combined_index values")
    if not gene_dirs:
        raise RuntimeError("no gene runs selected")

    expected_records = len(variants) * args.samples_per_gene
    records_by_gene: dict[int, list[dict[str, Any]]] = {}
    failures: dict[int, list[dict[str, Any]]] = {}
    for gene_index, inference_dir in sorted(gene_dirs.items()):
        summary = load_json(inference_dir / "summary.json")
        if int(summary.get("records_written") or 0) != expected_records:
            raise RuntimeError(
                f"gene {gene_index}: expected {expected_records} records, found {summary.get('records_written')}"
            )
        records = load_jsonl(inference_dir / "records.jsonl")
        if len(records) != expected_records:
            raise RuntimeError(f"gene {gene_index}: expected {expected_records} record rows, found {len(records)}")
        records_by_gene[gene_index] = records
        samples_by_variant: dict[int, list[dict[str, Any]]] = {index: [] for index in variants_by_index}
        for record in records:
            index = row_index(record, 0)
            if index not in variants_by_index:
                raise RuntimeError(f"gene {gene_index}: record refers to unknown combined_index {index}")
            samples_by_variant[index].append(record)
            failure = gene_record_failure(gene_index, record)
            if failure is not None:
                failures.setdefault(index, []).append(failure)
        expected_sample_indexes = set(range(1, args.samples_per_gene + 1))
        for index, samples in samples_by_variant.items():
            sample_indexes = {int(record.get("sample_index") or 0) for record in samples}
            if len(samples) != args.samples_per_gene or sample_indexes != expected_sample_indexes:
                failures.setdefault(index, []).append(
                    {
                        "gene_index": gene_index,
                        "status": "invalid_sample_set",
                        "expected_sample_indexes": sorted(expected_sample_indexes),
                        "observed_sample_indexes": sorted(sample_indexes),
                    }
                )

    excluded_indexes = set(failures)
    eligible_variants = [row for row in variants if row_index(row, 0) not in excluded_indexes]
    excluded_variants = [
        {**row, "gene_failures": failures[row_index(row, 0)]}
        for row in variants
        if row_index(row, 0) in excluded_indexes
    ]
    if not eligible_variants:
        raise RuntimeError("gene requests left no eligible model configurations")

    out_dir = run_dir / "gene-filter"
    records_dir = out_dir / "records"
    records_dir.mkdir(parents=True, exist_ok=False)
    eligible_variants_path = out_dir / "endpoint_variants.jsonl"
    write_jsonl(eligible_variants_path, eligible_variants)
    write_jsonl(out_dir / "excluded_variants.jsonl", excluded_variants)
    eligible_record_paths: dict[int, Path] = {}
    for gene_index, records in sorted(records_by_gene.items()):
        path = records_dir / f"gene-{gene_index}.jsonl"
        write_jsonl(path, [row for row in records if row_index(row, 0) not in excluded_indexes])
        eligible_record_paths[gene_index] = path

    summary = {
        "created_at": utc_now(),
        "input_variant_count": len(variants),
        "eligible_variant_count": len(eligible_variants),
        "excluded_variant_count": len(excluded_variants),
        "excluded_combined_indexes": sorted(excluded_indexes),
        "gene_indexes": sorted(gene_dirs),
        "samples_per_variant": args.samples_per_gene,
        "eligible_records_per_gene": len(eligible_variants) * args.samples_per_gene,
        "outputs": {
            "eligible_variants": display_path(eligible_variants_path),
            "excluded_variants": display_path(out_dir / "excluded_variants.jsonl"),
            "eligible_records": {
                str(index): display_path(path) for index, path in sorted(eligible_record_paths.items())
            },
        },
    }
    write_json(out_dir / "summary.json", summary)
    event("gene_filter_finished", **summary)
    return out_dir, eligible_record_paths, len(eligible_variants)


def select_pca_dimensions(args: argparse.Namespace, eligible_variant_count: int) -> int:
    embedding_count = eligible_variant_count * args.samples_per_gene
    if args.strict_pca_dimensions and args.pca_dimensions > embedding_count:
        raise RuntimeError(f"--pca-dimensions {args.pca_dimensions} exceeds embedding count {embedding_count}")
    return min(args.pca_dimensions, embedding_count)


def run_pca(
    args: argparse.Namespace,
    run_dir: Path,
    gene_record_paths: dict[int, Path],
    pca_dimensions: int,
) -> dict[int, Path]:
    out: dict[int, Path] = {}
    for gene_index, records_path in sorted(gene_record_paths.items()):
        pca_dir = run_dir / "genes" / f"gene-{gene_index}" / "pca"
        out[gene_index] = pca_dir
        cmd = [
            "uv",
            "run",
            "--no-cache",
            "--script",
            "tools/run_embedding_pca.py",
            "--records",
            display_path(records_path),
            "--out",
            display_path(pca_dir),
            "--dimensions",
            str(pca_dimensions),
        ]
        run_command(cmd, cwd=ROOT, stage=f"pca:{gene_index}")
    return out


def run_clustering(
    args: argparse.Namespace,
    run_dir: Path,
    pca_dirs: dict[int, Path],
    survivor_count: int,
    pca_dimensions: int,
) -> Path:
    out_dir = run_dir / "clusters"
    expected_rows = survivor_count * args.samples_per_gene
    cmd = [
        "uv",
        "run",
        "--no-cache",
        "--script",
        "tools/run_gene_pca_clustering.py",
        "--out",
        display_path(out_dir),
        "--pca-dimensions",
        str(pca_dimensions),
        "--min-k",
        str(args.min_k),
        "--max-k",
        str(args.max_k),
    ]
    cmd.extend(
        [
            "--expected-rows-per-gene",
            str(expected_rows),
            "--expected-variants-per-gene",
            str(survivor_count),
            "--expected-samples-per-variant",
            str(args.samples_per_gene),
        ]
    )
    for _, pca_dir in sorted(pca_dirs.items()):
        cmd.extend(["--pca-records", display_path(pca_dir / "pca-records.jsonl")])
    run_command(cmd, cwd=ROOT, stage="clusters")
    return out_dir


def run_aggregate(args: argparse.Namespace, run_dir: Path, filtered_dir: Path, clusters_dir: Path) -> Path:
    out_dir = run_dir / "variant-persona-clusters"
    cmd = [
        "uv",
        "run",
        "--no-cache",
        "--script",
        "tools/aggregate_variant_persona_clusters.py",
        "--clusters",
        display_path(clusters_dir / "clusters.jsonl"),
        "--cluster-fit",
        display_path(clusters_dir / "cluster-fit.json"),
        "--variants",
        display_path(filtered_dir / "endpoint_variants.jsonl"),
        "--out",
        display_path(out_dir),
    ]
    cmd.extend(["--expected-samples-per-gene", str(args.samples_per_gene)])
    run_command(cmd, cwd=ROOT, stage="aggregate")
    return out_dir


def run_pool(args: argparse.Namespace, run_dir: Path, aggregate_dir: Path) -> Path:
    out_dir = run_dir / "pool"
    pool_path = out_dir / "pool.jsonl"
    cmd = [
        "uv",
        "run",
        "--no-cache",
        "--script",
        "tools/sample-tuple-pool.py",
        display_path(aggregate_dir / "variant-persona-clusters.jsonl"),
        "--out",
        display_path(pool_path),
        "--diagnostics-out",
        display_path(out_dir / "diagnostics.jsonl"),
        "--equivalence-out",
        display_path(out_dir / "equivalence.jsonl"),
        "--pool-size",
        str(args.pool_size),
        "--seed",
        str(args.pool_seed),
    ]
    if args.no_dedupe_equivalent_endpoints:
        cmd.append("--no-dedupe-equivalent-endpoints")
    if args.without_replacement:
        cmd.append("--without-replacement")
    if args.one_per_model:
        cmd.append("--one-per-model")
    run_command(cmd, cwd=ROOT, stage="pool", stdout_path=out_dir / "sample.log")
    return out_dir


def should_stop(args: argparse.Namespace, stage: str) -> bool:
    return args.stop_after == stage


def collect_summary(run_dir: Path, pca_dimensions: int | None = None) -> dict[str, Any]:
    summary: dict[str, Any] = {
        "run_dir": display_path(run_dir),
        "updated_at": utc_now(),
    }
    paths = {
        "inventory": run_dir / "inventory" / "summary.json",
        "screen": run_dir / "screen" / "summary.json",
        "eval": run_dir / "eval" / "summary.json",
        "filtered": run_dir / "filtered" / "summary.json",
        "gene_filter": run_dir / "gene-filter" / "summary.json",
        "clusters": run_dir / "clusters" / "summary.json",
        "aggregate": run_dir / "variant-persona-clusters" / "summary.json",
    }
    for key, path in paths.items():
        if path.exists():
            summary[key] = load_json(path)
    pool_path = run_dir / "pool" / "pool.jsonl"
    diagnostics_path = run_dir / "pool" / "diagnostics.jsonl"
    equivalence_path = run_dir / "pool" / "equivalence.jsonl"
    if pool_path.exists():
        summary["pool"] = {
            "pool_path": display_path(pool_path),
            "diagnostics_path": display_path(diagnostics_path),
            "equivalence_path": display_path(equivalence_path),
            "pool_rows": line_count(pool_path),
            "diagnostic_rows": line_count(diagnostics_path),
            "equivalence_rows": line_count(equivalence_path),
        }
    gene_summaries = []
    for path in sorted((run_dir / "genes").glob("gene-*/inference/summary.json")):
        gene_summaries.append(load_json(path))
    if gene_summaries:
        summary["genes"] = gene_summaries
    pca_summaries = []
    for path in sorted((run_dir / "genes").glob("gene-*/pca/summary.json")):
        pca_summaries.append(load_json(path))
    if pca_summaries:
        summary["pca"] = pca_summaries
    if pca_dimensions is not None:
        summary["selected_pca_dimensions"] = pca_dimensions
    return summary


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the adj endpoint-variant model-pool pipeline.")
    parser.add_argument("--run-id", default=None, help="Run id under --out-root. Default: e2e-<UTC timestamp>.")
    parser.add_argument("--out-root", default="results")
    parser.add_argument("--root-count", type=int, default=5)
    parser.add_argument("--root-seed", type=int, default=0)
    parser.add_argument("--full-catalog", action="store_true", help="Inventory every model in the OpenRouter catalog.")
    parser.add_argument("--model-id", action="append", help="Specific OpenRouter model id. May be repeated. Overrides --root-count sampling.")
    parser.add_argument("--inventory-request-timeout", type=int, default=60)
    parser.add_argument("--inventory-retries", type=int, default=2)
    parser.add_argument("--inventory-sleep", type=float, default=0.0)
    parser.add_argument("--screen-command", default="../.bin/model-config-screen")
    parser.add_argument("--screen-processes", type=int, default=1)
    parser.add_argument("--screen-direct-timeout", type=int, default=20)
    parser.add_argument("--screen-pi-timeout", type=int, default=300)
    parser.add_argument("--screen-max-attempts", type=int, default=3)
    parser.add_argument("--podman-command", default="podman")
    parser.add_argument("--pi-image", default="agentcourt-pi-sandbox")
    parser.add_argument("--pi-mcp-adapter", default="/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter")
    parser.add_argument("--pi-mcp-host", default="127.0.0.1")
    parser.add_argument("--questions", default="sets/core20/questions.jsonl")
    parser.add_argument("--prompt", default="prompts/juror-single.md", help="Prompt file passed through the endpoint-evaluation stage.")
    parser.add_argument("--eval-trials", type=int, default=1)
    parser.add_argument("--eval-processes", type=int, default=1)
    parser.add_argument("--filter-provider-error-count", type=int, default=0)
    score_filter = parser.add_mutually_exclusive_group()
    score_filter.add_argument("--filter-min-deliberation-score", type=float, default=0.90)
    score_filter.add_argument("--filter-deliberation-score-gt", type=float)
    parser.add_argument("--genes", default="sampled-genes.json")
    parser.add_argument("--gene-count", type=int, default=2)
    parser.add_argument("--gene-index", action="append", type=int, help="Specific gene index. May be repeated. Overrides --gene-count.")
    parser.add_argument("--gene-processes", type=int, default=1)
    parser.add_argument("--persona", default="../common/etc/personas/generic.md")
    parser.add_argument("--persona-record-path", default="personas/generic.md")
    parser.add_argument("--samples-per-gene", type=int, default=1)
    parser.add_argument("--embedding-model", default="text-embedding-3-small")
    parser.add_argument("--temperature", type=float, default=0.7)
    parser.add_argument("--top-p", type=float, default=1.0)
    parser.add_argument("--max-tokens", type=int, default=512)
    parser.add_argument("--completion-attempts", type=int, default=3)
    parser.add_argument("--retry-sleep", type=float, default=2.0)
    parser.add_argument("--pca-dimensions", type=int, default=3)
    parser.add_argument("--strict-pca-dimensions", action="store_true")
    parser.add_argument("--min-k", type=int, default=2)
    parser.add_argument("--max-k", type=int, default=10)
    parser.add_argument("--pool-size", type=int, default=20)
    parser.add_argument("--pool-seed", type=int, default=0)
    parser.add_argument("--no-dedupe-equivalent-endpoints", action="store_true")
    parser.add_argument("--without-replacement", action="store_true")
    parser.add_argument("--one-per-model", action="store_true")
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument("--eval-no-progress-timeout", type=int)
    parser.add_argument("--eval-variant-timeout", type=int)
    parser.add_argument("--start-at", choices=["filter"])
    parser.add_argument("--existing-screen-variants")
    parser.add_argument("--existing-eval-summary")
    parser.add_argument("--stop-after", choices=STAGES)
    args = parser.parse_args(argv)

    if args.root_count < 1:
        raise SystemExit("--root-count must be positive")
    if args.full_catalog and args.model_id:
        raise SystemExit("--full-catalog and --model-id cannot be used together")
    if args.screen_processes < 1:
        raise SystemExit("--screen-processes must be positive")
    if args.screen_direct_timeout < 1 or args.screen_pi_timeout < 1:
        raise SystemExit("screen timeouts must be positive")
    if args.screen_max_attempts < 1 or args.screen_max_attempts > 4:
        raise SystemExit("--screen-max-attempts must be between 1 and 4")
    if args.eval_trials < 1:
        raise SystemExit("--eval-trials must be positive")
    if args.eval_processes < 1:
        raise SystemExit("--eval-processes must be positive")
    if args.filter_provider_error_count < 0:
        raise SystemExit("--filter-provider-error-count cannot be negative")
    if args.start_at == "filter":
        if not args.existing_screen_variants or not args.existing_eval_summary:
            raise SystemExit(
                "--start-at filter requires --existing-screen-variants and --existing-eval-summary"
            )
        if args.stop_after in {"inventory", "screen", "eval"}:
            raise SystemExit("--stop-after cannot precede --start-at filter")
    elif args.existing_screen_variants or args.existing_eval_summary:
        raise SystemExit("existing input paths require --start-at filter")
    if args.gene_count < 1:
        raise SystemExit("--gene-count must be positive")
    if args.gene_index and any(index < 0 for index in args.gene_index):
        raise SystemExit("--gene-index values cannot be negative")
    if args.gene_processes < 1:
        raise SystemExit("--gene-processes must be positive")
    if args.samples_per_gene < 1:
        raise SystemExit("--samples-per-gene must be positive")
    if args.completion_attempts < 1:
        raise SystemExit("--completion-attempts must be positive")
    if args.retry_sleep < 0:
        raise SystemExit("--retry-sleep cannot be negative")
    if args.pca_dimensions < 1:
        raise SystemExit("--pca-dimensions must be positive")
    if args.min_k < 2:
        raise SystemExit("--min-k must be at least 2")
    if args.max_k < args.min_k:
        raise SystemExit("--max-k must be greater than or equal to --min-k")
    if args.pool_size < 1:
        raise SystemExit("--pool-size must be positive")
    if args.timeout < 1:
        raise SystemExit("--timeout must be positive")
    if args.eval_no_progress_timeout is None:
        args.eval_no_progress_timeout = max(args.timeout * 2, 180)
    if args.eval_no_progress_timeout < 1:
        raise SystemExit("--eval-no-progress-timeout must be positive")
    if args.eval_variant_timeout is not None and args.eval_variant_timeout < args.eval_no_progress_timeout:
        raise SystemExit("--eval-variant-timeout must be greater than or equal to --eval-no-progress-timeout")
    return args


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    run_id = args.run_id or f"e2e-{timestamp()}"
    run_dir = resolve_path(args.out_root) / run_id
    summary_path = run_dir / "summary.json"

    if run_dir.exists():
        raise SystemExit(f"{display_path(run_dir)} already exists; use a different --run-id")
    run_dir.mkdir(parents=True, exist_ok=True)
    event("run_started", run_id=run_id, run_dir=display_path(run_dir))

    pca_dimensions: int | None = None
    try:
        if args.start_at == "filter":
            filtered_dir = filter_variants(
                args,
                run_dir,
                resolve_path(args.existing_screen_variants),
                resolve_path(args.existing_eval_summary),
            )
        else:
            inventory_dir = run_inventory(args, run_dir)
            if should_stop(args, "inventory"):
                write_json(summary_path, collect_summary(run_dir))
                return 0

            screen_dir = run_screen(args, run_dir, inventory_dir)
            if should_stop(args, "screen"):
                write_json(summary_path, collect_summary(run_dir))
                return 0

            eval_dir = run_eval(args, run_dir, screen_dir)
            if should_stop(args, "eval"):
                write_json(summary_path, collect_summary(run_dir))
                return 0

            filtered_dir = filter_variants(
                args,
                run_dir,
                screen_dir / "endpoint_variants.jsonl",
                eval_dir / "variant_summary.csv",
            )
        if should_stop(args, "filter"):
            write_json(summary_path, collect_summary(run_dir))
            return 0

        validate_pool_capacity(args, filtered_dir)
        gene_dirs = run_genes(args, run_dir, filtered_dir)
        eligible_dir, gene_record_paths, eligible_count = filter_gene_records(
            args, run_dir, filtered_dir, gene_dirs
        )
        validate_pool_capacity(args, eligible_dir)
        pca_dimensions = select_pca_dimensions(args, eligible_count)
        if should_stop(args, "genes"):
            write_json(summary_path, collect_summary(run_dir, pca_dimensions))
            return 0

        if pca_dimensions != args.pca_dimensions:
            event("pca_dimensions_capped", requested=args.pca_dimensions, selected=pca_dimensions)
        pca_dirs = run_pca(args, run_dir, gene_record_paths, pca_dimensions)
        if should_stop(args, "pca"):
            write_json(summary_path, collect_summary(run_dir, pca_dimensions))
            return 0

        clusters_dir = run_clustering(args, run_dir, pca_dirs, eligible_count, pca_dimensions)
        if should_stop(args, "clusters"):
            write_json(summary_path, collect_summary(run_dir, pca_dimensions))
            return 0

        aggregate_dir = run_aggregate(args, run_dir, eligible_dir, clusters_dir)
        if should_stop(args, "aggregate"):
            write_json(summary_path, collect_summary(run_dir, pca_dimensions))
            return 0

        run_pool(args, run_dir, aggregate_dir)
        write_json(summary_path, collect_summary(run_dir, pca_dimensions))
        event("run_finished", run_id=run_id, run_dir=display_path(run_dir), summary=display_path(summary_path))
        return 0
    except Exception as exc:
        write_json(
            summary_path,
            {
                **collect_summary(run_dir, pca_dimensions),
                "failed_at": utc_now(),
                "error_type": type(exc).__name__,
                "error": str(exc),
            },
        )
        event("run_failed", run_id=run_id, error_type=type(exc).__name__, error=str(exc))
        raise


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
