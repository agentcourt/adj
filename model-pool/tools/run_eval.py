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
import socket
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(Path(__file__).resolve().parent))
from tool_server import build_record_context, list_evidence, read_evidence, stat_evidence


class OpenRouterHTTPError(RuntimeError):
    def __init__(self, status_code: int, detail: str, body_json=None, retry_after_seconds: float | None = None):
        super().__init__(f"OpenRouter HTTP {status_code}: {detail}")
        self.status_code = status_code
        self.detail = detail
        self.body_json = body_json
        self.retry_after_seconds = retry_after_seconds


class OpenRouterResponseError(RuntimeError):
    def __init__(self, detail: str, body: dict):
        super().__init__(f"OpenRouter response error: {detail}")
        self.body = body


class ToolRoundError(RuntimeError):
    def __init__(self, cause: Exception, metadata: dict, trace: list[dict]):
        super().__init__(str(cause))
        self.cause = cause
        self.metadata = dict(metadata)
        self.trace = list(trace)


class OpenRouterBatchTerminalError(RuntimeError):
    def __init__(self, batch: dict):
        status = str(batch.get("status") or "unknown")
        batch_id = str(batch.get("id") or "unknown")
        super().__init__(f"OpenRouter batch {batch_id} ended with status {status}")
        self.batch = batch


class OpenRouterBatchPollError(RuntimeError):
    def __init__(self, batch_id: str, cause: Exception):
        super().__init__(f"OpenRouter batch {batch_id} polling failed: {cause}")
        self.batch_id = batch_id
        self.cause = cause


class OpenRouterBatchItemError(RuntimeError):
    def __init__(self, result: dict):
        response = result.get("response") if isinstance(result.get("response"), dict) else {}
        status_code = response.get("status_code")
        detail = result.get("error")
        if detail is None:
            detail = response.get("body")
        rendered = json.dumps(detail, ensure_ascii=False, sort_keys=True)
        super().__init__(f"OpenRouter batch item failed with status {status_code}: {rendered}")
        self.result = result


