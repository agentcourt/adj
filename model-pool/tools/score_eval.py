#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import json
import re
import statistics
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DELIBERATION_CATEGORIES = {"basic_human_knowledge", "basic_science_quantitative", "basic_reasoning", "juror_deliberation"}


def load_items(path: Path) -> dict[str, dict]:
    out = {}
    with path.open() as f:
        for line_no, line in enumerate(f, 1):
            if not line.strip():
                continue
            item = json.loads(line)
            if item["id"] in out:
                raise ValueError(f"duplicate item id {item['id']} at line {line_no}")
            out[item["id"]] = item
    return out


def evidence_ids(item: dict) -> set[str]:
    if item.get("mode") != "tool_record":
        return set()
    record_dir = Path(item["record_dir"])
    if not record_dir.is_absolute():
        record_dir = ROOT / record_dir
    manifest = json.loads((record_dir / "manifest.json").read_text())
    return {e["id"] for e in manifest["evidence"]}


def validate_items(path: Path) -> list[str]:
    errors = []
    items = load_items(path)
    categories = {}
    for item in items.values():
        categories[item["category"]] = categories.get(item["category"], 0) + 1
        required = ["id", "category", "capability", "mode", "prompt", "allowed_tools", "answer_type", "gold", "rubric", "source_note"]
        for k in required:
            if k not in item:
                errors.append(f"{item.get('id','<unknown>')}: missing {k}")
        if item.get("answer_type") == "multiple_choice" and len(item.get("choices", [])) != 4:
            errors.append(f"{item['id']}: multiple-choice item must have four choices")
        if item.get("mode") == "tool_record":
            if not item.get("record_dir"):
                errors.append(f"{item['id']}: missing record_dir")
            try:
                ids = evidence_ids(item)
                missing = set(item.get("required_citations", [])) - ids
                if missing:
                    errors.append(f"{item['id']}: missing required evidence ids {sorted(missing)}")
            except Exception as e:
                errors.append(f"{item['id']}: fixture error: {e}")
    rel = path.relative_to(ROOT) if path.is_relative_to(ROOT) else path
    if str(rel) == "sets/core20/questions.jsonl":
        expected = {"basic_human_knowledge": 4, "basic_science_quantitative": 4, "basic_reasoning": 4, "instruction_following": 4, "tool_evidence_use": 4}
        if len(items) != 20:
            errors.append(f"expected 20 items, found {len(items)}")
        if categories != expected:
            errors.append(f"category counts {categories}, expected {expected}")
    elif str(rel) == "sets/deliberation/questions.jsonl":
        expected = {"basic_human_knowledge": 4, "basic_science_quantitative": 4, "basic_reasoning": 4, "juror_deliberation": 8}
        if len(items) != 20:
            errors.append(f"expected 20 deliberation items, found {len(items)}")
        if categories != expected:
            errors.append(f"category counts {categories}, expected {expected}")
    return errors


def sentence_count(text: str) -> int:
    text = text.strip()
    if not text:
        return 0
    return max(1, len(re.findall(r"[.!?]+(?:\s|$)", text)) or 1)


def norm_answer(x) -> str:
    if isinstance(x, str):
        return re.sub(r"\s+", " ", x.strip()).lower()
    return json.dumps(x, sort_keys=True)


def schema_valid(item: dict, resp) -> tuple[bool, str]:
    if not isinstance(resp, dict):
        return False, "response is not an object"
    if item["mode"] == "tool_record":
        keys = {"vote", "confidence", "rationale", "evidence_ids"}
        if set(resp) != keys:
            return False, f"keys {sorted(resp)} != {sorted(keys)}"
        if resp["vote"] not in item.get("vote_options", []):
            return False, "invalid vote"
    else:
        keys = {"answer", "confidence", "rationale", "evidence_ids"}
        if set(resp) != keys:
            return False, f"keys {sorted(resp)} != {sorted(keys)}"
        if not isinstance(resp.get("answer"), str):
            return False, "invalid answer"
    if not isinstance(resp.get("confidence"), (int, float)) or not (0 <= resp["confidence"] <= 1):
        return False, "invalid confidence"
    if not isinstance(resp.get("rationale"), str):
        return False, "invalid rationale"
    if not isinstance(resp.get("evidence_ids"), list) or not all(isinstance(x, str) for x in resp["evidence_ids"]):
        return False, "invalid evidence_ids"
    return True, ""


