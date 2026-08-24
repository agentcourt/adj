#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import csv
import datetime as dt
import io
import json
import os
import re
import signal
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from run_record import (
    PENDING_FILE_PREFIX,
    atomic_write_bytes,
    exclusive_run_lock,
    input_file,
    parse_jsonl_objects,
    prepare_run_record,
    questions_snapshot,
    strict_json_loads,
    validate_run_record_inputs,
)
from run_eval import model_spec_from_object
from score_eval import SCORES_COMPLETE, load_items as load_score_items, score_one, summarize_model

ROOT = Path(__file__).resolve().parents[1]
TIMEOUT_EXIT_CODE = 124
STOP_EXIT_CODE = 130
VARIANT_ARTIFACT_NAMES = {
    "raw_results.jsonl",
    "run.json",
    "run_eval.log",
    "scores.json",
    "scores.jsonl",
    SCORES_COMPLETE,
}


@dataclass(frozen=True)
class CommandResult:
    exit_code: int
    status: str
    timeout_kind: str | None
    elapsed_seconds: float


@dataclass(frozen=True)
class RecoveredVariant:
    state: str
    run: dict[str, Any]


def safe_part(value: object) -> str:
    text = str(value or "unknown").lower()
    text = re.sub(r"[^a-z0-9._-]+", "-", text)
    text = re.sub(r"-+", "-", text).strip("-")
    return text[:80] or "unknown"


