#!/usr/bin/env -S uv run
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
    def __init__(self, status_code: int, detail: str, body_json=None):
        super().__init__(f"OpenRouter HTTP {status_code}: {detail}")
        self.status_code = status_code
        self.detail = detail
        self.body_json = body_json

TOOL_DEFS = [
    {
        "type": "function",
        "function": {
            "name": "list_evidence",
            "description": "List evidence ids and titles in the bounded record.",
            "parameters": {"type": "object", "properties": {}, "additionalProperties": False},
        },
    },
    {
        "type": "function",
        "function": {
            "name": "read_evidence",
            "description": "Read one evidence item by id.",
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
    if "openrouter http" in text or "openrouter response" in text or "http error" in text or "http " in text:
        return "provider_error"
    if "api_key" in text or "authorization" in text or "auth" in text:
        return "credential_error"
    return "runner_error"


def openrouter_request(payload: dict, timeout: int, extra_headers: dict | None = None) -> dict:
    key = load_openrouter_key()
    if not key:
        raise RuntimeError("OPENROUTER_API_KEY not found in environment or secrets/openrouter.api.txt")
    data = json.dumps(payload).encode()
    headers = {
        "Authorization": f"Bearer {key}",
        "Content-Type": "application/json",
        "HTTP-Referer": "https://github.com/agentcourt/adj",
        "X-Title": "adj-model-pool",
    }
    if extra_headers:
        headers.update(extra_headers)
    req = urllib.request.Request(
        "https://openrouter.ai/api/v1/chat/completions",
        data=data,
        headers=headers,
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        detail = e.read().decode(errors="replace")
        try:
            body_json = json.loads(detail)
        except json.JSONDecodeError:
            body_json = None
        raise OpenRouterHTTPError(e.code, detail, body_json) from e


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
    choices = body.get("choices")
    if not isinstance(choices, list) or not choices:
        snippet = json.dumps(body, ensure_ascii=False, sort_keys=True)[:1000]
        raise RuntimeError(f"OpenRouter response missing choices: {snippet}")
    choice = choices[0]
    message = choice.get("message", {})
    usage = body.get("usage", {}) or {}
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
    return message.get("content", ""), meta, message


def attach_openrouter_error_meta(meta: dict, exc: Exception) -> None:
    if not isinstance(exc, OpenRouterHTTPError):
        return
    meta["openrouter_status_code"] = exc.status_code
    if exc.body_json is not None:
        meta["raw_openrouter_error"] = exc.body_json
        if isinstance(exc.body_json, dict) and "openrouter_metadata" in exc.body_json:
            meta["openrouter_metadata"] = exc.body_json.get("openrouter_metadata")
    else:
        meta["raw_openrouter_error_text"] = exc.detail


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


def attach_request_spec_meta(meta: dict, spec: dict, timeout: int) -> dict:
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
    if spec.get("exact_variant") and isinstance(response_id, str) and response_id:
        generation, error = openrouter_generation_metadata(response_id, timeout)
        if generation is not None:
            meta["openrouter_generation"] = generation
        if error:
            meta["openrouter_generation_error"] = error
    return meta


def call_openrouter(spec: dict, prompt: str, timeout: int) -> tuple[str, dict, list[dict]]:
    messages = [
        {"role": "system", "content": "Return only strict JSON matching the requested schema. No markdown."},
        {"role": "user", "content": prompt},
    ]
    payload = openrouter_payload(spec, messages, 1000, {"response_format": {"type": "json_object"}})
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


def call_openrouter_tools(spec: dict, item: dict, prompt: str, timeout: int, max_rounds: int = 6) -> tuple[str, dict, list[dict]]:
    messages = [
        {"role": "system", "content": "Use tools when required. Return only strict JSON matching the requested schema as the final answer. No markdown."},
        {"role": "user", "content": prompt},
    ]
    trace: list[dict] = []
    meta: dict = {"tool_rounds": 0, "tool_call_count": 0, "tool_error_count": 0}
    started = time.time()
    for _ in range(max_rounds):
        payload = openrouter_payload(spec, messages, 1200, {
            "tools": TOOL_DEFS,
            "tool_choice": "auto",
            "response_format": {"type": "json_object"},
        })
        body = openrouter_request(payload, timeout, spec.get("headers"))
        content, call_meta, message = response_meta(body, started)
        meta.update(attach_request_spec_meta(call_meta, spec, timeout))
        tool_calls = message.get("tool_calls") or []
        if not tool_calls:
            meta["tool_rounds"] = meta.get("tool_rounds", 0)
            return content or "", meta, trace
        messages.append(message)
        meta["tool_rounds"] = int(meta.get("tool_rounds", 0)) + 1
        meta["tool_call_count"] = int(meta.get("tool_call_count", 0)) + len(tool_calls)
        for tc in tool_calls:
            fn = tc.get("function", {})
            name = fn.get("name", "")
            try:
                args = json.loads(fn.get("arguments") or "{}")
            except json.JSONDecodeError:
                args = {}
            try:
                result = execute_tool(item["record_dir"], name, args)
                result_for_trace = result
            except Exception as e:
                result = {"error": str(e)}
                result_for_trace = result
                meta["tool_error_count"] = int(meta.get("tool_error_count", 0)) + 1
            trace.append({"tool": name, "args": args, "result": result_for_trace})
            messages.append({"role": "tool", "tool_call_id": tc.get("id"), "name": name, "content": json.dumps(result, ensure_ascii=False)})
    raise RuntimeError("tool loop exceeded max rounds before final answer")


def parse_maybe_json(text: str):
    try:
        return json.loads(text)
    except Exception:
        m = re.search(r"\{.*\}", text, re.S)
        if m:
            try:
                return json.loads(m.group(0))
            except Exception:
                return None
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
                            raw, api_meta, api_trace = call_openrouter(spec, prompt, args.timeout)
                            if api_trace:
                                trace = api_trace
                        meta.update({"runner": "openrouter", "tool_mode": args.tool_mode, **api_meta})
                    except Exception as e:
                        raw = ""
                        meta.update({
                            "runner": "openrouter",
                            "tool_mode": args.tool_mode,
                            "elapsed_ms": round((time.time() - started) * 1000),
                            "error": str(e),
                            "error_type": classify_error(e),
                        })
                        attach_openrouter_error_meta(meta, e)
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