def instruction_valid(item: dict, resp: dict) -> tuple[bool, list[str]]:
    checks = item.get("rubric", {}).get("instruction_checks", {})
    failures = []
    if not checks:
        return True, failures
    answer = str(resp.get("answer", ""))
    rationale = str(resp.get("rationale", ""))
    full = json.dumps(resp, ensure_ascii=False).lower()
    if "answer_exact" in checks and answer != checks["answer_exact"]:
        failures.append("answer_exact")
    if "answer_regex" in checks and not re.fullmatch(checks["answer_regex"], answer):
        failures.append("answer_regex")
    if "confidence_exact" in checks and abs(float(resp.get("confidence", -1)) - float(checks["confidence_exact"])) > 1e-9:
        failures.append("confidence_exact")
    if checks.get("evidence_ids_empty") and resp.get("evidence_ids") != []:
        failures.append("evidence_ids_empty")
    for term in checks.get("forbid_terms_anywhere", []):
        if term.lower() in full:
            failures.append(f"forbid_terms_anywhere:{term}")
    for term in checks.get("forbid_terms_in_rationale", []):
        if term.lower() in rationale.lower():
            failures.append(f"forbid_terms_in_rationale:{term}")
    if "rationale_max_words" in checks and len(re.findall(r"\b\w+\b", rationale)) > checks["rationale_max_words"]:
        failures.append("rationale_max_words")
    return not failures, failures


def is_deliberation_item(item: dict) -> bool:
    return item.get("category") in DELIBERATION_CATEGORIES or item.get("eval_family") == "deliberation"


def tool_protocol(item: dict, row: dict) -> tuple[bool, bool, bool]:
    if item.get("mode") != "tool_record":
        return True, False, False
    allowed = set(item.get("allowed_tools", []))
    used = {t.get("tool") for t in row.get("tool_trace", [])}
    disallowed = bool(used - allowed)
    missing_required = not ({"list_evidence", "read_evidence"} <= used)
    tool_valid = bool(used) and not disallowed and not missing_required
    return tool_valid, disallowed, missing_required


def letter_match(value, gold: str) -> bool:
    if not isinstance(value, str) or not isinstance(gold, str):
        return False
    v = value.strip().lower()
    g = gold.strip().lower()
    return v == g or bool(re.fullmatch(re.escape(g) + r"(?:[\).:-].*)?", v))


def deliberation_correct(item: dict, resp) -> bool:
    if not isinstance(resp, dict):
        return False
    accepted = item.get("rubric", {}).get("deliberation_accept", [])
    values = []
    if "answer" in resp:
        values.append(resp.get("answer"))
    if "vote" in resp:
        values.append(resp.get("vote"))
    if accepted:
        return any(norm_answer(v) == norm_answer(a) for v in values for a in accepted)
    return any(letter_match(v, item.get("gold")) or norm_answer(v) == norm_answer(item.get("gold")) for v in values)


