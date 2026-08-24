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
import math
import os
import random
import re
import stat
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from run_record import strict_json_loads

ROOT = Path(__file__).resolve().parents[1]
API_BASE = "https://openrouter.ai/api/v1"
INVENTORY_FORMAT_VERSION = 1


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


CompletedEndpoint = tuple[dict[str, Any], dict[str, Any], list[dict[str, Any]], bytes, bool]


@dataclass(frozen=True)
class InventoryResumeState:
    manifest: dict[str, Any]
    catalog_body: dict[str, Any]
    models: list[dict[str, Any]]
    selected: list[dict[str, Any]]
    selected_records: list[dict[str, str]]
    completed: dict[str, CompletedEndpoint]
    partial_paths: tuple[Path, ...]


def canonical_json(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False, allow_nan=False, sort_keys=True, separators=(",", ":"))


def pretty_json(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False, allow_nan=False, sort_keys=True, indent=2) + "\n"


def sha256_json(obj: Any) -> str:
    return hashlib.sha256(canonical_json(obj).encode("utf-8")).hexdigest()


def atomic_write(path: Path, data: bytes) -> None:
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_bytes(data)
    os.replace(temporary, path)


def safe_name(value: str) -> str:
    cleaned = re.sub(r"[^A-Za-z0-9._-]+", "_", value.strip())
    return cleaned.strip("_") or "unnamed"


def endpoint_raw_filename(model_id: str) -> str:
    digest = hashlib.sha256(model_id.encode("utf-8")).hexdigest()[:12]
    return f"{safe_name(model_id)}-{digest}.json"


def selection_parameters(args: argparse.Namespace) -> dict[str, Any]:
    if args.model_id:
        return {
            "model_ids": sorted(set(args.model_id)),
            "sample_models": None,
            "sample_seed": None,
        }
    return {
        "model_ids": None,
        "sample_models": args.sample_models,
        "sample_seed": args.sample_seed if args.sample_models is not None else None,
    }


def validate_selection_parameters(parameters: dict[str, Any]) -> None:
    expected_fields = {"model_ids", "sample_models", "sample_seed"}
    if set(parameters) != expected_fields:
        missing = sorted(expected_fields - set(parameters))
        extra = sorted(set(parameters) - expected_fields)
        raise ValueError(f"selection parameters differ: missing={missing}, unexpected={extra}")
    model_ids = parameters["model_ids"]
    sample_models = parameters["sample_models"]
    sample_seed = parameters["sample_seed"]
    if model_ids is not None:
        if (
            not isinstance(model_ids, list)
            or not model_ids
            or any(not isinstance(model_id, str) or not model_id.strip() for model_id in model_ids)
            or model_ids != sorted(set(model_ids))
        ):
            raise ValueError("selection model_ids must be a nonempty sorted array of unique nonempty strings")
        if sample_models is not None or sample_seed is not None:
            raise ValueError("explicit model_ids cannot be combined with sampled selection")
        return
    if sample_models is None:
        if sample_seed is not None:
            raise ValueError("sample_seed requires sample_models")
        return
    if not isinstance(sample_models, int) or isinstance(sample_models, bool) or sample_models < 1:
        raise ValueError("sample_models must be a positive integer")
    if not isinstance(sample_seed, int) or isinstance(sample_seed, bool):
        raise ValueError("sample_seed must be an integer when sample_models is set")


def select_models_for_parameters(
    models: list[dict[str, Any]],
    parameters: dict[str, Any],
) -> list[dict[str, Any]]:
    validate_selection_parameters(parameters)
    model_ids = parameters["model_ids"]
    if model_ids is not None:
        wanted = set(model_ids)
        selected = [model for model in sorted(models, key=lambda item: str(item["id"])) if model["id"] in wanted]
        missing = sorted(wanted - {str(model["id"]) for model in selected})
        if missing:
            raise ValueError(f"requested model ids not present in catalog: {', '.join(missing)}")
        return selected
    return select_models(models, parameters["sample_models"], parameters["sample_seed"] or 0)


