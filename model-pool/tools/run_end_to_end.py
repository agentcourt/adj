#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "scikit-learn"]
# ///
import argparse
import csv
import datetime as dt
import importlib.util
import json
import math
import os
import random
import re
import shutil
import signal
import subprocess
import sys
from collections import Counter, defaultdict
from pathlib import Path, PurePosixPath
from typing import Any

import numpy as np

from run_record import (
    PENDING_FILE_PREFIX,
    atomic_write_bytes,
    exclusive_run_lock,
    files_equal,
    input_file,
    prepare_run_record,
    publish_run_record,
    questions_snapshot,
    strict_json_loads,
    validate_pending_saved_run_record,
    validate_run_record_inputs,
    validate_saved_run_record,
)
from run_variant_batch import load_progress, preflight_batch_resume, validate_batch_summary, validate_summary_csv

ROOT = Path(__file__).resolve().parents[1]

STAGES = ["inventory", "eval", "filter", "genes", "pca", "clusters", "aggregate", "pool"]


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def timestamp() -> str:
    return dt.datetime.now(dt.UTC).strftime("%Y%m%dT%H%M%SZ")


def resolve_path(path_text: str) -> Path:
    path = Path(path_text).expanduser()
    if path.is_absolute():
        return path
    return ROOT / path