def score_one(item: dict, row: dict) -> dict:
    resp = row.get("parsed_response")
    meta = row.get("metadata", {})
    trial_index = row.get("trial_index") or meta.get("trial_index") or 1
    sv, schema_error = schema_valid(item, resp)
    if item["mode"] == "tool_record":
        correct = sv and resp.get("vote") == item["gold"]["vote"]
    else:
        correct = sv and norm_answer(resp.get("answer")) == norm_answer(item["gold"])
    max_sent = item.get("rubric", {}).get("rationale_max_sentences", 3)
    rationale_valid = sv and sentence_count(resp.get("rationale", "")) <= max_sent and bool(resp.get("rationale", "").strip())
    instr_ok, instr_failures = instruction_valid(item, resp if isinstance(resp, dict) else {})
    ids = set(resp.get("evidence_ids", [])) if isinstance(resp, dict) else set()
    if item["mode"] == "tool_record":
        known = evidence_ids(item)
        required = set(item.get("required_citations", []))
        evidence_valid = sv and ids <= known and required <= ids
    else:
        evidence_valid = sv and ids == set()
    tool_valid, disallowed_tool_call, missing_required_tool_call = tool_protocol(item, row)
    malformed_json = bool(row.get("raw_response")) and resp is None
    invalid_vote = item["mode"] == "tool_record" and schema_error == "invalid vote"
    truthfulness_flag = not (sv and evidence_valid and rationale_valid)
    response_value = None
    if isinstance(resp, dict):
        response_value = resp.get("answer", resp.get("vote"))
    deliberation_ok = deliberation_correct(item, resp) if is_deliberation_item(item) else bool(correct)
    return {
        "item_id": item["id"],
        "model": row["model"],
        "trial_index": trial_index,
        "category": item.get("category"),
        "is_deliberation": is_deliberation_item(item),
        "schema_valid": sv,
        "schema_error": schema_error,
        "answer_correct": bool(correct),
        "deliberation_correct": bool(deliberation_ok),
        "response_value": response_value,
        "instruction_valid": bool(instr_ok),
        "instruction_failures": instr_failures,
        "rationale_valid": bool(rationale_valid),
        "evidence_valid": bool(evidence_valid),
        "tool_valid": bool(tool_valid),
        "truthfulness_flag": bool(truthfulness_flag),
        "metadata_error": meta.get("error", ""),
        "error_type": meta.get("error_type", ""),
        "elapsed_ms": meta.get("elapsed_ms"),
        "cost": meta.get("cost"),
        "malformed_json": malformed_json,
        "invalid_vote": invalid_vote,
        "disallowed_tool_call": disallowed_tool_call,
        "missing_required_tool_call": missing_required_tool_call,
    }


def mean(values: list[float]) -> float | None:
    vals = [v for v in values if isinstance(v, (int, float))]
    if not vals:
        return None
    return sum(vals) / len(vals)


def median(values: list[float]) -> float | None:
    vals = [v for v in values if isinstance(v, (int, float))]
    if not vals:
        return None
    return statistics.median(vals)


def stddev(values: list[float]) -> float | None:
    vals = [v for v in values if isinstance(v, (int, float))]
    if len(vals) < 2:
        return 0.0 if len(vals) == 1 else None
    return statistics.pstdev(vals)


def cost_sum(rows: list[dict]) -> float | None:
    total = 0.0
    seen = False
    for r in rows:
        c = r.get("cost")
        if isinstance(c, (int, float)):
            total += float(c)
            seen = True
        elif isinstance(c, str):
            try:
                total += float(c)
                seen = True
            except ValueError:
                pass
    return total if seen else None


def trial_scores(rows: list[dict]) -> dict[str, float]:
    by_trial: dict[str, list[dict]] = {}
    for r in rows:
        if r["is_deliberation"] and not r.get("metadata_error"):
            by_trial.setdefault(str(r.get("trial_index", 1)), []).append(r)
    out = {}
    for trial, trial_rows in by_trial.items():
        if trial_rows:
            out[trial] = sum(1 for r in trial_rows if r["deliberation_correct"]) / len(trial_rows)
    return out


def item_variation(rows: list[dict]) -> dict[str, dict]:
    by_item: dict[str, list[dict]] = {}
    for r in rows:
        if r["is_deliberation"] and not r.get("metadata_error"):
            by_item.setdefault(r["item_id"], []).append(r)
    varied = {}
    for item_id, item_rows in by_item.items():
        outcomes = [bool(r["deliberation_correct"]) for r in item_rows]
        schema = [bool(r["schema_valid"]) for r in item_rows]
        answers = [r.get("response_value") for r in item_rows]
        if len(set(outcomes)) > 1 or len(set(schema)) > 1 or len(set(map(str, answers))) > 1:
            varied[item_id] = {"outcomes": outcomes, "schema_valid": schema, "answers": answers}
    return varied