def validate_model_ids(models: list[dict[str, Any]]) -> None:
    seen: set[str] = set()
    for index, model in enumerate(models, start=1):
        model_id = model.get("id")
        if not isinstance(model_id, str) or not model_id.strip():
            raise ValueError(f"catalog model {index} has no nonempty string id")
        if model_id in seen:
            raise ValueError(f"catalog contains duplicate model id {model_id!r}")
        seen.add(model_id)


def select_requested_models(models: list[dict[str, Any]], args: argparse.Namespace) -> list[dict[str, Any]]:
    return select_models_for_parameters(models, selection_parameters(args))


def selected_model_records(selected: list[dict[str, Any]]) -> list[dict[str, str]]:
    records: list[dict[str, str]] = []
    for model in selected:
        model_id = str(model["id"])
        filename = endpoint_raw_filename(model_id)
        records.append(
            {
                "model_id": model_id,
                "endpoint_path": endpoint_path_for_model(model),
                "snapshot_path": f"inputs/endpoints/{filename}",
                "raw_path": f"raw/endpoints/{filename}",
            }
        )
    return records


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
                raw = response.read()
            try:
                return json.loads(raw.decode("utf-8"))
            except (UnicodeDecodeError, json.JSONDecodeError) as exc:
                raise OpenRouterError(f"OpenRouter returned invalid JSON for {url}") from exc
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
    if not all(isinstance(item, dict) for item in data):
        raise OpenRouterError("/models response data contained a non-object model")
    return data


def extract_endpoint_payload(body: Any) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    data = body.get("data") if isinstance(body, dict) else None
    if isinstance(data, dict):
        endpoints = data.get("endpoints")
        if not isinstance(endpoints, list):
            raise OpenRouterError("endpoint response data did not contain an endpoints list")
        if not all(isinstance(endpoint, dict) for endpoint in endpoints):
            raise OpenRouterError("endpoint response contained a non-object endpoint")
        return data, endpoints
    endpoints = body.get("endpoints") if isinstance(body, dict) else None
    if isinstance(endpoints, list):
        if not all(isinstance(endpoint, dict) for endpoint in endpoints):
            raise OpenRouterError("endpoint response contained a non-object endpoint")
        return body, endpoints
    if isinstance(data, list):
        if not all(isinstance(endpoint, dict) for endpoint in data):
            raise OpenRouterError("endpoint response contained a non-object endpoint")
        return body if isinstance(body, dict) else {}, data
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


def read_json_object(path: Path) -> dict[str, Any]:
    try:
        value = strict_json_loads(path.read_bytes(), path)
    except FileNotFoundError as exc:
        raise ValueError(f"required inventory file does not exist: {path}") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{path}: expected a JSON object")
    return value


def inventory_manifest(
    *,
    run_id: str,
    out_dir: Path,
    created_at: str,
    parameters: dict[str, Any],
    selected_records: list[dict[str, str]],
) -> dict[str, Any]:
    return {
        "format_version": INVENTORY_FORMAT_VERSION,
        "kind": "model-pool-inventory",
        "created_at": created_at,
        "run_id": run_id,
        "run_dir": str(out_dir),
        "selection_parameters": parameters,
        "catalog": {
            "snapshot_path": "inputs/models.json",
            "raw_path": "raw/models.json",
        },
        "selected_models": selected_records,
    }