TOOL_DEFS = [
    {
        "type": "function",
        "function": {
            "name": "list_evidence",
            "description": "List evidence ids and titles in the bounded record.",
            "strict": True,
            "parameters": {"type": "object", "properties": {}, "additionalProperties": False},
        },
    },
    {
        "type": "function",
        "function": {
            "name": "read_evidence",
            "description": "Read one evidence item by id.",
            "strict": True,
            "parameters": {
                "type": "object",
                "properties": {"evidence_id": {"type": "string"}},
                "required": ["evidence_id"],
                "additionalProperties": False,
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "stat_evidence",
            "description": "Return simple metadata for one evidence item by id.",
            "strict": True,
            "parameters": {
                "type": "object",
                "properties": {"evidence_id": {"type": "string"}},
                "required": ["evidence_id"],
                "additionalProperties": False,
            },
        },
    },
]


def load_items(path: Path) -> list[dict]:
    with path.open() as f:
        return [json.loads(line) for line in f if line.strip()]


def normalize_model(model: str) -> str:
    return model.removeprefix("openrouter://")


def compact_strings(values) -> list[str]:
    if isinstance(values, str):
        values = [values]
    if not isinstance(values, list):
        return []
    out = []
    for value in values:
        if isinstance(value, str) and value.strip():
            out.append(value.strip())
    return out


def first_string(obj: dict, *keys: str) -> str:
    for key in keys:
        value = obj.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""


def provider_from_spec(obj: dict) -> dict | None:
    raw = obj.get("provider")
    if isinstance(raw, dict):
        only = compact_strings(raw.get("only"))
        quantizations = [q.lower() for q in compact_strings(raw.get("quantizations")) if q.lower() != "unknown"]
        if not only and not quantizations:
            return None
        provider = {
            "allow_fallbacks": raw["allow_fallbacks"] if isinstance(raw.get("allow_fallbacks"), bool) else False,
            "require_parameters": raw["require_parameters"] if isinstance(raw.get("require_parameters"), bool) else True,
        }
        if only:
            provider["only"] = only
        if quantizations:
            provider["quantizations"] = quantizations
        return provider

    selected = first_string(obj, "endpoint_tag", "provider_tag", "selected_provider_or_endpoint")
    if not selected:
        selected = first_string(obj, "provider_name")
    if not selected:
        return None
    provider = {
        "only": [selected],
        "allow_fallbacks": False,
        "require_parameters": True,
    }
    quantization = first_string(obj, "quantization").lower()
    if quantization and quantization != "unknown":
        provider["quantizations"] = [quantization]
    return provider


def request_params_from_spec(obj: dict) -> dict:
    params = {}
    raw = obj.get("request")
    if isinstance(raw, dict):
        for key in ("temperature", "top_p", "max_tokens"):
            if isinstance(raw.get(key), (int, float)):
                params[key] = raw[key]
    for key in ("temperature", "top_p", "max_tokens"):
        if isinstance(obj.get(key), (int, float)):
            params[key] = obj[key]
    return params


def safe_headers_from_spec(obj: dict, exact_variant: bool) -> dict:
    allowed = {"x-openrouter-experimental-metadata", "http-referer", "x-title"}
    out = {}
    raw = obj.get("headers")
    if isinstance(raw, dict):
        for key, value in raw.items():
            if isinstance(key, str) and isinstance(value, str) and key.strip().lower() in allowed and value.strip():
                out[key.strip()] = value.strip()
    if exact_variant and not any(key.lower() == "x-openrouter-experimental-metadata" for key in out):
        out["X-OpenRouter-Experimental-Metadata"] = "enabled"
    return out


def variant_metadata_from_spec(obj: dict) -> dict:
    keys = [
        "catalog_snapshot_id",
        "snapshot_timestamp_utc",
        "openrouter_model_id",
        "canonical_slug",
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
        "model_supported_parameters",
        "endpoint_raw_path",
        "model_raw_path",
    ]
    return {key: obj[key] for key in keys if key in obj}


def model_spec_from_string(model: str) -> dict:
    return {
        "label": model,
        "model": model,
        "openrouter_model_id": normalize_model(model),
        "provider": None,
        "headers": {},
        "request": {},
        "variant_metadata": {},
        "exact_variant": False,
        "source": "--models",
    }


def model_spec_from_object(obj: dict, source: str) -> dict:
    model = first_string(obj, "model", "openrouter_model_id")
    if not model:
        raise ValueError(f"{source}: model or openrouter_model_id is required")
    model_id = normalize_model(model)
    provider = provider_from_spec(obj)
    exact_variant = provider is not None or "openrouter_model_id" in obj
    headers = safe_headers_from_spec(obj, exact_variant)
    metadata = variant_metadata_from_spec(obj)
    provider_name = first_string(obj, "endpoint_tag", "provider_name")
    label = first_string(obj, "model_spec_id", "endpoint_variant_id")
    if not label:
        label = f"openrouter://{model_id}" + (f"@{provider_name}" if provider_name else "")
    return {
        "label": label,
        "model": "openrouter://" + model_id,
        "openrouter_model_id": model_id,
        "provider": provider,
        "headers": headers,
        "request": request_params_from_spec(obj),
        "variant_metadata": metadata,
        "exact_variant": exact_variant,
        "source": source,
    }


def load_model_spec_file(path: Path) -> dict:
    obj = json.loads(path.read_text())
    if not isinstance(obj, dict):
        raise ValueError(f"{path}: model spec file must contain one JSON object")
    return model_spec_from_object(obj, str(path))


def load_model_spec_jsonl(path: Path) -> list[dict]:
    specs = []
    with path.open() as handle:
        for line_no, line in enumerate(handle, 1):
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            obj = json.loads(line)
            if not isinstance(obj, dict):
                raise ValueError(f"{path}:{line_no}: model spec JSONL row must be an object")
            specs.append(model_spec_from_object(obj, f"{path}:{line_no}"))
    if not specs:
        raise ValueError(f"{path}: no model specs found")
    return specs


def load_model_specs(models: list[str] | None, model_specs: list[str] | None, model_spec_jsonls: list[str] | None) -> list[dict]:
    specs = []
    for model in models or []:
        specs.append(model_spec_from_string(model))
    for raw_path in model_specs or []:
        specs.append(load_model_spec_file(Path(raw_path)))
    for raw_path in model_spec_jsonls or []:
        specs.extend(load_model_spec_jsonl(Path(raw_path)))
    if not specs:
        raise ValueError("at least one --models, --model-spec, or --model-spec-jsonl entry is required")
    return specs


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
        for pat in patterns:
            m = pat.search(text)
            if m:
                return m.group(1).strip()
    return None


def item_prompt(item: dict, base_prompt: str, tool_mode: str = "context") -> tuple[str, list[dict]]:
    trace: list[dict] = []
    lines = [base_prompt.strip(), "", f"Item id: {item['id']}", f"Question: {item['prompt']}"]
    if item["mode"] == "tool_record":
        lines.append("Response contract: use exactly this JSON object shape:")
        lines.append('{"vote":"demonstrated|not_demonstrated|indeterminate","confidence":0.0,"rationale":"...","evidence_ids":["E1"]}')
        lines.append("Do not use an `answer` key.")
        if tool_mode == "function":
            lines.append("Use the provided evidence tools before answering.")
            lines.append("You must call list_evidence and at least one read_evidence call before the final JSON answer.")
            lines.append("Vote options: demonstrated, not_demonstrated, indeterminate.")
            lines.append("Cite the evidence ids that support your vote. Do not use outside facts.")
        else:
            context, trace = build_record_context(item["record_dir"])
            lines.append("Bounded record:")
            lines.append(context)
            lines.append("Vote options: demonstrated, not_demonstrated, indeterminate.")
            lines.append("Cite the evidence ids that support your vote. Do not use outside facts.")
    elif item["answer_type"] == "multiple_choice":
        lines.append("Response contract: use exactly this JSON object shape:")
        lines.append('{"answer":"A|B|C|D","confidence":0.0,"rationale":"...","evidence_ids":[]}')
        lines.append("Set `answer` to the choice letter only. Do not use a `vote` key, even if the choices describe adjudicative conclusions.")
        lines.append("Choices:")
        lines.extend(item["choices"])
    else:
        lines.append("Response contract: use exactly this JSON object shape:")
        lines.append('{"answer":"...","confidence":0.0,"rationale":"...","evidence_ids":[]}')
        lines.append("Do not use a `vote` key.")
        lines.append("Follow the exact answer-format instruction in the question.")
    return "\n".join(lines), trace


def response_format_for_item(item: dict) -> dict:
    common_properties = {
        "confidence": {
            "type": "number",
            "description": "Confidence from 0 through 1.",
        },
        "rationale": {
            "type": "string",
            "description": "A concise explanation of the answer.",
        },
        "evidence_ids": {
            "type": "array",
            "items": {"type": "string"},
            "description": "Identifiers for supporting record evidence, or an empty array when the item has no record.",
        },
    }
    if item.get("mode") == "tool_record":
        properties = {
            "vote": {
                "type": "string",
                "enum": item.get("vote_options") or ["demonstrated", "not_demonstrated", "indeterminate"],
                "description": "The decision supported by the record.",
            },
            **common_properties,
        }
        required = ["vote", "confidence", "rationale", "evidence_ids"]
        name = "record_decision"
    else:
        answer = {
            "type": "string",
            "description": "The answer in the format required by the item.",
        }
        if item.get("answer_type") == "multiple_choice":
            answer["enum"] = ["A", "B", "C", "D"]
        properties = {"answer": answer, **common_properties}
        required = ["answer", "confidence", "rationale", "evidence_ids"]
        name = "item_answer"
    return {
        "type": "json_schema",
        "json_schema": {
            "name": name,
            "strict": True,
            "schema": {
                "type": "object",
                "properties": properties,
                "required": required,
                "additionalProperties": False,
            },
        },
    }


def mock_response(item: dict, mode: str) -> dict:
    if item["mode"] == "tool_record":
        vote = item["gold"]["vote"] if mode == "perfect" else "indeterminate"
        return {"vote": vote, "confidence": 0.91, "rationale": "The cited record supports this vote.", "evidence_ids": item.get("required_citations", []) if mode == "perfect" else []}
    answer = item["gold"] if mode == "perfect" else "A"
    conf = item.get("rubric", {}).get("instruction_checks", {}).get("confidence_exact", 0.80)
    return {"answer": answer, "confidence": conf, "rationale": "The response follows the requested format.", "evidence_ids": []}


def classify_error(exc: Exception) -> str:
    text = str(exc).lower()
    if isinstance(exc, TimeoutError) or isinstance(exc, socket.timeout) or "timed out" in text or "timeout" in text:
        return "timeout"
    if "context" in text and ("limit" in text or "length" in text or "too long" in text):
        return "context_limit"
    if "rate limit" in text or "429" in text:
        return "rate_limit"
    if "openrouter http" in text or "openrouter response" in text or "openrouter batch" in text or "http error" in text or "http " in text:
        return "provider_error"
    if "api_key" in text or "authorization" in text or "auth" in text:
        return "credential_error"
    return "runner_error"


def openrouter_json_request(
    url: str,
    method: str,
    timeout: int,
    payload: dict | None = None,
    extra_headers: dict | None = None,
) -> tuple[int, dict]:
    key = load_openrouter_key()
    if not key:
        raise RuntimeError("OPENROUTER_API_KEY not found in environment or secrets/openrouter.api.txt")
    data = json.dumps(payload).encode() if payload is not None else None
    headers = {
        "Authorization": f"Bearer {key}",
        "HTTP-Referer": "https://github.com/agentcourt/adj",
        "X-Title": "adj-model-pool",
    }
    if payload is not None:
        headers["Content-Type"] = "application/json"
    if extra_headers:
        headers.update(extra_headers)
    req = urllib.request.Request(
        url,
        data=data,
        headers=headers,
        method=method,
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = json.loads(resp.read().decode())
            if not isinstance(body, dict):
                raise RuntimeError(f"OpenRouter response must be an object: {type(body).__name__}")
            return int(resp.status), body
    except urllib.error.HTTPError as e:
        detail = e.read().decode(errors="replace")
        try:
            body_json = json.loads(detail)
        except json.JSONDecodeError:
            body_json = None
        retry_after_seconds = None
        retry_after = e.headers.get("Retry-After") if e.headers is not None else None
        if retry_after:
            try:
                retry_after_seconds = float(retry_after)
            except ValueError:
                pass
        raise OpenRouterHTTPError(e.code, detail, body_json, retry_after_seconds) from e


def openrouter_request(payload: dict, timeout: int, extra_headers: dict | None = None) -> dict:
    _, body = openrouter_json_request(
        "https://openrouter.ai/api/v1/chat/completions",
        "POST",
        timeout,
        payload,
        extra_headers,
    )
    return body


def openrouter_generation_metadata(generation_id: str, timeout: int) -> tuple[dict | None, str]:
    generation_id = generation_id.strip()
    if not generation_id:
        return None, ""
    key = load_openrouter_key()
    if not key:
        return None, "OPENROUTER_API_KEY not found"
    url = "https://openrouter.ai/api/v1/generation?id=" + urllib.parse.quote(generation_id)
    req = urllib.request.Request(
        url,
        headers={"Authorization": f"Bearer {key}", "Accept": "application/json"},
        method="GET",
    )
    try:
        with urllib.request.urlopen(req, timeout=min(timeout, 30)) as resp:
            body = json.loads(resp.read().decode())
            return body if isinstance(body, dict) else {"raw": body}, ""
    except urllib.error.HTTPError as e:
        detail = e.read().decode(errors="replace")[:1000]
        return None, f"OpenRouter generation HTTP {e.code}: {detail}"
    except Exception as e:
        return None, str(e)


def response_meta(body: dict, started: float) -> tuple[str, dict, dict]:
    elapsed_ms = round((time.time() - started) * 1000)
    if body.get("error") is not None:
        detail = json.dumps(body["error"], ensure_ascii=False, sort_keys=True)[:1000]
        raise OpenRouterResponseError(detail, body)
    choices = body.get("choices")
    if not isinstance(choices, list) or not choices:
        snippet = json.dumps(body, ensure_ascii=False, sort_keys=True)[:1000]
        raise OpenRouterResponseError(f"missing choices: {snippet}", body)
    choice = choices[0]
    if not isinstance(choice, dict):
        raise OpenRouterResponseError("first choice is not an object", body)
    if choice.get("finish_reason") == "error" or choice.get("error") is not None:
        detail = choice.get("error") or {"finish_reason": "error"}
        raise OpenRouterResponseError(json.dumps(detail, ensure_ascii=False, sort_keys=True)[:1000], body)
    message = choice.get("message")
    if not isinstance(message, dict):
        raise OpenRouterResponseError("first choice has no message object", body)
    usage = body.get("usage") or {}
    if not isinstance(usage, dict):
        raise OpenRouterResponseError("usage is not an object", body)
    content = message.get("content")
    if content is not None and not isinstance(content, str):
        raise OpenRouterResponseError("message content is neither a string nor null", body)
    meta = {
        "provider_model": body.get("model"),
        "elapsed_ms": elapsed_ms,
        "usage": usage,
        "cost": usage.get("cost") or body.get("usage_cost") or body.get("cost"),
        "finish_reason": choice.get("finish_reason"),
        "response_id": body.get("id"),
        "openrouter_metadata": body.get("openrouter_metadata"),
        "raw_openrouter_response": body,
    }
    return content or "", meta, message


def numeric_value(value) -> int | float | None:
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, (int, float)):
        return value
    if isinstance(value, str):
        try:
            return float(value)
        except ValueError:
            return None
    return None


def accumulate_usage(total: dict, current: dict) -> dict:
    for key, value in current.items():
        if isinstance(value, dict):
            existing = total.get(key)
            if not isinstance(existing, dict):
                existing = {}
            total[key] = accumulate_usage(existing, value)
        elif numeric_value(value) is not None:
            existing = total.get(key)
            existing_number = numeric_value(existing)
            value_number = numeric_value(value)
            if existing_number is not None:
                total[key] = existing_number + value_number
            else:
                total[key] = value_number
        else:
            total[key] = value
    return total


def attach_openrouter_error_meta(meta: dict, exc: Exception) -> None:
    if isinstance(exc, OpenRouterHTTPError):
        meta["openrouter_status_code"] = exc.status_code
        if exc.body_json is not None:
            meta["raw_openrouter_error"] = exc.body_json
            if isinstance(exc.body_json, dict) and "openrouter_metadata" in exc.body_json:
                meta["openrouter_metadata"] = exc.body_json.get("openrouter_metadata")
        else:
            meta["raw_openrouter_error_text"] = exc.detail
    elif isinstance(exc, OpenRouterResponseError):
        meta["raw_openrouter_error_response"] = exc.body
    elif isinstance(exc, OpenRouterBatchTerminalError):
        meta["openrouter_batch_id"] = exc.batch.get("id")
        meta["openrouter_batch_status"] = exc.batch.get("status")
        meta["openrouter_batch_request_counts"] = exc.batch.get("request_counts")
        meta["raw_openrouter_batch_error"] = exc.batch.get("error")
    elif isinstance(exc, OpenRouterBatchItemError):
        meta["raw_openrouter_batch_error"] = exc.result.get("error")
        meta["raw_openrouter_batch_response"] = exc.result.get("response")


def openrouter_payload(spec: dict, messages: list[dict], default_max_tokens: int, extra: dict | None = None) -> dict:
    request = spec.get("request") or {}
    payload = {
        "model": spec["openrouter_model_id"],
        "messages": messages,
        "temperature": request.get("temperature", 0),
        "max_tokens": int(request.get("max_tokens", default_max_tokens)),
    }
    if "top_p" in request:
        payload["top_p"] = request["top_p"]
    if spec.get("provider"):
        payload["provider"] = spec["provider"]
    if extra:
        payload.update(extra)
    return payload


def is_batch_spec(spec: dict) -> bool:
    return str(spec.get("openrouter_model_id") or "").endswith(":batch")


def batch_model_id(spec: dict) -> str:
    model_id = str(spec.get("openrouter_model_id") or "")
    if not model_id.endswith(":batch"):
        raise ValueError(f"batch model id must end with :batch: {model_id}")
    return model_id.removesuffix(":batch")


def batch_event(kind: str, **data) -> None:
    print(
        json.dumps(
            {
                "at": dt.datetime.now(dt.timezone.utc).isoformat(),
                "kind": kind,
                **data,
            },
            sort_keys=True,
        ),
        flush=True,
    )


def openrouter_batch(
    spec: dict,
    requests: list[dict],
    timeout: int,
    round_index: int,
    poll_seconds: float = 10.0,
    max_poll_failures: int = 5,
) -> dict:
    if not requests:
        raise ValueError("OpenRouter batch requires at least one request")
    model_id = batch_model_id(spec)
    payload = {
        "endpoint": "/v1/chat/completions",
        "model": model_id,
        "requests": requests,
    }
    status_code, batch = openrouter_json_request(
        "https://openrouter.ai/api/beta/batches",
        "POST",
        timeout,
        payload,
        spec.get("headers"),
    )
    if status_code != 202:
        raise RuntimeError(f"OpenRouter batch submission returned HTTP {status_code}")
    batch_id = str(batch.get("id") or "")
    if not batch_id:
        raise RuntimeError("OpenRouter batch submission returned no batch id")

    terminal = {"completed", "failed", "expired", "cancelled"}
    statuses = terminal | {"validating", "in_progress", "finalizing", "cancelling"}
    poll_failures = 0
    next_poll_delay = poll_seconds
    while True:
        status = str(batch.get("status") or "")
        if not status:
            raise RuntimeError(f"OpenRouter batch {batch_id} returned no status")
        if status not in statuses:
            raise RuntimeError(f"OpenRouter batch {batch_id} returned unknown status {status!r}")
        batch_event(
            "openrouter_batch_status",
            batch_id=batch_id,
            batch_round=round_index,
            status=status,
            request_counts=batch.get("request_counts"),
            usage=batch.get("usage"),
        )
        if status in terminal:
            break
        time.sleep(next_poll_delay)
        next_poll_delay = poll_seconds
        try:
            poll_status, batch = openrouter_json_request(
                "https://openrouter.ai/api/beta/batches/" + urllib.parse.quote(batch_id, safe=""),
                "GET",
                timeout,
            )
        except OpenRouterHTTPError as exc:
            if exc.status_code not in {429, 503}:
                raise OpenRouterBatchPollError(batch_id, exc) from exc
            poll_failures += 1
            if poll_failures >= max_poll_failures:
                raise OpenRouterBatchPollError(batch_id, exc) from exc
            delay = exc.retry_after_seconds if exc.retry_after_seconds and exc.retry_after_seconds > 0 else poll_seconds
            batch_event(
                "openrouter_batch_poll_retry",
                batch_id=batch_id,
                batch_round=round_index,
                failure_count=poll_failures,
                status_code=exc.status_code,
                retry_after_seconds=delay,
            )
            next_poll_delay = delay
            continue
        except (OSError, TimeoutError) as exc:
            poll_failures += 1
            if poll_failures >= max_poll_failures:
                raise OpenRouterBatchPollError(batch_id, exc) from exc
            batch_event(
                "openrouter_batch_poll_retry",
                batch_id=batch_id,
                batch_round=round_index,
                failure_count=poll_failures,
                error=str(exc),
                retry_after_seconds=poll_seconds,
            )
            continue
        poll_failures = 0
        if poll_status != 200:
            raise RuntimeError(f"OpenRouter batch retrieval returned HTTP {poll_status}")
        if str(batch.get("id") or "") != batch_id:
            raise RuntimeError(f"OpenRouter batch retrieval returned the wrong batch id for {batch_id}")

    if status != "completed":
        raise OpenRouterBatchTerminalError(batch)
    if not isinstance(batch.get("results"), list):
        raise RuntimeError(f"OpenRouter batch {batch_id} completed without a results array")
    return batch


def indexed_batch_results(batch: dict, expected_ids: set[str]) -> dict[str, dict]:
    indexed: dict[str, dict] = {}
    for result in batch["results"]:
        if not isinstance(result, dict):
            raise RuntimeError(f"OpenRouter batch {batch.get('id')} returned a non-object result")
        custom_id = str(result.get("custom_id") or "")
        if custom_id not in expected_ids:
            raise RuntimeError(f"OpenRouter batch {batch.get('id')} returned unexpected custom_id {custom_id!r}")
        if custom_id in indexed:
            raise RuntimeError(f"OpenRouter batch {batch.get('id')} returned duplicate custom_id {custom_id!r}")
        indexed[custom_id] = result
    missing = sorted(expected_ids - set(indexed))
    if missing:
        raise RuntimeError(f"OpenRouter batch {batch.get('id')} omitted custom_ids: {', '.join(missing)}")
    return indexed


def batch_result_body(result: dict) -> tuple[dict, dict]:
    if result.get("error") is not None:
        raise OpenRouterBatchItemError(result)
    response = result.get("response")
    if not isinstance(response, dict):
        raise OpenRouterBatchItemError(result)
    status_code = response.get("status_code")
    if not isinstance(status_code, int) or status_code < 200 or status_code >= 300:
        raise OpenRouterBatchItemError(result)
    body = response.get("body")
    if not isinstance(body, dict):
        raise OpenRouterBatchItemError(result)
    return body, {
        "openrouter_batch_request_id": result.get("id"),
        "openrouter_request_id": response.get("request_id"),
        "openrouter_batch_status_code": status_code,
    }


def attach_request_spec_meta(meta: dict, spec: dict, timeout: int, fetch_generation: bool = True) -> dict:
    meta.update(
        {
            "model_spec_label": spec["label"],
            "openrouter_model_id": spec["openrouter_model_id"],
            "exact_variant": spec.get("exact_variant", False),
            "requested_provider_constraints": spec.get("provider"),
            "requested_quantization_constraints": (spec.get("provider") or {}).get("quantizations"),
            "allow_fallbacks": (spec.get("provider") or {}).get("allow_fallbacks"),
            "require_parameters": (spec.get("provider") or {}).get("require_parameters"),
            "request_parameters": spec.get("request") or {},
            "variant_metadata": spec.get("variant_metadata") or {},
        }
    )
    response_id = meta.get("response_id")
    if fetch_generation and spec.get("exact_variant") and isinstance(response_id, str) and response_id:
        generation, error = openrouter_generation_metadata(response_id, timeout)
        if generation is not None:
            meta["openrouter_generation"] = generation
        if error:
            meta["openrouter_generation_error"] = error
    return meta


def call_openrouter(spec: dict, item: dict, prompt: str, timeout: int) -> tuple[str, dict, list[dict]]:
    messages = [
        {"role": "system", "content": "Return only strict JSON matching the requested schema. No markdown."},
        {"role": "user", "content": prompt},
    ]
    payload = openrouter_payload(spec, messages, 1000, {"response_format": response_format_for_item(item)})
    started = time.time()
    body = openrouter_request(payload, timeout, spec.get("headers"))
    content, meta, _ = response_meta(body, started)
    meta = attach_request_spec_meta(meta, spec, timeout)
    return content, meta, []


def execute_tool(record_dir: str, name: str, args: dict) -> dict:
    if name == "list_evidence":
        return {"evidence": list_evidence(record_dir)}
    if name == "read_evidence":
        return read_evidence(record_dir, args.get("evidence_id", ""))
    if name == "stat_evidence":
        return stat_evidence(record_dir, args.get("evidence_id", ""))
    raise RuntimeError(f"unknown tool: {name}")


def validated_tool_calls(message: dict, finish_reason, body: dict) -> list[dict]:
    raw_calls = message.get("tool_calls")
    if raw_calls is None:
        raw_calls = []
    if not isinstance(raw_calls, list):
        raise OpenRouterResponseError("message tool_calls is not an array", body)
    if finish_reason == "tool_calls" and not raw_calls:
        raise OpenRouterResponseError("finish_reason is tool_calls but message has no tool calls", body)
    if finish_reason != "tool_calls" and raw_calls:
        raise OpenRouterResponseError(
            f"message has tool calls with finish_reason {finish_reason!r}",
            body,
        )

    calls = []
    seen_ids = set()
    for index, raw_call in enumerate(raw_calls):
        if not isinstance(raw_call, dict):
            raise OpenRouterResponseError(f"tool call {index} is not an object", body)
        call_id = raw_call.get("id")
        if not isinstance(call_id, str) or not call_id:
            raise OpenRouterResponseError(f"tool call {index} has no id", body)
        if call_id in seen_ids:
            raise OpenRouterResponseError(f"duplicate tool call id {call_id!r}", body)
        seen_ids.add(call_id)
        if raw_call.get("type") != "function":
            raise OpenRouterResponseError(f"tool call {call_id!r} has type {raw_call.get('type')!r}", body)
        function = raw_call.get("function")
        if not isinstance(function, dict):
            raise OpenRouterResponseError(f"tool call {call_id!r} has no function object", body)
        name = function.get("name")
        if name not in {"list_evidence", "read_evidence", "stat_evidence"}:
            raise OpenRouterResponseError(f"tool call {call_id!r} names unknown tool {name!r}", body)
        encoded_arguments = function.get("arguments")
        if not isinstance(encoded_arguments, str):
            raise OpenRouterResponseError(f"tool call {call_id!r} arguments is not a JSON string", body)
        try:
            arguments = json.loads(encoded_arguments)
        except json.JSONDecodeError as exc:
            raise OpenRouterResponseError(f"tool call {call_id!r} arguments is invalid JSON: {exc}", body) from exc
        if not isinstance(arguments, dict):
            raise OpenRouterResponseError(f"tool call {call_id!r} arguments is not an object", body)
        if name == "list_evidence":
            if arguments:
                raise OpenRouterResponseError(f"tool call {call_id!r} list_evidence arguments must be empty", body)
        elif set(arguments) != {"evidence_id"} or not isinstance(arguments["evidence_id"], str) or not arguments["evidence_id"]:
            raise OpenRouterResponseError(
                f"tool call {call_id!r} {name} arguments must contain one nonempty evidence_id string",
                body,
            )
        calls.append({"id": call_id, "name": name, "arguments": arguments})
    return calls


def call_openrouter_tools(spec: dict, item: dict, prompt: str, timeout: int, max_rounds: int = 6) -> tuple[str, dict, list[dict]]:
    messages = [
        {"role": "system", "content": "Use tools when required. Return only strict JSON matching the requested schema as the final answer. No markdown."},
        {"role": "user", "content": prompt},
    ]
    trace: list[dict] = []
    meta: dict = {"tool_rounds": 0, "tool_call_count": 0, "tool_error_count": 0}
    usage: dict = {}
    total_cost = 0.0
    cost_seen = False
    started = time.time()
    for _ in range(max_rounds):
        payload = openrouter_payload(spec, messages, 1200, {
            "tools": TOOL_DEFS,
            "tool_choice": "auto",
            "response_format": response_format_for_item(item),
        })
        try:
            body = openrouter_request(payload, timeout, spec.get("headers"))
            content, call_meta, message = response_meta(body, started)
        except Exception as e:
            raise ToolRoundError(e, meta, trace) from e
        call_usage = call_meta.get("usage")
        if isinstance(call_usage, dict):
            accumulate_usage(usage, call_usage)
        call_cost = numeric_value(call_meta.get("cost"))
        if call_cost is not None:
            total_cost += float(call_cost)
            cost_seen = True
        meta.update(attach_request_spec_meta(call_meta, spec, timeout, fetch_generation=False))
        meta["usage"] = usage
        meta["cost"] = total_cost if cost_seen else None
        try:
            tool_calls = validated_tool_calls(message, call_meta.get("finish_reason"), body)
        except OpenRouterResponseError as exc:
            meta["tool_error_count"] = int(meta.get("tool_error_count", 0)) + 1
            raise ToolRoundError(exc, meta, trace) from exc
        if not tool_calls:
            meta["tool_rounds"] = meta.get("tool_rounds", 0)
            return content or "", meta, trace
        messages.append(message)
        meta["tool_rounds"] = int(meta.get("tool_rounds", 0)) + 1
        meta["tool_call_count"] = int(meta.get("tool_call_count", 0)) + len(tool_calls)
        for tool_call in tool_calls:
            name = tool_call["name"]
            args = tool_call["arguments"]
            try:
                result = execute_tool(item["record_dir"], name, args)
                result_for_trace = result
            except Exception as e:
                result = {"error": str(e)}
                result_for_trace = result
                meta["tool_error_count"] = int(meta.get("tool_error_count", 0)) + 1
            trace.append({"tool": name, "args": args, "result": result_for_trace})
            messages.append({"role": "tool", "tool_call_id": tool_call["id"], "content": json.dumps(result, ensure_ascii=False)})
    raise ToolRoundError(RuntimeError("tool loop exceeded max rounds before final answer"), meta, trace)


def batch_request_body(spec: dict, task: dict) -> dict:
    extra = {"response_format": response_format_for_item(task["item"])}
    default_max_tokens = 1000
    if task["uses_tools"]:
        default_max_tokens = 1200
        extra.update({"tools": TOOL_DEFS, "tool_choice": "auto"})
    payload = openrouter_payload(spec, task["messages"], default_max_tokens, extra)
    payload.pop("model")
    return payload


def finish_batch_task_error(task: dict, exc: Exception) -> None:
    task["raw"] = ""
    task["done"] = True
    task["meta"].update(
        {
            "runner": "openrouter",
            "elapsed_ms": round((time.time() - task["started"]) * 1000),
            "error": str(exc),
            "error_type": classify_error(exc),
        }
    )
    attach_openrouter_error_meta(task["meta"], exc)


def run_openrouter_batch_spec(
    spec: dict,
    items: list[dict],
    base_prompt: str,
    tool_mode: str,
    trials: int,
    timeout: int,
    max_rounds: int = 6,
) -> list[dict]:
    if not items:
        raise ValueError("batch evaluation requires at least one item")
    tasks: list[dict] = []
    request_number = 0
    for trial_index in range(1, trials + 1):
        for item in items:
            request_number += 1
            prompt, trace = item_prompt(item, base_prompt, tool_mode)
            uses_tools = item.get("mode") == "tool_record" and tool_mode == "function"
            system = "Return only strict JSON matching the requested schema. No markdown."
            if uses_tools:
                system = "Use tools when required. Return only strict JSON matching the requested schema as the final answer. No markdown."
            meta = {
                "created_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                "prompt_chars": len(prompt),
                "trial_index": trial_index,
                "error": "",
                "error_type": "",
                "runner": "openrouter",
                "tool_mode": tool_mode,
            }
            if uses_tools:
                meta.update({"tool_rounds": 0, "tool_call_count": 0, "tool_error_count": 0})
            attach_request_spec_meta(meta, spec, timeout, fetch_generation=False)
            tasks.append(
                {
                    "custom_id": f"request-{request_number:06d}",
                    "item": item,
                    "trial_index": trial_index,
                    "messages": [
                        {"role": "system", "content": system},
                        {"role": "user", "content": prompt},
                    ],
                    "trace": trace,
                    "meta": meta,
                    "usage": {},
                    "uses_tools": uses_tools,
                    "started": time.time(),
                    "raw": "",
                    "done": False,
                    "batch_ids": [],
                }
            )

    pending = list(tasks)
    batch_ids: list[str] = []
    batch_usage: dict = {}
    for round_index in range(1, max_rounds + 1):
        requests = [
            {"custom_id": task["custom_id"], "body": batch_request_body(spec, task)}
            for task in pending
        ]
        try:
            batch = openrouter_batch(spec, requests, timeout, round_index)
        except OpenRouterBatchTerminalError as exc:
            if isinstance(exc.batch.get("usage"), dict):
                accumulate_usage(batch_usage, exc.batch["usage"])
            batch_id = str(exc.batch.get("id") or "")
            if batch_id:
                batch_ids.append(batch_id)
            for task in pending:
                if batch_id:
                    task["batch_ids"].append(batch_id)
                finish_batch_task_error(task, exc)
            pending = []
            break
        except OpenRouterHTTPError as exc:
            for task in pending:
                finish_batch_task_error(task, exc)
            pending = []
            break

        batch_id = str(batch["id"])
        batch_ids.append(batch_id)
        for task in pending:
            task["batch_ids"].append(batch_id)
        if isinstance(batch.get("usage"), dict):
            accumulate_usage(batch_usage, batch["usage"])
        indexed = indexed_batch_results(batch, {task["custom_id"] for task in pending})
        next_pending: list[dict] = []
        for task in pending:
            result = indexed[task["custom_id"]]
            response = result.get("response") if isinstance(result.get("response"), dict) else {}
            task["meta"].update(
                {
                    "openrouter_batch_id": batch_id,
                    "openrouter_batch_round": round_index,
                    "openrouter_batch_request_id": result.get("id"),
                    "openrouter_request_id": response.get("request_id"),
                    "openrouter_batch_status_code": response.get("status_code"),
                }
            )
            try:
                body, result_meta = batch_result_body(result)
                content, call_meta, message = response_meta(body, task["started"])
                call_meta = attach_request_spec_meta(call_meta, spec, timeout, fetch_generation=False)
                call_usage = call_meta.get("usage")
                if isinstance(call_usage, dict):
                    accumulate_usage(task["usage"], call_usage)
                task["meta"].update(call_meta)
                task["meta"].update(result_meta)
                task["meta"].update(
                    {
                        "openrouter_batch_id": batch_id,
                        "openrouter_batch_round": round_index,
                        "usage": task["usage"],
                        "cost": None,
                    }
                )

                tool_calls = validated_tool_calls(message, call_meta.get("finish_reason"), body)
                if tool_calls and not task["uses_tools"]:
                    raise OpenRouterResponseError("model returned a tool call when no tools were requested", body)
                if not tool_calls:
                    task["raw"] = content or ""
                    task["done"] = True
                    continue

                task["messages"].append(message)
                task["meta"]["tool_rounds"] = int(task["meta"].get("tool_rounds", 0)) + 1
                task["meta"]["tool_call_count"] = int(task["meta"].get("tool_call_count", 0)) + len(tool_calls)
                for tool_call in tool_calls:
                    name = tool_call["name"]
                    arguments = tool_call["arguments"]
                    try:
                        tool_result = execute_tool(task["item"]["record_dir"], name, arguments)
                    except Exception as exc:
                        tool_result = {"error": str(exc)}
                        task["meta"]["tool_error_count"] = int(task["meta"].get("tool_error_count", 0)) + 1
                    task["trace"].append({"tool": name, "args": arguments, "result": tool_result})
                    task["messages"].append(
                        {
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "content": json.dumps(tool_result, ensure_ascii=False),
                        }
                    )
                if round_index == max_rounds:
                    finish_batch_task_error(task, RuntimeError("tool loop exceeded max rounds before final answer"))
                else:
                    next_pending.append(task)
            except (OpenRouterBatchItemError, OpenRouterResponseError) as exc:
                if isinstance(exc, OpenRouterResponseError) and task["uses_tools"]:
                    task["meta"]["tool_error_count"] = int(task["meta"].get("tool_error_count", 0)) + 1
                finish_batch_task_error(task, exc)
        pending = next_pending
        if not pending:
            break

    if pending:
        raise RuntimeError("batch tool loop ended with pending requests")
    if any(not task["done"] for task in tasks):
        raise RuntimeError("batch evaluation ended with incomplete requests")

    for task in tasks:
        task["meta"]["openrouter_batch_ids"] = task["batch_ids"]
        task["meta"]["openrouter_batch_count"] = len(task["batch_ids"])
        task["meta"]["cost"] = None
        if batch_usage:
            task["meta"]["openrouter_batch_usage"] = batch_usage
            task["meta"]["openrouter_batch_usage_batch_ids"] = batch_ids

    model = spec["label"]
    return [
        {
            "item_id": task["item"]["id"],
            "model": model,
            "trial_index": task["trial_index"],
            "raw_response": task["raw"],
            "parsed_response": parse_maybe_json(task["raw"]) if task["raw"] else None,
            "tool_trace": task["trace"],
            "metadata": task["meta"],
        }
        for task in tasks
    ]


def parse_maybe_json(text: str):
    try:
        return json.loads(text)
    except Exception:
        return None


def hydrate_posthoc_generation_metadata(results: list[dict], timeout: int, attempts: int = 5, delay_seconds: float = 3.0) -> None:
    """Fetch delayed OpenRouter generation metadata after the run has completed.

    OpenRouter may return 404 for `/generation` immediately after chat completion
    even when the generation id is valid. Retry briefly so exact-variant runs can
    preserve post-hoc provider, usage, cost, latency, and upstream metadata.
    """
    rows_by_id: dict[str, list[dict]] = {}
    for row in results:
        meta = row.get("metadata") or {}
        if not meta.get("exact_variant") or meta.get("openrouter_generation"):
            continue
        response_id = meta.get("response_id")
        if isinstance(response_id, str) and response_id:
            rows_by_id.setdefault(response_id, []).append(row)

    pending = set(rows_by_id)
    last_errors: dict[str, str] = {}
    for attempt in range(1, attempts + 1):
        for response_id in list(pending):
            generation, error = openrouter_generation_metadata(response_id, timeout)
            if generation is not None:
                for row in rows_by_id[response_id]:
                    meta = row["metadata"]
                    meta["openrouter_generation"] = generation
                    meta.pop("openrouter_generation_error", None)
                pending.remove(response_id)
            elif error:
                last_errors[response_id] = error
        if not pending or attempt == attempts:
            break
        time.sleep(delay_seconds)

    for response_id in pending:
        for row in rows_by_id[response_id]:
            row["metadata"]["openrouter_generation_error"] = last_errors.get(response_id, "generation metadata unavailable")


def main() -> int:
    ap = argparse.ArgumentParser(description="Run adj model-pool items against mock or OpenRouter models.")
    ap.add_argument("--questions", default="sets/core20/questions.jsonl")
    ap.add_argument("--prompt", default="prompts/juror-single.md", help="Prompt file. Relative paths resolve from model-pool/.")
    ap.add_argument("--models", nargs="+")
    ap.add_argument("--model-spec", action="append", help="JSON file containing one OpenRouter model/variant request spec. May be repeated.")
    ap.add_argument("--model-spec-jsonl", action="append", help="JSONL file containing OpenRouter model/variant request specs. May be repeated.")
    ap.add_argument("--out", required=True)
    ap.add_argument("--limit", type=int)
    ap.add_argument("--item-id", action="append")
    ap.add_argument("--mock", choices=["perfect", "weak"])
    ap.add_argument("--timeout", type=int, default=90)
    ap.add_argument("--tool-mode", choices=["context", "function"], default="context")
    ap.add_argument("--trials", type=int, default=3, help="Number of repeated trials per model/item. Defaults to 3.")
    args = ap.parse_args()

    qpath = Path(args.questions)
    if not qpath.is_absolute():
        qpath = ROOT / qpath
    prompt_path = Path(args.prompt)
    if not prompt_path.is_absolute():
        prompt_path = ROOT / prompt_path
    try:
        base_prompt = prompt_path.read_text()
    except OSError as e:
        raise SystemExit(f"cannot read prompt file {prompt_path}: {e}") from e
    if not base_prompt.strip():
        raise SystemExit(f"prompt file is empty: {prompt_path}")
    items = load_items(qpath)
    if args.item_id:
        wanted = set(args.item_id)
        items = [i for i in items if i["id"] in wanted]
    if args.limit:
        items = items[:args.limit]
    if not items:
        raise SystemExit("no evaluation items selected")

    out = Path(args.out)
    if not out.is_absolute():
        out = ROOT / out
    out.mkdir(parents=True, exist_ok=True)
    raw_path = out / "raw_results.jsonl"
    if raw_path.exists():
        raw_path.unlink()
    results = []

    if args.trials < 1:
        raise SystemExit("--trials must be at least 1")

    try:
        model_specs = load_model_specs(args.models, args.model_spec, args.model_spec_jsonl)
    except Exception as e:
        raise SystemExit(str(e)) from e

    for spec in model_specs:
        model = spec["label"]
        if not args.mock and not model.startswith("mock:") and is_batch_spec(spec):
            batch_rows = run_openrouter_batch_spec(
                spec,
                items,
                base_prompt,
                args.tool_mode,
                args.trials,
                args.timeout,
            )
            results.extend(batch_rows)
            with raw_path.open("a") as f:
                for row in batch_rows:
                    f.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")
            continue
        for trial_index in range(1, args.trials + 1):
            for item in items:
                prompt, trace = item_prompt(item, base_prompt, args.tool_mode)
                meta = {"created_at": dt.datetime.now(dt.timezone.utc).isoformat(), "prompt_chars": len(prompt), "trial_index": trial_index, "error": "", "error_type": ""}
                attach_request_spec_meta(meta, spec, args.timeout)
                if args.mock or model.startswith("mock:"):
                    mode = args.mock or model.split(":", 1)[1]
                    obj = mock_response(item, mode)
                    raw = json.dumps(obj, ensure_ascii=False)
                    meta.update({"runner": "mock", "mock_mode": mode, "elapsed_ms": 0})
                else:
                    started = time.time()
                    try:
                        if item.get("mode") == "tool_record" and args.tool_mode == "function":
                            raw, api_meta, trace = call_openrouter_tools(spec, item, prompt, args.timeout)
                        else:
                            raw, api_meta, api_trace = call_openrouter(spec, item, prompt, args.timeout)
                            if api_trace:
                                trace = api_trace
                        meta.update({"runner": "openrouter", "tool_mode": args.tool_mode, **api_meta})
                    except Exception as e:
                        cause = e
                        if isinstance(e, ToolRoundError):
                            meta.update(e.metadata)
                            trace = e.trace
                            cause = e.cause
                        raw = ""
                        meta.update({
                            "runner": "openrouter",
                            "tool_mode": args.tool_mode,
                            "elapsed_ms": round((time.time() - started) * 1000),
                            "error": str(cause),
                            "error_type": classify_error(cause),
                        })
                        attach_openrouter_error_meta(meta, cause)
                parsed = parse_maybe_json(raw) if raw else None
                row = {"item_id": item["id"], "model": model, "trial_index": trial_index, "raw_response": raw, "parsed_response": parsed, "tool_trace": trace, "metadata": meta}
                results.append(row)
                with raw_path.open("a") as f:
                    f.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")

    hydrate_posthoc_generation_metadata(results, args.timeout)
    with raw_path.open("w") as f:
        for row in results:
            f.write(json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n")

    print(json.dumps({"run": str(out), "results": len(results)}, sort_keys=True))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
