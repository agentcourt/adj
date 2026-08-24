#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Fetch OpenRouter model/provider endpoint inventory.

This is a static catalog inventory. It does not run inference probes.
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import hashlib
import json
import os
import random
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
API_BASE = "https://openrouter.ai/api/v1"


CSV_FIELDS = [
    "catalog_snapshot_id",
    "snapshot_timestamp_utc",
    "openrouter_model_id",
    "canonical_slug",
    "model_name",
    "model_created",
    "model_description",
    "hugging_face_id",
    "knowledge_cutoff",
    "modality",
    "input_modalities",
    "output_modalities",
    "tokenizer",
    "instruct_type",
    "model_context_length",
    "model_supported_parameters",
    "model_default_parameters",
    "model_pricing",
    "model_top_provider",
    "model_per_request_limits",
    "model_supported_voices",
    "model_raw_path",
    "raw_model_sha256",
    "endpoint_index",
    "endpoint_variant_id",
    "endpoint_variant_key",
    "provider_name",
    "endpoint_name",
    "endpoint_tag",
    "endpoint_id",
    "endpoint_model_id",
    "endpoint_model_name",
    "endpoint_model_permaslug",
    "quantization",
    "unknown_quantization_endpoint_variant",
    "context_length",
    "max_prompt_tokens",
    "max_completion_tokens",
    "supported_parameters",
    "supports_implicit_caching",
    "pricing_prompt",
    "pricing_completion",
    "pricing_input_cache_read",
    "pricing_input_cache_write",
    "pricing_discount",
    "endpoint_pricing",
    "status",
    "uptime_last_5m",
    "uptime_last_30m",
    "uptime_last_1d",
    "latency_last_30m",
    "throughput_last_30m",
    "endpoint_raw_path",
    "raw_endpoint_sha256",
]


class OpenRouterError(RuntimeError):
    pass