def load_inventory_manifest(
    manifest_path: Path,
    *,
    run_id: str,
    out_dir: Path,
    parameters: dict[str, Any],
) -> dict[str, Any]:
    manifest = read_json_object(manifest_path)
    expected_keys = {
        "format_version",
        "kind",
        "created_at",
        "run_id",
        "run_dir",
        "selection_parameters",
        "catalog",
        "selected_models",
    }
    if set(manifest) != expected_keys:
        missing = sorted(expected_keys - set(manifest))
        extra = sorted(set(manifest) - expected_keys)
        raise ValueError(f"{manifest_path}: manifest fields differ: missing={missing}, unexpected={extra}")
    expected_scalars = {
        "format_version": INVENTORY_FORMAT_VERSION,
        "kind": "model-pool-inventory",
        "run_id": run_id,
        "run_dir": str(out_dir),
        "selection_parameters": parameters,
    }
    for key, expected in expected_scalars.items():
        if manifest.get(key) != expected:
            raise ValueError(f"{manifest_path}: {key}={manifest.get(key)!r}; expected {expected!r}")
    if not isinstance(manifest.get("created_at"), str) or not manifest["created_at"]:
        raise ValueError(f"{manifest_path}: created_at must be a nonempty string")
    catalog = manifest.get("catalog")
    if not isinstance(catalog, dict) or set(catalog) != {"snapshot_path", "raw_path"}:
        raise ValueError(f"{manifest_path}: catalog must contain exactly snapshot_path and raw_path")
    if catalog != {"snapshot_path": "inputs/models.json", "raw_path": "raw/models.json"}:
        raise ValueError(f"{manifest_path}: invalid catalog record")
    selected = manifest.get("selected_models")
    if not isinstance(selected, list) or not selected:
        raise ValueError(f"{manifest_path}: selected_models must be a nonempty array")
    return manifest


def validate_endpoint_payload_identity(
    selection: dict[str, str],
    payload: dict[str, Any],
    source: Path | str,
) -> None:
    payload_id = payload.get("id")
    if payload_id is not None and payload_id != selection["model_id"]:
        raise ValueError(
            f"endpoint payload id is {payload_id!r}; expected {selection['model_id']!r}: {source}"
        )


def load_completed_endpoint(
    out_dir: Path,
    selection: dict[str, str],
) -> CompletedEndpoint | None:
    snapshot_path = out_dir / selection["snapshot_path"]
    raw_path = out_dir / selection["raw_path"]
    try:
        snapshot_metadata = snapshot_path.lstat()
    except FileNotFoundError:
        return None
    if not stat.S_ISREG(snapshot_metadata.st_mode):
        raise ValueError(f"recorded endpoint snapshot is not a regular file: {snapshot_path}")
    snapshot_bytes = snapshot_path.read_bytes()
    try:
        raw_metadata = raw_path.lstat()
    except FileNotFoundError:
        raw_missing = True
    else:
        raw_missing = False
        if not stat.S_ISREG(raw_metadata.st_mode):
            raise ValueError(f"raw endpoint artifact is not a regular file: {raw_path}")
    if not raw_missing and raw_path.read_bytes() != snapshot_bytes:
        raise ValueError(f"raw endpoint file differs from its saved snapshot: {raw_path}")
    body = strict_json_loads(snapshot_bytes, snapshot_path)
    payload, endpoints = extract_endpoint_payload(body)
    validate_endpoint_payload_identity(selection, payload, snapshot_path)
    return body, payload, endpoints, snapshot_bytes, raw_missing


def _regular_file_bytes(path: Path, label: str) -> bytes:
    try:
        metadata = path.lstat()
    except FileNotFoundError as exc:
        raise ValueError(f"{label} does not exist: {path}") from exc
    if not stat.S_ISREG(metadata.st_mode):
        raise ValueError(f"{label} is not a regular file: {path}")
    return path.read_bytes()


def _validate_endpoint_directory(
    directory: Path,
    expected_names: set[str],
) -> list[Path]:
    try:
        metadata = directory.lstat()
    except FileNotFoundError as exc:
        raise ValueError(f"inventory endpoint data directory does not exist: {directory}") from exc
    if not stat.S_ISDIR(metadata.st_mode):
        raise ValueError(f"inventory endpoint data path is not a directory: {directory}")
    partial_paths: list[Path] = []
    for path in directory.iterdir():
        try:
            entry_metadata = path.lstat()
        except FileNotFoundError as exc:
            raise ValueError(f"inventory endpoint artifact disappeared during validation: {path}") from exc
        if not stat.S_ISREG(entry_metadata.st_mode):
            raise ValueError(f"unexpected inventory endpoint artifact: {path}")
        if path.name.endswith(".tmp") and path.name[:-4] in expected_names:
            partial_paths.append(path)
        elif path.name not in expected_names:
            raise ValueError(f"unexpected inventory endpoint artifact: {path}")
    return partial_paths


