#!/usr/bin/env -S uv run --no-cache --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import datetime as dt
import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = ROOT.parent


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def event(kind: str, **values: object) -> None:
    print(json.dumps({"at": utc_now(), "kind": kind, **values}, sort_keys=True), flush=True)


def resolve(path_text: str) -> Path:
    path = Path(path_text)
    return path if path.is_absolute() else ROOT / path


def display(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def load_jsonl(path: Path) -> list[dict]:
    with path.open() as handle:
        return [json.loads(line) for line in handle if line.strip()]


def write_json(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False, sort_keys=True) + "\n")


def write_jsonl(path: Path, rows: list[dict]) -> None:
    with path.open("w") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")


def safe_part(value: object) -> str:
    text = re.sub(r"[^a-z0-9._-]+", "-", str(value or "unknown").lower())
    return re.sub(r"-+", "-", text).strip("-")[:80] or "unknown"


def precheck_reasons(spec: dict) -> list[str]:
    reasons: list[str] = []
    if spec.get("exact_route_ambiguous") is True:
        reasons.append("exact_route_ambiguous")
    if not str(spec.get("openrouter_model_id") or "").strip():
        reasons.append("missing_openrouter_model_id")
    if not str(spec.get("endpoint_tag") or spec.get("provider_name") or "").strip():
        reasons.append("missing_provider_route")
    input_modalities = spec.get("input_modalities")
    if not isinstance(input_modalities, list) or "text" not in {
        str(value).strip().lower() for value in input_modalities
    }:
        reasons.append("text_input_unsupported")
    output_modalities = spec.get("output_modalities")
    if not isinstance(output_modalities, list) or "text" not in {
        str(value).strip().lower() for value in output_modalities
    }:
        reasons.append("text_output_unsupported")
    supported_parameters = spec.get("supported_parameters")
    if not isinstance(supported_parameters, list) or "tools" not in {
        str(value).strip().lower() for value in supported_parameters
    }:
        reasons.append("tools_unsupported")
    return reasons


def load_openrouter_key() -> str:
    key = os.environ.get("OPENROUTER_API_KEY", "").strip()
    if key:
        return key
    path = ROOT / "secrets" / "openrouter.api.txt"
    try:
        text = path.read_text()
    except FileNotFoundError as exc:
        raise RuntimeError("OPENROUTER_API_KEY or model-pool/secrets/openrouter.api.txt is required") from exc
    patterns = (
        re.compile(r"^\s*export\s+OPENROUTER_API_KEY\s*=\s*['\"]?([^'\"\s]+)", re.M),
        re.compile(r"^\s*OPENROUTER_API_KEY\s*[:=]\s*['\"]?([^'\"\s]+)", re.M),
        re.compile(r"^\s*openrouter[^:=]*[:=]\s*['\"]?([^'\"\s]+)", re.I | re.M),
    )
    for pattern in patterns:
        match = pattern.search(text)
        if match:
            return match.group(1).strip()
    raise RuntimeError(f"could not read OPENROUTER_API_KEY from {display(path)}")