def summarize_model(rows: list[dict]) -> dict:
    n = len(rows)
    deliberation_rows = [r for r in rows if r["is_deliberation"] and not r.get("metadata_error")]
    trials = trial_scores(rows)
    trial_values = list(trials.values())
    score = mean(trial_values)
    aggregate_score = None
    if deliberation_rows:
        aggregate_score = sum(1 for r in deliberation_rows if r["deliberation_correct"]) / len(deliberation_rows)
    error_types = [r.get("error_type", "") for r in rows]
    completed_rows = [r for r in rows if not r.get("metadata_error")]
    operational = {
        "n": n,
        "completed_count": len(completed_rows),
        "latency_ms_avg": mean([r.get("elapsed_ms") for r in rows]),
        "latency_ms_median": median([r.get("elapsed_ms") for r in rows]),
        "timeout_count": sum(1 for e in error_types if e == "timeout"),
        "provider_error_count": sum(1 for e in error_types if e in {"provider_error", "rate_limit", "credential_error", "runner_error"}),
        "schema_violation_count": sum(1 for r in completed_rows if not r["schema_valid"]),
        "tool_call_failure_count": sum(1 for r in completed_rows if not r["tool_valid"]),
        "disallowed_tool_call_count": sum(1 for r in completed_rows if r["disallowed_tool_call"]),
        "missing_required_tool_call_count": sum(1 for r in completed_rows if r["missing_required_tool_call"]),
        "invalid_vote_count": sum(1 for r in completed_rows if r["invalid_vote"]),
        "malformed_json_count": sum(1 for r in completed_rows if r["malformed_json"]),
        "context_limit_error_count": sum(1 for e in error_types if e == "context_limit"),
        "cost": cost_sum(rows),
    }
    dimension_rows = completed_rows or rows
    dims = ["schema_valid", "answer_correct", "deliberation_correct", "instruction_valid", "rationale_valid", "evidence_valid", "tool_valid"]
    dimensions = {d: sum(1 for r in dimension_rows if r[d]) / len(dimension_rows) for d in dims} if dimension_rows else {d: None for d in dims}
    varied = item_variation(rows)
    return {
        "deliberation_score": score,
        "deliberation_score_mean": score,
        "deliberation_score_stddev": stddev(trial_values),
        "deliberation_score_min": min(trial_values) if trial_values else None,
        "deliberation_score_max": max(trial_values) if trial_values else None,
        "deliberation_score_aggregate": aggregate_score,
        "deliberation_n": len(deliberation_rows),
        "trial_scores": trials,
        "trial_count": len(trials),
        "item_variation_count": len(varied),
        "item_variation": varied,
        "operational_metrics": operational,
        "dimensions": dimensions,
    }


def score_run(run_dir: Path, questions: Path) -> dict:
    items = load_items(questions)
    with (run_dir / "raw_results.jsonl").open() as handle:
        results = [json.loads(line) for line in handle if line.strip()]
    scores = [score_one(items[row["item_id"]], row) for row in results]
    by_model = {}
    for s in scores:
        by_model.setdefault(s["model"], []).append(s)
    summary = {model: summarize_model(rows) for model, rows in by_model.items()}
    out = {"scores": scores, "summary": summary}
    (run_dir / "scores.json").write_text(json.dumps(out, indent=2, sort_keys=True) + "\n")
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description="Validate and score adj model-pool runs.")
    sub = ap.add_subparsers(dest="cmd", required=True)
    v = sub.add_parser("validate-items")
    v.add_argument("--questions", default="sets/core20/questions.jsonl")
    s = sub.add_parser("score")
    s.add_argument("--run", required=True)
    s.add_argument("--questions", default="sets/core20/questions.jsonl")
    args = ap.parse_args()
    q = Path(args.questions)
    if not q.is_absolute():
        q = ROOT / q
    if args.cmd == "validate-items":
        errors = validate_items(q)
        print(json.dumps({"ok": not errors, "errors": errors}, indent=2, sort_keys=True))
        return 0 if not errors else 1
    run_dir = Path(args.run)
    if not run_dir.is_absolute():
        run_dir = ROOT / run_dir
    out = score_run(run_dir, q)
    print(json.dumps({"summary": out["summary"]}, indent=2, sort_keys=True))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