def validate_inventory_resume(
    out_dir: Path,
    *,
    run_id: str,
    parameters: dict[str, Any],
) -> InventoryResumeState:
    out_dir = out_dir.expanduser().resolve()
    if not out_dir.is_dir():
        raise ValueError(f"resume directory does not exist: {out_dir}")
    validate_selection_parameters(parameters)
    manifest_path = out_dir / "manifest.json"
    manifest = load_inventory_manifest(
        manifest_path,
        run_id=run_id,
        out_dir=out_dir,
        parameters=parameters,
    )
    catalog_snapshot_path = out_dir / manifest["catalog"]["snapshot_path"]
    catalog_raw_path = out_dir / manifest["catalog"]["raw_path"]
    catalog_bytes = _regular_file_bytes(catalog_snapshot_path, "recorded catalog snapshot")
    if _regular_file_bytes(catalog_raw_path, "raw catalog") != catalog_bytes:
        raise ValueError(f"raw catalog differs from its saved snapshot: {catalog_raw_path}")
    catalog_value = strict_json_loads(catalog_bytes, catalog_snapshot_path)
    if not isinstance(catalog_value, dict):
        raise ValueError(f"{catalog_snapshot_path}: expected a JSON object")
    models = extract_models(catalog_value)
    validate_model_ids(models)
    selected = select_models_for_parameters(models, parameters)
    if not selected:
        raise ValueError("catalog selection contains no models")
    selected_records = selected_model_records(selected)
    if manifest["selected_models"] != selected_records:
        raise ValueError(f"{manifest_path}: selected_models differs from the saved catalog selection")

    endpoint_dir = out_dir / "raw" / "endpoints"
    endpoint_snapshot_dir = out_dir / "inputs" / "endpoints"
    expected_raw_names = {Path(record["raw_path"]).name for record in selected_records}
    expected_snapshot_names = {Path(record["snapshot_path"]).name for record in selected_records}
    partial_paths = [
        *_validate_endpoint_directory(endpoint_dir, expected_raw_names),
        *_validate_endpoint_directory(endpoint_snapshot_dir, expected_snapshot_names),
    ]

    completed: dict[str, CompletedEndpoint] = {}
    for selection in selected_records:
        snapshot_path = out_dir / selection["snapshot_path"]
        raw_path = out_dir / selection["raw_path"]
        if raw_path.exists() and not snapshot_path.exists():
            raise ValueError(f"raw endpoint file has no saved snapshot: {raw_path}")
        saved = load_completed_endpoint(out_dir, selection)
        if saved is not None:
            completed[selection["model_id"]] = saved
    return InventoryResumeState(
        manifest=manifest,
        catalog_body=catalog_value,
        models=models,
        selected=selected,
        selected_records=selected_records,
        completed=completed,
        partial_paths=tuple(sorted(partial_paths)),
    )