def canonical_json(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def pretty_json(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, indent=2) + "\n"


def sha256_json(obj: Any) -> str:
    return hashlib.sha256(canonical_json(obj).encode("utf-8")).hexdigest()


def safe_name(value: str) -> str:
    cleaned = re.sub(r"[^A-Za-z0-9._-]+", "_", value.strip())
    return cleaned.strip("_") or "unnamed"


def endpoint_raw_filename(model_id: str) -> str:
    digest = hashlib.sha256(model_id.encode("utf-8")).hexdigest()[:12]
    return f"{safe_name(model_id)}-{digest}.json"


def load_openrouter_key() -> str | None:
    if os.environ.get("OPENROUTER_API_KEY"):
        return os.environ["OPENROUTER_API_KEY"]
    candidates = [ROOT / "secrets" / "openrouter.api.txt"]
    patterns = [
        re.compile(r"^\s*export\s+OPENROUTER_API_KEY\s*=\s*['\"]?([^'\"\s]+)", re.M),
        re.compile(r"^\s*OPENROUTER_API_KEY\s*[:=]\s*['\"]?([^'\"\s]+)", re.M),
        re.compile(r"^\s*openrouter[^:=]*[:=]\s*['\"]?([^'\"\s]+)", re.I | re.M),
    ]
    for path in candidates:
        try:
            text = path.read_text()
        except FileNotFoundError:
            continue
        for pattern in patterns:
            match = pattern.search(text)
            if match:
                return match.group(1).strip()
    return None


def api_get(path_or_url: str, key: str, timeout: int, retries: int) -> Any:
    if path_or_url.startswith("http://") or path_or_url.startswith("https://"):
        url = path_or_url
    else:
        url = f"{API_BASE}{path_or_url if path_or_url.startswith('/') else '/' + path_or_url}"
    last_error: Exception | None = None
    for attempt in range(retries + 1):
        request = urllib.request.Request(
            url,
            headers={
                "Authorization": f"Bearer {key}",
                "Accept": "application/json",
                "HTTP-Referer": "https://github.com/agentcourt/adj",
                "X-Title": "adj-model-pool-inventory",
            },
            method="GET",
        )
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                return json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode(errors="replace")[:500]
            last_error = OpenRouterError(f"OpenRouter HTTP {exc.code} for {url}: {detail}")
            if exc.code not in {408, 429, 500, 502, 503, 504} or attempt == retries:
                break
        except (urllib.error.URLError, TimeoutError) as exc:
            last_error = exc
            if attempt == retries:
                break
        sleep_for = min(2 ** attempt, 8)
        time.sleep(sleep_for)
    raise OpenRouterError(str(last_error))


def endpoint_path_for_model(model: dict[str, Any]) -> str:
    details = (model.get("links") or {}).get("details")
    if isinstance(details, str) and details.startswith("/api/v1/models/") and details.endswith("/endpoints"):
        return details.removeprefix("/api/v1")
    model_id = model.get("id")
    if not isinstance(model_id, str) or "/" not in model_id:
        raise ValueError(f"model id is not usable for endpoint lookup: {model_id!r}")
    author, slug = model_id.split("/", 1)
    return f"/models/{urllib.parse.quote(author, safe='')}/{urllib.parse.quote(slug, safe='')}/endpoints"


def extract_models(body: Any) -> list[dict[str, Any]]:
    data = body.get("data") if isinstance(body, dict) else None
    if not isinstance(data, list):
        raise OpenRouterError("/models response did not contain a data list")
    return [item for item in data if isinstance(item, dict)]


def extract_endpoint_payload(body: Any) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    data = body.get("data") if isinstance(body, dict) else None
    if isinstance(data, dict):
        endpoints = data.get("endpoints")
        if not isinstance(endpoints, list):
            endpoints = []
        return data, [endpoint for endpoint in endpoints if isinstance(endpoint, dict)]
    endpoints = body.get("endpoints") if isinstance(body, dict) else None
    if isinstance(endpoints, list):
        return body, [endpoint for endpoint in endpoints if isinstance(endpoint, dict)]
    if isinstance(data, list):
        return body if isinstance(body, dict) else {}, [endpoint for endpoint in data if isinstance(endpoint, dict)]
    raise OpenRouterError("endpoint response did not contain endpoints")


def select_models(models: list[dict[str, Any]], sample_models: int | None, sample_seed: int) -> list[dict[str, Any]]:
    ordered = sorted(models, key=lambda item: str(item.get("id", "")))
    if sample_models is None:
        return ordered
    if sample_models < 1:
        raise ValueError("--sample-models must be positive")
    if sample_models >= len(ordered):
        return ordered
    rng = random.Random(sample_seed)
    return sorted(rng.sample(ordered, sample_models), key=lambda item: str(item.get("id", "")))


def json_for_cell(value: Any) -> str:
    if value is None or isinstance(value, (str, int, float, bool)):
        return value  # type: ignore[return-value]
    return json.dumps(value, ensure_ascii=False, sort_keys=True)


def architecture_field(model: dict[str, Any], key: str) -> Any:
    architecture = model.get("architecture")
    if isinstance(architecture, dict):
        return architecture.get(key)
    return None


def make_endpoint_variant_id(snapshot_id: str, model_id: str, endpoint_index: int, endpoint: dict[str, Any]) -> str:
    basis = {
        "snapshot_id": snapshot_id,
        "model_id": model_id,
        "endpoint_index": endpoint_index,
        "provider_name": endpoint.get("provider_name"),
        "tag": endpoint.get("tag"),
        "name": endpoint.get("name"),
        "quantization": endpoint.get("quantization"),
        "endpoint_sha256": sha256_json(endpoint),
    }
    return hashlib.sha256(canonical_json(basis).encode("utf-8")).hexdigest()[:24]


def endpoint_variant_key(snapshot_id: str, model_id: str, endpoint_index: int, endpoint: dict[str, Any]) -> str:
    parts = [
        snapshot_id,
        model_id,
        str(endpoint_index),
        str(endpoint.get("provider_name") or ""),
        str(endpoint.get("tag") or ""),
        str(endpoint.get("name") or ""),
        str(endpoint.get("quantization") or "unknown"),
    ]
    return " | ".join(parts)


def normalized_row(
    *,
    snapshot_id: str,
    snapshot_timestamp: str,
    model: dict[str, Any],
    model_raw_path: str,
    endpoint_raw_path: str,
    endpoint_index: int,
    endpoint: dict[str, Any],
) -> dict[str, Any]:
    pricing = endpoint.get("pricing") if isinstance(endpoint.get("pricing"), dict) else {}
    model_id = str(model.get("id") or endpoint.get("model_id") or "")
    quantization = endpoint.get("quantization") or "unknown"
    return {
        "catalog_snapshot_id": snapshot_id,
        "snapshot_timestamp_utc": snapshot_timestamp,
        "openrouter_model_id": model_id,
        "canonical_slug": model.get("canonical_slug"),
        "model_name": model.get("name"),
        "model_created": model.get("created"),
        "model_description": model.get("description"),
        "hugging_face_id": model.get("hugging_face_id"),
        "knowledge_cutoff": model.get("knowledge_cutoff"),
        "modality": architecture_field(model, "modality"),
        "input_modalities": architecture_field(model, "input_modalities"),
        "output_modalities": architecture_field(model, "output_modalities"),
        "tokenizer": architecture_field(model, "tokenizer"),
        "instruct_type": architecture_field(model, "instruct_type"),
        "model_context_length": model.get("context_length"),
        "model_supported_parameters": model.get("supported_parameters"),
        "model_default_parameters": model.get("default_parameters"),
        "model_pricing": model.get("pricing"),
        "model_top_provider": model.get("top_provider"),
        "model_per_request_limits": model.get("per_request_limits"),
        "model_supported_voices": model.get("supported_voices"),
        "model_raw_path": model_raw_path,
        "raw_model_sha256": sha256_json(model),
        "endpoint_index": endpoint_index,
        "endpoint_variant_id": make_endpoint_variant_id(snapshot_id, model_id, endpoint_index, endpoint),
        "endpoint_variant_key": endpoint_variant_key(snapshot_id, model_id, endpoint_index, endpoint),
        "provider_name": endpoint.get("provider_name"),
        "endpoint_name": endpoint.get("name"),
        "endpoint_tag": endpoint.get("tag"),
        "endpoint_id": endpoint.get("endpoint_id") or endpoint.get("id"),
        "endpoint_model_id": endpoint.get("model_id"),
        "endpoint_model_name": endpoint.get("model_name"),
        "endpoint_model_permaslug": endpoint.get("model_permaslug") or endpoint.get("model_perma_slug"),
        "quantization": quantization,
        "unknown_quantization_endpoint_variant": str(quantization).lower() == "unknown",
        "context_length": endpoint.get("context_length"),
        "max_prompt_tokens": endpoint.get("max_prompt_tokens"),
        "max_completion_tokens": endpoint.get("max_completion_tokens"),
        "supported_parameters": endpoint.get("supported_parameters"),
        "supports_implicit_caching": endpoint.get("supports_implicit_caching"),
        "pricing_prompt": pricing.get("prompt"),
        "pricing_completion": pricing.get("completion"),
        "pricing_input_cache_read": pricing.get("input_cache_read"),
        "pricing_input_cache_write": pricing.get("input_cache_write"),
        "pricing_discount": pricing.get("discount"),
        "endpoint_pricing": endpoint.get("pricing"),
        "status": endpoint.get("status"),
        "uptime_last_5m": endpoint.get("uptime_last_5m"),
        "uptime_last_30m": endpoint.get("uptime_last_30m"),
        "uptime_last_1d": endpoint.get("uptime_last_1d"),
        "latency_last_30m": endpoint.get("latency_last_30m"),
        "throughput_last_30m": endpoint.get("throughput_last_30m"),
        "endpoint_raw_path": endpoint_raw_path,
        "raw_endpoint_sha256": sha256_json(endpoint),
    }


def write_jsonl(path: Path, rows: list[dict[str, Any]]) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")


def write_csv(path: Path, rows: list[dict[str, Any]]) -> None:
    with path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=CSV_FIELDS, extrasaction="ignore")
        writer.writeheader()
        for row in rows:
            writer.writerow({field: json_for_cell(row.get(field)) for field in CSV_FIELDS})


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Fetch OpenRouter model endpoint inventory.")
    parser.add_argument("--sample-models", type=int, default=None, help="Sample N models but include all endpoint variants for each sampled model.")
    parser.add_argument("--sample-seed", type=int, default=0, help="Deterministic sampling seed. Default: 0.")
    parser.add_argument("--out-root", type=Path, default=ROOT / "results", help="Output root. Default: model-pool/results.")
    parser.add_argument("--run-id", default=None, help="Output run id. Default: model-inventory-<UTC timestamp>.")
    parser.add_argument("--request-timeout", type=int, default=60, help="HTTP request timeout seconds. Default: 60.")
    parser.add_argument("--retries", type=int, default=2, help="Retry count for transient HTTP errors. Default: 2.")
    parser.add_argument("--sleep", type=float, default=0.0, help="Optional sleep between endpoint requests.")
    parser.add_argument("--model-id", action="append", default=None, help="Restrict to a specific OpenRouter model id. May be repeated.")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    key = load_openrouter_key()
    if not key:
        raise SystemExit("OPENROUTER_API_KEY not found in environment or secrets/openrouter.api.txt")

    started = dt.datetime.now(dt.UTC)
    snapshot_timestamp = started.isoformat().replace("+00:00", "Z")
    run_id = args.run_id or f"model-inventory-{started.strftime('%Y%m%dT%H%M%SZ')}"
    out_dir = args.out_root / run_id
    raw_dir = out_dir / "raw"
    endpoint_dir = raw_dir / "endpoints"
    endpoint_dir.mkdir(parents=True, exist_ok=False)

    catalog_body = api_get("/models", key, args.request_timeout, args.retries)
    (raw_dir / "models.json").write_text(pretty_json(catalog_body), encoding="utf-8")
    models = extract_models(catalog_body)
    if args.model_id:
        wanted = set(args.model_id)
        selected = [model for model in sorted(models, key=lambda item: str(item.get("id", ""))) if model.get("id") in wanted]
        missing = sorted(wanted - {str(model.get("id")) for model in selected})
        if missing:
            raise SystemExit(f"requested model ids not present in catalog: {', '.join(missing)}")
    else:
        selected = select_models(models, args.sample_models, args.sample_seed)

    rows: list[dict[str, Any]] = []
    endpoint_fetches: list[dict[str, Any]] = []
    errors: list[dict[str, str]] = []
    models_by_id = {str(model.get("id")): model for model in models}

    for model_number, model in enumerate(selected, start=1):
        model_id = str(model.get("id") or "")
        try:
            endpoint_path = endpoint_path_for_model(model)
            endpoint_body = api_get(endpoint_path, key, args.request_timeout, args.retries)
            endpoint_payload, endpoints = extract_endpoint_payload(endpoint_body)
        except Exception as exc:  # The summary records failures without leaking credentials.
            errors.append({"model_id": model_id, "error": str(exc)})
            continue

        endpoint_file = endpoint_dir / endpoint_raw_filename(model_id)
        endpoint_file.write_text(pretty_json(endpoint_body), encoding="utf-8")
        endpoint_rel = endpoint_file.relative_to(out_dir).as_posix()
        endpoint_fetches.append({"model_id": model_id, "endpoint_count": len(endpoints), "endpoint_path": endpoint_path, "raw_path": endpoint_rel})

        endpoint_model = endpoint_payload if endpoint_payload.get("id") else model
        model_for_rows = models_by_id.get(str(endpoint_model.get("id")), model)
        for endpoint_index, endpoint in enumerate(endpoints):
            rows.append(
                normalized_row(
                    snapshot_id=run_id,
                    snapshot_timestamp=snapshot_timestamp,
                    model=model_for_rows,
                    model_raw_path="raw/models.json",
                    endpoint_raw_path=endpoint_rel,
                    endpoint_index=endpoint_index,
                    endpoint=endpoint,
                )
            )
        if args.sleep and model_number < len(selected):
            time.sleep(args.sleep)

    jsonl_path = out_dir / "endpoint_variants.jsonl"
    csv_path = out_dir / "endpoint_variants.csv"
    write_jsonl(jsonl_path, rows)
    write_csv(csv_path, rows)

    provider_counts: dict[str, int] = {}
    quantization_counts: dict[str, int] = {}
    status_counts: dict[str, int] = {}
    for row in rows:
        provider_counts[str(row.get("provider_name") or "unknown")] = provider_counts.get(str(row.get("provider_name") or "unknown"), 0) + 1
        quantization_counts[str(row.get("quantization") or "unknown")] = quantization_counts.get(str(row.get("quantization") or "unknown"), 0) + 1
        status_counts[str(row.get("status") if row.get("status") is not None else "missing")] = status_counts.get(str(row.get("status") if row.get("status") is not None else "missing"), 0) + 1

    summary = {
        "inventory_run_id": run_id,
        "started_at_utc": snapshot_timestamp,
        "completed_at_utc": dt.datetime.now(dt.UTC).isoformat().replace("+00:00", "Z"),
        "catalog_model_count": len(models),
        "selected_model_count": len(selected),
        "sample_models": args.sample_models,
        "sample_seed": args.sample_seed if args.sample_models is not None else None,
        "endpoint_variant_count": len(rows),
        "endpoint_fetch_count": len(endpoint_fetches),
        "endpoint_fetch_error_count": len(errors),
        "endpoint_fetch_errors": errors,
        "model_endpoint_fetches": endpoint_fetches,
        "selected_model_ids": [str(model.get("id")) for model in selected],
        "provider_counts": dict(sorted(provider_counts.items())),
        "quantization_counts": dict(sorted(quantization_counts.items())),
        "status_counts": dict(sorted(status_counts.items())),
        "unknown_quantization_endpoint_variant_count": quantization_counts.get("unknown", 0),
        "output_files": [
            "raw/models.json",
            "raw/endpoints/*.json",
            "endpoint_variants.jsonl",
            "endpoint_variants.csv",
            "summary.json",
            "summary.md",
        ],
        "notes": [
            "This run used only OpenRouter catalog and endpoint APIs. It did not run inference probes.",
            "Rows are endpoint variants. Unknown quantization is endpoint-specific and rows are not collapsed by quantization label.",
        ],
    }
    (out_dir / "summary.json").write_text(pretty_json(summary), encoding="utf-8")
    summary_md = [
        "# OpenRouter model inventory summary",
        "",
        f"- Run id: `{run_id}`",
        f"- Started: {snapshot_timestamp}",
        f"- Catalog models: {len(models)}",
        f"- Selected models: {len(selected)}",
        f"- Endpoint variants: {len(rows)}",
        f"- Endpoint fetch errors: {len(errors)}",
        f"- Unknown-quantization endpoint variants: {summary['unknown_quantization_endpoint_variant_count']}",
        "",
        "## Selected models",
        "",
    ]
    summary_md.extend(f"- `{model.get('id')}`" for model in selected)
    summary_md.extend(["", "## Quantization counts", ""])
    summary_md.extend(f"- `{key}`: {value}" for key, value in sorted(quantization_counts.items()))
    summary_md.extend(["", "## Provider counts", ""])
    summary_md.extend(f"- `{key}`: {value}" for key, value in sorted(provider_counts.items()))
    if errors:
        summary_md.extend(["", "## Endpoint fetch errors", ""])
        summary_md.extend(f"- `{item['model_id']}`: {item['error']}" for item in errors)
    summary_md.extend([
        "",
        "## Files",
        "",
        "- `endpoint_variants.jsonl`",
        "- `endpoint_variants.csv`",
        "- `summary.json`",
        "- `raw/models.json`",
        "- `raw/endpoints/*.json`",
        "",
    ])
    (out_dir / "summary.md").write_text("\n".join(summary_md), encoding="utf-8")

    print(json.dumps({"run_id": run_id, "out_dir": str(out_dir), "selected_model_count": len(selected), "endpoint_variant_count": len(rows), "endpoint_fetch_error_count": len(errors)}, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
