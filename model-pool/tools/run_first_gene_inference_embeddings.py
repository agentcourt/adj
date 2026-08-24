#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import copy
import datetime as dt
import http.client
import json
import math
import os
import re
import socket
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(Path(__file__).resolve().parent))

from run_eval import (  # noqa: E402
    OpenRouterHTTPError,
    attach_openrouter_error_meta,
    attach_request_spec_meta,
    classify_error,
    hydrate_posthoc_generation_metadata,
    load_openrouter_key,
    model_spec_from_object,
    openrouter_request,
    response_meta,
)
from run_record import (  # noqa: E402
    PENDING_FILE_PREFIX,
    atomic_write_bytes,
    exclusive_run_lock,
    fsync_directory,
    input_file,
    parse_jsonl_objects,
    prepare_run_record,
    strict_json_loads,
    validate_run_record_inputs,
)

REQUEST_PARAMETER_KEYS = ("temperature", "top_p", "max_tokens")


class CompletionServiceError(RuntimeError):
    pass


class EmbeddingServiceError(RuntimeError):
    pass


EXPECTED_COMPLETION_ERRORS = (
    OpenRouterHTTPError,
    CompletionServiceError,
    urllib.error.URLError,
    TimeoutError,
    socket.timeout,
    http.client.HTTPException,
    ConnectionError,
)


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def display_path(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def load_jsonl(path: Path) -> list[dict]:
    with path.open() as handle:
        return [json.loads(line) for line in handle if line.strip()]


def load_openai_key() -> str | None:
    if os.environ.get("OPENAI_API_KEY"):
        return os.environ["OPENAI_API_KEY"]
    candidates = [ROOT / "secrets" / "openai.api.txt"]
    patterns = [
        re.compile(r"^\s*export\s+OPENAI_API_KEY\s*=\s*['\"]?([^'\"\s]+)", re.M),
        re.compile(r"^\s*OPENAI_API_KEY\s*[:=]\s*['\"]?([^'\"\s]+)", re.M),
        re.compile(r"^\s*openai[^:=]*[:=]\s*['\"]?([^'\"\s]+)", re.I | re.M),
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


def provider_request_from_variant(row: dict) -> dict:
    provider = {
        "only": [row["endpoint_tag"]],
        "allow_fallbacks": False,
        "require_parameters": True,
    }
    quantization = str(row.get("quantization") or "").lower()
    if quantization and quantization != "unknown":
        provider["quantizations"] = [quantization]
    return provider


def request_params_from_variant(row: dict, requested: dict) -> tuple[dict, dict]:
    supported = row.get("supported_parameters")
    if not isinstance(supported, list):
        return dict(requested), {}
    supported_set = {str(value).strip() for value in supported if str(value).strip()}
    effective = {key: value for key, value in requested.items() if key in supported_set}
    omitted = {key: value for key, value in requested.items() if key not in supported_set}
    return effective, omitted


def spec_from_variant(row: dict) -> dict:
    obj = dict(row)
    obj["provider"] = provider_request_from_variant(row)
    obj["headers"] = {"X-OpenRouter-Experimental-Metadata": "enabled"}
    return model_spec_from_object(obj, f"variants/filtered-20260529/endpoint_variants.jsonl:{row.get('combined_index')}")


def chat_payload(spec: dict, persona: str, gene: str, request_params: dict) -> dict:
    payload = {
        "model": spec["openrouter_model_id"],
        "messages": [
            {"role": "system", "content": persona},
            {"role": "user", "content": gene},
        ],
    }
    for key in REQUEST_PARAMETER_KEYS:
        if key in request_params:
            payload[key] = request_params[key]
    if spec.get("provider"):
        payload["provider"] = spec["provider"]
    return payload


def openrouter_completion_once(spec: dict, persona: str, gene: str, request_params: dict, timeout: int) -> tuple[str, dict]:
    payload = chat_payload(spec, persona, gene, request_params)
    started = time.time()
    body = openrouter_request(payload, timeout, spec.get("headers"))
    try:
        text, meta, _ = response_meta(body, started)
    except (KeyError, TypeError, RuntimeError) as exc:
        raise CompletionServiceError(str(exc)) from exc
    if not isinstance(text, str) or not text.strip():
        raise CompletionServiceError("OpenRouter completion response contained no text")
    meta = attach_request_spec_meta(meta, {**spec, "request": request_params}, timeout)
    return text, meta


def retry_after_seconds(exc: Exception) -> float | None:
    if not isinstance(exc, OpenRouterHTTPError):
        return None
    body = exc.body_json
    if not isinstance(body, dict):
        return None
    error = body.get("error")
    if not isinstance(error, dict):
        return None
    metadata = error.get("metadata")
    if not isinstance(metadata, dict):
        return None
    value = metadata.get("retry_after_seconds")
    if isinstance(value, (int, float)) and value >= 0:
        return float(value)
    return None


def retryable_completion_error(exc: Exception) -> bool:
    error_type = classify_error(exc)
    if error_type in {"rate_limit", "timeout"}:
        return True
    if isinstance(exc, (http.client.IncompleteRead, http.client.RemoteDisconnected, urllib.error.URLError, socket.timeout)):
        return True
    text = str(exc).lower()
    transient_fragments = [
        "incompleteread",
        "remote end closed",
        "connection reset",
        "temporarily unavailable",
        "service unavailable",
        "bad gateway",
        "gateway timeout",
    ]
    return any(fragment in text for fragment in transient_fragments)


def openrouter_completion(
    spec: dict,
    persona: str,
    gene: str,
    request_params: dict,
    timeout: int,
    attempts: int,
    retry_sleep: float,
) -> tuple[str, dict]:
    errors: list[dict] = []
    for attempt in range(1, attempts + 1):
        try:
            text, metadata = openrouter_completion_once(spec, persona, gene, request_params, timeout)
        except EXPECTED_COMPLETION_ERRORS as exc:
            errors.append(
                {
                    "attempt": attempt,
                    "error_type": classify_error(exc),
                    "error_message": str(exc),
                }
            )
            if attempt >= attempts or not retryable_completion_error(exc):
                raise
            sleep_seconds = retry_after_seconds(exc)
            if sleep_seconds is None:
                sleep_seconds = retry_sleep
            time.sleep(max(0.0, sleep_seconds))
            continue
        metadata["completion_attempt_count"] = attempt
        if errors:
            metadata["completion_retry_errors"] = errors
        return text, metadata
    raise RuntimeError("completion retry loop exited without a result")


def embedding_request(text: str, model: str, timeout: int) -> tuple[list[float], dict]:
    key = load_openai_key()
    if not key:
        raise RuntimeError("OPENAI_API_KEY not found in environment or secrets/openai.api.txt")
    body = json.dumps({"model": model, "input": text}).encode()
    request = urllib.request.Request(
        "https://api.openai.com/v1/embeddings",
        data=body,
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        method="POST",
    )
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            raw = json.loads(response.read().decode())
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode(errors="replace")[:2000]
        raise EmbeddingServiceError(f"OpenAI embeddings HTTP {exc.code}: {detail}") from exc
    except (urllib.error.URLError, TimeoutError, socket.timeout, http.client.HTTPException, json.JSONDecodeError) as exc:
        raise EmbeddingServiceError(f"OpenAI embeddings request failed: {exc}") from exc
    data = raw.get("data")
    if not isinstance(data, list) or not data or not isinstance(data[0].get("embedding"), list):
        raise EmbeddingServiceError("OpenAI embeddings response missing embedding vector")
    meta = {
        "embedding_model": raw.get("model", model),
        "embedding_usage": raw.get("usage"),
        "embedding_elapsed_ms": round((time.time() - started) * 1000),
        "raw_embedding_response": {k: v for k, v in raw.items() if k != "data"},
    }
    return data[0]["embedding"], meta


def error_row(base: dict, status: str, exc: Exception) -> dict:
    meta = {
        "error_type": classify_error(exc),
        "error_message": str(exc),
    }
    attach_openrouter_error_meta(meta, exc)
    return {**base, "status": status, "response_text": "", "embedding": None, "metadata": meta}


def record_key(row: dict) -> tuple[int, int]:
    variant_order = row.get("variant_order")
    sample_index = row.get("sample_index")
    if not isinstance(variant_order, int) or isinstance(variant_order, bool):
        raise ValueError("record variant_order must be an integer")
    if not isinstance(sample_index, int) or isinstance(sample_index, bool):
        raise ValueError("record sample_index must be an integer")
    return variant_order, sample_index


def reusable_record(row: dict | None) -> bool:
    if not isinstance(row, dict):
        return False
    return row.get("status") == "ok" and isinstance(row.get("embedding"), list)


def embedding_retry_record(row: dict | None) -> bool:
    return isinstance(row, dict) and row.get("status") == "embedding_error"


def resolve_path(value: str) -> Path:
    path = Path(value).expanduser()
    return path if path.is_absolute() else ROOT / path


def run_parameters(args: argparse.Namespace) -> dict[str, Any]:
    return {
        "samples": args.samples,
        "gene_index": args.gene_index,
        "timeout": args.timeout,
        "embedding_model": args.embedding_model,
        "temperature": args.temperature,
        "top_p": args.top_p,
        "max_tokens": args.max_tokens,
        "completion_attempts": args.completion_attempts,
        "retry_sleep": args.retry_sleep,
    }


def run_inputs(args: argparse.Namespace) -> list[Any]:
    persona_path = resolve_path(args.persona)
    suffix = persona_path.suffix or ".txt"
    return [
        input_file("variants", resolve_path(args.variants), "inputs/variants.jsonl"),
        input_file("genes", resolve_path(args.genes), "inputs/genes.json"),
        input_file("persona", persona_path, f"inputs/persona{suffix}"),
    ]


def saved_input_paths(out: Path, inputs: list[Any]) -> tuple[Path, Path, Path]:
    return tuple(out / item.snapshot_path for item in inputs)  # type: ignore[return-value]


def load_context(args: argparse.Namespace, out: Path, inputs: list[Any]) -> tuple[list[dict], str, str, Path, list[dict]]:
    variants_path, genes_path, persona_path = saved_input_paths(out, inputs)
    variants = parse_jsonl_objects(variants_path.read_bytes(), variants_path)
    genes_value = strict_json_loads(genes_path.read_bytes(), genes_path)
    if not isinstance(genes_value, list) or not genes_value:
        raise ValueError(f"{genes_path}: expected a nonempty JSON array")
    if args.gene_index < 0 or args.gene_index >= len(genes_value):
        raise ValueError(f"--gene-index {args.gene_index} is outside sampled gene range 0..{len(genes_value) - 1}")
    gene = genes_value[args.gene_index]
    if not isinstance(gene, str) or not gene.strip():
        raise ValueError(f"{genes_path}: selected gene is not a nonempty string")
    persona = persona_path.read_text().strip()
    if not persona:
        raise ValueError(f"{persona_path}: persona is empty")
    if not variants:
        raise ValueError(f"{variants_path}: no variants found")
    requested = {"temperature": args.temperature, "top_p": args.top_p, "max_tokens": args.max_tokens}
    bases: list[dict] = []
    for variant_order, variant in enumerate(variants, start=1):
        request_parameters, omitted = request_params_from_variant(variant, requested)
        for sample_index in range(1, args.samples + 1):
            bases.append(
                {
                    "run_id": out.name,
                    "gene_index": args.gene_index,
                    "gene": gene.strip(),
                    "persona_id": persona_path.stem,
                    "persona_path": display_path(persona_path),
                    "variant_order": variant_order,
                    "combined_index": variant.get("combined_index"),
                    "endpoint_variant_id": variant.get("endpoint_variant_id"),
                    "openrouter_model_id": variant.get("openrouter_model_id"),
                    "provider_name": variant.get("provider_name"),
                    "endpoint_tag": variant.get("endpoint_tag"),
                    "quantization": variant.get("quantization"),
                    "sample_index": sample_index,
                    "request_parameters": request_parameters,
                    "requested_request_parameters": requested,
                    "omitted_request_parameters": omitted,
                    "embedding_model": args.embedding_model,
                }
            )
    return variants, gene.strip(), persona, persona_path, bases


def row_record_path(out: Path, key: tuple[int, int]) -> Path:
    return out / "row-records" / f"{key[0]:04d}-{key[1]:04d}.json"


def row_transaction_path(out: Path, key: tuple[int, int]) -> Path:
    return out / "row-transactions" / f"{key[0]:04d}-{key[1]:04d}.json"


def validate_record(row: dict, expected: dict, source: Path) -> None:
    allowed = set(expected) | {"created_at", "status", "response_text", "embedding", "metadata"}
    if set(row) != allowed:
        raise ValueError(f"{source}: record fields differ from the expected schema")
    for field, value in expected.items():
        if row.get(field) != value:
            raise ValueError(f"{source}: record field {field} differs from the recorded inputs")
    if not isinstance(row.get("created_at"), str) or not row["created_at"]:
        raise ValueError(f"{source}: created_at must be a nonempty string")
    status = row.get("status")
    if status not in {"ok", "completion_error", "embedding_error"}:
        raise ValueError(f"{source}: invalid status {status!r}")
    if not isinstance(row.get("metadata"), dict):
        raise ValueError(f"{source}: metadata must be an object")
    response_text = row.get("response_text")
    embedding = row.get("embedding")
    metadata = row["metadata"]
    if status == "ok":
        if not isinstance(response_text, str) or not response_text.strip() or not isinstance(embedding, list) or not embedding:
            raise ValueError(f"{source}: completed record lacks response text or embedding")
        if any(
            not isinstance(value, (int, float))
            or isinstance(value, bool)
            or not math.isfinite(float(value))
            for value in embedding
        ):
            raise ValueError(f"{source}: embedding contains a nonnumeric value")
        if "embedding_error_type" in metadata or "embedding_error_message" in metadata:
            raise ValueError(f"{source}: completed record contains embedding-error metadata")
    elif status == "embedding_error":
        if not isinstance(response_text, str) or not response_text.strip() or embedding is not None:
            raise ValueError(f"{source}: embedding_error requires a completed response and a null embedding")
        if not isinstance(metadata.get("embedding_error_type"), str) or not metadata["embedding_error_type"]:
            raise ValueError(f"{source}: embedding_error lacks embedding_error_type")
        if not isinstance(metadata.get("embedding_error_message"), str) or not metadata["embedding_error_message"]:
            raise ValueError(f"{source}: embedding_error lacks embedding_error_message")
    else:
        if response_text not in {"", None} or embedding is not None:
            raise ValueError(f"{source}: completion_error cannot contain a completed response or embedding")
        if not isinstance(metadata.get("error_type"), str) or not metadata["error_type"]:
            raise ValueError(f"{source}: completion_error lacks error_type")
        if not isinstance(metadata.get("error_message"), str) or not metadata["error_message"]:
            raise ValueError(f"{source}: completion_error lacks error_message")
        if "embedding_error_type" in metadata or "embedding_error_message" in metadata:
            raise ValueError(f"{source}: completion_error contains embedding-error metadata")


def load_prior_records(
    out: Path,
    expected_bases: list[dict],
    *,
    require_complete_aggregate: bool = False,
) -> dict[tuple[int, int], dict]:
    expected = {record_key(base): base for base in expected_bases}
    directory = out / "row-records"
    records: dict[tuple[int, int], dict] = {}
    if directory.exists():
        if not directory.is_dir():
            raise ValueError(f"row-records is not a directory: {directory}")
        expected_paths = {row_record_path(out, key) for key in expected}
        found_paths = {path for path in directory.iterdir() if path.is_file()}
        if found_paths - expected_paths:
            raise ValueError(f"unexpected row record: {min(found_paths - expected_paths, key=str)}")
        for path in sorted(found_paths):
            row = strict_json_loads(path.read_bytes(), path)
            if not isinstance(row, dict):
                raise ValueError(f"{path}: expected a JSON object")
            key = record_key(row)
            if key not in expected or row_record_path(out, key) != path:
                raise ValueError(f"{path}: row identity does not match its recorded path")
            if key in records:
                raise ValueError(f"{path}: duplicate row identity {key}")
            validate_record(row, expected[key], path)
            records[key] = row

    transactions_dir = out / "row-transactions"
    transaction: tuple[tuple[int, int], dict | None, dict] | None = None
    if transactions_dir.exists():
        if not transactions_dir.is_dir():
            raise ValueError(f"row-transactions is not a directory: {transactions_dir}")
        transaction_paths = [path for path in transactions_dir.iterdir() if path.is_file() and path.suffix == ".json"]
        unexpected = [
            path
            for path in transactions_dir.iterdir()
            if path.is_file() and path.suffix != ".json" and not path.name.startswith(PENDING_FILE_PREFIX)
        ]
        if unexpected:
            raise ValueError(f"unexpected row transaction artifact: {min(unexpected, key=str)}")
        if len(transaction_paths) > 1:
            raise ValueError(f"multiple row transactions exist: {transactions_dir}")
        if transaction_paths:
            transaction_path = transaction_paths[0]
            value = strict_json_loads(transaction_path.read_bytes(), transaction_path)
            if not isinstance(value, dict) or set(value) != {"prior", "next"}:
                raise ValueError(f"{transaction_path}: invalid row transaction")
            next_row = value["next"]
            if not isinstance(next_row, dict):
                raise ValueError(f"{transaction_path}: next row must be an object")
            key = record_key(next_row)
            if key not in expected or row_transaction_path(out, key) != transaction_path:
                raise ValueError(f"{transaction_path}: transaction identity differs from its path")
            validate_record(next_row, expected[key], transaction_path)
            prior = value["prior"]
            if prior is not None:
                if not isinstance(prior, dict):
                    raise ValueError(f"{transaction_path}: prior row must be an object or null")
                validate_record(prior, expected[key], transaction_path)
            physical = records.get(key)
            allowed_physical = [candidate for candidate in (prior, next_row) if candidate is not None]
            if physical is not None and physical not in allowed_physical:
                raise ValueError(f"{transaction_path}: row snapshot is neither the prior nor next transaction row")
            if prior is None and physical is not None and physical != next_row:
                raise ValueError(f"{transaction_path}: new-row transaction has an unexpected prior snapshot")
            records[key] = next_row
            transaction = key, prior, next_row

    expected_order = [record_key(base) for base in expected_bases]
    recorded_order = [key for key in expected_order if key in records]
    if recorded_order != expected_order[: len(recorded_order)]:
        raise ValueError(f"{directory}: row-record snapshots do not form the expected ordered prefix")

    records_path = out / "records.jsonl"
    if records_path.exists():
        aggregate = parse_jsonl_objects(records_path.read_bytes(), records_path)
        aggregate_by_key: dict[tuple[int, int], dict] = {}
        for line_number, row in enumerate(aggregate, start=1):
            key = record_key(row)
            if key in aggregate_by_key:
                raise ValueError(f"{records_path}:{line_number}: duplicate row identity {key}")
            if key not in expected:
                raise ValueError(f"{records_path}:{line_number}: unexpected row identity {key}")
            validate_record(row, expected[key], records_path)
            allowed_rows = [records.get(key)]
            if transaction is not None and key == transaction[0]:
                allowed_rows.append(transaction[1])
            if row not in allowed_rows:
                raise ValueError(f"{records_path}:{line_number}: row differs from its row-record snapshot or transaction")
            aggregate_by_key[key] = row
        aggregate_keys = list(aggregate_by_key)
        if aggregate_keys != recorded_order[: len(aggregate_keys)]:
            raise ValueError(f"{records_path}: rows do not form the expected ordered prefix")
        if require_complete_aggregate and (transaction is not None or aggregate_keys != recorded_order):
            raise ValueError(f"{records_path}: row set differs from row-record snapshots")
    elif require_complete_aggregate and records:
        raise ValueError(f"records file is missing while row-record snapshots exist: {records_path}")
    return records


def atomic_write(path: Path, data: bytes) -> None:
    atomic_write_bytes(path, data)


def publish_row_record(
    out: Path,
    bases: list[dict],
    records: dict[tuple[int, int], dict],
    key: tuple[int, int],
    row: dict,
) -> None:
    prior = records.get(key)
    transaction_path = row_transaction_path(out, key)
    transaction = {"prior": prior, "next": row}
    atomic_write(
        transaction_path,
        (json.dumps(transaction, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n").encode("utf-8"),
    )
    atomic_write(
        row_record_path(out, key),
        (json.dumps(row, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n").encode("utf-8"),
    )
    records[key] = row
    write_current_records(out, bases, records)
    transaction_path.unlink()
    fsync_directory(transaction_path.parent)


def recover_row_transaction(out: Path, bases: list[dict], records: dict[tuple[int, int], dict]) -> None:
    directory = out / "row-transactions"
    if not directory.is_dir():
        return
    for temporary in directory.iterdir():
        if temporary.is_file() and temporary.name.startswith(PENDING_FILE_PREFIX):
            temporary.unlink()
    paths = [path for path in directory.iterdir() if path.is_file() and path.suffix == ".json"]
    if not paths:
        return
    path = paths[0]
    value = strict_json_loads(path.read_bytes(), path)
    assert isinstance(value, dict) and isinstance(value.get("next"), dict)
    row = value["next"]
    key = record_key(row)
    atomic_write(
        row_record_path(out, key),
        (json.dumps(row, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n").encode("utf-8"),
    )
    records[key] = row
    write_current_records(out, bases, records)
    path.unlink()
    fsync_directory(path.parent)


def write_current_records(out: Path, expected_bases: list[dict], records: dict[tuple[int, int], dict]) -> None:
    data = b"".join(
        (json.dumps(records[key], ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n").encode("utf-8")
        for key in (record_key(base) for base in expected_bases)
        if key in records
    )
    atomic_write(out / "records.jsonl", data)


def validate_summary(
    out: Path,
    args: argparse.Namespace,
    bases: list[dict],
    records: dict[tuple[int, int], dict],
    *,
    require_current: bool = False,
) -> None:
    path = out / "summary.json"
    if not path.exists():
        if require_current:
            raise ValueError(f"completed gene summary does not exist: {path}")
        return
    summary = strict_json_loads(path.read_bytes(), path)
    if not isinstance(summary, dict):
        raise ValueError(f"{path}: expected a JSON object")
    counts: dict[str, int] = {}
    for row in records.values():
        counts[row["status"]] = counts.get(row["status"], 0) + 1
    expected = {
        "run_id": out.name,
        "gene_index": args.gene_index,
        "gene": bases[0]["gene"],
        "persona_path": bases[0]["persona_path"],
        "variant_count": len(bases) // args.samples,
        "samples_per_variant": args.samples,
        "expected_records": len(bases),
        "requested_request_parameters": bases[0]["requested_request_parameters"],
        "embedding_model": args.embedding_model,
        "completion_attempts": args.completion_attempts,
        "retry_sleep_seconds": args.retry_sleep,
        "records_path": display_path(out / "records.jsonl"),
        "records_written": len(records),
        "status_counts": counts,
        "completion_error_count": counts.get("completion_error", 0),
        "embedding_error_count": counts.get("embedding_error", 0),
        "embedding_count": sum(1 for row in records.values() if isinstance(row.get("embedding"), list)),
    }
    allowed = set(expected) | {
        "finished_at",
        "scope",
        "variants_path",
        "reused_record_count",
        "partial_records_allowed",
    }
    if set(summary) != allowed:
        raise ValueError(f"{path}: summary fields differ from the expected schema")
    differs = [field for field, value in expected.items() if summary.get(field) != value]
    if (
        not isinstance(summary.get("finished_at"), str)
        or not summary["finished_at"]
        or summary.get("scope") != "one_sampled_gene_only"
        or summary.get("variants_path") != display_path(out / "inputs" / "variants.jsonl")
        or not isinstance(summary.get("reused_record_count"), int)
        or isinstance(summary.get("reused_record_count"), bool)
        or not 0 <= summary["reused_record_count"] <= len(records)
        or summary.get("partial_records_allowed") is not True
    ):
        differs.append("summary metadata")
    if differs:
        raise ValueError(f"{path}: summary differs from recorded rows or inputs: {', '.join(differs)}")


def preflight_gene_resume(
    args: argparse.Namespace,
    out: Path,
    *,
    require_complete: bool = False,
) -> tuple[list[dict], dict[tuple[int, int], dict]]:
    inputs = run_inputs(args)
    validate_run_record_inputs(
        out,
        kind="model-pool-gene-inference",
        parameters=run_parameters(args),
        inputs=inputs,
    )
    saved = saved_input_paths(out, inputs)
    if not all(path.is_file() for path in saved):
        if require_complete:
            raise ValueError(f"{out}: completed gene run lacks one or more saved inputs")
        durable_work = [
            path
            for path in (out / "row-records", out / "records.jsonl", out / "summary.json")
            if path.exists()
        ]
        if durable_work:
            raise ValueError(f"{out}: gene work exists before all run inputs were published: {durable_work[0]}")
        return [], {}
    _, _, _, _, bases = load_context(args, out, inputs)
    records = load_prior_records(out, bases, require_complete_aggregate=require_complete)
    if require_complete and set(records) != {record_key(base) for base in bases}:
        raise ValueError(f"{out}: completed gene run lacks one or more expected row records")
    validate_summary(out, args, bases, records, require_current=require_complete)
    return bases, records


def require_service_keys(bases: list[dict], records: dict[tuple[int, int], dict]) -> None:
    completion_required = not bases or any(
        records.get(record_key(base), {}).get("status") not in {"ok", "embedding_error"}
        for base in bases
    )
    embedding_required = completion_required or any(
        records.get(record_key(base), {}).get("status") == "embedding_error"
        for base in bases
    )
    if completion_required and not load_openrouter_key():
        raise RuntimeError("OPENROUTER_API_KEY not found in environment or secrets/openrouter.api.txt")
    if embedding_required and not load_openai_key():
        raise RuntimeError("OPENAI_API_KEY not found in environment or secrets/openai.api.txt")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run one sampled gene through filtered variants and embed responses.")
    parser.add_argument("--out", required=True)
    parser.add_argument("--variants", default="variants/filtered-20260529/endpoint_variants.jsonl")
    parser.add_argument("--genes", default="sampled-genes.json")
    parser.add_argument("--persona", default="../common/etc/personas/generic.md")
    parser.add_argument("--samples", type=int, default=3)
    parser.add_argument("--gene-index", type=int, default=0)
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument("--embedding-model", default="text-embedding-3-small")
    parser.add_argument("--temperature", type=float, default=0.7)
    parser.add_argument("--top-p", type=float, default=1.0)
    parser.add_argument("--max-tokens", type=int, default=512)
    parser.add_argument("--completion-attempts", type=int, default=3)
    parser.add_argument("--retry-sleep", type=float, default=2.0)
    parser.add_argument("--resume", action="store_true")
    args = parser.parse_args(argv)
    if args.samples < 1:
        raise RuntimeError("--samples must be positive")
    if args.timeout < 1:
        raise RuntimeError("--timeout must be positive")
    if args.completion_attempts < 1:
        raise RuntimeError("--completion-attempts must be positive")
    if not math.isfinite(args.temperature):
        raise RuntimeError("--temperature must be finite")
    if not math.isfinite(args.top_p) or not 0 < args.top_p <= 1:
        raise RuntimeError("--top-p must be greater than 0 and no greater than 1")
    if args.max_tokens < 1:
        raise RuntimeError("--max-tokens must be positive")
    if not math.isfinite(args.retry_sleep) or args.retry_sleep < 0:
        raise RuntimeError("--retry-sleep must be a finite non-negative number")
    return args


def execute(args: argparse.Namespace, out: Path, inputs: list[Any]) -> int:
    variants, gene, persona, persona_path, bases = load_context(args, out, inputs)
    prior_records = load_prior_records(out, bases)
    validate_summary(out, args, bases, prior_records)
    recover_row_transaction(out, bases, prior_records)
    try:
        (out / "summary.json").unlink()
    except FileNotFoundError:
        pass

    print(json.dumps({"event": "started", "run_id": out.name, "expected": len(bases), "out": str(out)}, sort_keys=True), flush=True)
    records = dict(prior_records)
    bases_by_key = {record_key(base): base for base in bases}
    reused_records = 0
    completed = 0
    for variant_order, variant in enumerate(variants, start=1):
        spec = spec_from_variant(variant)
        for sample_index in range(1, args.samples + 1):
            key = (variant_order, sample_index)
            base = bases_by_key[key]
            prior = records.get(key)
            if reusable_record(prior):
                record = prior
                reused_records += 1
            elif embedding_retry_record(prior):
                metadata = dict(prior["metadata"])
                metadata.pop("embedding_error_type", None)
                metadata.pop("embedding_error_message", None)
                try:
                    embedding, embedding_meta = embedding_request(prior["response_text"], args.embedding_model, args.timeout)
                except EmbeddingServiceError as exc:
                    metadata["embedding_error_type"] = classify_error(exc)
                    metadata["embedding_error_message"] = str(exc)
                    record = {**prior, "metadata": metadata}
                else:
                    metadata.update(embedding_meta)
                    record = {**prior, "status": "ok", "embedding": embedding, "metadata": metadata}
                validate_record(record, base, row_record_path(out, key))
                publish_row_record(out, bases, records, key, record)
            else:
                created_base = {**base, "created_at": utc_now()}
                try:
                    response_text, metadata = openrouter_completion(
                        spec,
                        persona,
                        gene,
                        base["request_parameters"],
                        args.timeout,
                        args.completion_attempts,
                        args.retry_sleep,
                    )
                except EXPECTED_COMPLETION_ERRORS as exc:
                    record = error_row(created_base, "completion_error", exc)
                else:
                    try:
                        embedding, embedding_meta = embedding_request(response_text, args.embedding_model, args.timeout)
                    except EmbeddingServiceError as exc:
                        metadata["embedding_error_type"] = classify_error(exc)
                        metadata["embedding_error_message"] = str(exc)
                        record = {
                            **created_base,
                            "status": "embedding_error",
                            "response_text": response_text,
                            "embedding": None,
                            "metadata": metadata,
                        }
                    else:
                        metadata.update(embedding_meta)
                        record = {
                            **created_base,
                            "status": "ok",
                            "response_text": response_text,
                            "embedding": embedding,
                            "metadata": metadata,
                        }
                validate_record(record, base, row_record_path(out, key))
                publish_row_record(out, bases, records, key, record)
            completed += 1
            print(
                json.dumps(
                    {
                        "event": "progress",
                        "completed": completed,
                        "expected": len(bases),
                        "combined_index": variant.get("combined_index"),
                        "sample_index": sample_index,
                        "status": record["status"],
                        "reused": record is prior,
                    },
                    sort_keys=True,
                ),
                flush=True,
            )

    ordered_rows = [copy.deepcopy(records[record_key(base)]) for base in bases]
    hydrate_posthoc_generation_metadata(ordered_rows, args.timeout)
    for base, row in zip(bases, ordered_rows, strict=True):
        key = record_key(base)
        validate_record(row, base, row_record_path(out, key))
        if records[key] != row:
            publish_row_record(out, bases, records, key, row)
    write_current_records(out, bases, records)

    counts: dict[str, int] = {}
    for row in ordered_rows:
        counts[row["status"]] = counts.get(row["status"], 0) + 1
    summary = {
        "run_id": out.name,
        "finished_at": utc_now(),
        "scope": "one_sampled_gene_only",
        "gene_index": args.gene_index,
        "gene": gene,
        "persona_path": display_path(persona_path),
        "variants_path": display_path(out / inputs[0].snapshot_path),
        "variant_count": len(variants),
        "samples_per_variant": args.samples,
        "expected_records": len(bases),
        "requested_request_parameters": bases[0]["requested_request_parameters"],
        "embedding_model": args.embedding_model,
        "completion_attempts": args.completion_attempts,
        "retry_sleep_seconds": args.retry_sleep,
        "records_path": display_path(out / "records.jsonl"),
        "records_written": len(ordered_rows),
        "reused_record_count": reused_records,
        "status_counts": counts,
        "completion_error_count": counts.get("completion_error", 0),
        "embedding_error_count": counts.get("embedding_error", 0),
        "embedding_count": sum(1 for row in ordered_rows if isinstance(row.get("embedding"), list)),
        "partial_records_allowed": True,
    }
    atomic_write(
        out / "summary.json",
        (json.dumps(summary, indent=2, ensure_ascii=False, allow_nan=False, sort_keys=True) + "\n").encode("utf-8"),
    )
    print(json.dumps({"event": "finished", "summary": summary}, sort_keys=True), flush=True)
    return 0


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    out = resolve_path(args.out)
    inputs = run_inputs(args)
    if args.resume:
        if not out.is_dir():
            raise RuntimeError(f"resume directory does not exist: {out}")
    else:
        out.parent.mkdir(parents=True, exist_ok=True)
        out.mkdir(exist_ok=True)
    with exclusive_run_lock(out, "model-pool-gene-inference", [str(Path(__file__)), *(argv or sys.argv[1:])]) as run_lock:
        if args.resume:
            bases, records = preflight_gene_resume(args, out)
        else:
            bases, records = [], {}
        require_service_keys(bases, records)
        prepare_run_record(
            out,
            kind="model-pool-gene-inference",
            parameters=run_parameters(args),
            inputs=inputs,
            resume=args.resume,
        )
        _, _, _, _, bases = load_context(args, out, inputs)
        records = load_prior_records(out, bases)
        validate_summary(out, args, bases, records)
        run_lock.record_owner()
        return execute(args, out, inputs)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except BrokenPipeError:
        raise SystemExit(1)
    except (TimeoutError, socket.timeout):
        raise