def json_for_cell(value: Any) -> str:
    if value is None or isinstance(value, (str, int, float, bool)):
        return value  # type: ignore[return-value]
    return json.dumps(value, ensure_ascii=False, allow_nan=False, sort_keys=True)


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
            handle.write(json.dumps(row, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n")


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
    parser.add_argument("--resume", action="store_true", help="Continue an incomplete recorded inventory after validating saved catalog and endpoint records.")
    args = parser.parse_args(argv)
    if args.sample_models is not None and args.sample_models < 1:
        raise SystemExit("--sample-models must be positive")
    if args.request_timeout < 1:
        raise SystemExit("--request-timeout must be positive")
    if args.retries < 0:
        raise SystemExit("--retries cannot be negative")
    if not math.isfinite(args.sleep) or args.sleep < 0:
        raise SystemExit("--sleep must be a finite non-negative number")
    if args.model_id:
        args.model_id = sorted(set(args.model_id))
    return args


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    now = dt.datetime.now(dt.UTC)
    run_id = args.run_id or f"model-inventory-{now.strftime('%Y%m%dT%H%M%SZ')}"
    out_root = args.out_root.expanduser()
    if not out_root.is_absolute():
        out_root = ROOT / out_root
    out_dir = (out_root / run_id).resolve()
    input_dir = out_dir / "inputs"
    raw_dir = out_dir / "raw"
    endpoint_snapshot_dir = input_dir / "endpoints"
    endpoint_dir = raw_dir / "endpoints"
    manifest_path = out_dir / "manifest.json"
    parameters = selection_parameters(args)

    if args.resume:
        try:
            resume_state = validate_inventory_resume(
                out_dir,
                run_id=run_id,
                parameters=parameters,
            )
        except (OSError, ValueError, OpenRouterError) as exc:
            raise SystemExit(str(exc)) from exc
        manifest = resume_state.manifest
        catalog_body = resume_state.catalog_body
        models = resume_state.models
        selected = resume_state.selected
        selected_records = resume_state.selected_records
        completed = dict(resume_state.completed)
        partial_paths = list(resume_state.partial_paths)
        snapshot_timestamp = manifest["created_at"]
    else:
        if out_dir.exists() and any(out_dir.iterdir()):
            raise SystemExit(f"output directory is not empty: {out_dir}; use --resume for the recorded inventory")
        key = load_openrouter_key()
        if not key:
            raise SystemExit("OPENROUTER_API_KEY not found in environment or secrets/openrouter.api.txt")
        try:
            catalog_body = api_get("/models", key, args.request_timeout, args.retries)
            models = extract_models(catalog_body)
            validate_model_ids(models)
            selected = select_requested_models(models, args)
            if not selected:
                raise ValueError("catalog selection contains no models")
            selected_records = selected_model_records(selected)
            snapshot_timestamp = now.isoformat().replace("+00:00", "Z")
            catalog_bytes = pretty_json(catalog_body).encode("utf-8")
            manifest = inventory_manifest(
                run_id=run_id,
                out_dir=out_dir,
                created_at=snapshot_timestamp,
                parameters=parameters,
                selected_records=selected_records,
            )
        except (ValueError, OpenRouterError) as exc:
            raise SystemExit(str(exc)) from exc
        endpoint_dir.mkdir(parents=True, exist_ok=False)
        endpoint_snapshot_dir.mkdir(parents=True, exist_ok=False)
        atomic_write(input_dir / "models.json", catalog_bytes)
        atomic_write(raw_dir / "models.json", catalog_bytes)
        atomic_write(manifest_path, pretty_json(manifest).encode("utf-8"))
        partial_paths = []
        completed = {}

    for path in partial_paths:
        try:
            path.unlink()
        except FileNotFoundError:
            pass

    for selection in selected_records:
        saved = completed.get(selection["model_id"])
        if saved is not None and saved[4]:
            atomic_write(out_dir / selection["raw_path"], saved[3])

    missing_records = [record for record in selected_records if record["model_id"] not in completed]
    key: str | None = None
    if missing_records:
        key = load_openrouter_key()
        if not key:
            raise SystemExit("OPENROUTER_API_KEY not found in environment or secrets/openrouter.api.txt")

    errors: list[dict[str, str]] = []
    fresh_model_ids: set[str] = set()
    for missing_number, selection in enumerate(missing_records, start=1):
        model_id = selection["model_id"]
        try:
            assert key is not None
            endpoint_body = api_get(selection["endpoint_path"], key, args.request_timeout, args.retries)
            endpoint_payload, endpoints = extract_endpoint_payload(endpoint_body)
            validate_endpoint_payload_identity(selection, endpoint_payload, selection["endpoint_path"])
        except OpenRouterError as exc:
            errors.append({"model_id": model_id, "error": str(exc)})
        else:
            raw_bytes = pretty_json(endpoint_body).encode("utf-8")
            snapshot_path = out_dir / selection["snapshot_path"]
            raw_path = out_dir / selection["raw_path"]
            atomic_write(snapshot_path, raw_bytes)
            atomic_write(raw_path, raw_bytes)
            completed[model_id] = (endpoint_body, endpoint_payload, endpoints, raw_bytes, False)
            fresh_model_ids.add(model_id)
        if args.sleep and missing_number < len(missing_records):
            time.sleep(args.sleep)

    rows: list[dict[str, Any]] = []
    endpoint_fetches: list[dict[str, Any]] = []
    models_by_id = {str(model["id"]): model for model in models}
    selected_by_id = {str(model["id"]): model for model in selected}
    for selection in selected_records:
        model_id = selection["model_id"]
        saved = completed.get(model_id)
        if saved is None:
            continue
        _, endpoint_payload, endpoints, _, _ = saved
        endpoint_fetches.append(
            {
                "model_id": model_id,
                "endpoint_count": len(endpoints),
                "endpoint_path": selection["endpoint_path"],
                "raw_path": selection["raw_path"],
                "reused": model_id not in fresh_model_ids,
            }
        )
        model = selected_by_id[model_id]
        endpoint_model = endpoint_payload if endpoint_payload.get("id") else model
        model_for_rows = models_by_id.get(str(endpoint_model.get("id")), model)
        for endpoint_index, endpoint in enumerate(endpoints):
            rows.append(
                normalized_row(
                    snapshot_id=run_id,
                    snapshot_timestamp=snapshot_timestamp,
                    model=model_for_rows,
                    model_raw_path="raw/models.json",
                    endpoint_raw_path=selection["raw_path"],
                    endpoint_index=endpoint_index,
                    endpoint=endpoint,
                )
            )

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
        "sample_models": parameters["sample_models"],
        "sample_seed": parameters["sample_seed"],
        "endpoint_variant_count": len(rows),
        "endpoint_fetch_count": len(endpoint_fetches),
        "reused_endpoint_fetch_count": sum(1 for item in endpoint_fetches if item["reused"]),
        "new_endpoint_fetch_count": sum(1 for item in endpoint_fetches if not item["reused"]),
        "endpoint_fetch_error_count": len(errors),
        "endpoint_fetch_errors": errors,
        "model_endpoint_fetches": endpoint_fetches,
        "selected_model_ids": [str(model.get("id")) for model in selected],
        "provider_counts": dict(sorted(provider_counts.items())),
        "quantization_counts": dict(sorted(quantization_counts.items())),
        "status_counts": dict(sorted(status_counts.items())),
        "unknown_quantization_endpoint_variant_count": quantization_counts.get("unknown", 0),
        "complete": len(endpoint_fetches) == len(selected) and not errors and bool(rows),
        "failure": "no endpoint variants found" if not rows and not errors else None,
        "output_files": [
            "manifest.json",
            "inputs/models.json",
            "inputs/endpoints/*.json",
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
        "- `manifest.json`",
        "- `summary.json`",
        "- `inputs/models.json`",
        "- `inputs/endpoints/*.json`",
        "- `raw/models.json`",
        "- `raw/endpoints/*.json`",
        "",
    ])
    (out_dir / "summary.md").write_text("\n".join(summary_md), encoding="utf-8")

    print(
        json.dumps(
            {
                "run_id": run_id,
                "out_dir": str(out_dir),
                "selected_model_count": len(selected),
                "endpoint_variant_count": len(rows),
                "endpoint_fetch_error_count": len(errors),
                "complete": summary["complete"],
            },
            sort_keys=True,
        )
    )
    return 0 if summary["complete"] else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
