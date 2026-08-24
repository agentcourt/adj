#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import json
from pathlib import Path

from model_inventory import endpoint_raw_filename
from run_eval import model_spec_from_object

ROOT = Path(__file__).resolve().parents[1]
QUESTION_FILES = [ROOT / "sets/core20/questions.jsonl", ROOT / "sets/deliberation/questions.jsonl"]


def load_items(path: Path) -> list[dict]:
    out = []
    with path.open() as f:
        for line_no, line in enumerate(f, 1):
            if line.strip():
                item = json.loads(line)
                item["_file"] = str(path.relative_to(ROOT))
                item["_line"] = line_no
                out.append(item)
    return out


def issue(code: str, where: str, detail) -> dict:
    return {"code": code, "where": where, "detail": detail}


def audit_items() -> list[dict]:
    issues = []
    for path in QUESTION_FILES:
        for item in load_items(path):
            where = f"{item['_file']}:{item['_line']}:{item['id']}"
            if item["mode"] == "tool_record":
                if item["answer_type"] != "adjudication_vote":
                    issues.append(issue("tool_record_answer_type", where, item["answer_type"]))
                if not item.get("allowed_tools"):
                    issues.append(issue("tool_record_without_tools", where, None))
                for key in ["record_dir", "required_citations", "vote_options"]:
                    if key not in item:
                        issues.append(issue("tool_record_missing_field", where, key))
            else:
                if item.get("allowed_tools") != []:
                    issues.append(issue("single_turn_has_tools", where, item.get("allowed_tools")))
                if item["answer_type"] == "adjudication_vote":
                    issues.append(issue("single_turn_vote_type", where, None))
            if item["answer_type"] == "multiple_choice":
                if len(item.get("choices", [])) != 4:
                    issues.append(issue("multiple_choice_not_four_choices", where, len(item.get("choices", []))))
                if item.get("gold") not in ["A", "B", "C", "D"]:
                    issues.append(issue("multiple_choice_gold_not_letter", where, item.get("gold")))
    return issues


def audit_schemas() -> list[dict]:
    issues = []
    item_schema = json.loads((ROOT / "schemas/item.schema.json").read_text())
    pattern = item_schema.get("properties", {}).get("id", {}).get("pattern", "")
    if "deliberation" not in pattern:
        issues.append(issue("item_schema_missing_deliberation_id", "schemas/item.schema.json", pattern))
    result_schema = json.loads((ROOT / "schemas/result.schema.json").read_text())
    props = result_schema.get("properties", {})
    if "prompt" not in props or "prompt" not in result_schema.get("required", []):
        issues.append(issue("result_schema_missing_prompt", "schemas/result.schema.json", None))
    if "trials" not in props:
        issues.append(issue("result_schema_missing_trials", "schemas/result.schema.json", None))
    row_props = props.get("results", {}).get("items", {}).get("properties", {})
    if "trial_index" not in row_props:
        issues.append(issue("result_schema_missing_trial_index", "schemas/result.schema.json", None))
    response_schema = json.loads((ROOT / "schemas/response.schema.json").read_text())
    ordinary = response_schema.get("oneOf", [{}])[0]
    answer_type = ordinary.get("properties", {}).get("answer", {}).get("type")
    if answer_type != "string":
        issues.append(issue("response_schema_answer_not_string", "schemas/response.schema.json", answer_type))
    return issues


def audit_prompts() -> list[dict]:
    issues = []
    for rel in ["prompts/juror-single.md", "prompts/council-member.md"]:
        text = (ROOT / rel).read_text()
        if "Ordinary item JSON" in text or "Adjudication item JSON" in text:
            issues.append(issue("prompt_contains_multiple_contracts", rel, None))
    return issues


def audit_variant_specs() -> list[dict]:
    issues = []
    known = model_spec_from_object(
        {
            "openrouter_model_id": "deepseek/deepseek-v4-flash",
            "provider": {"only": ["deepinfra/fp4"], "quantizations": ["fp4"]},
        },
        "audit",
    )
    if known["provider"] != {"only": ["deepinfra/fp4"], "allow_fallbacks": False, "require_parameters": True, "quantizations": ["fp4"]}:
        issues.append(issue("variant_provider_defaults_invalid", "tools/run_eval.py", known["provider"]))

    unknown = model_spec_from_object(
        {
            "openrouter_model_id": "deepseek/deepseek-v4-flash",
            "endpoint_tag": "alibaba",
            "quantization": "unknown",
        },
        "audit",
    )
    if unknown["provider"] != {"only": ["alibaba"], "allow_fallbacks": False, "require_parameters": True}:
        issues.append(issue("unknown_quantization_provider_invalid", "tools/run_eval.py", unknown["provider"]))

    filenames = {endpoint_raw_filename("a/b_c"), endpoint_raw_filename("a_b/c")}
    if len(filenames) != 2:
        issues.append(issue("endpoint_raw_filename_collision", "tools/model_inventory.py", sorted(filenames)))
    return issues


def main() -> int:
    ap = argparse.ArgumentParser(description="Audit adj model-pool internal consistency.")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()
    issues = audit_items() + audit_schemas() + audit_prompts() + audit_variant_specs()
    out = {"ok": not issues, "issues": issues}
    if args.json:
        print(json.dumps(out, indent=2, sort_keys=True))
    else:
        if not issues:
            print("ok")
        for item in issues:
            print(json.dumps(item, sort_keys=True))
    return 0 if not issues else 1


if __name__ == "__main__":
    raise SystemExit(main())