def display_path(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def load_json(path: Path) -> dict[str, Any]:
    value = strict_json_loads(path.read_bytes(), path)
    if not isinstance(value, dict):
        raise RuntimeError(f"{path}: expected a JSON object")
    return value


def load_jsonl(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    with path.open() as handle:
        for line_number, line in enumerate(handle, start=1):
            if not line.strip():
                continue
            row = strict_json_loads(line, f"{path}:{line_number}")
            if not isinstance(row, dict):
                raise RuntimeError(f"{path}:{line_number}: expected a JSON object")
            rows.append(row)
    return rows


def write_json(path: Path, payload: dict[str, Any]) -> None:
    atomic_write_bytes(
        path,
        (json.dumps(payload, indent=2, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n").encode("utf-8"),
    )


def write_jsonl(path: Path, rows: list[dict[str, Any]]) -> None:
    with path.open("w") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n")


def line_count(path: Path) -> int:
    try:
        with path.open() as handle:
            return sum(1 for line in handle if line.strip())
    except FileNotFoundError:
        return 0


def event(kind: str, **data: Any) -> None:
    print(json.dumps({"at": utc_now(), "kind": kind, **data}, ensure_ascii=False, sort_keys=True), flush=True)


def command_env() -> dict[str, str]:
    env = os.environ.copy()
    env.setdefault("UV_CACHE_DIR", "/tmp/uv-cache")
    return env


def validate_command_log(commands_path: Path) -> list[dict[str, Any]]:
    if not commands_path.exists():
        return []
    records = load_jsonl(commands_path)
    for line_number, record in enumerate(records, start=1):
        common = {"at", "stage", "cmd"}
        if not isinstance(record.get("at"), str) or not record["at"]:
            raise ValueError(f"{commands_path}:{line_number}: at must be a nonempty string")
        if not isinstance(record.get("stage"), str) or not record["stage"]:
            raise ValueError(f"{commands_path}:{line_number}: stage must be a nonempty string")
        if not isinstance(record.get("cmd"), list) or not all(isinstance(value, str) for value in record["cmd"]):
            raise ValueError(f"{commands_path}:{line_number}: cmd must be an array of strings")
        fields = set(record)
        if fields == common | {"cwd", "dry_run", "stdout_path"}:
            if not isinstance(record.get("cwd"), str) or not isinstance(record.get("dry_run"), bool):
                raise ValueError(f"{commands_path}:{line_number}: invalid command-start record")
            if record.get("stdout_path") is not None and not isinstance(record["stdout_path"], str):
                raise ValueError(f"{commands_path}:{line_number}: invalid stdout_path")
        elif fields == common | {"exit_code", "tail"}:
            if not isinstance(record.get("exit_code"), int) or isinstance(record.get("exit_code"), bool):
                raise ValueError(f"{commands_path}:{line_number}: exit_code must be an integer")
            if not isinstance(record.get("tail"), list) or not all(isinstance(value, str) for value in record["tail"]):
                raise ValueError(f"{commands_path}:{line_number}: tail must be an array of strings")
        else:
            raise ValueError(f"{commands_path}:{line_number}: command record fields differ from the expected schema")
    return records


def append_command_record(commands_path: Path, record: dict[str, Any]) -> None:
    records = validate_command_log(commands_path)
    records.append(record)
    data = b"".join(
        (json.dumps(value, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n").encode("utf-8")
        for value in records
    )
    atomic_write_bytes(commands_path, data)


def cleanup_root_atomic_temporaries(run_dir: Path) -> None:
    for path in run_dir.iterdir():
        if path.is_file() and path.name.startswith(PENDING_FILE_PREFIX):
            path.unlink()


def run_command(
    cmd: list[str],
    *,
    cwd: Path,
    commands_path: Path,
    stage: str,
    dry_run: bool,
    lock_fd: int,
    stdout_path: Path | None = None,
) -> None:
    record = {
        "at": utc_now(),
        "stage": stage,
        "cmd": cmd,
        "cwd": display_path(cwd),
        "dry_run": dry_run,
        "stdout_path": display_path(stdout_path) if stdout_path else None,
    }
    append_command_record(commands_path, record)
    event("command_started", stage=stage, cmd=cmd)
    if dry_run:
        event("command_skipped", stage=stage, reason="dry_run")
        return

    stdout_handle = None
    if stdout_path is not None:
        stdout_path.parent.mkdir(parents=True, exist_ok=True)
        stdout_handle = stdout_path.open("w")

    process: subprocess.Popen[str] | None = None
    try:
        process = subprocess.Popen(
            cmd,
            cwd=cwd,
            env=command_env(),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
            pass_fds=(lock_fd,),
            start_new_session=True,
        )
        assert process.stdout is not None
        tail: list[str] = []
        for line in process.stdout:
            print(line, end="")
            if stdout_handle is not None:
                stdout_handle.write(line)
            tail.append(line.rstrip())
            tail = tail[-20:]
        code = process.wait()
    except BaseException as exc:
        if process is not None and process.poll() is None:
            try:
                os.killpg(process.pid, signal.SIGTERM)
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
            except Exception as cleanup_error:
                exc.add_note(f"failed to terminate stage child {process.pid}: {cleanup_error}")
        raise
    finally:
        if stdout_handle is not None:
            stdout_handle.close()

    append_command_record(
        commands_path,
        {
            "at": utc_now(),
            "stage": stage,
            "cmd": cmd,
            "exit_code": code,
            "tail": tail[-8:],
        },
    )
    if code != 0:
        raise RuntimeError(f"{stage} command failed with exit code {code}: {' '.join(cmd)}")
    event("command_finished", stage=stage, exit_code=code)


def jsonl_object_count(path: Path) -> int:
    count = 0
    with path.open() as handle:
        for line_number, line in enumerate(handle, start=1):
            if not line.strip():
                continue
            row = strict_json_loads(line, f"{path}:{line_number}")
            if not isinstance(row, dict):
                raise RuntimeError(f"{path}:{line_number}: expected a JSON object")
            count += 1
    return count


def csv_matches_rows(path: Path, fieldnames: list[str], rows: list[dict[str, Any]], cell: Any = None) -> bool:
    if not path.is_file():
        return False
    converter = cell or (lambda value: "" if value is None else str(value))
    with path.open(newline="") as handle:
        reader = csv.DictReader(handle)
        if reader.fieldnames != fieldnames:
            return False
        found = list(reader)
    expected = [{field: converter(row.get(field)) for field in fieldnames} for row in rows]
    return found == expected


def exact_completed_files(
    root: Path,
    expected: set[Path],
    *,
    ignored_names: set[str] | None = None,
) -> bool:
    ignored = ignored_names or set()
    found: set[Path] = set()
    for path in root.rglob("*"):
        if path.is_symlink():
            return False
        if path.is_file() and path.name not in ignored:
            found.add(path.resolve())
    return found == {path.resolve() for path in expected}


def stage_record_dir(run_dir: Path, stage: str) -> Path:
    return run_dir / "stage-records" / stage


def stage_files(root: Path) -> list[Path]:
    if not root.is_dir():
        return []
    excluded = {"ACTIVE_PID", "RUN_OWNER.json", "STOP"}
    return sorted(
        path.resolve()
        for path in root.rglob("*")
        if path.is_file() and path.name not in excluded and not path.name.endswith(".tmp")
    )


def stage_sources(output_dir: Path, upstream: list[Path]) -> list[Path]:
    sources = {path.expanduser().resolve() for path in upstream}
    sources.update(stage_files(output_dir))
    return sorted(sources)


def stage_parameters(args: argparse.Namespace, stage: str, **extra: Any) -> dict[str, Any]:
    return {"stage": stage, "run": run_parameters(args), **extra}


def stage_record_complete(
    args: argparse.Namespace,
    run_dir: Path,
    stage: str,
    output_dir: Path,
    upstream: list[Path],
    record_path: Path | None = None,
    **extra: Any,
) -> bool:
    record_dir = record_path or stage_record_dir(run_dir, stage)
    if not record_dir.exists():
        return False
    sources = stage_sources(output_dir, upstream)
    inputs = [input_file(str(source), source, f"files/{index:04d}-{source.name}") for index, source in enumerate(sources, start=1)]
    recorded_dir = stage_record_dir(run_dir, stage)
    if record_path is not None and record_path != recorded_dir:
        manifest_path = record_dir / "manifest.json"
        manifest = strict_json_loads(manifest_path.read_bytes(), manifest_path)
        if not isinstance(manifest, dict):
            raise ValueError(f"pending stage manifest must be a JSON object: {manifest_path}")
        if manifest.get("run_dir") == str(record_dir.resolve()):
            recorded_dir = record_dir
    validate_run_record_inputs(
        record_dir,
        kind="model-pool-stage",
        parameters=stage_parameters(args, stage, **extra),
        inputs=inputs,
        recorded_run_dir=recorded_dir,
    )
    return True


def record_stage(
    args: argparse.Namespace,
    run_dir: Path,
    stage: str,
    output_dir: Path,
    upstream: list[Path],
    **extra: Any,
) -> None:
    sources = stage_sources(output_dir, upstream)
    if not sources:
        raise RuntimeError(f"stage {stage}: no files to record")
    inputs = [input_file(str(source), source, f"files/{index:04d}-{source.name}") for index, source in enumerate(sources, start=1)]
    publish_run_record(
        stage_record_dir(run_dir, stage),
        kind="model-pool-stage",
        parameters=stage_parameters(args, stage, **extra),
        inputs=inputs,
    )


def pending_stage_name(path: Path) -> str | None:
    match = re.fullmatch(r"\.(inventory|eval|filter|clusters|aggregate|pool|gene-[0-9]+|pca-[0-9]+)\.pending-.+", path.name)
    return match.group(1) if match else None


def preflight_stage_records(run_dir: Path) -> dict[str, tuple[Path, bool]]:
    root = run_dir / "stage-records"
    if not root.exists():
        return {}
    if not root.is_dir():
        raise ValueError(f"stage-records is not a directory: {root}")
    pending: dict[str, tuple[Path, bool]] = {}
    for path in sorted(root.iterdir()):
        stage = pending_stage_name(path)
        if stage is not None and path.is_dir() and not path.is_symlink():
            if stage in pending:
                raise ValueError(f"multiple pending stage records exist for {stage}: {pending[stage][0]} and {path}")
            final = stage_record_dir(run_dir, stage)
            if final.exists():
                raise ValueError(f"both pending and published stage records exist for {stage}: {path} and {final}")
            pending[stage] = (
                path,
                validate_pending_saved_run_record(path, final),
            )
        elif not path.is_dir():
            raise ValueError(f"unexpected stage-record entry: {path}")
        else:
            validate_saved_run_record(path)
    return pending


def provider_stage(stage: str) -> bool:
    return stage in {"inventory", "eval"} or stage.startswith("gene-")


def finalize_pending_stage_records(run_dir: Path, pending: dict[str, tuple[Path, bool]]) -> set[str]:
    interrupted: set[str] = set()
    for stage, (path, complete) in sorted(pending.items()):
        final = stage_record_dir(run_dir, stage)
        if complete and provider_stage(stage) and not final.exists():
            manifest_path = path / "manifest.json"
            manifest = strict_json_loads(manifest_path.read_bytes(), manifest_path)
            if not isinstance(manifest, dict):
                raise ValueError(f"pending stage manifest must be a JSON object: {manifest_path}")
            manifest["run_dir"] = str(final.resolve())
            atomic_write_bytes(
                manifest_path,
                (json.dumps(manifest, ensure_ascii=False, allow_nan=False, indent=2, sort_keys=True) + "\n").encode(
                    "utf-8"
                ),
            )
            os.rename(path, final)
        else:
            shutil.rmtree(path)
            if not final.exists():
                interrupted.add(stage)
    return interrupted


def inventory_parameters(args: argparse.Namespace) -> dict[str, Any]:
    return {
        "model_ids": args.model_id,
        "sample_models": None if args.model_id else args.root_count,
        "sample_seed": None if args.model_id else args.root_seed,
    }


def inventory_complete(out_dir: Path, parameters: dict[str, Any]) -> bool:
    summary_path = out_dir / "summary.json"
    variants_path = out_dir / "endpoint_variants.jsonl"
    variants_csv = out_dir / "endpoint_variants.csv"
    manifest_path = out_dir / "manifest.json"
    summary_md = out_dir / "summary.md"
    endpoint_dir = out_dir / "raw" / "endpoints"
    snapshot_dir = out_dir / "inputs" / "endpoints"
    catalog = out_dir / "raw" / "models.json"
    catalog_snapshot = out_dir / "inputs" / "models.json"
    if (
        not summary_path.exists()
        or not variants_path.exists()
        or not variants_csv.is_file()
        or not manifest_path.is_file()
        or not summary_md.is_file()
        or not summary_md.read_bytes().strip()
        or not endpoint_dir.is_dir()
        or not snapshot_dir.is_dir()
        or not catalog.is_file()
        or not catalog_snapshot.is_file()
        or not files_equal(catalog, catalog_snapshot)
    ):
        return False
    summary = load_json(summary_path)
    manifest = load_json(manifest_path)
    if not manifest:
        return False
    variant_rows = load_jsonl(variants_path)
    from model_inventory import CSV_FIELDS, OpenRouterError, json_for_cell, normalized_row, validate_inventory_resume

    try:
        state = validate_inventory_resume(
            out_dir,
            run_id="inventory",
            parameters=parameters,
        )
    except (TypeError, ValueError, OpenRouterError):
        return False
    if len(state.completed) != len(state.selected_records):
        return False
    models_by_id = {str(model["id"]): model for model in state.models}
    selected_by_id = {str(model["id"]): model for model in state.selected}
    derived_rows: list[dict[str, Any]] = []
    for selection in state.selected_records:
        model_id = selection["model_id"]
        saved = state.completed[model_id]
        endpoint_payload = saved[1]
        endpoints = saved[2]
        model = selected_by_id[model_id]
        endpoint_model = endpoint_payload if endpoint_payload.get("id") else model
        model_for_rows = models_by_id.get(str(endpoint_model.get("id")), model)
        for endpoint_index, endpoint in enumerate(endpoints):
            derived_rows.append(
                normalized_row(
                    snapshot_id=str(manifest["run_id"]),
                    snapshot_timestamp=str(manifest["created_at"]),
                    model=model_for_rows,
                    model_raw_path="raw/models.json",
                    endpoint_raw_path=selection["raw_path"],
                    endpoint_index=endpoint_index,
                    endpoint=endpoint,
                )
            )

    def inventory_cell(value: Any) -> str:
        encoded = json_for_cell(value)
        return "" if encoded is None else str(encoded)

    expected_summary_fields = {
        "inventory_run_id",
        "started_at_utc",
        "completed_at_utc",
        "catalog_model_count",
        "selected_model_count",
        "sample_models",
        "sample_seed",
        "endpoint_variant_count",
        "endpoint_fetch_count",
        "reused_endpoint_fetch_count",
        "new_endpoint_fetch_count",
        "endpoint_fetch_error_count",
        "endpoint_fetch_errors",
        "model_endpoint_fetches",
        "selected_model_ids",
        "provider_counts",
        "quantization_counts",
        "status_counts",
        "unknown_quantization_endpoint_variant_count",
        "complete",
        "failure",
        "output_files",
        "notes",
    }
    if set(summary) != expected_summary_fields:
        return False
    selected = summary.get("selected_model_count")
    variants = summary.get("endpoint_variant_count")
    provider_counts = Counter(str(row.get("provider_name") or "unknown") for row in derived_rows)
    quantization_counts = Counter(str(row.get("quantization") or "unknown") for row in derived_rows)
    status_counts = Counter(
        str(row.get("status") if row.get("status") is not None else "missing")
        for row in derived_rows
    )
    fetches = summary.get("model_endpoint_fetches")
    if not isinstance(fetches, list) or len(fetches) != len(state.selected_records):
        return False
    for fetch, selection in zip(fetches, state.selected_records, strict=True):
        if not isinstance(fetch, dict) or set(fetch) != {
            "model_id",
            "endpoint_count",
            "endpoint_path",
            "raw_path",
            "reused",
        }:
            return False
        saved = state.completed[selection["model_id"]]
        if (
            fetch.get("model_id") != selection["model_id"]
            or fetch.get("endpoint_count") != len(saved[2])
            or fetch.get("endpoint_path") != selection["endpoint_path"]
            or fetch.get("raw_path") != selection["raw_path"]
            or not isinstance(fetch.get("reused"), bool)
        ):
            return False
    output_files = [
        "manifest.json",
        "inputs/models.json",
        "inputs/endpoints/*.json",
        "raw/models.json",
        "raw/endpoints/*.json",
        "endpoint_variants.jsonl",
        "endpoint_variants.csv",
        "summary.json",
        "summary.md",
    ]
    notes = [
        "This run used only OpenRouter catalog and endpoint APIs. It did not run inference probes.",
        "Rows are endpoint variants. Unknown quantization is endpoint-specific and rows are not collapsed by quantization label.",
    ]
    markdown = [
        "# OpenRouter model inventory summary",
        "",
        f"- Run id: `{manifest['run_id']}`",
        f"- Started: {manifest['created_at']}",
        f"- Catalog models: {len(state.models)}",
        f"- Selected models: {len(state.selected)}",
        f"- Endpoint variants: {len(derived_rows)}",
        "- Endpoint fetch errors: 0",
        f"- Unknown-quantization endpoint variants: {quantization_counts.get('unknown', 0)}",
        "",
        "## Selected models",
        "",
        *(f"- `{model.get('id')}`" for model in state.selected),
        "",
        "## Quantization counts",
        "",
        *(f"- `{key}`: {value}" for key, value in sorted(quantization_counts.items())),
        "",
        "## Provider counts",
        "",
        *(f"- `{key}`: {value}" for key, value in sorted(provider_counts.items())),
        "",
        "## Files",
        "",
        "- `endpoint_variants.jsonl`",
        "- `endpoint_variants.csv`",
        "- `manifest.json`",
        "- `summary.json`",
        "- `inputs/models.json`",
        "- `inputs/endpoints/*.json`",
        "- `raw/models.json`",
        "- `raw/endpoints/*.json`",
        "",
    ]
    return (
        summary.get("complete") is True
        and isinstance(selected, int)
        and selected > 0
        and summary.get("endpoint_fetch_count") == selected
        and summary.get("endpoint_fetch_error_count") == 0
        and summary.get("endpoint_fetch_errors") == []
        and summary.get("inventory_run_id") == manifest["run_id"]
        and summary.get("started_at_utc") == manifest["created_at"]
        and isinstance(summary.get("completed_at_utc"), str)
        and bool(summary["completed_at_utc"])
        and summary.get("catalog_model_count") == len(state.models)
        and summary.get("sample_models") == manifest["selection_parameters"]["sample_models"]
        and summary.get("sample_seed") == manifest["selection_parameters"]["sample_seed"]
        and summary.get("selected_model_ids") == [str(model.get("id")) for model in state.selected]
        and summary.get("provider_counts") == dict(sorted(provider_counts.items()))
        and summary.get("quantization_counts") == dict(sorted(quantization_counts.items()))
        and summary.get("status_counts") == dict(sorted(status_counts.items()))
        and summary.get("unknown_quantization_endpoint_variant_count") == quantization_counts.get("unknown", 0)
        and summary.get("reused_endpoint_fetch_count") == sum(1 for item in fetches if item["reused"])
        and summary.get("new_endpoint_fetch_count") == sum(1 for item in fetches if not item["reused"])
        and summary.get("reused_endpoint_fetch_count") + summary.get("new_endpoint_fetch_count") == selected
        and summary.get("failure") is None
        and summary.get("output_files") == output_files
        and summary.get("notes") == notes
        and summary_md.read_text(encoding="utf-8") == "\n".join(markdown)
        and len(list(endpoint_dir.glob("*.json"))) == selected
        and len(list(snapshot_dir.glob("*.json"))) == selected
        and isinstance(variants, int)
        and variants > 0
        and len(variant_rows) == variants
        and variant_rows == derived_rows
        and csv_matches_rows(variants_csv, CSV_FIELDS, variant_rows, inventory_cell)
        and {path.name for path in endpoint_dir.glob("*.json")} == {path.name for path in snapshot_dir.glob("*.json")}
        and all((snapshot_dir / path.name).read_bytes() == path.read_bytes() for path in endpoint_dir.glob("*.json"))
        and exact_completed_files(
            out_dir,
            {
            manifest_path.resolve(),
            catalog.resolve(),
            catalog_snapshot.resolve(),
            variants_path.resolve(),
            variants_csv.resolve(),
            summary_path.resolve(),
            summary_md.resolve(),
            *(path.resolve() for path in endpoint_dir.glob("*.json")),
            *(path.resolve() for path in snapshot_dir.glob("*.json")),
            },
        )
    )


def eval_complete(
    out_dir: Path,
    variants_path: Path,
    item_ids: list[str],
    trials: int,
    questions_path: Path,
    prompt_path: Path,
) -> bool:
    summary_path = out_dir / "summary.json"
    progress_path = out_dir / "progress.jsonl"
    csv_path = out_dir / "variant_summary.csv"
    if not summary_path.exists() or not progress_path.exists() or not csv_path.exists():
        return False
    variants = load_jsonl(variants_path)
    expected = len(variants)
    summary = load_json(summary_path)
    if summary.get("total_variants") != expected or summary.get("completed_variants") != expected:
        return False
    if summary.get("stopped") or summary.get("command_failed") or summary.get("score_failed"):
        return False
    progress = load_progress(progress_path, variants, out_dir, item_ids, trials, questions_path, prompt_path)
    if len(progress) != expected:
        return False
    validate_summary_csv(csv_path, progress)
    validate_batch_summary(summary_path, progress, expected)
    return len(list((out_dir / "specs").glob("*.json"))) == expected


def filter_complete(
    out_dir: Path,
    source_path: Path,
    eval_summary_path: Path,
    source_specs_dir: Path,
    provider_error_count: int,
    minimum_deliberation_score: float,
) -> bool:
    summary_path = out_dir / "summary.json"
    variants_path = out_dir / "endpoint_variants.jsonl"
    removed_path = out_dir / "removed_variants.jsonl"
    manifest_path = out_dir / "manifest.jsonl"
    variant_summary_path = out_dir / "variant_summary.jsonl"
    variants_csv_path = out_dir / "endpoint_variants.csv"
    if not all(
        path.exists()
        for path in (
            summary_path,
            variants_path,
            removed_path,
            manifest_path,
            variant_summary_path,
            variants_csv_path,
        )
    ):
        return False
    summary = load_json(summary_path)
    source_rows = load_jsonl(source_path)
    source_variant_count = len(source_rows)
    eval_rows = load_eval_summary(eval_summary_path)
    variants_rows = load_jsonl(variants_path)
    summary_rows = load_jsonl(variant_summary_path)
    manifest_rows = load_jsonl(manifest_path)
    removed_rows = load_jsonl(removed_path)
    survivors = summary.get("survivor_count")
    removed = summary.get("removed_count")
    if not isinstance(survivors, int) or survivors < 1 or not isinstance(removed, int):
        return False
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
    summary_by_index: dict[int, dict[str, Any]] = {}
    for position, row in enumerate(eval_rows, start=1):
        index = row_index(row, position)
        if index in summary_by_index:
            return False
        summary_by_index[index] = row
    expected_variants: list[dict[str, Any]] = []
    expected_summaries: list[dict[str, Any]] = []
    expected_manifest: list[dict[str, Any]] = []
    expected_removed: list[dict[str, Any]] = []
    expected_specs: dict[str, Path] = {}
    source_indexes: set[int] = set()
    for position, variant in enumerate(source_rows, start=1):
        index = row_index(variant, position)
        if index in source_indexes:
            return False
        source_indexes.add(index)
        eval_row = summary_by_index.get(index)
        if eval_row is None:
            return False
        base = {
            "combined_index": index,
            "endpoint_variant_id": variant.get("endpoint_variant_id"),
            "openrouter_model_id": variant.get("openrouter_model_id"),
            "provider_name": variant.get("provider_name"),
            "endpoint_tag": variant.get("endpoint_tag"),
            "quantization": variant.get("quantization"),
        }
        run_exit_code = int_field(eval_row, "run_exit_code")
        if run_exit_code != 0:
            expected_removed.append(
                {
                    **base,
                    "reason": "run_exit_code",
                    "run_exit_code": run_exit_code,
                    "variant_status": eval_row.get("variant_status"),
                    "timeout_kind": eval_row.get("timeout_kind"),
                }
            )
            continue
        found_provider_errors = int_field(eval_row, "provider_error_count")
        score = float_field(eval_row, "deliberation_score")
        if found_provider_errors != provider_error_count:
            expected_removed.append(
                {
                    **base,
                    "reason": "provider_error_count",
                    "provider_error_count": found_provider_errors,
                }
            )
            continue
        if score is None or score < minimum_deliberation_score:
            expected_removed.append(
                {
                    **base,
                    "reason": "deliberation_score",
                    "deliberation_score": score,
                }
            )
            continue
        survivor = dict(variant)
        survivor.update(
            {
                "combined_index": index,
                "filter_provider_error_count": found_provider_errors,
                "filter_deliberation_score": score,
            }
        )
        expected_variants.append(survivor)
        survivor_summary = dict(eval_row)
        survivor_summary.update(
            {
                "combined_index": index,
                "provider_error_count": found_provider_errors,
                "deliberation_score": score,
            }
        )
        expected_summaries.append(survivor_summary)
        expected_manifest.append(
            {
                **base,
                "run_dir": eval_row.get("variant_run_dir") or eval_row.get("run_dir"),
            }
        )
        matches = sorted(source_specs_dir.glob(f"{index:02d}-*.json"))
        if len(matches) != 1:
            return False
        expected_specs[matches[0].name] = matches[0]
    if set(summary_by_index) != source_indexes:
        return False
    found_specs_dir = out_dir / "specs"
    if not found_specs_dir.is_dir():
        return False
    found_specs = {path.name: path for path in found_specs_dir.iterdir() if path.is_file()}
    if set(found_specs) != set(expected_specs):
        return False
    if any(not files_equal(found_specs[name], source) for name, source in expected_specs.items()):
        return False
    return (
        survivors + removed == source_variant_count
        and survivors == len(expected_variants)
        and removed == len(expected_removed)
        and len(variants_rows) == survivors
        and len(summary_rows) == survivors
        and len(manifest_rows) == survivors
        and len(removed_rows) == removed
        and variants_rows == expected_variants
        and summary_rows == expected_summaries
        and manifest_rows == expected_manifest
        and removed_rows == expected_removed
        and csv_matches_rows(variants_csv_path, fields, variants_rows)
        and summary.get("source_variant_file") == display_path(source_path)
        and summary.get("source_eval_summary_file") == display_path(eval_summary_path)
        and summary.get("source_specs_dir") == display_path(source_specs_dir)
        and summary.get("filter_criteria")
        == {
            "provider_error_count": provider_error_count,
            "deliberation_score_minimum": minimum_deliberation_score,
        }
        and summary.get("total_variants") == source_variant_count
        and summary.get("survivor_combined_indexes") == [row["combined_index"] for row in expected_variants]
        and summary.get("removed_variant_indexes") == [row["combined_index"] for row in expected_removed]
        and summary.get("outputs")
        == [
            "endpoint_variants.jsonl",
            "endpoint_variants.csv",
            "variant_summary.jsonl",
            "manifest.jsonl",
            "removed_variants.jsonl",
            "specs/*.json",
            "summary.json",
        ]
        and exact_completed_files(
            out_dir,
            {
                summary_path,
                variants_path,
                removed_path,
                manifest_path,
                variant_summary_path,
                variants_csv_path,
                *found_specs.values(),
            },
        )
    )


def numeric_lists_close(first: list[Any], second: list[Any], tolerance: float = 1e-8) -> bool:
    return len(first) == len(second) and all(
        isinstance(left, (int, float))
        and not isinstance(left, bool)
        and isinstance(right, (int, float))
        and not isinstance(right, bool)
        and math.isclose(float(left), float(right), rel_tol=tolerance, abs_tol=tolerance)
        for left, right in zip(first, second, strict=True)
    )


def numeric_matrices_close(first: list[Any], second: list[Any], tolerance: float = 1e-8) -> bool:
    return len(first) == len(second) and all(
        isinstance(left, list)
        and isinstance(right, list)
        and numeric_lists_close(left, right, tolerance)
        for left, right in zip(first, second, strict=True)
    )


def pca_identity(row: dict[str, Any]) -> dict[str, Any]:
    keys = (
        "run_id",
        "gene_index",
        "gene",
        "gene_sha256",
        "persona_id",
        "persona_path",
        "variant_order",
        "combined_index",
        "endpoint_variant_id",
        "openrouter_model_id",
        "provider_name",
        "endpoint_tag",
        "quantization",
        "sample_index",
    )
    return {key: row.get(key) for key in keys if key in row}


def pca_complete(out_dir: Path, expected_dimensions: int, source_records: Path) -> bool:
    summary_path = out_dir / "summary.json"
    records_path = out_dir / "pca-records.jsonl"
    fit_path = out_dir / "pca-fit.json"
    if not all(path.exists() for path in (summary_path, records_path, fit_path)):
        return False
    summary = load_json(summary_path)
    fit = load_json(fit_path)
    source_rows = load_jsonl(source_records)
    included_rows = [row for row in source_rows if row.get("status") == "ok"]
    projected_rows = load_jsonl(records_path)
    included = summary.get("included_records")
    if (
        not isinstance(included, int)
        or included < 1
        or included != len(included_rows)
        or len(projected_rows) != included
        or summary.get("pca_dimensions") != expected_dimensions
        or fit.get("pca_dimensions") != expected_dimensions
        or fit.get("included_records") != included
        or summary.get("source_records") != len(source_rows)
    ):
        return False
    embeddings = [row.get("embedding") for row in included_rows]
    if not embeddings or not all(isinstance(vector, list) and vector for vector in embeddings):
        return False
    width = len(embeddings[0])
    if any(len(vector) != width for vector in embeddings) or fit.get("embedding_dimension") != width:
        return False
    if expected_dimensions > width or expected_dimensions > included:
        return False
    matrix = np.array(embeddings, dtype=np.float64)
    mean_array = matrix.mean(axis=0)
    centered = matrix - mean_array
    _, singular_values, vt = np.linalg.svd(centered, full_matrices=False)
    expected_components = vt[:expected_dimensions]
    expected_projected = centered @ expected_components.T
    denominator = max(included - 1, 1)
    explained_all = (singular_values ** 2) / denominator
    total_variance = float(explained_all.sum())
    expected_explained = explained_all[:expected_dimensions]
    expected_ratios = (
        expected_explained / total_variance
        if total_variance > 0
        else np.zeros_like(expected_explained)
    )
    mean = mean_array.tolist()
    fit_mean = fit.get("mean")
    components = fit.get("components")
    if not isinstance(fit_mean, list) or not numeric_lists_close(fit_mean, mean):
        return False
    if not isinstance(components, list) or not numeric_matrices_close(
        components,
        expected_components.tolist(),
        1e-10,
    ):
        return False
    for source, projected, expected_vector in zip(
        included_rows,
        projected_rows,
        expected_projected.tolist(),
        strict=True,
    ):
        if pca_identity(projected) != pca_identity(source):
            return False
        vector = projected.get("pca")
        if not isinstance(vector, list) or len(vector) != expected_dimensions:
            return False
        if not numeric_lists_close(vector, expected_vector, 1e-10):
            return False
    explained = expected_explained.tolist()
    ratios = expected_ratios.tolist()
    singular = singular_values[:expected_dimensions].tolist()
    return (
        isinstance(fit.get("explained_variance"), list)
        and numeric_lists_close(fit["explained_variance"], explained, 1e-7)
        and isinstance(fit.get("explained_variance_ratio"), list)
        and numeric_lists_close(fit["explained_variance_ratio"], ratios, 1e-7)
        and isinstance(fit.get("singular_values"), list)
        and numeric_lists_close(fit["singular_values"], singular, 1e-7)
        and summary.get("embedding_dimension") == width
        and fit.get("created_at") == summary.get("created_at")
        and isinstance(fit.get("created_at"), str)
        and bool(fit["created_at"])
        and fit.get("records_path") == display_path(source_records)
        and summary.get("records_path") == display_path(source_records)
        and summary.get("projected_records_path") == display_path(records_path)
        and fit.get("centered") is True
        and summary.get("explained_variance") == fit.get("explained_variance")
        and summary.get("explained_variance_ratio") == fit.get("explained_variance_ratio")
        and isinstance(summary.get("explained_variance_ratio_sum"), (int, float))
        and math.isclose(float(summary["explained_variance_ratio_sum"]), sum(ratios), rel_tol=1e-7, abs_tol=1e-7)
        and exact_completed_files(out_dir, {summary_path, records_path, fit_path})
    )


def clusters_complete(
    out_dir: Path,
    expected_genes: int,
    expected_rows_per_gene: int | None,
    expected_variants_per_gene: int | None,
    expected_samples_per_variant: int | None,
    min_k: int,
    max_k: int,
    pca_sources: list[Path],
) -> bool:
    summary_path = out_dir / "summary.json"
    records_path = out_dir / "clusters.jsonl"
    fit_path = out_dir / "cluster-fit.json"
    csv_path = out_dir / "clusters.csv"
    if not all(path.exists() for path in (summary_path, records_path, fit_path, csv_path)):
        return False
    summary = load_json(summary_path)
    fit = load_json(fit_path)
    rows_data = load_jsonl(records_path)
    rows = summary.get("rows_written")
    if not isinstance(rows, int) or rows < 1 or summary.get("gene_count") != expected_genes:
        return False
    if expected_rows_per_gene is not None and rows != expected_genes * expected_rows_per_gene:
        return False
    dimensions = summary.get("pca_dimensions")
    if not isinstance(dimensions, int) or dimensions < 1:
        return False
    fields = [
        "gene_index",
        "gene",
        "combined_index",
        "endpoint_variant_id",
        "openrouter_model_id",
        "provider_name",
        "endpoint_tag",
        "quantization",
        "persona_id",
        "sample_index",
        *[f"pc{index}" for index in range(1, dimensions + 1)],
        "cluster",
    ]
    csv_rows = []
    for row in rows_data:
        csv_row = dict(row)
        pca = row.get("pca")
        if not isinstance(pca, list) or len(pca) != dimensions:
            return False
        for index, value in enumerate(pca, start=1):
            csv_row[f"pc{index}"] = value
        csv_rows.append(csv_row)
    from run_gene_pca_clustering import cluster_gene, validate_gene_rows

    expected_source_rows: list[dict[str, Any]] = []
    source_by_gene: dict[int, list[dict[str, Any]]] = {}
    expected_fits: dict[str, dict[str, Any]] = {}
    expected_gene_summaries: list[dict[str, Any]] = []
    for source in pca_sources:
        source_rows = load_jsonl(source)
        if not source_rows:
            return False
        gene_values = {row.get("gene_index") for row in source_rows}
        if len(gene_values) != 1 or not isinstance(next(iter(gene_values)), int):
            return False
        gene_index = int(next(iter(gene_values)))
        if gene_index in source_by_gene:
            return False
        source_by_gene[gene_index] = source_rows
        validation = validate_gene_rows(
            source,
            source_rows,
            expected_rows_per_gene,
            expected_variants_per_gene,
            expected_samples_per_variant,
            dimensions,
        )
        labels, expected_fit = cluster_gene(
            np.array([row["pca"] for row in source_rows], dtype=np.float64),
            min_k,
            max_k,
        )
        cluster_counts = Counter(int(label) for label in labels)
        source_display = display_path(source)
        expected_fits[str(gene_index)] = {
            "gene_index": gene_index,
            "gene": validation["gene"],
            "source_pca_path": source_display,
            **expected_fit,
        }
        expected_gene_summaries.append(
            {
                **validation,
                "source_pca_path": source_display,
                "status": expected_fit["status"],
                "chosen_k": expected_fit["chosen_k"],
                "chosen_silhouette_score": expected_fit.get("chosen_silhouette_score"),
                "cluster_counts": {str(key): cluster_counts[key] for key in sorted(cluster_counts)},
                "candidate_scores": expected_fit["candidate_scores"],
            }
        )
        expected_source_rows.extend(
            [
                {
                    **pca_identity(row),
                    "source_pca_path": source_display,
                    "pca": [float(value) for value in row["pca"]],
                    "cluster": int(label),
                }
                for row, label in zip(source_rows, labels, strict=True)
            ]
        )
    if len(source_by_gene) != expected_genes or len(expected_source_rows) != len(rows_data):
        return False
    if rows_data != expected_source_rows:
        return False
    fits = fit.get("genes")
    if fits != expected_fits:
        return False
    return (
        len(rows_data) == rows
        and csv_matches_rows(csv_path, fields, csv_rows)
        and fit.get("pca_dimensions") == dimensions
        and fit.get("min_k") == min_k
        and fit.get("max_k") == max_k
        and summary.get("pca_dimensions") == dimensions
        and summary.get("min_k") == min_k
        and summary.get("max_k") == max_k
        and summary.get("expected_rows_per_gene") == expected_rows_per_gene
        and summary.get("expected_variants_per_gene") == expected_variants_per_gene
        and summary.get("expected_samples_per_variant") == expected_samples_per_variant
        and summary.get("genes") == sorted(expected_gene_summaries, key=lambda item: item["gene_index"])
        and fit.get("created_at") == summary.get("created_at")
        and summary.get("output_dir") == display_path(out_dir)
        and summary.get("clusters_jsonl") == display_path(records_path)
        and summary.get("clusters_csv") == display_path(csv_path)
        and summary.get("cluster_fit") == display_path(fit_path)
        and exact_completed_files(out_dir, {summary_path, records_path, fit_path, csv_path})
    )


def aggregate_complete(
    out_dir: Path,
    clusters_path: Path,
    fit_path: Path,
    variants_path: Path,
    expected_samples_per_gene: int | None,
    allow_missing_gene_samples: bool,
) -> bool:
    summary_path = out_dir / "summary.json"
    records_path = out_dir / "variant-persona-clusters.jsonl"
    json_path = out_dir / "variant-persona-clusters.json"
    if not summary_path.exists() or not records_path.exists() or not json_path.exists():
        return False
    summary = load_json(summary_path)
    if set(summary) != {
        "created_at",
        "clusters_path",
        "cluster_fit_path",
        "variants_path",
        "jsonl_path",
        "json_path",
        "variant_persona_rows",
        "gene_indexes",
        "clusters_per_row",
        "input_cluster_rows",
        "skipped_variant_count",
        "skipped_variants",
        "aggregation_method_counts",
        "expected_samples_per_gene",
        "allow_missing_gene_samples",
    }:
        return False
    expected = summary.get("variant_persona_rows")
    rows = load_jsonl(records_path)
    value = strict_json_loads(json_path.read_bytes(), json_path)
    if not isinstance(expected, int) or expected < 1 or len(rows) != expected or value != rows:
        return False
    from aggregate_variant_persona_clusters import choose_cluster

    cluster_rows = load_jsonl(clusters_path)
    variants_rows = load_jsonl(variants_path)
    variants: dict[str, dict[str, Any]] = {}
    for variant in variants_rows:
        endpoint = variant.get("endpoint_variant_id")
        if not isinstance(endpoint, str) or not endpoint or endpoint in variants:
            return False
        variants[endpoint] = variant
    fit_value = load_json(fit_path)
    fits = fit_value.get("genes")
    if not isinstance(fits, dict):
        return False
    grouped: dict[tuple[str, str], dict[int, list[dict[str, Any]]]] = defaultdict(lambda: defaultdict(list))
    for row in cluster_rows:
        endpoint = row.get("endpoint_variant_id")
        persona_id = row.get("persona_id")
        gene_index = row.get("gene_index")
        if not isinstance(endpoint, str) or not isinstance(persona_id, str) or not isinstance(gene_index, int):
            return False
        grouped[(endpoint, persona_id)][gene_index].append(row)
    expected_genes = sorted({int(row["gene_index"]) for row in cluster_rows})
    derived: list[dict[str, Any]] = []
    skipped: list[dict[str, Any]] = []
    methods: Counter[str] = Counter()
    for endpoint, persona_id in sorted(grouped):
        by_gene = grouped[(endpoint, persona_id)]
        present = sorted(by_gene)
        if present != expected_genes:
            missing = [index for index in expected_genes if index not in by_gene]
            if not allow_missing_gene_samples:
                return False
            skipped.append(
                {
                    "endpoint_variant_id": endpoint,
                    "persona_id": persona_id,
                    "missing_gene_indexes": missing,
                    "present_gene_indexes": present,
                    "reason": "missing_gene_samples",
                }
            )
            continue
        variant = variants.get(endpoint)
        if variant is None:
            return False
        exemplar = by_gene[expected_genes[0]][0]
        clusters: list[int] = []
        details: list[dict[str, Any]] = []
        for gene_index in expected_genes:
            gene_rows = sorted(by_gene[gene_index], key=lambda item: item["sample_index"])
            if expected_samples_per_gene is not None and len(gene_rows) != expected_samples_per_gene:
                return False
            gene_fit = fits.get(str(gene_index))
            if not isinstance(gene_fit, dict):
                return False
            chosen = choose_cluster(gene_rows, gene_fit)
            methods[chosen["method"]] += 1
            clusters.append(chosen["cluster"])
            details.append({"gene_index": gene_index, "gene": gene_rows[0]["gene"], **chosen})
        derived.append(
            {
                "endpoint_variant_id": endpoint,
                "combined_index": variant.get("combined_index", exemplar.get("combined_index")),
                "openrouter_model_id": variant.get("openrouter_model_id", exemplar.get("openrouter_model_id")),
                "provider_name": variant.get("provider_name", exemplar.get("provider_name")),
                "endpoint_tag": variant.get("endpoint_tag", exemplar.get("endpoint_tag")),
                "quantization": variant.get("quantization", exemplar.get("quantization")),
                "persona": {"id": persona_id, "path": exemplar.get("persona_path")},
                "clusters": clusters,
                "cluster_details": details,
                "variant": variant,
            }
        )
    return (
        rows == derived
        and summary.get("clusters_path") == display_path(clusters_path)
        and summary.get("cluster_fit_path") == display_path(fit_path)
        and summary.get("variants_path") == display_path(variants_path)
        and summary.get("jsonl_path") == display_path(records_path)
        and summary.get("json_path") == display_path(json_path)
        and isinstance(summary.get("created_at"), str)
        and bool(summary["created_at"])
        and summary.get("gene_indexes") == expected_genes
        and summary.get("clusters_per_row") == len(expected_genes)
        and summary.get("input_cluster_rows") == len(cluster_rows)
        and summary.get("skipped_variant_count") == len(skipped)
        and summary.get("skipped_variants") == skipped
        and summary.get("aggregation_method_counts") == {key: methods[key] for key in sorted(methods)}
        and summary.get("expected_samples_per_gene") == expected_samples_per_gene
        and summary.get("allow_missing_gene_samples") == allow_missing_gene_samples
        and exact_completed_files(out_dir, {summary_path, records_path, json_path})
    )


def load_pool_sampler() -> Any:
    path = ROOT / "tools" / "sample-tuple-pool.py"
    spec = importlib.util.spec_from_file_location("model_pool_sample_tuple_pool", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load pool sampler: {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def without_persona_reference(row: dict[str, Any]) -> dict[str, Any]:
    value = json.loads(json.dumps(row, ensure_ascii=False, allow_nan=False))
    persona = value.get("persona")
    if isinstance(persona, dict):
        persona.pop("path", None)
        persona.pop("file", None)
        persona.pop("persona_file", None)
    elif isinstance(persona, str):
        value.pop("persona", None)
    value.pop("persona_file", None)
    return value


def pool_complete(
    out_dir: Path,
    expected_rows: int,
    expected_persona: Path,
    source_path: Path,
    seed: int,
    no_dedupe_equivalent_endpoints: bool,
) -> bool:
    pool_path = out_dir / "pool.jsonl"
    diagnostics_path = out_dir / "diagnostics.jsonl"
    equivalence_path = out_dir / "equivalence.jsonl"
    if not all(path.exists() for path in (pool_path, diagnostics_path, equivalence_path)):
        return False
    pool_rows = load_jsonl(pool_path)
    diagnostics = load_jsonl(diagnostics_path)
    equivalence = load_jsonl(equivalence_path)
    if len(pool_rows) != expected_rows or len(diagnostics) != expected_rows:
        return False
    sampler = load_pool_sampler()
    source_rows = sampler.load_jsonl(source_path)
    sampler.validate_rows(source_rows)
    representatives, expected_equivalence = sampler.annotate_equivalence(source_rows)
    if equivalence != expected_equivalence:
        return False
    sampling_rows = source_rows if no_dedupe_equivalent_endpoints else representatives
    grouped = sampler.group_by_tuple(sampling_rows)
    tuples = sorted(grouped)
    if not tuples:
        return False
    rng = random.Random(seed)
    tuple_counts: Counter[tuple[int, ...]] = Counter()
    row_counts: Counter[int] = Counter()
    model_counts: Counter[str] = Counter()
    provider_counts: Counter[str] = Counter()
    endpoint_counts: Counter[str] = Counter()
    selected_rows: list[dict[str, Any]] = []
    expected_diagnostics: list[dict[str, Any]] = []
    console_lines: list[str] = []
    for step in range(1, expected_rows + 1):
        cluster_tuple = tuples[rng.randrange(len(tuples))]
        candidates = grouped[cluster_tuple]
        row = candidates[rng.randrange(len(candidates))]
        tuple_counts[cluster_tuple] += 1
        row_counts[row["_source_row"]] += 1
        endpoint = sampler.endpoint_key(row)
        model_id = str(row.get("openrouter_model_id", ""))
        provider_name = str(row.get("provider_name", ""))
        model_counts[model_id] += 1
        provider_counts[provider_name] += 1
        endpoint_counts[endpoint] += 1
        selected_rows.append(row)
        console_lines.append(
            f"{step}: tuple={list(cluster_tuple)} tuple_size={len(grouped[cluster_tuple])} "
            f"available_before={len(candidates)} tuple_count={tuple_counts[cluster_tuple]} "
            f"row={row['_source_row']} row_count={row_counts[row['_source_row']]} "
            f"model={model_id} provider={provider_name} endpoint={row.get('endpoint_tag', '')} "
            f"quantization={row.get('quantization', '')}"
        )
        expected_diagnostics.append(
            {
                "step": step,
                "cluster_tuple": list(cluster_tuple),
                "cluster_tuple_count": tuple_counts[cluster_tuple],
                "cluster_tuple_size": len(candidates),
                "cluster_tuple_available_before": len(candidates),
                "source_row": row["_source_row"],
                "source_row_count": row_counts[row["_source_row"]],
                "openrouter_model_id": model_id,
                "provider_name": provider_name,
                "endpoint_tag": row.get("endpoint_tag"),
                "quantization": row.get("quantization"),
                "endpoint_identifier": endpoint,
                "equivalence_key": row.get("_equivalence_key"),
                "equivalence_class_size": row.get("_equivalence_class_size", 1),
                "representative_source_row": row.get("_representative_source_row", row["_source_row"]),
                "representative_endpoint_variant_id": row.get(
                    "_representative_endpoint_variant_id",
                    row.get("endpoint_variant_id"),
                ),
                "equivalent_endpoints": row.get("_equivalent_endpoints", [sampler.endpoint_summary(row)]),
                "without_replacement": False,
            }
        )
    if diagnostics != expected_diagnostics:
        return False
    console_lines.append(
        f"summary: input_rows={len(source_rows)} deduped_rows={len(sampling_rows)} "
        f"equivalence_classes={len(expected_equivalence)} gene_count={len(tuples[0])} "
        f"unique_tuples={len(tuples)} without_replacement=False emitted={expected_rows} "
        f"emitted_unique_tuples={len(tuple_counts)} unique_rows={len(row_counts)} "
        f"unique_models={len(model_counts)} unique_providers={len(provider_counts)} "
        f"unique_endpoints={len(endpoint_counts)}"
    )
    sample_log = out_dir / "sample.log"
    if not sample_log.is_file() or sample_log.read_text(encoding="utf-8") != "\n".join(console_lines) + "\n":
        return False
    expected_pool_rows = [sampler.clean_row(row) for row in selected_rows]
    if any(
        without_persona_reference(found) != without_persona_reference(expected)
        for found, expected in zip(pool_rows, expected_pool_rows, strict=True)
    ):
        return False
    expected_persona_bytes = expected_persona.read_bytes()
    equivalence_by_key: dict[str, dict[str, Any]] = {}
    for record in equivalence:
        key = record.get("equivalence_key")
        members = record.get("equivalent_endpoints")
        size = record.get("equivalence_class_size")
        if not isinstance(key, dict) or not isinstance(members, list) or not members:
            return False
        if not isinstance(size, int) or isinstance(size, bool) or size != len(members):
            return False
        encoded = json.dumps(key, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        if encoded in equivalence_by_key:
            return False
        representative = record.get("representative_endpoint_variant_id")
        if representative not in {member.get("endpoint_variant_id") for member in members if isinstance(member, dict)}:
            return False
        equivalence_by_key[encoded] = record

    persona_files: set[Path] = set()
    for index, (row, diagnostic) in enumerate(zip(pool_rows, diagnostics, strict=True), start=1):
        persona = row.get("persona")
        persona_path = persona.get("path") if isinstance(persona, dict) else persona
        if not isinstance(persona_path, str) or not persona_path.strip():
            return False
        pure = PurePosixPath(persona_path)
        if pure.is_absolute() or ".." in pure.parts or len(pure.parts) < 2 or pure.parts[0] != "personas":
            return False
        path = (pool_path.parent / Path(*pure.parts)).resolve()
        persona_root = (pool_path.parent / "personas").resolve()
        if persona_root not in path.parents:
            return False
        if not path.is_file() or path.read_bytes() != expected_persona_bytes:
            return False
        persona_files.add(path)
        identity_fields = ("openrouter_model_id", "provider_name", "endpoint_tag", "quantization")
        if diagnostic.get("step") != index or diagnostic.get("cluster_tuple") != row.get("clusters"):
            return False
        if any(diagnostic.get(field) != row.get(field) for field in identity_fields):
            return False
        endpoint_identifier = row.get("endpoint_variant_id") or "|".join(str(row.get(field, "")) for field in identity_fields)
        if diagnostic.get("endpoint_identifier") != endpoint_identifier:
            return False
        key = row.get("equivalence_key")
        if not isinstance(key, dict):
            return False
        encoded = json.dumps(key, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        record = equivalence_by_key.get(encoded)
        if record is None:
            return False
        if row.get("equivalence_class_size") != record.get("equivalence_class_size"):
            return False
        if row.get("representative_endpoint_variant_id") != record.get("representative_endpoint_variant_id"):
            return False
        if row.get("equivalent_endpoints") != record.get("equivalent_endpoints"):
            return False
        if diagnostic.get("equivalence_key") != key:
            return False
        if diagnostic.get("representative_endpoint_variant_id") != record.get("representative_endpoint_variant_id"):
            return False
    return exact_completed_files(
        out_dir,
        {pool_path, diagnostics_path, equivalence_path, sample_log, *persona_files},
    )


def completed_gene_summary(summary_path: Path, args: argparse.Namespace, expected_records: int) -> bool:
    if not summary_path.exists():
        return False
    summary = load_json(summary_path)
    if summary.get("records_written") != expected_records:
        return False
    records_path = summary_path.parent / "records.jsonl"
    if not records_path.exists() or jsonl_object_count(records_path) != expected_records:
        return False
    if not summary.get("completion_error_count") and not summary.get("embedding_error_count"):
        return True
    return not args.strict_gene_completions and bool(summary.get("partial_records_allowed"))


def interrupted_stage(args: argparse.Namespace, stage: str) -> bool:
    return stage in getattr(args, "interrupted_stage_records", set())


def run_inventory(args: argparse.Namespace, run_dir: Path, commands_path: Path) -> Path:
    out_dir = run_dir / "inventory"
    parameters = inventory_parameters(args)
    recorded = args.resume and stage_record_complete(args, run_dir, "inventory", out_dir, [])
    if recorded:
        if not inventory_complete(out_dir, parameters):
            raise RuntimeError("recorded inventory stage fails completion validation")
        event("stage_skipped", stage="inventory", reason="resume", output=display_path(out_dir))
        return out_dir
    if args.resume and interrupted_stage(args, "inventory") and inventory_complete(out_dir, parameters):
        record_stage(args, run_dir, "inventory", out_dir, [])
        event("stage_skipped", stage="inventory", reason="recovered_stage_record", output=display_path(out_dir))
        return out_dir

    cmd = [
        "uv",
        "run",
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
    else:
        cmd.extend(["--sample-models", str(args.root_count)])
    if args.inventory_sleep:
        cmd.extend(["--sleep", str(args.inventory_sleep)])
    if args.resume and out_dir.exists() and any(out_dir.iterdir()):
        cmd.append("--resume")
    run_command(cmd, cwd=ROOT, commands_path=commands_path, stage="inventory", dry_run=args.dry_run, lock_fd=args.run_lock_fd)
    if not args.dry_run:
        if not inventory_complete(out_dir, parameters):
            raise RuntimeError("inventory command finished without complete validated output")
        record_stage(args, run_dir, "inventory", out_dir, [])
    return out_dir


def run_eval(args: argparse.Namespace, run_dir: Path, inventory_dir: Path, commands_path: Path) -> Path:
    out_dir = run_dir / "eval"
    variants_path = inventory_dir / "endpoint_variants.jsonl"
    questions_path = resolve_path(args.questions)
    prompt_path = resolve_path(args.prompt)
    item_ids = [str(row.get("id")) for row in load_jsonl(questions_path)]
    upstream = [variants_path, questions_path, prompt_path]
    recorded = args.resume and stage_record_complete(args, run_dir, "eval", out_dir, upstream)
    if recorded:
        if not eval_complete(out_dir, variants_path, item_ids, args.eval_trials, questions_path, prompt_path):
            raise RuntimeError("recorded eval stage fails completion validation")
        event("stage_skipped", stage="eval", reason="resume", output=display_path(out_dir))
        return out_dir
    if (
        args.resume
        and interrupted_stage(args, "eval")
        and eval_complete(out_dir, variants_path, item_ids, args.eval_trials, questions_path, prompt_path)
    ):
        record_stage(args, run_dir, "eval", out_dir, upstream)
        event("stage_skipped", stage="eval", reason="recovered_stage_record", output=display_path(out_dir))
        return out_dir

    cmd = [
        "uv",
        "run",
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
        "--no-progress-timeout",
        str(args.eval_no_progress_timeout),
    ]
    if args.eval_variant_timeout is not None:
        cmd.extend(["--variant-timeout", str(args.eval_variant_timeout)])
    if args.resume and out_dir.exists() and any(out_dir.iterdir()):
        cmd.append("--resume")
    run_command(cmd, cwd=ROOT, commands_path=commands_path, stage="eval", dry_run=args.dry_run, lock_fd=args.run_lock_fd)
    if not args.dry_run:
        if not eval_complete(out_dir, variants_path, item_ids, args.eval_trials, questions_path, prompt_path):
            raise RuntimeError("eval command finished without complete validated output")
        record_stage(args, run_dir, "eval", out_dir, upstream)
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


def copy_spec_for_index(specs_dir: Path, out_specs_dir: Path, index: int) -> None:
    matches = sorted(specs_dir.glob(f"{index:02d}-*.json"))
    if len(matches) != 1:
        raise RuntimeError(f"expected one spec for variant index {index}, found {len(matches)}")
    shutil.copy2(matches[0], out_specs_dir / matches[0].name)


def filter_variants(args: argparse.Namespace, run_dir: Path, inventory_dir: Path, eval_dir: Path) -> Path:
    out_dir = run_dir / "filtered"
    summary_path = out_dir / "summary.json"
    variant_path = inventory_dir / "endpoint_variants.jsonl"
    eval_summary_path = eval_dir / "variant_summary.csv"
    specs_dir = eval_dir / "specs"
    upstream = [variant_path, eval_summary_path, *sorted(specs_dir.glob("*.json"))]
    source_variant_count = jsonl_object_count(variant_path)
    recorded = args.resume and stage_record_complete(args, run_dir, "filter", out_dir, upstream)
    if recorded:
        if not filter_complete(
            out_dir,
            variant_path,
            eval_summary_path,
            specs_dir,
            args.filter_provider_error_count,
            args.filter_min_deliberation_score,
        ):
            raise RuntimeError("recorded filter stage fails completion validation")
        event("stage_skipped", stage="filter", reason="resume", output=display_path(out_dir))
        return out_dir
    if args.resume and interrupted_stage(args, "filter") and filter_complete(
        out_dir,
        variant_path,
        eval_summary_path,
        specs_dir,
        args.filter_provider_error_count,
        args.filter_min_deliberation_score,
    ):
        record_stage(args, run_dir, "filter", out_dir, upstream)
        event("stage_skipped", stage="filter", reason="recovered_stage_record", output=display_path(out_dir))
        return out_dir
    if args.dry_run:
        event("stage_skipped", stage="filter", reason="dry_run")
        return out_dir

    out_specs_dir = out_dir / "specs"
    out_specs_dir.mkdir(parents=True, exist_ok=True)

    variants = load_jsonl(variant_path)
    summaries = load_eval_summary(eval_summary_path)
    summary_by_index = {row_index(row, position): row for position, row in enumerate(summaries, start=1)}

    survivor_variants: list[dict[str, Any]] = []
    survivor_summaries: list[dict[str, Any]] = []
    survivor_manifest: list[dict[str, Any]] = []
    removed_variants: list[dict[str, Any]] = []
    for position, variant in enumerate(variants, start=1):
        index = row_index(variant, position)
        eval_row = summary_by_index.get(index)
        if eval_row is None:
            raise RuntimeError(f"missing eval summary for variant index {index}")
        run_exit_code = int_field(eval_row, "run_exit_code")
        if run_exit_code != 0:
            removed_variants.append(
                {
                    "combined_index": index,
                    "endpoint_variant_id": variant.get("endpoint_variant_id"),
                    "openrouter_model_id": variant.get("openrouter_model_id"),
                    "provider_name": variant.get("provider_name"),
                    "endpoint_tag": variant.get("endpoint_tag"),
                    "quantization": variant.get("quantization"),
                    "reason": "run_exit_code",
                    "run_exit_code": run_exit_code,
                    "variant_status": eval_row.get("variant_status"),
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
                    "endpoint_variant_id": variant.get("endpoint_variant_id"),
                    "openrouter_model_id": variant.get("openrouter_model_id"),
                    "provider_name": variant.get("provider_name"),
                    "endpoint_tag": variant.get("endpoint_tag"),
                    "quantization": variant.get("quantization"),
                    "reason": "provider_error_count",
                    "provider_error_count": provider_errors,
                }
            )
            continue
        if score is None or score < args.filter_min_deliberation_score:
            removed_variants.append(
                {
                    "combined_index": index,
                    "endpoint_variant_id": variant.get("endpoint_variant_id"),
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

        summary_row = dict(eval_row)
        summary_row["combined_index"] = index
        summary_row["provider_error_count"] = provider_errors
        summary_row["deliberation_score"] = score
        survivor_summaries.append(summary_row)
        survivor_manifest.append(
            {
                "combined_index": index,
                "endpoint_variant_id": survivor.get("endpoint_variant_id"),
                "openrouter_model_id": survivor.get("openrouter_model_id"),
                "provider_name": survivor.get("provider_name"),
                "endpoint_tag": survivor.get("endpoint_tag"),
                "quantization": survivor.get("quantization"),
                "run_dir": eval_row.get("variant_run_dir") or eval_row.get("run_dir"),
            }
        )
        copy_spec_for_index(specs_dir, out_specs_dir, index)

    if not survivor_variants:
        raise RuntimeError("filter produced zero survivor variants")

    write_jsonl(out_dir / "endpoint_variants.jsonl", survivor_variants)
    write_jsonl(out_dir / "variant_summary.jsonl", survivor_summaries)
    write_jsonl(out_dir / "manifest.jsonl", survivor_manifest)
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
        "source_specs_dir": display_path(specs_dir),
        "filter_criteria": {
            "provider_error_count": args.filter_provider_error_count,
            "deliberation_score_minimum": args.filter_min_deliberation_score,
        },
        "total_variants": len(variants),
        "survivor_count": len(survivor_variants),
        "survivor_combined_indexes": [row["combined_index"] for row in survivor_summaries],
        "removed_count": len(removed_variants),
        "removed_variant_indexes": [row["combined_index"] for row in removed_variants],
        "outputs": [
            "endpoint_variants.jsonl",
            "endpoint_variants.csv",
            "variant_summary.jsonl",
            "manifest.jsonl",
            "removed_variants.jsonl",
            "specs/*.json",
            "summary.json",
        ],
    }
    write_json(summary_path, summary)
    if not filter_complete(
        out_dir,
        variant_path,
        eval_summary_path,
        specs_dir,
        args.filter_provider_error_count,
        args.filter_min_deliberation_score,
    ):
        raise RuntimeError("filter stage finished without complete validated output")
    record_stage(args, run_dir, "filter", out_dir, upstream)
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


def run_genes(args: argparse.Namespace, run_dir: Path, filtered_dir: Path, commands_path: Path) -> dict[int, Path]:
    out: dict[int, Path] = {}
    survivor_count = line_count(filtered_dir / "endpoint_variants.jsonl")
    expected_records = survivor_count * args.samples_per_gene
    for gene_index in selected_gene_indexes(args):
        gene_dir = run_dir / "genes" / f"gene-{gene_index}"
        inference_dir = gene_dir / "inference"
        summary_path = inference_dir / "summary.json"
        records_path = inference_dir / "records.jsonl"
        out[gene_index] = inference_dir
        upstream = [
            filtered_dir / "endpoint_variants.jsonl",
            resolve_path(args.genes),
            resolve_path(args.persona),
        ]
        stage = f"gene-{gene_index}"
        recorded = args.resume and stage_record_complete(
            args,
            run_dir,
            stage,
            inference_dir,
            upstream,
            gene_index=gene_index,
        )
        if recorded:
            if not completed_gene_summary(summary_path, args, expected_records):
                raise RuntimeError(f"recorded gene {gene_index} stage fails completion validation")
            event("stage_skipped", stage="genes", gene_index=gene_index, reason="resume", output=display_path(inference_dir))
            continue
        if args.resume and interrupted_stage(args, stage) and completed_gene_summary(summary_path, args, expected_records):
            record_stage(args, run_dir, stage, inference_dir, upstream, gene_index=gene_index)
            event(
                "stage_skipped",
                stage="genes",
                gene_index=gene_index,
                reason="recovered_stage_record",
                output=display_path(inference_dir),
            )
            continue
        cmd = [
            "uv",
            "run",
            "--script",
            "tools/run_first_gene_inference_embeddings.py",
            "--variants",
            display_path(filtered_dir / "endpoint_variants.jsonl"),
            "--genes",
            args.genes,
            "--persona",
            args.persona,
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
        ]
        if args.resume and records_path.exists():
            cmd.append("--resume")
        run_command(
            cmd,
            cwd=ROOT,
            commands_path=commands_path,
            stage=f"genes:{gene_index}",
            dry_run=args.dry_run,
            lock_fd=args.run_lock_fd,
        )
        if not completed_gene_summary(summary_path, args, expected_records):
            raise RuntimeError(f"gene {gene_index} command finished without complete validated output")
        record_stage(args, run_dir, stage, inference_dir, upstream, gene_index=gene_index)
    return out


def validate_gene_summaries(args: argparse.Namespace, gene_dirs: dict[int, Path], survivor_count: int) -> int:
    expected = survivor_count * args.samples_per_gene
    embedding_counts: list[int] = []
    for gene_index, inference_dir in sorted(gene_dirs.items()):
        summary = load_json(inference_dir / "summary.json")
        completion_errors = int(summary.get("completion_error_count") or 0)
        embedding_errors = int(summary.get("embedding_error_count") or 0)
        if args.strict_gene_completions and (completion_errors or embedding_errors):
            raise RuntimeError(f"gene {gene_index}: completion or embedding errors present")
        if summary.get("records_written") != expected:
            raise RuntimeError(f"gene {gene_index}: expected {expected} records, found {summary.get('records_written')}")
        embedding_count = int(summary.get("embedding_count") or 0)
        if args.strict_gene_completions and embedding_count != expected:
            raise RuntimeError(f"gene {gene_index}: expected {expected} embeddings, found {summary.get('embedding_count')}")
        if embedding_count < 1:
            raise RuntimeError(f"gene {gene_index}: no usable embeddings")
        if completion_errors or embedding_errors:
            event(
                "gene_errors_allowed",
                gene_index=gene_index,
                completion_error_count=completion_errors,
                embedding_error_count=embedding_errors,
                embedding_count=embedding_count,
                expected_records=expected,
            )
        embedding_counts.append(embedding_count)
    if not embedding_counts:
        raise RuntimeError("no gene runs selected")
    min_embeddings = min(embedding_counts)
    if args.strict_pca_dimensions and args.pca_dimensions > min_embeddings:
        raise RuntimeError(f"--pca-dimensions {args.pca_dimensions} exceeds minimum embedding count {min_embeddings}")
    return min(args.pca_dimensions, min_embeddings)


def run_pca(args: argparse.Namespace, run_dir: Path, gene_dirs: dict[int, Path], pca_dimensions: int, commands_path: Path) -> dict[int, Path]:
    out: dict[int, Path] = {}
    for gene_index, inference_dir in sorted(gene_dirs.items()):
        pca_dir = run_dir / "genes" / f"gene-{gene_index}" / "pca"
        out[gene_index] = pca_dir
        upstream = [inference_dir / "records.jsonl"]
        stage = f"pca-{gene_index}"
        recorded = args.resume and stage_record_complete(
            args,
            run_dir,
            stage,
            pca_dir,
            upstream,
            gene_index=gene_index,
            pca_dimensions=pca_dimensions,
        )
        if recorded:
            if not pca_complete(pca_dir, pca_dimensions, inference_dir / "records.jsonl"):
                raise RuntimeError(f"recorded PCA {gene_index} stage fails completion validation")
            event("stage_skipped", stage="pca", gene_index=gene_index, reason="resume", output=display_path(pca_dir))
            continue
        if args.resume and interrupted_stage(args, stage) and pca_complete(
            pca_dir,
            pca_dimensions,
            inference_dir / "records.jsonl",
        ):
            record_stage(
                args,
                run_dir,
                stage,
                pca_dir,
                upstream,
                gene_index=gene_index,
                pca_dimensions=pca_dimensions,
            )
            event(
                "stage_skipped",
                stage="pca",
                gene_index=gene_index,
                reason="recovered_stage_record",
                output=display_path(pca_dir),
            )
            continue
        cmd = [
            "uv",
            "run",
            "--script",
            "tools/run_embedding_pca.py",
            "--records",
            display_path(inference_dir / "records.jsonl"),
            "--out",
            display_path(pca_dir),
            "--dimensions",
            str(pca_dimensions),
        ]
        run_command(
            cmd,
            cwd=ROOT,
            commands_path=commands_path,
            stage=f"pca:{gene_index}",
            dry_run=args.dry_run,
            lock_fd=args.run_lock_fd,
        )
        if not pca_complete(pca_dir, pca_dimensions, inference_dir / "records.jsonl"):
            raise RuntimeError(f"PCA {gene_index} command finished without complete validated output")
        record_stage(
            args,
            run_dir,
            stage,
            pca_dir,
            upstream,
            gene_index=gene_index,
            pca_dimensions=pca_dimensions,
        )
    return out


def run_clustering(
    args: argparse.Namespace,
    run_dir: Path,
    pca_dirs: dict[int, Path],
    survivor_count: int,
    pca_dimensions: int,
    commands_path: Path,
) -> Path:
    out_dir = run_dir / "clusters"
    expected_rows = survivor_count * args.samples_per_gene
    required_rows = expected_rows if args.strict_gene_completions else None
    required_variants = survivor_count if args.strict_gene_completions else None
    required_samples = args.samples_per_gene if args.strict_gene_completions else None
    upstream = [pca_dir / "pca-records.jsonl" for _, pca_dir in sorted(pca_dirs.items())]
    recorded = args.resume and stage_record_complete(
        args,
        run_dir,
        "clusters",
        out_dir,
        upstream,
        pca_dimensions=pca_dimensions,
    )
    if recorded:
        if not clusters_complete(
            out_dir,
            len(pca_dirs),
            required_rows,
            required_variants,
            required_samples,
            args.min_k,
            args.max_k,
            upstream,
        ):
            raise RuntimeError("recorded clusters stage fails completion validation")
        event("stage_skipped", stage="clusters", reason="resume", output=display_path(out_dir))
        return out_dir
    if args.resume and interrupted_stage(args, "clusters") and clusters_complete(
        out_dir,
        len(pca_dirs),
        required_rows,
        required_variants,
        required_samples,
        args.min_k,
        args.max_k,
        upstream,
    ):
        record_stage(args, run_dir, "clusters", out_dir, upstream, pca_dimensions=pca_dimensions)
        event("stage_skipped", stage="clusters", reason="recovered_stage_record", output=display_path(out_dir))
        return out_dir
    cmd = [
        "uv",
        "run",
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
    if args.strict_gene_completions:
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
    run_command(cmd, cwd=ROOT, commands_path=commands_path, stage="clusters", dry_run=args.dry_run, lock_fd=args.run_lock_fd)
    if not clusters_complete(
        out_dir,
        len(pca_dirs),
        required_rows,
        required_variants,
        required_samples,
        args.min_k,
        args.max_k,
        upstream,
    ):
        raise RuntimeError("clusters command finished without complete validated output")
    record_stage(args, run_dir, "clusters", out_dir, upstream, pca_dimensions=pca_dimensions)
    return out_dir


def run_aggregate(args: argparse.Namespace, run_dir: Path, filtered_dir: Path, clusters_dir: Path, commands_path: Path) -> Path:
    out_dir = run_dir / "variant-persona-clusters"
    upstream = [
        clusters_dir / "clusters.jsonl",
        clusters_dir / "cluster-fit.json",
        filtered_dir / "endpoint_variants.jsonl",
    ]
    expected_samples = args.samples_per_gene if args.strict_gene_completions else None
    allow_missing = not args.strict_gene_completions
    recorded = args.resume and stage_record_complete(args, run_dir, "aggregate", out_dir, upstream)
    if recorded:
        if not aggregate_complete(out_dir, *upstream, expected_samples, allow_missing):
            raise RuntimeError("recorded aggregate stage fails completion validation")
        event("stage_skipped", stage="aggregate", reason="resume", output=display_path(out_dir))
        return out_dir
    if args.resume and interrupted_stage(args, "aggregate") and aggregate_complete(
        out_dir,
        *upstream,
        expected_samples,
        allow_missing,
    ):
        record_stage(args, run_dir, "aggregate", out_dir, upstream)
        event("stage_skipped", stage="aggregate", reason="recovered_stage_record", output=display_path(out_dir))
        return out_dir
    cmd = [
        "uv",
        "run",
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
    if args.strict_gene_completions:
        cmd.extend(["--expected-samples-per-gene", str(args.samples_per_gene)])
    else:
        cmd.append("--allow-missing-gene-samples")
    run_command(cmd, cwd=ROOT, commands_path=commands_path, stage="aggregate", dry_run=args.dry_run, lock_fd=args.run_lock_fd)
    if not aggregate_complete(out_dir, *upstream, expected_samples, allow_missing):
        raise RuntimeError("aggregate command finished without complete validated output")
    record_stage(args, run_dir, "aggregate", out_dir, upstream)
    return out_dir


def run_pool(args: argparse.Namespace, run_dir: Path, aggregate_dir: Path, commands_path: Path) -> Path:
    out_dir = run_dir / "pool"
    pool_path = out_dir / "pool.jsonl"
    upstream = [aggregate_dir / "variant-persona-clusters.jsonl", resolve_path(args.persona)]
    recorded = args.resume and stage_record_complete(args, run_dir, "pool", out_dir, upstream)
    if recorded:
        if not pool_complete(
            out_dir,
            args.pool_size,
            resolve_path(args.persona),
            aggregate_dir / "variant-persona-clusters.jsonl",
            args.pool_seed,
            args.no_dedupe_equivalent_endpoints,
        ):
            raise RuntimeError("recorded pool stage fails completion validation")
        event("stage_skipped", stage="pool", reason="resume", output=display_path(out_dir))
        return out_dir
    if args.resume and interrupted_stage(args, "pool") and pool_complete(
        out_dir,
        args.pool_size,
        resolve_path(args.persona),
        aggregate_dir / "variant-persona-clusters.jsonl",
        args.pool_seed,
        args.no_dedupe_equivalent_endpoints,
    ):
        record_stage(args, run_dir, "pool", out_dir, upstream)
        event("stage_skipped", stage="pool", reason="recovered_stage_record", output=display_path(out_dir))
        return out_dir
    cmd = [
        "uv",
        "run",
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
    run_command(
        cmd,
        cwd=ROOT,
        commands_path=commands_path,
        stage="pool",
        dry_run=args.dry_run,
        lock_fd=args.run_lock_fd,
        stdout_path=out_dir / "sample.log",
    )
    if not pool_complete(
        out_dir,
        args.pool_size,
        resolve_path(args.persona),
        aggregate_dir / "variant-persona-clusters.jsonl",
        args.pool_seed,
        args.no_dedupe_equivalent_endpoints,
    ):
        raise RuntimeError("pool command finished without complete validated output")
    record_stage(args, run_dir, "pool", out_dir, upstream)
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
        "eval": run_dir / "eval" / "summary.json",
        "filtered": run_dir / "filtered" / "summary.json",
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


def run_parameters(args: argparse.Namespace) -> dict[str, Any]:
    return {
        "stages": STAGES,
        "root_count": None if args.model_id else args.root_count,
        "root_seed": None if args.model_id else args.root_seed,
        "model_id": args.model_id,
        "inventory_request_timeout": args.inventory_request_timeout,
        "inventory_retries": args.inventory_retries,
        "inventory_sleep": args.inventory_sleep,
        "eval_trials": args.eval_trials,
        "filter_provider_error_count": args.filter_provider_error_count,
        "filter_min_deliberation_score": args.filter_min_deliberation_score,
        "gene_count": None if args.gene_index else args.gene_count,
        "gene_index": args.gene_index,
        "samples_per_gene": args.samples_per_gene,
        "embedding_model": args.embedding_model,
        "temperature": args.temperature,
        "top_p": args.top_p,
        "max_tokens": args.max_tokens,
        "completion_attempts": args.completion_attempts,
        "retry_sleep": args.retry_sleep,
        "strict_gene_completions": args.strict_gene_completions,
        "pca_dimensions": args.pca_dimensions,
        "strict_pca_dimensions": args.strict_pca_dimensions,
        "min_k": args.min_k,
        "max_k": args.max_k,
        "pool_size": args.pool_size,
        "pool_seed": args.pool_seed,
        "no_dedupe_equivalent_endpoints": args.no_dedupe_equivalent_endpoints,
        "timeout": args.timeout,
        "eval_no_progress_timeout": args.eval_no_progress_timeout,
        "eval_variant_timeout": args.eval_variant_timeout,
        "dry_run": args.dry_run,
    }


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the adj endpoint-variant model-pool pipeline.")
    parser.add_argument("--run-id", default=None, help="Run id under --out-root. Default: e2e-<UTC timestamp>.")
    parser.add_argument("--out-root", default="results")
    parser.add_argument("--root-count", type=int, default=5)
    parser.add_argument("--root-seed", type=int, default=0)
    parser.add_argument("--model-id", action="append", help="Specific OpenRouter model id. May be repeated. Overrides --root-count sampling.")
    parser.add_argument("--inventory-request-timeout", type=int, default=60)
    parser.add_argument("--inventory-retries", type=int, default=2)
    parser.add_argument("--inventory-sleep", type=float, default=0.0)
    parser.add_argument("--questions", default="sets/core20/questions.jsonl")
    parser.add_argument("--prompt", default="prompts/juror-single.md", help="Prompt file passed through the endpoint-evaluation stage.")
    parser.add_argument("--eval-trials", type=int, default=1)
    parser.add_argument("--filter-provider-error-count", type=int, default=0)
    parser.add_argument("--filter-min-deliberation-score", type=float, default=0.90)
    parser.add_argument("--genes", default="sampled-genes.json")
    parser.add_argument("--gene-count", type=int, default=2)
    parser.add_argument("--gene-index", action="append", type=int, help="Specific gene index. May be repeated. Overrides --gene-count.")
    parser.add_argument("--persona", default="../common/etc/personas/generic.md")
    parser.add_argument("--samples-per-gene", type=int, default=1)
    parser.add_argument("--embedding-model", default="text-embedding-3-small")
    parser.add_argument("--temperature", type=float, default=0.7)
    parser.add_argument("--top-p", type=float, default=1.0)
    parser.add_argument("--max-tokens", type=int, default=512)
    parser.add_argument("--completion-attempts", type=int, default=3)
    parser.add_argument("--retry-sleep", type=float, default=2.0)
    parser.add_argument("--strict-gene-completions", action="store_true")
    parser.add_argument("--pca-dimensions", type=int, default=3)
    parser.add_argument("--strict-pca-dimensions", action="store_true")
    parser.add_argument("--min-k", type=int, default=2)
    parser.add_argument("--max-k", type=int, default=10)
    parser.add_argument("--pool-size", type=int, default=20)
    parser.add_argument("--pool-seed", type=int, default=0)
    parser.add_argument("--no-dedupe-equivalent-endpoints", action="store_true")
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument("--eval-no-progress-timeout", type=int)
    parser.add_argument("--eval-variant-timeout", type=int)
    parser.add_argument("--resume", action="store_true", help="Continue a recorded run after verifying its arguments, inputs, and stage outputs")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--stop-after", choices=STAGES)
    args = parser.parse_args(argv)

    if args.root_count < 1:
        raise SystemExit("--root-count must be positive")
    if args.model_id:
        args.model_id = sorted(set(args.model_id))
    if args.inventory_request_timeout < 1:
        raise SystemExit("--inventory-request-timeout must be positive")
    if args.inventory_retries < 0:
        raise SystemExit("--inventory-retries cannot be negative")
    if not math.isfinite(args.inventory_sleep) or args.inventory_sleep < 0:
        raise SystemExit("--inventory-sleep must be a finite non-negative number")
    if args.eval_trials < 1:
        raise SystemExit("--eval-trials must be positive")
    if not resolve_path(args.prompt).is_file():
        raise SystemExit(f"prompt file does not exist: {resolve_path(args.prompt)}")
    if args.filter_provider_error_count < 0:
        raise SystemExit("--filter-provider-error-count cannot be negative")
    if not math.isfinite(args.filter_min_deliberation_score) or not 0 <= args.filter_min_deliberation_score <= 1:
        raise SystemExit("--filter-min-deliberation-score must be between 0 and 1")
    if args.gene_count < 1:
        raise SystemExit("--gene-count must be positive")
    if args.gene_index and any(index < 0 for index in args.gene_index):
        raise SystemExit("--gene-index values cannot be negative")
    if args.gene_index:
        args.gene_index = sorted(set(args.gene_index))
    if args.samples_per_gene < 1:
        raise SystemExit("--samples-per-gene must be positive")
    if not math.isfinite(args.temperature):
        raise SystemExit("--temperature must be finite")
    if not math.isfinite(args.top_p) or not 0 < args.top_p <= 1:
        raise SystemExit("--top-p must be greater than 0 and no greater than 1")
    if args.max_tokens < 1:
        raise SystemExit("--max-tokens must be positive")
    if args.completion_attempts < 1:
        raise SystemExit("--completion-attempts must be positive")
    if not math.isfinite(args.retry_sleep) or args.retry_sleep < 0:
        raise SystemExit("--retry-sleep must be a finite non-negative number")
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


def gene_child_args(args: argparse.Namespace, gene_index: int) -> argparse.Namespace:
    return argparse.Namespace(
        out="",
        variants=display_path(Path(args._run_dir) / "filtered" / "endpoint_variants.jsonl"),
        genes=args.genes,
        persona=args.persona,
        samples=args.samples_per_gene,
        gene_index=gene_index,
        timeout=args.timeout,
        embedding_model=args.embedding_model,
        temperature=args.temperature,
        top_p=args.top_p,
        max_tokens=args.max_tokens,
        completion_attempts=args.completion_attempts,
        retry_sleep=args.retry_sleep,
        resume=True,
    )


def preflight_incomplete_children(args: argparse.Namespace, run_dir: Path) -> None:
    inventory_dir = run_dir / "inventory"
    if inventory_dir.is_dir() and any(inventory_dir.iterdir()) and not stage_record_dir(run_dir, "inventory").exists():
        from model_inventory import validate_inventory_resume

        validate_inventory_resume(
            inventory_dir,
            run_id="inventory",
            parameters=inventory_parameters(args),
        )

    eval_dir = run_dir / "eval"
    variants_path = inventory_dir / "endpoint_variants.jsonl"
    if eval_dir.is_dir() and any(eval_dir.iterdir()) and not stage_record_dir(run_dir, "eval").exists():
        preflight_batch_resume(
            variants_path=variants_path,
            out_dir=eval_dir,
            questions_path=resolve_path(args.questions),
            prompt_path=resolve_path(args.prompt),
            trials=args.eval_trials,
            timeout=args.timeout,
            no_progress_timeout=args.eval_no_progress_timeout,
            variant_timeout=args.eval_variant_timeout,
        )

    from run_first_gene_inference_embeddings import preflight_gene_resume, require_service_keys

    args._run_dir = str(run_dir)
    for gene_index in selected_gene_indexes(args):
        gene_dir = run_dir / "genes" / f"gene-{gene_index}" / "inference"
        stage = f"gene-{gene_index}"
        if gene_dir.is_dir() and any(gene_dir.iterdir()) and not stage_record_dir(run_dir, stage).exists():
            bases, records = preflight_gene_resume(gene_child_args(args, gene_index), gene_dir)
            require_service_keys(bases, records)


def completed_stage_record_paths(
    args: argparse.Namespace,
    run_dir: Path,
    pending: dict[str, tuple[Path, bool]],
) -> dict[str, Path]:
    root = run_dir / "stage-records"
    records: dict[str, Path] = {}
    if root.is_dir():
        for path in sorted(root.iterdir()):
            if pending_stage_name(path) is not None:
                continue
            if path.is_dir() and not path.is_symlink():
                records[path.name] = path
    for stage, (path, complete) in pending.items():
        if complete and provider_stage(stage):
            records[stage] = path
    selected = selected_gene_indexes(args)
    allowed = {
        "inventory",
        "eval",
        "filter",
        "clusters",
        "aggregate",
        "pool",
        *(f"gene-{index}" for index in selected),
        *(f"pca-{index}" for index in selected),
    }
    unexpected = sorted(set(records) - allowed)
    if unexpected:
        raise ValueError(f"unexpected stage record for the current run: {unexpected[0]}")
    return records


def saved_pca_dimensions(args: argparse.Namespace, run_dir: Path) -> int:
    counts: list[int] = []
    expected_records = line_count(run_dir / "filtered" / "endpoint_variants.jsonl") * args.samples_per_gene
    for gene_index in selected_gene_indexes(args):
        summary_path = run_dir / "genes" / f"gene-{gene_index}" / "inference" / "summary.json"
        if not completed_gene_summary(summary_path, args, expected_records):
            raise ValueError(f"gene {gene_index} output is incomplete before the PCA stage")
        summary = load_json(summary_path)
        count = summary.get("embedding_count")
        if not isinstance(count, int) or isinstance(count, bool) or count < 1:
            raise ValueError(f"gene {gene_index} summary has no usable embedding count")
        counts.append(count)
    if not counts:
        raise ValueError("no gene outputs exist before the PCA stage")
    minimum = min(counts)
    if args.strict_pca_dimensions and args.pca_dimensions > minimum:
        raise ValueError(
            f"--pca-dimensions {args.pca_dimensions} exceeds the recorded minimum embedding count {minimum}"
        )
    return min(args.pca_dimensions, minimum)


def require_stage_dependencies(records: dict[str, Path], selected_genes: list[int]) -> None:
    gene_stages = {f"gene-{index}" for index in selected_genes}
    pca_stages = {f"pca-{index}" for index in selected_genes}
    prerequisites: dict[str, set[str]] = {
        "eval": {"inventory"},
        "filter": {"inventory", "eval"},
        **{stage: {"inventory", "eval", "filter"} for stage in gene_stages},
        **{
            stage: {"inventory", "eval", "filter", *gene_stages}
            for stage in pca_stages
        },
        "clusters": {"inventory", "eval", "filter", *gene_stages, *pca_stages},
        "aggregate": {"inventory", "eval", "filter", *gene_stages, *pca_stages, "clusters"},
        "pool": {
            "inventory",
            "eval",
            "filter",
            *gene_stages,
            *pca_stages,
            "clusters",
            "aggregate",
        },
    }
    for stage in records:
        missing = sorted(prerequisites.get(stage, set()) - set(records))
        if missing:
            raise ValueError(f"stage record {stage} exists without prerequisite stage record {missing[0]}")


def validate_completed_stage_semantics(
    args: argparse.Namespace,
    run_dir: Path,
    stage: str,
    record_path: Path,
    pca_dimensions: int | None,
) -> None:
    inventory_dir = run_dir / "inventory"
    eval_dir = run_dir / "eval"
    filtered_dir = run_dir / "filtered"
    questions_path = resolve_path(args.questions)
    prompt_path = resolve_path(args.prompt)
    if stage == "inventory":
        stage_record_complete(args, run_dir, stage, inventory_dir, [], record_path=record_path)
        complete = inventory_complete(inventory_dir, inventory_parameters(args))
    elif stage == "eval":
        variants_path = inventory_dir / "endpoint_variants.jsonl"
        upstream = [variants_path, questions_path, prompt_path]
        stage_record_complete(args, run_dir, stage, eval_dir, upstream, record_path=record_path)
        item_ids = [str(row.get("id")) for row in load_jsonl(questions_path)]
        complete = eval_complete(
            eval_dir,
            variants_path,
            item_ids,
            args.eval_trials,
            questions_path,
            prompt_path,
        )
    elif stage == "filter":
        variant_path = inventory_dir / "endpoint_variants.jsonl"
        eval_summary_path = eval_dir / "variant_summary.csv"
        specs_dir = eval_dir / "specs"
        upstream = [variant_path, eval_summary_path, *sorted(specs_dir.glob("*.json"))]
        stage_record_complete(args, run_dir, stage, filtered_dir, upstream, record_path=record_path)
        complete = filter_complete(
            filtered_dir,
            variant_path,
            eval_summary_path,
            specs_dir,
            args.filter_provider_error_count,
            args.filter_min_deliberation_score,
        )
    elif stage.startswith("gene-"):
        gene_index = int(stage.removeprefix("gene-"))
        inference_dir = run_dir / "genes" / stage / "inference"
        upstream = [filtered_dir / "endpoint_variants.jsonl", resolve_path(args.genes), resolve_path(args.persona)]
        stage_record_complete(
            args,
            run_dir,
            stage,
            inference_dir,
            upstream,
            record_path=record_path,
            gene_index=gene_index,
        )
        from run_first_gene_inference_embeddings import preflight_gene_resume

        preflight_gene_resume(gene_child_args(args, gene_index), inference_dir, require_complete=True)
        expected = line_count(filtered_dir / "endpoint_variants.jsonl") * args.samples_per_gene
        complete = completed_gene_summary(inference_dir / "summary.json", args, expected)
    elif stage.startswith("pca-"):
        if pca_dimensions is None:
            raise ValueError(f"cannot validate {stage} without completed gene outputs")
        gene_index = int(stage.removeprefix("pca-"))
        inference_dir = run_dir / "genes" / f"gene-{gene_index}" / "inference"
        pca_dir = run_dir / "genes" / f"gene-{gene_index}" / "pca"
        upstream = [inference_dir / "records.jsonl"]
        stage_record_complete(
            args,
            run_dir,
            stage,
            pca_dir,
            upstream,
            record_path=record_path,
            gene_index=gene_index,
            pca_dimensions=pca_dimensions,
        )
        complete = pca_complete(pca_dir, pca_dimensions, upstream[0])
    elif stage == "clusters":
        if pca_dimensions is None:
            raise ValueError("cannot validate clusters without completed gene outputs")
        pca_sources = [
            run_dir / "genes" / f"gene-{index}" / "pca" / "pca-records.jsonl"
            for index in selected_gene_indexes(args)
        ]
        stage_record_complete(
            args,
            run_dir,
            stage,
            run_dir / "clusters",
            pca_sources,
            record_path=record_path,
            pca_dimensions=pca_dimensions,
        )
        survivor_count = line_count(filtered_dir / "endpoint_variants.jsonl")
        required_rows = survivor_count * args.samples_per_gene if args.strict_gene_completions else None
        complete = clusters_complete(
            run_dir / "clusters",
            len(pca_sources),
            required_rows,
            survivor_count if args.strict_gene_completions else None,
            args.samples_per_gene if args.strict_gene_completions else None,
            args.min_k,
            args.max_k,
            pca_sources,
        )
    elif stage == "aggregate":
        upstream = [
            run_dir / "clusters" / "clusters.jsonl",
            run_dir / "clusters" / "cluster-fit.json",
            filtered_dir / "endpoint_variants.jsonl",
        ]
        out_dir = run_dir / "variant-persona-clusters"
        stage_record_complete(args, run_dir, stage, out_dir, upstream, record_path=record_path)
        expected_samples = args.samples_per_gene if args.strict_gene_completions else None
        complete = aggregate_complete(out_dir, *upstream, expected_samples, not args.strict_gene_completions)
    elif stage == "pool":
        aggregate_path = run_dir / "variant-persona-clusters" / "variant-persona-clusters.jsonl"
        persona_path = resolve_path(args.persona)
        out_dir = run_dir / "pool"
        stage_record_complete(
            args,
            run_dir,
            stage,
            out_dir,
            [aggregate_path, persona_path],
            record_path=record_path,
        )
        complete = pool_complete(
            out_dir,
            args.pool_size,
            persona_path,
            aggregate_path,
            args.pool_seed,
            args.no_dedupe_equivalent_endpoints,
        )
    else:
        raise ValueError(f"unsupported stage record: {stage}")
    if not complete:
        raise ValueError(f"recorded stage {stage} fails semantic completion validation")


def preflight_completed_stages(
    args: argparse.Namespace,
    run_dir: Path,
    pending: dict[str, tuple[Path, bool]],
) -> dict[str, Path]:
    records = completed_stage_record_paths(args, run_dir, pending)
    selected = selected_gene_indexes(args)
    require_stage_dependencies(records, selected)
    needs_pca = any(stage.startswith("pca-") or stage in {"clusters", "aggregate", "pool"} for stage in records)
    pca_dimensions = saved_pca_dimensions(args, run_dir) if needs_pca else None
    order = [
        "inventory",
        "eval",
        "filter",
        *(f"gene-{index}" for index in selected),
        *(f"pca-{index}" for index in selected),
        "clusters",
        "aggregate",
        "pool",
    ]
    for stage in order:
        record = records.get(stage)
        if record is not None:
            validate_completed_stage_semantics(args, run_dir, stage, record, pca_dimensions)
    return records


def local_stage_output_paths(args: argparse.Namespace, run_dir: Path) -> dict[str, Path]:
    paths = {
        "filter": run_dir / "filtered",
        "clusters": run_dir / "clusters",
        "aggregate": run_dir / "variant-persona-clusters",
        "pool": run_dir / "pool",
    }
    paths.update(
        {
            f"pca-{index}": run_dir / "genes" / f"gene-{index}" / "pca"
            for index in selected_gene_indexes(args)
        }
    )
    return paths


def quarantine_interrupted_local_stages(
    args: argparse.Namespace,
    run_dir: Path,
    completed_records: dict[str, Path],
) -> None:
    destinations_root = run_dir / "interrupted-local-stages"
    for stage, path in local_stage_output_paths(args, run_dir).items():
        if stage in completed_records or not path.exists():
            continue
        if not path.is_dir() or path.is_symlink():
            raise ValueError(f"interrupted local stage output is not a real directory: {path}")
        if not any(path.iterdir()):
            path.rmdir()
            continue
        destinations_root.mkdir(parents=True, exist_ok=True)
        destination = destinations_root / f"{stage}-{timestamp()}"
        suffix = 2
        while destination.exists():
            destination = destinations_root / f"{stage}-{timestamp()}-{suffix}"
            suffix += 1
        os.rename(path, destination)
        event("stage_quarantined", stage=stage, output=display_path(destination))


def execute_end_to_end(
    args: argparse.Namespace,
    run_id: str,
    run_dir: Path,
    commands_path: Path,
    summary_path: Path,
) -> int:
    pca_dimensions: int | None = None
    try:
        inventory_dir = run_inventory(args, run_dir, commands_path)
        if should_stop(args, "inventory") or args.dry_run:
            write_json(summary_path, collect_summary(run_dir))
            return 0

        eval_dir = run_eval(args, run_dir, inventory_dir, commands_path)
        if should_stop(args, "eval"):
            write_json(summary_path, collect_summary(run_dir))
            return 0

        filtered_dir = filter_variants(args, run_dir, inventory_dir, eval_dir)
        if should_stop(args, "filter"):
            write_json(summary_path, collect_summary(run_dir))
            return 0

        survivor_count = int(load_json(filtered_dir / "summary.json")["survivor_count"])
        gene_dirs = run_genes(args, run_dir, filtered_dir, commands_path)
        if should_stop(args, "genes"):
            write_json(summary_path, collect_summary(run_dir))
            return 0

        pca_dimensions = validate_gene_summaries(args, gene_dirs, survivor_count)
        if pca_dimensions != args.pca_dimensions:
            event("pca_dimensions_capped", requested=args.pca_dimensions, selected=pca_dimensions)
        pca_dirs = run_pca(args, run_dir, gene_dirs, pca_dimensions, commands_path)
        if should_stop(args, "pca"):
            write_json(summary_path, collect_summary(run_dir, pca_dimensions))
            return 0

        clusters_dir = run_clustering(args, run_dir, pca_dirs, survivor_count, pca_dimensions, commands_path)
        if should_stop(args, "clusters"):
            write_json(summary_path, collect_summary(run_dir, pca_dimensions))
            return 0

        aggregate_dir = run_aggregate(args, run_dir, filtered_dir, clusters_dir, commands_path)
        if should_stop(args, "aggregate"):
            write_json(summary_path, collect_summary(run_dir, pca_dimensions))
            return 0

        run_pool(args, run_dir, aggregate_dir, commands_path)
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


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    run_id = args.run_id or f"e2e-{timestamp()}"
    run_dir = (resolve_path(args.out_root) / run_id).expanduser().resolve()
    source_questions = resolve_path(args.questions)
    source_prompt = resolve_path(args.prompt)
    source_genes = resolve_path(args.genes)
    source_persona = resolve_path(args.persona)
    try:
        question_inputs, runtime_questions, _ = questions_snapshot(source_questions, ROOT, run_dir)
        prompt_input = input_file("prompt", source_prompt, "inputs/prompt.md")
        genes_input = input_file("genes", source_genes, "inputs/genes.json")
        persona_suffix = source_persona.suffix if source_persona.suffix else ".txt"
        persona_input = input_file("persona", source_persona, f"inputs/persona{persona_suffix}")
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc
    if args.resume:
        if not run_dir.is_dir():
            raise SystemExit(f"resume directory does not exist: {run_dir}")
    else:
        run_dir.parent.mkdir(parents=True, exist_ok=True)
        try:
            run_dir.mkdir(exist_ok=True)
        except OSError as exc:
            raise SystemExit(f"cannot create run directory {run_dir}: {exc}") from exc
    try:
        with exclusive_run_lock(run_dir, "model-pool-end-to-end", [str(Path(__file__)), *argv]) as run_lock:
            root_inputs = [*question_inputs, prompt_input, genes_input, persona_input]
            parameters = run_parameters(args)
            if args.resume:
                validate_run_record_inputs(
                    run_dir,
                    kind="model-pool-end-to-end",
                    parameters=parameters,
                    inputs=root_inputs,
                    generated_inputs=[runtime_questions],
                )
            args.questions = display_path(run_dir / runtime_questions.snapshot_path)
            args.prompt = display_path(run_dir / prompt_input.snapshot_path)
            args.genes = display_path(run_dir / genes_input.snapshot_path)
            args.persona = display_path(run_dir / persona_input.snapshot_path)
            if args.resume:
                validate_command_log(run_dir / "commands.jsonl")
            pending_stage_records = preflight_stage_records(run_dir) if args.resume else {}
            if args.resume:
                completed_records = preflight_completed_stages(args, run_dir, pending_stage_records)
                preflight_incomplete_children(args, run_dir)
            else:
                completed_records = {}
            prepare_run_record(
                run_dir,
                kind="model-pool-end-to-end",
                parameters=parameters,
                inputs=root_inputs,
                generated_inputs=[runtime_questions],
                resume=args.resume,
            )
            args.interrupted_stage_records = finalize_pending_stage_records(run_dir, pending_stage_records)
            if args.resume:
                quarantine_interrupted_local_stages(args, run_dir, completed_records)
                cleanup_root_atomic_temporaries(run_dir)
            args.run_lock_fd = run_lock.file_descriptor
            run_lock.record_owner()
            event("run_started", run_id=run_id, run_dir=display_path(run_dir), dry_run=args.dry_run)
            return execute_end_to_end(
                args,
                run_id,
                run_dir,
                run_dir / "commands.jsonl",
                run_dir / "summary.json",
            )
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
