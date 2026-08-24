#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import datetime as dt
import hashlib
import http.client
import json
import os
import re
import socket
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

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

REQUEST_PARAMETER_KEYS = ("temperature", "top_p", "max_tokens")


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
    text, meta, _ = response_meta(body, started)
    meta = attach_request_spec_meta(meta, {**spec, "request": request_params}, timeout)
    return text or "", meta


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
        except Exception as exc:
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
        raise RuntimeError(f"OpenAI embeddings HTTP {exc.code}: {detail}") from exc
    data = raw.get("data")
    if not isinstance(data, list) or not data or not isinstance(data[0].get("embedding"), list):
        raise RuntimeError("OpenAI embeddings response missing embedding vector")
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


def record_key(row: dict) -> tuple[str, str]:
    endpoint = row.get("endpoint_variant_id")
    if endpoint in (None, ""):
        endpoint = row.get("combined_index")
    return str(endpoint), str(row.get("sample_index"))


def reusable_record(row: dict | None) -> bool:
    if not isinstance(row, dict):
        return False
    return row.get("status") == "ok" and isinstance(row.get("embedding"), list)


def main() -> int:
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
    args = parser.parse_args()

    if not load_openrouter_key():
        raise RuntimeError("OPENROUTER_API_KEY not found in environment or secrets/openrouter.api.txt")
    if not load_openai_key():
        raise RuntimeError("OPENAI_API_KEY not found in environment or secrets/openai.api.txt")
    if args.completion_attempts < 1:
        raise RuntimeError("--completion-attempts must be positive")
    if args.retry_sleep < 0:
        raise RuntimeError("--retry-sleep cannot be negative")

    variants_path = ROOT / args.variants
    genes_path = ROOT / args.genes
    persona_path = ROOT / args.persona
    out = Path(args.out)
    if not out.is_absolute():
        out = ROOT / out
    out.mkdir(parents=True, exist_ok=True)

    genes = json.loads(genes_path.read_text())
    if not isinstance(genes, list) or not genes:
        raise RuntimeError(f"{genes_path}: expected a non-empty JSON array")
    if args.gene_index < 0 or args.gene_index >= len(genes):
        raise RuntimeError(f"--gene-index {args.gene_index} is outside sampled gene range 0..{len(genes) - 1}")
    gene = genes[args.gene_index]
    if not isinstance(gene, str) or not gene.strip():
        raise RuntimeError(f"{genes_path}: first gene is not a non-empty string")
    gene = gene.strip()
    persona = persona_path.read_text().strip()
    variants = load_jsonl(variants_path)
    if not variants:
        raise RuntimeError(f"{variants_path}: no variants found")

    requested_params = {"temperature": args.temperature, "top_p": args.top_p, "max_tokens": args.max_tokens}
    run_id = out.name
    expected = len(variants) * args.samples
    records_path = out / "records.jsonl"
    summary_path = out / "summary.json"
    temp_records_path = out / "records.jsonl.tmp"
    prior_records: dict[tuple[str, str], dict] = {}
    if args.resume and records_path.exists():
        for row in load_jsonl(records_path):
            prior_records[record_key(row)] = row

    manifest = {
        "run_id": run_id,
        "started_at": utc_now(),
        "scope": "one_sampled_gene_only",
        "gene_index": args.gene_index,
        "gene": gene,
        "persona_path": display_path(persona_path),
        "variants_path": display_path(variants_path),
        "variant_count": len(variants),
        "samples_per_variant": args.samples,
        "expected_records": expected,
        "requested_request_parameters": requested_params,
        "embedding_model": args.embedding_model,
        "completion_attempts": args.completion_attempts,
        "retry_sleep_seconds": args.retry_sleep,
        "resume": args.resume,
        "records_path": display_path(records_path),
    }
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print(json.dumps({"event": "started", "run_id": run_id, "expected": expected, "out": str(out)}, sort_keys=True), flush=True)

    rows: list[dict] = []
    reused_records = 0
    with temp_records_path.open("w") as handle:
        completed = 0
        for variant_order, row in enumerate(variants, 1):
            spec = spec_from_variant(row)
            request_params, omitted_request_params = request_params_from_variant(row, requested_params)
            for sample_index in range(1, args.samples + 1):
                base = {
                    "run_id": run_id,
                    "created_at": utc_now(),
                    "gene_index": args.gene_index,
                    "gene": gene,
                    "gene_sha256": hashlib.sha256(gene.encode()).hexdigest(),
                    "persona_id": "generic",
                    "persona_path": display_path(persona_path),
                    "variant_order": variant_order,
                    "combined_index": row.get("combined_index"),
                    "endpoint_variant_id": row.get("endpoint_variant_id"),
                    "openrouter_model_id": row.get("openrouter_model_id"),
                    "provider_name": row.get("provider_name"),
                    "endpoint_tag": row.get("endpoint_tag"),
                    "quantization": row.get("quantization"),
                    "sample_index": sample_index,
                    "request_parameters": request_params,
                    "requested_request_parameters": requested_params,
                    "omitted_request_parameters": omitted_request_params,
                    "embedding_model": args.embedding_model,
                }
                prior = prior_records.get(record_key(base))
                if reusable_record(prior):
                    record = prior
                    reused_records += 1
                else:
                    try:
                        response_text, metadata = openrouter_completion(
                            spec,
                            persona,
                            gene,
                            request_params,
                            args.timeout,
                            args.completion_attempts,
                            args.retry_sleep,
                        )
                    except Exception as exc:
                        record = error_row(base, "completion_error", exc)
                    else:
                        try:
                            embedding, embedding_meta = embedding_request(response_text, args.embedding_model, args.timeout)
                        except Exception as exc:
                            metadata["embedding_error_type"] = classify_error(exc)
                            metadata["embedding_error_message"] = str(exc)
                            record = {
                                **base,
                                "status": "embedding_error",
                                "response_text": response_text,
                                "embedding": None,
                                "metadata": metadata,
                            }
                        else:
                            metadata.update(embedding_meta)
                            record = {
                                **base,
                                "status": "ok",
                                "response_text": response_text,
                                "embedding": embedding,
                                "metadata": metadata,
                            }
                rows.append(record)
                handle.write(json.dumps(record, ensure_ascii=False) + "\n")
                handle.flush()
                completed += 1
                print(
                    json.dumps(
                        {
                            "event": "progress",
                            "completed": completed,
                            "expected": expected,
                            "combined_index": row.get("combined_index"),
                            "sample_index": sample_index,
                            "status": record["status"],
                            "reused": record is prior,
                        },
                        sort_keys=True,
                    ),
                    flush=True,
                )

    hydrate_posthoc_generation_metadata(rows, args.timeout)
    with temp_records_path.open("w") as handle:
        for row in rows:
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")
    os.replace(temp_records_path, records_path)

    counts: dict[str, int] = {}
    for row in rows:
        counts[row["status"]] = counts.get(row["status"], 0) + 1
    summary = {
        **manifest,
        "finished_at": utc_now(),
        "records_written": len(rows),
        "reused_record_count": reused_records,
        "status_counts": counts,
        "completion_error_count": counts.get("completion_error", 0),
        "embedding_error_count": counts.get("embedding_error", 0),
        "embedding_count": sum(1 for row in rows if isinstance(row.get("embedding"), list)),
        "partial_records_allowed": True,
    }
    summary_path.write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps({"event": "finished", "summary": summary}, sort_keys=True), flush=True)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except BrokenPipeError:
        raise SystemExit(1)
    except (TimeoutError, socket.timeout):
        raise