def run_screen_command(command: list[str], log_path: Path, env: dict[str, str], progress: dict) -> int:
    started = time.monotonic()
    last_report = started
    with log_path.open("w") as log:
        process = subprocess.Popen(
            command,
            cwd=REPO_ROOT,
            env=env,
            stdout=log,
            stderr=subprocess.STDOUT,
            text=True,
        )
        while process.poll() is None:
            now = time.monotonic()
            if now - last_report >= 60:
                last_report = now
                event(
                    "configuration_in_progress",
                    elapsed_seconds=round(now - started, 1),
                    **progress,
                )
            time.sleep(1)
    return int(process.returncode)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Screen OpenRouter model configurations through Quick-style and Pi/MCP tool calls.")
    parser.add_argument("--variants", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--screen-command", default="../.bin/model-config-screen")
    parser.add_argument("--direct-timeout", type=int, default=20)
    parser.add_argument("--pi-timeout", type=int, default=300)
    parser.add_argument("--max-attempts", type=int, default=3)
    parser.add_argument("--podman-command", default="podman")
    parser.add_argument("--pi-image", default="agentcourt-pi-sandbox")
    parser.add_argument("--pi-mcp-adapter", default="/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter")
    parser.add_argument("--pi-mcp-host", default="127.0.0.1")
    args = parser.parse_args(argv)
    if args.direct_timeout < 1 or args.pi_timeout < 1:
        raise SystemExit("timeouts must be positive")
    if args.max_attempts < 1 or args.max_attempts > 4:
        raise SystemExit("--max-attempts must be between 1 and 4")
    return args


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    variants_path = resolve(args.variants)
    out_dir = resolve(args.out)
    screen_command = resolve(args.screen_command)
    if not screen_command.is_file():
        raise SystemExit(f"screen command does not exist: {display(screen_command)}")
    if out_dir.exists() and (not out_dir.is_dir() or any(out_dir.iterdir())):
        raise SystemExit(f"{display(out_dir)} must be absent or an empty directory")
    out_dir.mkdir(parents=True, exist_ok=True)
    specs_dir = out_dir / "specs"
    runs_dir = out_dir / "configurations"
    specs_dir.mkdir()
    runs_dir.mkdir()

    variants = load_jsonl(variants_path)
    env = os.environ.copy()
    env["OPENROUTER_API_KEY"] = load_openrouter_key()
    results: list[dict] = []
    accepted: list[dict] = []
    rejected: list[dict] = []
    observed_cost = 0.0
    cost_observations = 0
    event("screen_started", total_configurations=len(variants), variants=display(variants_path), out=display(out_dir))

    for index, spec in enumerate(variants, 1):
        model = str(spec.get("openrouter_model_id") or "unknown-model")
        provider = str(spec.get("endpoint_tag") or spec.get("provider_name") or "unknown-provider")
        label = f"{index:04d}-{safe_part(model)}-{safe_part(provider)}"
        spec_path = specs_dir / f"{label}.json"
        run_dir = runs_dir / label
        write_json(spec_path, spec)
        reasons = precheck_reasons(spec)
        if reasons:
            record = {
                "index": index,
                "combined_index": spec.get("combined_index", index),
                "openrouter_model_id": model,
                "provider_name": spec.get("provider_name"),
                "endpoint_tag": spec.get("endpoint_tag"),
                "endpoint_variant_id": spec.get("endpoint_variant_id"),
                "status": "metadata_rejected",
                "reasons": reasons,
                "observed_cost_usd": 0,
                "cost_observation_count": 0,
            }
            results.append(record)
            rejected.append({**spec, "screen": record})
            event(
                "configuration_finished",
                completed_configurations=index,
                total_configurations=len(variants),
                cumulative_observed_cost_usd=observed_cost,
                cumulative_cost_observation_count=cost_observations,
                **record,
            )
            continue

        command = [
            str(screen_command),
            "--spec", str(spec_path),
            "--out", str(run_dir),
            "--direct-timeout", f"{args.direct_timeout}s",
            "--pi-timeout", f"{args.pi_timeout}s",
            "--max-attempts", str(args.max_attempts),
            "--podman-command", args.podman_command,
            "--pi-image", args.pi_image,
            "--pi-mcp-adapter", args.pi_mcp_adapter,
            "--pi-mcp-host", args.pi_mcp_host,
        ]
        event(
            "configuration_started",
            index=index,
            total_configurations=len(variants),
            openrouter_model_id=model,
            endpoint_tag=spec.get("endpoint_tag"),
            run_dir=display(run_dir),
        )
        exit_code = run_screen_command(
            command,
            run_dir.parent / f"{label}.log",
            env,
            {
                "index": index,
                "total_configurations": len(variants),
                "openrouter_model_id": model,
                "endpoint_tag": spec.get("endpoint_tag"),
                "observed_cost_usd": observed_cost,
                "cost_observation_count": cost_observations,
            },
        )
        result_path = run_dir / "result.json"
        if not result_path.is_file():
            raise RuntimeError(f"screen command exited {exit_code} without {display(result_path)}")
        detail = json.loads(result_path.read_text())
        record = {
            "index": index,
            "combined_index": spec.get("combined_index", index),
            "openrouter_model_id": model,
            "provider_name": spec.get("provider_name"),
            "endpoint_tag": spec.get("endpoint_tag"),
            "endpoint_variant_id": spec.get("endpoint_variant_id"),
            "status": "accepted" if exit_code == 0 else "rejected",
            "exit_code": exit_code,
            "result": display(result_path),
            "direct": detail.get("direct"),
            "pi_mcp": detail.get("pi_mcp"),
            "observed_cost_usd": float(detail.get("observed_cost_usd") or 0),
            "cost_observation_count": int(detail.get("cost_observation_count") or 0),
        }
        results.append(record)
        observed_cost += record["observed_cost_usd"]
        cost_observations += record["cost_observation_count"]
        if exit_code == 0:
            accepted.append(spec)
        else:
            rejected.append({**spec, "screen": record})
        write_jsonl(out_dir / "results.jsonl", results)
        write_jsonl(out_dir / "endpoint_variants.jsonl", accepted)
        write_jsonl(out_dir / "rejected_variants.jsonl", rejected)
        event(
            "configuration_finished",
            completed_configurations=index,
            total_configurations=len(variants),
            cumulative_observed_cost_usd=observed_cost,
            cumulative_cost_observation_count=cost_observations,
            **record,
        )
        if exit_code == 2:
            raise RuntimeError(f"screen infrastructure error for {model} at {provider}; see {display(result_path)}")

    summary = {
        "created_at": utc_now(),
        "source_variants": display(variants_path),
        "total_configurations": len(variants),
        "accepted": len(accepted),
        "rejected": len(rejected),
        "observed_cost_usd": observed_cost,
        "cost_observation_count": cost_observations,
        "outputs": {
            "accepted_variants": display(out_dir / "endpoint_variants.jsonl"),
            "rejected_variants": display(out_dir / "rejected_variants.jsonl"),
            "results": display(out_dir / "results.jsonl"),
        },
    }
    write_jsonl(out_dir / "results.jsonl", results)
    write_jsonl(out_dir / "endpoint_variants.jsonl", accepted)
    write_jsonl(out_dir / "rejected_variants.jsonl", rejected)
    write_json(out_dir / "summary.json", summary)
    event("screen_finished", **summary)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