def display_path(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def variant_fields(spec: dict[str, Any]) -> dict[str, Any]:
    return {
        "endpoint_variant_id": spec.get("endpoint_variant_id"),
        "openrouter_model_id": spec.get("openrouter_model_id") or "unknown-model",
        "provider_name": spec.get("provider_name") or "unknown-provider",
        "endpoint_tag": spec.get("endpoint_tag") or "unknown-endpoint",
        "quantization": spec.get("quantization") or "unknown",
    }


def variant_output_paths(out_dir: Path, index: int, spec: dict[str, Any]) -> tuple[Path, Path, Path]:
    identity = variant_fields(spec)
    stem = (
        f"{index:02d}-{safe_part(identity['openrouter_model_id'])}-"
        f"{safe_part(identity['provider_name'])}-{safe_part(identity['endpoint_tag'])}"
    )
    spec_path = out_dir / "specs" / f"{stem}.json"
    variant_dir = out_dir / "variant-runs" / stem
    return spec_path, variant_dir, variant_dir / "run_eval.log"


def validate_variants(variants: list[dict[str, Any]], source: Path) -> None:
    seen: dict[tuple[Any, ...], int] = {}
    for index, spec in enumerate(variants, start=1):
        identity = variant_fields(spec)
        key = tuple(identity.values())
        prior = seen.get(key)
        if prior is not None:
            raise ValueError(f"{source}: variant rows {prior} and {index} have the same identity")
        seen[key] = index


def expected_model_label(spec: dict[str, Any]) -> str:
    for key in ("model_spec_id", "endpoint_variant_id"):
        value = spec.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    model = spec.get("model") or spec.get("openrouter_model_id")
    if not isinstance(model, str) or not model.strip():
        raise ValueError("variant model or openrouter_model_id must be a nonempty string")
    model_id = model.strip().removeprefix("openrouter://")
    provider = spec.get("endpoint_tag") or spec.get("provider_name")
    suffix = f"@{provider.strip()}" if isinstance(provider, str) and provider.strip() else ""
    return f"openrouter://{model_id}{suffix}"


def expected_result_keys(model: str, item_ids: list[str], trials: int) -> set[tuple[str, str, int]]:
    return {(model, item_id, trial) for trial in range(1, trials + 1) for item_id in item_ids}


def validate_raw_results(
    path: Path,
    spec: dict[str, Any],
    item_ids: list[str],
    trials: int,
    *,
    require_complete: bool,
    allow_trailing_partial: bool = False,
) -> list[dict[str, Any]]:
    if not path.is_file():
        if require_complete:
            raise ValueError(f"raw results file does not exist: {path}")
        return []
    data = path.read_bytes()
    if not data.strip() and not require_complete:
        return []
    rows: list[dict[str, Any]]
    if allow_trailing_partial and data and not data.endswith((b"\n", b"\r")):
        last_break = data.rfind(b"\n")
        complete_data = data[: last_break + 1]
        trailing = data[last_break + 1 :]
        try:
            trailing_row = strict_json_loads(trailing, f"{path}:trailing")
        except ValueError:
            rows = parse_jsonl_objects(complete_data, path) if complete_data.strip() else []
        else:
            if not isinstance(trailing_row, dict):
                raise ValueError(f"{path}:trailing: expected a JSON object")
            rows = parse_jsonl_objects(data, path)
    else:
        rows = parse_jsonl_objects(data, path)
    normalized_spec = model_spec_from_object(spec, "recorded variant")
    model = normalized_spec["label"]
    expected = expected_result_keys(model, item_ids, trials)
    found: set[tuple[str, str, int]] = set()
    expected_metadata = {
        "model_spec_label": normalized_spec["label"],
        "openrouter_model_id": normalized_spec["openrouter_model_id"],
        "exact_variant": normalized_spec["exact_variant"],
        "requested_provider_constraints": normalized_spec["provider"],
        "requested_quantization_constraints": (normalized_spec["provider"] or {}).get("quantizations"),
        "allow_fallbacks": (normalized_spec["provider"] or {}).get("allow_fallbacks"),
        "require_parameters": (normalized_spec["provider"] or {}).get("require_parameters"),
        "request_parameters": normalized_spec["request"],
        "variant_metadata": normalized_spec["variant_metadata"],
    }
    for line_number, row in enumerate(rows, start=1):
        item_id = row.get("item_id")
        trial = row.get("trial_index")
        key = (row.get("model"), item_id, trial)
        if key not in expected:
            raise ValueError(f"{path}:{line_number}: raw result identity {key!r} is not expected")
        if key in found:
            raise ValueError(f"{path}:{line_number}: duplicate raw result identity {key!r}")
        found.add(key)
        metadata = row.get("metadata")
        if not isinstance(metadata, dict):
            raise ValueError(f"{path}:{line_number}: raw result metadata must be an object")
        for field, value in expected_metadata.items():
            if metadata.get(field) != value:
                raise ValueError(f"{path}:{line_number}: raw result metadata {field} differs")
        if metadata.get("trial_index") != trial:
            raise ValueError(f"{path}:{line_number}: raw result metadata trial_index differs")
    if require_complete and found != expected:
        missing = sorted(expected - found)
        raise ValueError(f"{path}: raw result row set is incomplete; missing {missing[:5]!r}")
    return rows


def validate_run_json(
    path: Path,
    raw_rows: list[dict[str, Any]],
    spec: dict[str, Any],
    item_ids: list[str],
    trials: int,
    questions_path: Path,
    prompt_path: Path,
    spec_path: Path,
) -> dict[str, Any]:
    try:
        run = strict_json_loads(path.read_bytes(), path)
    except FileNotFoundError as exc:
        raise ValueError(f"run file does not exist: {path}") from exc
    if not isinstance(run, dict):
        raise ValueError(f"{path}: expected a JSON object")
    label = expected_model_label(spec)
    expected_fields = {
        "run_id": path.parent.name,
        "models": [label],
        "trials": trials,
        "items": item_ids,
        "questions": display_path(questions_path),
        "prompt": display_path(prompt_path),
        "results": raw_rows,
    }
    if set(run) != set(expected_fields) | {"created_at", "model_specs"}:
        raise ValueError(f"{path}: run fields differ from the expected schema")
    if not isinstance(run.get("created_at"), str) or not run["created_at"]:
        raise ValueError(f"{path}: created_at must be a nonempty string")
    for key, expected in expected_fields.items():
        if run.get(key) != expected:
            raise ValueError(f"{path}: {key} differs from the recorded batch inputs or raw results")
    expected_spec = model_spec_from_object(spec, display_path(spec_path))
    if run.get("model_specs") != [expected_spec]:
        raise ValueError(f"{path}: model_specs differ from the normalized recorded variant spec")
    return run


def recovered_run_json(
    variant_dir: Path,
    raw_rows: list[dict[str, Any]],
    spec: dict[str, Any],
    item_ids: list[str],
    trials: int,
    questions_path: Path,
    prompt_path: Path,
    spec_path: Path,
) -> dict[str, Any]:
    created_at = next(
        (
            row["metadata"]["created_at"]
            for row in raw_rows
            if isinstance(row.get("metadata"), dict)
            and isinstance(row["metadata"].get("created_at"), str)
            and row["metadata"]["created_at"]
        ),
        dt.datetime.now(dt.UTC).isoformat(),
    )
    return {
        "run_id": variant_dir.name,
        "created_at": created_at,
        "models": [expected_model_label(spec)],
        "model_specs": [model_spec_from_object(spec, display_path(spec_path))],
        "trials": trials,
        "questions": display_path(questions_path),
        "prompt": display_path(prompt_path),
        "items": item_ids,
        "results": raw_rows,
    }


def expected_score_payload(run: dict[str, Any], questions_path: Path) -> dict[str, Any]:
    items = load_score_items(questions_path)
    try:
        scores = [score_one(items[row["item_id"]], row) for row in run["results"]]
    except (KeyError, TypeError, ValueError) as exc:
        raise ValueError(f"cannot recompute scores from {questions_path}: {exc}") from exc
    by_model: dict[str, list[dict[str, Any]]] = {}
    for score in scores:
        by_model.setdefault(score["model"], []).append(score)
    summary = {model: summarize_model(rows) for model, rows in by_model.items()}
    return {"run_id": run.get("run_id"), "scores": scores, "summary": summary}


def validate_score_outputs(
    run_dir: Path,
    run: dict[str, Any],
    questions_path: Path,
    *,
    require_complete: bool,
) -> bool:
    scores_path = run_dir / "scores.json"
    scores_jsonl_path = run_dir / "scores.jsonl"
    expected = expected_score_payload(run, questions_path)
    if scores_path.exists():
        scores = strict_json_loads(scores_path.read_bytes(), scores_path)
        if not isinstance(scores, dict):
            raise ValueError(f"{scores_path}: expected a JSON object")
        if scores != expected:
            raise ValueError(f"{scores_path}: scores differ from run.json and the recorded questions")
    if scores_jsonl_path.exists():
        score_rows = parse_jsonl_objects(scores_jsonl_path.read_bytes(), scores_jsonl_path)
        if score_rows != expected["scores"]:
            raise ValueError(f"{scores_jsonl_path}: score rows differ from scores.json")
    marker_path = run_dir / SCORES_COMPLETE
    if marker_path.exists():
        marker = strict_json_loads(marker_path.read_bytes(), marker_path)
        expected_marker = {"run_id": run.get("run_id"), "files": ["scores.json", "scores.jsonl"]}
        if marker != expected_marker:
            raise ValueError(f"{marker_path}: score completion marker differs from run.json")
        if not scores_path.is_file() or not scores_jsonl_path.is_file():
            raise ValueError(f"{marker_path}: marked score files are missing")
        return True
    if require_complete:
        raise ValueError(f"score completion marker does not exist: {marker_path}")
    return False


def validate_scores(run_dir: Path, run: dict[str, Any], questions_path: Path) -> None:
    validate_score_outputs(run_dir, run, questions_path, require_complete=True)


def load_progress(
    path: Path,
    variants: list[dict[str, Any]],
    out_dir: Path,
    item_ids: list[str],
    trials: int,
    questions_path: Path,
    prompt_path: Path,
) -> dict[int, dict[str, Any]]:
    if not path.exists():
        return {}
    rows: dict[int, dict[str, Any]] = {}
    for line_number, line in enumerate(path.read_text().splitlines(), start=1):
        if not line.strip():
            continue
        row = strict_json_loads(line, f"{path}:{line_number}")
        if not isinstance(row, dict):
            raise ValueError(f"{path}:{line_number}: expected a JSON object")
        index = row.get("index")
        if not isinstance(index, int) or isinstance(index, bool) or not 1 <= index <= len(variants):
            raise ValueError(f"{path}:{line_number}: index must be an integer from 1 through {len(variants)}")
        if index in rows:
            raise ValueError(f"{path}:{line_number}: duplicate variant index {index}")
        expected_identity = variant_fields(variants[index - 1])
        for key, expected in expected_identity.items():
            if row.get(key) != expected:
                raise ValueError(
                    f"{path}:{line_number}: variant index {index} has {key}={row.get(key)!r}; expected {expected!r}"
                )
        expected_spec, expected_variant_dir, expected_log = variant_output_paths(out_dir, index, variants[index - 1])
        expected_paths = {
            "variant_run_dir": display_path(expected_variant_dir),
            "run_log": display_path(expected_log),
        }
        for key, expected in expected_paths.items():
            if row.get(key) != expected:
                raise ValueError(f"{path}:{line_number}: {key}={row.get(key)!r}; expected {expected!r}")
        code = row.get("run_exit_code")
        if not isinstance(code, int) or isinstance(code, bool):
            raise ValueError(f"{path}:{line_number}: run_exit_code must be an integer")
        status = row.get("variant_status")
        if status not in {"scored", "timed_out", "command_failed", "score_failed"}:
            raise ValueError(f"{path}:{line_number}: invalid variant_status {status!r}")
        if status != "timed_out" and row.get("timeout_kind") is not None:
            raise ValueError(f"{path}:{line_number}: {status} variant must not have timeout_kind")
        expected_spec_bytes = (json.dumps(variants[index - 1], allow_nan=False, indent=2, sort_keys=True) + "\n").encode()
        if not expected_spec.is_file() or expected_spec.read_bytes() != expected_spec_bytes:
            raise ValueError(f"{path}:{line_number}: variant spec is missing or differs from the recorded input: {expected_spec}")
        if not expected_log.is_file():
            raise ValueError(f"{path}:{line_number}: run log does not exist: {expected_log}")
        raw_path = expected_variant_dir / "raw_results.jsonl"
        complete_eval = status in {"scored", "score_failed"}
        parsed_raw_rows = validate_raw_results(
            raw_path,
            variants[index - 1],
            item_ids,
            trials,
            require_complete=complete_eval,
            allow_trailing_partial=status in {"command_failed", "timed_out"},
        )
        raw_rows = len(parsed_raw_rows)
        result_rows = row.get("result_rows")
        if not isinstance(result_rows, int) or isinstance(result_rows, bool) or result_rows != raw_rows:
            raise ValueError(f"{path}:{line_number}: result_rows={result_rows!r}; found {raw_rows} raw result rows")
        run = None
        if complete_eval:
            run = validate_run_json(
                expected_variant_dir / "run.json",
                parsed_raw_rows,
                variants[index - 1],
                item_ids,
                trials,
                questions_path,
                prompt_path,
                expected_spec,
            )
        elif (expected_variant_dir / "run.json").exists():
            complete_rows = validate_raw_results(
                raw_path,
                variants[index - 1],
                item_ids,
                trials,
                require_complete=True,
            )
            run = validate_run_json(
                expected_variant_dir / "run.json",
                complete_rows,
                variants[index - 1],
                item_ids,
                trials,
                questions_path,
                prompt_path,
                expected_spec,
            )
        if status == "scored":
            if code != 0:
                raise ValueError(f"{path}:{line_number}: scored variant has exit code {code}")
            if row.get("score_exit_code") != 0:
                raise ValueError(f"{path}:{line_number}: scored variant must have score_exit_code 0")
            assert run is not None
            validate_scores(expected_variant_dir, run, questions_path)
        elif status == "score_failed":
            if code != 0:
                raise ValueError(f"{path}:{line_number}: score_failed variant has exit code {code}")
            score_code = row.get("score_exit_code")
            if not isinstance(score_code, int) or isinstance(score_code, bool) or score_code == 0:
                raise ValueError(f"{path}:{line_number}: score_failed variant requires a nonzero score_exit_code")
            assert run is not None
            validate_score_outputs(expected_variant_dir, run, questions_path, require_complete=False)
        elif status == "timed_out":
            if row.get("score_exit_code") is not None:
                raise ValueError(f"{path}:{line_number}: timed_out variant must not have score_exit_code")
            if code != TIMEOUT_EXIT_CODE or row.get("timeout_kind") not in {"variant", "no_progress"}:
                raise ValueError(f"{path}:{line_number}: timed_out variant has inconsistent exit code or timeout_kind")
            if any(
                (expected_variant_dir / name).exists()
                for name in ("scores.json", "scores.jsonl", SCORES_COMPLETE)
            ):
                raise ValueError(f"{path}:{line_number}: timed_out variant has unexpected score artifacts")
        else:
            if row.get("score_exit_code") is not None:
                raise ValueError(f"{path}:{line_number}: command_failed variant must not have score_exit_code")
            if code in {0, TIMEOUT_EXIT_CODE}:
                raise ValueError(f"{path}:{line_number}: command_failed variant has inconsistent exit code {code}")
            if run is None and any(
                (expected_variant_dir / name).exists()
                for name in ("scores.json", "scores.jsonl", SCORES_COMPLETE)
            ):
                raise ValueError(f"{path}:{line_number}: command_failed variant has score artifacts without run.json")
            if run is not None:
                validate_score_outputs(expected_variant_dir, run, questions_path, require_complete=False)
        try:
            artifact_summary = summarize_variant(expected_variant_dir) if status == "scored" else {"result_rows": raw_rows}
        except (OSError, ValueError, json.JSONDecodeError) as exc:
            raise ValueError(f"{path}:{line_number}: cannot summarize recorded variant artifacts: {exc}") from exc
        for key, expected in artifact_summary.items():
            if row.get(key) != expected:
                raise ValueError(f"{path}:{line_number}: {key}={row.get(key)!r}; recorded artifacts give {expected!r}")
        rows[index] = row
    return rows


def line_count(path: Path) -> int:
    try:
        with path.open() as f:
            return sum(1 for _ in f)
    except FileNotFoundError:
        return 0


def file_size(path: Path) -> int:
    try:
        return path.stat().st_size
    except FileNotFoundError:
        return 0


def tail_lines(path: Path, limit: int) -> list[str]:
    try:
        return path.read_text(encoding="utf-8", errors="replace").splitlines()[-limit:]
    except FileNotFoundError:
        return []


def active_process(path: Path) -> int | None:
    if not path.exists():
        return None
    try:
        pid = int(path.read_text().strip())
    except ValueError as exc:
        raise ValueError(f"{path}: active PID must be an integer") from exc
    if pid < 1:
        raise ValueError(f"{path}: active PID must be positive")
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return None
    except PermissionError:
        return pid
    return pid


def cleanup_atomic_temporaries(root: Path) -> None:
    for path in root.rglob(f"{PENDING_FILE_PREFIX}*"):
        if path.is_file():
            path.unlink()


def validate_variant_artifact_names(variant_dir: Path) -> None:
    if not variant_dir.exists():
        return
    if variant_dir.is_symlink() or not variant_dir.is_dir():
        raise ValueError(f"variant run path is not a real directory: {variant_dir}")
    for path in variant_dir.iterdir():
        if path.is_symlink() or not path.is_file():
            raise ValueError(f"unexpected variant-run artifact: {path}")
        if path.name not in VARIANT_ARTIFACT_NAMES and not path.name.startswith(PENDING_FILE_PREFIX):
            raise ValueError(f"unexpected variant-run artifact: {path}")


def send_event(kind: str, **data: object) -> None:
    payload = {
        "at": dt.datetime.now(dt.UTC).isoformat(),
        "kind": kind,
        **data,
    }
    print(json.dumps(payload, sort_keys=True), flush=True)


def summarize_variant(run_dir: Path) -> dict:
    scores_path = run_dir / "scores.json"
    raw_path = run_dir / "raw_results.jsonl"
    out = {"result_rows": line_count(raw_path)}
    if scores_path.exists():
        data = strict_json_loads(scores_path.read_bytes(), scores_path)
        if not isinstance(data, dict):
            raise ValueError(f"{scores_path}: expected a JSON object")
        summary = data.get("summary")
        if not isinstance(summary, dict) or len(summary) != 1:
            raise ValueError(f"{scores_path}: summary must contain exactly one model")
        model, model_summary = next(iter(summary.items()))
        if not isinstance(model_summary, dict):
            raise ValueError(f"{scores_path}: model summary must be an object")
        ops = model_summary.get("operational_metrics") or {}
        if not isinstance(ops, dict):
            raise ValueError(f"{scores_path}: operational_metrics must be an object")
        out.update(
            {
                "model": model,
                "completed_count": ops.get("completed_count", 0),
                "provider_error_count": ops.get("provider_error_count", 0),
                "timeout_count": ops.get("timeout_count", 0),
                "context_limit_error_count": ops.get("context_limit_error_count", 0),
                "schema_violation_count": ops.get("schema_violation_count", 0),
                "deliberation_score": model_summary.get("deliberation_score"),
            }
        )
    return out


def exit_code_value(row: dict) -> int | None:
    value = row.get("run_exit_code")
    if value in (None, ""):
        return None
    return int(value)


def row_timed_out(row: dict) -> bool:
    return row.get("variant_status") == "timed_out" or bool(row.get("timeout_kind")) or exit_code_value(row) == TIMEOUT_EXIT_CODE


def row_score_failed(row: dict) -> bool:
    return row.get("variant_status") == "score_failed"


def row_command_failed(row: dict) -> bool:
    if row.get("variant_status") == "command_failed":
        return True
    code = exit_code_value(row)
    if code in (None, 0, TIMEOUT_EXIT_CODE):
        return False
    return not row_score_failed(row)


def progress_counts(rows: dict[int, dict[str, Any]]) -> tuple[int, int, int]:
    completed = sum(1 for row in rows.values() if row.get("variant_status") in {"scored", "timed_out"})
    succeeded = sum(1 for row in rows.values() if row.get("variant_status") == "scored")
    failed = sum(1 for row in rows.values() if row.get("variant_status") in {"command_failed", "score_failed"})
    return completed, succeeded, failed


def write_progress(path: Path, rows: dict[int, dict[str, Any]]) -> None:
    data = b"".join(
        (json.dumps(rows[index], allow_nan=False, sort_keys=True) + "\n").encode("utf-8")
        for index in sorted(rows)
    )
    atomic_write_bytes(path, data)


def csv_value(value: Any) -> str:
    return "" if value is None else str(value)


def validate_summary_csv(path: Path, progress: dict[int, dict[str, Any]]) -> None:
    if not path.is_file():
        raise ValueError(f"variant summary CSV does not exist: {path}")
    expected_rows = [progress[index] for index in sorted(progress)]
    expected_fields = sorted({field for row in expected_rows for field in row})
    with path.open(newline="") as handle:
        reader = csv.DictReader(handle)
        if reader.fieldnames != expected_fields:
            raise ValueError(f"{path}: columns differ from progress.jsonl")
        found_rows = list(reader)
    if len(found_rows) != len(expected_rows):
        raise ValueError(f"{path}: expected {len(expected_rows)} rows, found {len(found_rows)}")
    for row_number, (found, expected) in enumerate(zip(found_rows, expected_rows, strict=True), start=2):
        normalized = {field: csv_value(expected.get(field)) for field in expected_fields}
        if found != normalized:
            raise ValueError(f"{path}:{row_number}: row differs from progress.jsonl")


def validate_batch_summary(path: Path, progress: dict[int, dict[str, Any]], total_variants: int) -> None:
    try:
        summary = strict_json_loads(path.read_bytes(), path)
    except FileNotFoundError:
        return
    if not isinstance(summary, dict):
        raise ValueError(f"{path}: expected a JSON object")
    completed, succeeded, failed = progress_counts(progress)
    rows = list(progress.values())
    expected = {
        "total_variants": total_variants,
        "completed_variants": len(progress),
        "succeeded": succeeded,
        "failed": failed,
        "timed_out": sum(1 for row in rows if row_timed_out(row)),
        "score_failed": sum(1 for row in rows if row_score_failed(row)),
        "command_failed": sum(1 for row in rows if row_command_failed(row)),
    }
    if completed != sum(1 for row in rows if row.get("variant_status") in {"scored", "timed_out"}):
        raise ValueError(f"{path}: internal progress completion count differs")
    for key, value in expected.items():
        if summary.get(key) != value:
            raise ValueError(f"{path}: {key}={summary.get(key)!r}; expected {value!r}")


def terminate_process(proc: subprocess.Popen, grace_seconds: int = 10) -> int:
    if proc.poll() is not None:
        return int(proc.returncode)
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        return int(proc.wait(timeout=grace_seconds))
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        return int(proc.wait())


def run_score_command(cmd: list[str], cwd: Path, lock_fd: int) -> tuple[int, str]:
    process: subprocess.Popen[str] | None = None
    try:
        process = subprocess.Popen(
            cmd,
            cwd=cwd,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            start_new_session=True,
            pass_fds=(lock_fd,),
        )
        output, _ = process.communicate()
        return int(process.returncode), output
    except BaseException as exc:
        if process is not None and process.poll() is None:
            try:
                terminate_process(process)
            except Exception as cleanup_error:
                exc.add_note(f"failed to terminate score child {process.pid}: {cleanup_error}")
        raise


def run_command(
    cmd: list[str],
    cwd: Path,
    active_pid: Path,
    stop_file: Path,
    raw_path: Path,
    log_path: Path,
    expected_rows: int,
    variant_label: str,
    completed_variants: int,
    total_variants: int,
    no_progress_timeout: int,
    variant_timeout: int,
    lock_fd: int,
) -> CommandResult:
    started = time.monotonic()
    last_report = 0.0
    last_activity = started
    last_rows = line_count(raw_path)
    last_log_size = file_size(log_path)
    log_path.parent.mkdir(parents=True, exist_ok=True)
    log_handle = log_path.open("w", encoding="utf-8")
    proc: subprocess.Popen[str] | None = None
    try:
        proc = subprocess.Popen(
            cmd,
            cwd=cwd,
            stdout=log_handle,
            stderr=subprocess.STDOUT,
            text=True,
            start_new_session=True,
            pass_fds=(lock_fd,),
        )
        atomic_write_bytes(active_pid, (str(proc.pid) + "\n").encode("utf-8"))
        while True:
            code = proc.poll()
            now = time.monotonic()
            rows = line_count(raw_path)
            log_size = file_size(log_path)
            if rows != last_rows or log_size != last_log_size:
                last_activity = now
                last_rows = rows
                last_log_size = log_size

            if code is not None:
                elapsed = round(now - started, 1)
                if code != 0:
                    send_event(
                        "command_failed",
                        completed_variants=completed_variants,
                        total_variants=total_variants,
                        current_variant=variant_label,
                        exit_code=code,
                        result_rows=rows,
                        expected_rows=expected_rows,
                        elapsed_seconds=elapsed,
                        log_path=display_path(log_path),
                        tail=tail_lines(log_path, 8),
                    )
                    return CommandResult(int(code), "command_failed", None, elapsed)
                return CommandResult(0, "finished", None, elapsed)

            if stop_file.exists():
                terminate_process(proc)
                return CommandResult(STOP_EXIT_CODE, "stopped", None, round(now - started, 1))

            elapsed_seconds = now - started
            seconds_since_progress = now - last_activity
            if elapsed_seconds >= variant_timeout:
                terminate_process(proc)
                elapsed = round(elapsed_seconds, 1)
                send_event(
                    "variant_timed_out",
                    completed_variants=completed_variants,
                    total_variants=total_variants,
                    current_variant=variant_label,
                    timeout_kind="variant",
                    timeout_seconds=variant_timeout,
                    result_rows=rows,
                    expected_rows=expected_rows,
                    elapsed_seconds=elapsed,
                    seconds_since_progress=round(seconds_since_progress, 1),
                    log_path=display_path(log_path),
                    tail=tail_lines(log_path, 8),
                )
                return CommandResult(TIMEOUT_EXIT_CODE, "timed_out", "variant", elapsed)

            if seconds_since_progress >= no_progress_timeout:
                terminate_process(proc)
                elapsed = round(elapsed_seconds, 1)
                send_event(
                    "variant_timed_out",
                    completed_variants=completed_variants,
                    total_variants=total_variants,
                    current_variant=variant_label,
                    timeout_kind="no_progress",
                    timeout_seconds=no_progress_timeout,
                    result_rows=rows,
                    expected_rows=expected_rows,
                    elapsed_seconds=elapsed,
                    seconds_since_progress=round(seconds_since_progress, 1),
                    log_path=display_path(log_path),
                    tail=tail_lines(log_path, 8),
                )
                return CommandResult(TIMEOUT_EXIT_CODE, "timed_out", "no_progress", elapsed)

            if now - last_report >= 60:
                last_report = now
                send_event(
                    "variant_in_progress",
                    completed_variants=completed_variants,
                    total_variants=total_variants,
                    current_variant=variant_label,
                    result_rows=rows,
                    expected_rows=expected_rows,
                    elapsed_seconds=round(now - started, 1),
                    seconds_since_progress=round(seconds_since_progress, 1),
                )
            time.sleep(2)
    except BaseException as exc:
        if proc is not None and proc.poll() is None:
            try:
                terminate_process(proc)
            except Exception as cleanup_error:
                exc.add_note(f"failed to terminate batch child {proc.pid}: {cleanup_error}")
        raise
    finally:
        log_handle.close()
        try:
            active_pid.unlink()
        except FileNotFoundError:
            pass


def execute_batch(
    args: argparse.Namespace,
    out_dir: Path,
    variants: list[dict[str, Any]],
    item_ids: list[str],
    questions_path: Path,
    prompt_path: Path,
    expected_rows: int,
    prior_by_index: dict[int, dict[str, Any]],
    recovered_by_index: dict[int, RecoveredVariant],
) -> int:
    stop_file = out_dir / "STOP"
    active_pid = out_dir / "ACTIVE_PID"
    state_path = out_dir / "progress.jsonl"
    summary_csv = out_dir / "variant_summary.csv"
    specs_dir = out_dir / "specs"
    runs_dir = out_dir / "variant-runs"
    specs_dir.mkdir(parents=True, exist_ok=True)
    runs_dir.mkdir(parents=True, exist_ok=True)
    try:
        active_pid.unlink()
    except FileNotFoundError:
        pass

    send_event(
        "run_started",
        run_dir=display_path(out_dir),
        stop_file=display_path(stop_file),
        total_variants=len(variants),
        already_completed_variants=len(prior_by_index),
        expected_rows_per_variant=expected_rows,
        questions=display_path(questions_path),
        prompt=display_path(prompt_path),
        trials=args.trials,
        request_timeout=args.timeout,
        no_progress_timeout=args.no_progress_timeout,
        variant_timeout=args.variant_timeout,
    )

    summaries_by_index = dict(prior_by_index)
    completed = len(summaries_by_index)
    _, succeeded, failed = progress_counts(summaries_by_index)
    stopped_by_request = False

    for index, spec in enumerate(variants, 1):
        prior = prior_by_index.get(index)
        if prior is not None and prior.get("variant_status") in {"scored", "timed_out"}:
            continue
        recovered = recovered_by_index.get(index)

        identity = variant_fields(spec)
        provider = identity["provider_name"]
        tag = identity["endpoint_tag"]
        quant = identity["quantization"]
        model_id = identity["openrouter_model_id"]
        variant_label = f"{index:02d}/{len(variants)} {model_id} @ {provider} ({tag}, {quant})"

        if stop_file.exists():
            stopped_by_request = True
            send_event(
                "run_stopped",
                completed_variants=completed,
                total_variants=len(variants),
                succeeded=succeeded,
                failed=failed,
                next_variant=variant_label,
                stop_file=display_path(stop_file),
            )
            break

        spec_path, variant_dir, log_path = variant_output_paths(out_dir, index, spec)
        raw_path = variant_dir / "raw_results.jsonl"
        if recovered is not None and recovered.state == "scored":
            for temporary in variant_dir.glob(f"{PENDING_FILE_PREFIX}*"):
                if temporary.is_file():
                    temporary.unlink()
            artifact_summary = summarize_variant(variant_dir)
            summary = {
                "index": index,
                "endpoint_variant_id": identity["endpoint_variant_id"],
                "openrouter_model_id": model_id,
                "provider_name": provider,
                "endpoint_tag": tag,
                "quantization": quant,
                "variant_run_dir": display_path(variant_dir),
                "run_log": display_path(log_path),
                "run_exit_code": 0,
                "score_exit_code": 0,
                "variant_status": "scored",
                "timeout_kind": None,
                "elapsed_seconds": 0.0,
                **artifact_summary,
            }
            summaries_by_index[index] = summary
            write_progress(state_path, summaries_by_index)
            completed = len(summaries_by_index)
            _, succeeded, failed = progress_counts(summaries_by_index)
            send_event("variant_recovered", current_variant=variant_label, recovery="scored", **summary)
            continue
        score_only_retry = (
            prior is not None and prior.get("variant_status") == "score_failed"
        ) or (recovered is not None and recovered.state == "evaluated")
        if not score_only_retry:
            atomic_write_bytes(
                spec_path,
                (json.dumps(spec, allow_nan=False, indent=2, sort_keys=True) + "\n").encode("utf-8"),
            )
        elif not spec_path.exists():
            atomic_write_bytes(
                spec_path,
                (json.dumps(spec, allow_nan=False, indent=2, sort_keys=True) + "\n").encode("utf-8"),
            )
        if recovered is not None and not (variant_dir / "run.json").exists():
            atomic_write_bytes(
                variant_dir / "run.json",
                (json.dumps(recovered.run, allow_nan=False, indent=2, sort_keys=True) + "\n").encode("utf-8"),
            )
        if recovered is not None:
            for temporary in variant_dir.glob(f"{PENDING_FILE_PREFIX}*"):
                if temporary.is_file():
                    temporary.unlink()

        send_event(
            "variant_started",
            completed_variants=completed,
            total_variants=len(variants),
            current_variant=variant_label,
            variant_run_dir=display_path(variant_dir),
            retry="score_only" if score_only_retry else "evaluation" if prior is not None else None,
            stop_file=display_path(stop_file),
        )

        if score_only_retry:
            prior_elapsed = prior.get("elapsed_seconds") if prior is not None else 0
            result = CommandResult(0, "finished", None, float(prior_elapsed or 0))
        else:
            cmd = [
                "uv",
                "run",
                "tools/run_eval.py",
                "--questions",
                display_path(questions_path),
                "--prompt",
                display_path(prompt_path),
                "--model-spec",
                display_path(spec_path),
                "--out",
                display_path(variant_dir),
                "--trials",
                str(args.trials),
                "--timeout",
                str(args.timeout),
            ]
            result = run_command(
                cmd,
                ROOT,
                active_pid,
                stop_file,
                raw_path,
                log_path,
                expected_rows,
                variant_label,
                completed,
                len(variants),
                args.no_progress_timeout,
                args.variant_timeout,
                args.run_lock_fd,
            )
        code = result.exit_code
        variant_status = result.status
        score_exit_code: int | None = None
        if result.status == "stopped":
            stopped_by_request = True
            send_event(
                "run_stopped",
                completed_variants=completed,
                total_variants=len(variants),
                succeeded=succeeded,
                failed=failed,
                stopped_during=variant_label,
                stop_file=display_path(stop_file),
            )
            break

        if code == 0:
            score_cmd = [
                "uv",
                "run",
                "tools/score_eval.py",
                "score",
                "--run",
                display_path(variant_dir),
                "--questions",
                display_path(questions_path),
            ]
            score_exit_code, score_output = run_score_command(score_cmd, ROOT, args.run_lock_fd)
            if score_exit_code != 0:
                variant_status = "score_failed"
                send_event("score_failed", current_variant=variant_label, exit_code=score_exit_code, output=score_output[-2000:])
            else:
                variant_status = "scored"

        if variant_status == "scored":
            artifact_summary = summarize_variant(variant_dir)
        elif variant_status in {"command_failed", "timed_out"}:
            artifact_summary = {
                "result_rows": len(
                    validate_raw_results(
                        raw_path,
                        spec,
                        item_ids,
                        args.trials,
                        require_complete=False,
                        allow_trailing_partial=True,
                    )
                )
            }
        else:
            artifact_summary = {"result_rows": line_count(raw_path)}
        summary = {
            "index": index,
            "endpoint_variant_id": identity["endpoint_variant_id"],
            "openrouter_model_id": model_id,
            "provider_name": provider,
            "endpoint_tag": tag,
            "quantization": quant,
            "variant_run_dir": display_path(variant_dir),
            "run_log": display_path(log_path),
            "run_exit_code": code,
            "score_exit_code": score_exit_code,
            "variant_status": variant_status,
            "timeout_kind": result.timeout_kind,
            "elapsed_seconds": result.elapsed_seconds,
            **artifact_summary,
        }
        summaries_by_index[index] = summary
        write_progress(state_path, summaries_by_index)
        completed = len(summaries_by_index)
        _, succeeded, failed = progress_counts(summaries_by_index)
        send_event(
            "variant_finished",
            completed_variants=completed,
            total_variants=len(variants),
            succeeded=succeeded,
            failed=failed,
            current_variant=variant_label,
            **summary,
        )

    summaries = [summaries_by_index[index] for index in sorted(summaries_by_index)]
    if summaries:
        fieldnames = sorted({key for row in summaries for key in row})
        handle = io.StringIO(newline="")
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(summaries)
        atomic_write_bytes(summary_csv, handle.getvalue().encode("utf-8"))

    final = {
        "finished_at": dt.datetime.now(dt.UTC).isoformat(),
        "run_dir": display_path(out_dir),
        "total_variants": len(variants),
        "completed_variants": completed,
        "succeeded": succeeded,
        "failed": failed,
        "timed_out": sum(1 for row in summaries if row_timed_out(row)),
        "score_failed": sum(1 for row in summaries if row_score_failed(row)),
        "command_failed": sum(1 for row in summaries if row_command_failed(row)),
        "stopped": stopped_by_request,
        "stop_file": display_path(stop_file),
        "prompt": display_path(prompt_path),
        "summary_csv": display_path(summary_csv),
    }
    atomic_write_bytes(
        out_dir / "summary.json",
        (json.dumps(final, allow_nan=False, indent=2, sort_keys=True) + "\n").encode("utf-8"),
    )
    send_event("run_finished", **final)
    if final["stopped"]:
        return STOP_EXIT_CODE
    return 1 if final["command_failed"] or final["score_failed"] else 0


def preflight_batch_resume(
    *,
    variants_path: Path,
    out_dir: Path,
    questions_path: Path,
    prompt_path: Path,
    trials: int,
    timeout: int,
    no_progress_timeout: int,
    variant_timeout: int | None,
) -> dict[int, RecoveredVariant]:
    out_dir = out_dir.expanduser().resolve()
    variants_input = input_file("variants", variants_path, "inputs/variants.jsonl")
    variants = parse_jsonl_objects(variants_input.source_path.read_bytes(), variants_input.source_path)
    validate_variants(variants, variants_input.source_path)
    question_inputs, runtime_questions, items = questions_snapshot(questions_path, ROOT, out_dir)
    prompt_input = input_file("prompt", prompt_path, "inputs/prompt.md")
    item_ids: list[str] = []
    for index, item in enumerate(items, start=1):
        item_id = item.get("id")
        if not isinstance(item_id, str) or not item_id.strip():
            raise ValueError(f"question row {index} has no nonempty string id")
        if item_id in item_ids:
            raise ValueError(f"question id is duplicated: {item_id}")
        item_ids.append(item_id)
    expected_rows = len(items) * trials
    effective_variant_timeout = variant_timeout
    if effective_variant_timeout is None:
        effective_variant_timeout = max(timeout * expected_rows * 2, no_progress_timeout)
    parameters = {
        "trials": trials,
        "request_timeout": timeout,
        "no_progress_timeout": no_progress_timeout,
        "variant_timeout": effective_variant_timeout,
    }
    validate_run_record_inputs(
        out_dir,
        kind="model-pool-variant-batch",
        parameters=parameters,
        inputs=[variants_input, *question_inputs, prompt_input],
        generated_inputs=[runtime_questions],
    )
    if not (out_dir / "manifest.json").is_file():
        unexpected = [path for path in out_dir.iterdir() if not path.name.startswith(".run-record.pending-")]
        if unexpected:
            raise ValueError(f"unpublished eval run record has unexpected artifacts: {unexpected}")
        return {}

    saved_questions = out_dir / runtime_questions.snapshot_path
    saved_prompt = out_dir / prompt_input.snapshot_path
    progress = load_progress(
        out_dir / "progress.jsonl",
        variants,
        out_dir,
        item_ids,
        trials,
        saved_questions,
        saved_prompt,
    )
    if (out_dir / "variant_summary.csv").exists():
        validate_summary_csv(out_dir / "variant_summary.csv", progress)
    validate_batch_summary(out_dir / "summary.json", progress, len(variants))
    running_pid = active_process(out_dir / "ACTIVE_PID")
    if running_pid is not None:
        raise ValueError(f"batch child process {running_pid} is still running according to {out_dir / 'ACTIVE_PID'}")

    expected_specs: set[Path] = set()
    expected_runs: set[Path] = set()
    recovered: dict[int, RecoveredVariant] = {}
    for index, spec in enumerate(variants, start=1):
        spec_path, variant_dir, _ = variant_output_paths(out_dir, index, spec)
        expected_specs.add(spec_path)
        expected_runs.add(variant_dir)
        validate_variant_artifact_names(variant_dir)
        prior = progress.get(index)
        if prior is not None and prior.get("variant_status") in {"scored", "timed_out"}:
            continue
        expected_spec_bytes = (json.dumps(spec, allow_nan=False, indent=2, sort_keys=True) + "\n").encode()
        if spec_path.exists() and (
            spec_path.is_symlink()
            or not spec_path.is_file()
            or spec_path.read_bytes() != expected_spec_bytes
        ):
            raise ValueError(f"unrecorded variant spec differs from the saved input: {spec_path}")
        raw_path = variant_dir / "raw_results.jsonl"
        raw_rows = validate_raw_results(
            raw_path,
            spec,
            item_ids,
            trials,
            require_complete=False,
            allow_trailing_partial=True,
        )
        run_path = variant_dir / "run.json"
        run: dict[str, Any] | None = None
        if run_path.exists():
            complete_rows = validate_raw_results(raw_path, spec, item_ids, trials, require_complete=True)
            run = validate_run_json(
                run_path,
                complete_rows,
                spec,
                item_ids,
                trials,
                saved_questions,
                saved_prompt,
                spec_path,
            )
        else:
            score_artifacts = [
                variant_dir / "scores.json",
                variant_dir / "scores.jsonl",
                variant_dir / SCORES_COMPLETE,
            ]
            if any(path.exists() for path in score_artifacts):
                raise ValueError(f"unrecorded score artifacts exist without run.json: {variant_dir}")
            if len(raw_rows) == expected_rows:
                complete_rows = validate_raw_results(raw_path, spec, item_ids, trials, require_complete=True)
                run = recovered_run_json(
                    variant_dir,
                    complete_rows,
                    spec,
                    item_ids,
                    trials,
                    saved_questions,
                    saved_prompt,
                    spec_path,
                )
        if run is not None:
            if not spec_path.is_file():
                raise ValueError(f"unrecorded completed evaluation lacks its variant spec: {spec_path}")
            log_path = variant_output_paths(out_dir, index, spec)[2]
            if not log_path.is_file():
                raise ValueError(f"unrecorded completed evaluation lacks its run log: {log_path}")
            scores_complete = validate_score_outputs(
                variant_dir,
                run,
                saved_questions,
                require_complete=False,
            )
            recovered[index] = RecoveredVariant("scored" if scores_complete else "evaluated", run)
    specs_dir = out_dir / "specs"
    if specs_dir.is_dir():
        if specs_dir.is_symlink():
            raise ValueError(f"variant specs path is not a real directory: {specs_dir}")
        unexpected_specs = {
            path
            for path in specs_dir.iterdir()
            if not path.name.startswith(PENDING_FILE_PREFIX)
        } - expected_specs
        if unexpected_specs:
            raise ValueError(f"unexpected variant spec: {min(unexpected_specs, key=str)}")
        invalid_specs = {
            path
            for path in specs_dir.iterdir()
            if path.name.startswith(PENDING_FILE_PREFIX) and (path.is_symlink() or not path.is_file())
        }
        if invalid_specs:
            raise ValueError(f"invalid variant spec temporary: {min(invalid_specs, key=str)}")
    elif specs_dir.exists():
        raise ValueError(f"variant specs path is not a directory: {specs_dir}")
    runs_dir = out_dir / "variant-runs"
    if runs_dir.is_dir():
        if runs_dir.is_symlink():
            raise ValueError(f"variant runs path is not a real directory: {runs_dir}")
        unexpected_runs = set(runs_dir.iterdir()) - expected_runs
        if unexpected_runs:
            raise ValueError(f"unexpected variant run directory: {min(unexpected_runs, key=str)}")
    elif runs_dir.exists():
        raise ValueError(f"variant runs path is not a directory: {runs_dir}")
    return recovered


def main() -> int:
    ap = argparse.ArgumentParser(description="Run adj model-pool evals over OpenRouter endpoint variants one variant at a time.")
    ap.add_argument("--variants", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--questions", default="sets/core20/questions.jsonl")
    ap.add_argument("--prompt", default="prompts/juror-single.md", help="Prompt file passed to run_eval.py. Relative paths resolve from model-pool/.")
    ap.add_argument("--trials", type=int, default=3)
    ap.add_argument("--timeout", type=int, default=90)
    ap.add_argument("--no-progress-timeout", type=int)
    ap.add_argument("--variant-timeout", type=int)
    ap.add_argument("--resume", action="store_true", help="Continue a recorded run after verifying its arguments and inputs")
    args = ap.parse_args()

    variants_path = Path(args.variants).expanduser()
    if not variants_path.is_absolute():
        variants_path = ROOT / variants_path
    out_dir = Path(args.out).expanduser()
    if not out_dir.is_absolute():
        out_dir = ROOT / out_dir
    out_dir = out_dir.expanduser().resolve()
    questions_path = Path(args.questions).expanduser()
    if not questions_path.is_absolute():
        questions_path = ROOT / questions_path
    prompt_path = Path(args.prompt).expanduser()
    if not prompt_path.is_absolute():
        prompt_path = ROOT / prompt_path
    try:
        variants_input = input_file("variants", variants_path, "inputs/variants.jsonl")
        variants = parse_jsonl_objects(variants_input.source_path.read_bytes(), variants_input.source_path)
        validate_variants(variants, variants_input.source_path)
        question_inputs, runtime_questions, items = questions_snapshot(questions_path, ROOT, out_dir)
        prompt_input = input_file("prompt", prompt_path, "inputs/prompt.md")
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc
    expected_rows = len(items) * args.trials
    item_ids: list[str] = []
    for index, item in enumerate(items, start=1):
        item_id = item.get("id")
        if not isinstance(item_id, str) or not item_id.strip():
            raise SystemExit(f"question row {index} has no nonempty string id")
        if item_id in item_ids:
            raise SystemExit(f"question id is duplicated: {item_id}")
        item_ids.append(item_id)
    if args.trials < 1:
        raise SystemExit("--trials must be positive")
    if args.timeout < 1:
        raise SystemExit("--timeout must be positive")
    if args.no_progress_timeout is None:
        args.no_progress_timeout = max(args.timeout * 2, 180)
    if args.no_progress_timeout < 1:
        raise SystemExit("--no-progress-timeout must be positive")
    if args.variant_timeout is None:
        args.variant_timeout = max(args.timeout * expected_rows * 2, args.no_progress_timeout)
    if args.variant_timeout < args.no_progress_timeout:
        raise SystemExit("--variant-timeout must be greater than or equal to --no-progress-timeout")
    parameters = {
        "trials": args.trials,
        "request_timeout": args.timeout,
        "no_progress_timeout": args.no_progress_timeout,
        "variant_timeout": args.variant_timeout,
    }
    if args.resume:
        if not out_dir.is_dir():
            raise SystemExit(f"resume directory does not exist: {out_dir}")
    else:
        out_dir.parent.mkdir(parents=True, exist_ok=True)
        try:
            out_dir.mkdir(exist_ok=True)
        except OSError as exc:
            raise SystemExit(f"cannot create run directory {out_dir}: {exc}") from exc
    try:
        with exclusive_run_lock(
            out_dir,
            "model-pool-variant-batch",
            [str(Path(__file__)), *sys.argv[1:]],
        ) as run_lock:
            if args.resume:
                recovered_by_index = preflight_batch_resume(
                    variants_path=variants_input.source_path,
                    out_dir=out_dir,
                    questions_path=questions_path,
                    prompt_path=prompt_path,
                    trials=args.trials,
                    timeout=args.timeout,
                    no_progress_timeout=args.no_progress_timeout,
                    variant_timeout=args.variant_timeout,
                )
            else:
                recovered_by_index = {}
            prepare_run_record(
                out_dir,
                kind="model-pool-variant-batch",
                parameters=parameters,
                inputs=[variants_input, *question_inputs, prompt_input],
                generated_inputs=[runtime_questions],
                resume=args.resume,
            )
            if args.resume:
                cleanup_atomic_temporaries(out_dir)
            saved_questions = out_dir / runtime_questions.snapshot_path
            saved_prompt = out_dir / prompt_input.snapshot_path
            state_path = out_dir / "progress.jsonl"
            prior_by_index = (
                load_progress(state_path, variants, out_dir, item_ids, args.trials, saved_questions, saved_prompt)
                if args.resume
                else {}
            )
            if args.resume and (out_dir / "variant_summary.csv").exists():
                validate_summary_csv(out_dir / "variant_summary.csv", prior_by_index)
            if args.resume:
                validate_batch_summary(out_dir / "summary.json", prior_by_index, len(variants))
            running_pid = active_process(out_dir / "ACTIVE_PID")
            if running_pid is not None:
                raise ValueError(
                    f"batch child process {running_pid} is still running according to {out_dir / 'ACTIVE_PID'}"
                )
            args.run_lock_fd = run_lock.file_descriptor
            run_lock.record_owner()
            return execute_batch(
                args,
                out_dir,
                variants,
                item_ids,
                saved_questions,
                saved_prompt,
                expected_rows,
                prior_by_index,
                recovered_by_index,
            )
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc

if __name__ == "__main__":
    raise SystemExit(main())
