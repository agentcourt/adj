#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import csv
import datetime as dt
import json
import os
import re
import signal
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TIMEOUT_EXIT_CODE = 124
STOP_EXIT_CODE = 130


@dataclass(frozen=True)
class CommandResult:
    exit_code: int
    status: str
    timeout_kind: str | None
    elapsed_seconds: float


def safe_part(value: object) -> str:
    text = str(value or "unknown").lower()
    text = re.sub(r"[^a-z0-9._-]+", "-", text)
    text = re.sub(r"-+", "-", text).strip("-")
    return text[:80] or "unknown"


def display_path(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def load_jsonl(path: Path) -> list[dict]:
    rows = []
    for line in path.read_text().splitlines():
        if line.strip():
            rows.append(json.loads(line))
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


def load_items(path: Path) -> list[dict]:
    return load_jsonl(path)


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
        data = json.loads(scores_path.read_text())
        summary = data.get("summary") or {}
        if summary:
            model, model_summary = next(iter(summary.items()))
            ops = model_summary.get("operational_metrics") or {}
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
) -> CommandResult:
    started = time.monotonic()
    last_report = 0.0
    last_activity = started
    last_rows = line_count(raw_path)
    last_log_size = file_size(log_path)
    log_path.parent.mkdir(parents=True, exist_ok=True)
    log_handle = log_path.open("w", encoding="utf-8")
    try:
        proc = subprocess.Popen(
            cmd,
            cwd=cwd,
            stdout=log_handle,
            stderr=subprocess.STDOUT,
            text=True,
            start_new_session=True,
        )
        active_pid.write_text(str(proc.pid) + "\n")
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
    finally:
        log_handle.close()
        try:
            active_pid.unlink()
        except FileNotFoundError:
            pass


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
    args = ap.parse_args()

    variants_path = Path(args.variants)
    if not variants_path.is_absolute():
        variants_path = ROOT / variants_path
    out_dir = Path(args.out)
    if not out_dir.is_absolute():
        out_dir = ROOT / out_dir
    questions_path = Path(args.questions)
    if not questions_path.is_absolute():
        questions_path = ROOT / questions_path

    variants = load_jsonl(variants_path)
    items = load_items(questions_path)
    expected_rows = len(items) * args.trials
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
    specs_dir = out_dir / "specs"
    runs_dir = out_dir / "variant-runs"
    out_dir.mkdir(parents=True, exist_ok=True)
    specs_dir.mkdir(parents=True, exist_ok=True)
    runs_dir.mkdir(parents=True, exist_ok=True)

    stop_file = out_dir / "STOP"
    active_pid = out_dir / "ACTIVE_PID"
    state_path = out_dir / "progress.jsonl"
    summary_csv = out_dir / "variant_summary.csv"
    try:
        active_pid.unlink()
    except FileNotFoundError:
        pass

    prior_by_index: dict[int, dict] = {}
    if state_path.exists():
        for line in state_path.read_text().splitlines():
            if not line.strip():
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            index = row.get("index")
            if isinstance(index, int):
                prior_by_index[index] = row

    send_event(
        "run_started",
        run_dir=display_path(out_dir),
        stop_file=display_path(stop_file),
        total_variants=len(variants),
        already_completed_variants=len(prior_by_index),
        expected_rows_per_variant=expected_rows,
        questions=display_path(questions_path),
        prompt=args.prompt,
        trials=args.trials,
        request_timeout=args.timeout,
        no_progress_timeout=args.no_progress_timeout,
        variant_timeout=args.variant_timeout,
    )

    completed = len(prior_by_index)
    succeeded = sum(1 for row in prior_by_index.values() if row.get("run_exit_code") == 0)
    failed = sum(1 for row in prior_by_index.values() if row.get("run_exit_code") not in (None, 0))
    summaries: list[dict] = [prior_by_index[i] for i in sorted(prior_by_index)]

    for index, spec in enumerate(variants, 1):
        if index in prior_by_index:
            continue

        provider = spec.get("provider_name") or "unknown-provider"
        tag = spec.get("endpoint_tag") or "unknown-endpoint"
        quant = spec.get("quantization") or "unknown"
        model_id = spec.get("openrouter_model_id") or "unknown-model"
        variant_label = f"{index:02d}/{len(variants)} {model_id} @ {provider} ({tag}, {quant})"

        if stop_file.exists():
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

        spec_path = specs_dir / f"{index:02d}-{safe_part(model_id)}-{safe_part(provider)}-{safe_part(tag)}.json"
        spec_path.write_text(json.dumps(spec, indent=2, sort_keys=True) + "\n")
        variant_dir = runs_dir / f"{index:02d}-{safe_part(model_id)}-{safe_part(provider)}-{safe_part(tag)}"
        raw_path = variant_dir / "raw_results.jsonl"
        log_path = variant_dir / "run_eval.log"

        send_event(
            "variant_started",
            completed_variants=completed,
            total_variants=len(variants),
            current_variant=variant_label,
            variant_run_dir=display_path(variant_dir),
            stop_file=display_path(stop_file),
        )

        cmd = [
            "uv",
            "run",
            "tools/run_eval.py",
            "--questions",
            display_path(questions_path),
            "--prompt",
            args.prompt,
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
        )
        code = result.exit_code
        variant_status = result.status
        if code == STOP_EXIT_CODE:
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
            score_cmd = ["uv", "run", "tools/score_eval.py", "score", "--run", display_path(variant_dir), "--questions", display_path(questions_path)]
            score = subprocess.run(score_cmd, cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            if score.returncode != 0:
                failed += 1
                variant_status = "score_failed"
                send_event("score_failed", current_variant=variant_label, exit_code=score.returncode, output=score.stdout[-2000:])
            else:
                succeeded += 1
                variant_status = "scored"
        else:
            failed += 1

        completed += 1
        summary = {
            "index": index,
            "openrouter_model_id": model_id,
            "provider_name": provider,
            "endpoint_tag": tag,
            "quantization": quant,
            "variant_run_dir": display_path(variant_dir),
            "run_log": display_path(log_path),
            "run_exit_code": code,
            "variant_status": variant_status,
            "timeout_kind": result.timeout_kind,
            "elapsed_seconds": result.elapsed_seconds,
            **summarize_variant(variant_dir),
        }
        summaries.append(summary)
        with state_path.open("a") as f:
            f.write(json.dumps(summary, sort_keys=True) + "\n")
        send_event(
            "variant_finished",
            completed_variants=completed,
            total_variants=len(variants),
            succeeded=succeeded,
            failed=failed,
            current_variant=variant_label,
            **summary,
        )

    if summaries:
        fieldnames = sorted({key for row in summaries for key in row})
        with summary_csv.open("w", newline="") as f:
            writer = csv.DictWriter(f, fieldnames=fieldnames)
            writer.writeheader()
            writer.writerows(summaries)

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
        "stopped": stop_file.exists() and completed < len(variants),
        "stop_file": display_path(stop_file),
        "prompt": args.prompt,
        "summary_csv": display_path(summary_csv),
    }
    (out_dir / "summary.json").write_text(json.dumps(final, indent=2, sort_keys=True) + "\n")
    send_event("run_finished", **final)
    if final["stopped"]:
        return STOP_EXIT_CODE
    return 1 if final["command_failed"] or final["score_failed"] else 0


if __name__ == "__main__":
    raise SystemExit(main())
