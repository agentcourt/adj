import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


TOOLS = Path(__file__).resolve().parents[1] / "tools"


def load_tool(name: str):
    path = TOOLS / f"{name}.py"
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


run_eval = load_tool("run_eval")
score_eval = load_tool("score_eval")


def tool_call_response(usage: dict) -> dict:
    return {
        "id": "round-1",
        "model": "example/model",
        "usage": usage,
        "choices": [
            {
                "finish_reason": "tool_calls",
                "message": {
                    "content": "",
                    "tool_calls": [
                        {"id": "call-1", "type": "function", "function": {"name": "list_evidence", "arguments": "{}"}}
                    ],
                },
            }
        ],
    }


class ToolRoundMetadataTests(unittest.TestCase):
    def test_usage_and_cost_include_every_provider_round(self):
        responses = [
            tool_call_response(
                {
                    "prompt_tokens": 10,
                    "completion_tokens": 2,
                    "total_tokens": 12,
                    "cost": 0.01,
                    "prompt_tokens_details": {"cached_tokens": 3},
                    "is_byok": False,
                }
            ),
            {
                "id": "round-2",
                "model": "example/model",
                "usage": {
                    "prompt_tokens": 15,
                    "completion_tokens": 3,
                    "total_tokens": 18,
                    "cost": "0.02",
                    "prompt_tokens_details": {"cached_tokens": 4},
                    "is_byok": False,
                },
                "choices": [
                    {
                        "finish_reason": "stop",
                        "message": {
                            "content": '{"vote":"demonstrated","confidence":1,"rationale":"supported","evidence_ids":["E1"]}'
                        },
                    }
                ],
            },
        ]
        spec = run_eval.model_spec_from_string("openrouter://example/model")
        item = {"record_dir": "/record"}

        with mock.patch.object(run_eval, "openrouter_request", side_effect=responses), mock.patch.object(
            run_eval, "execute_tool", return_value={"evidence": [{"id": "E1"}]}
        ):
            raw, metadata, trace = run_eval.call_openrouter_tools(spec, item, "question", 30)

        self.assertIn('"vote":"demonstrated"', raw)
        self.assertEqual(metadata["usage"]["prompt_tokens"], 25)
        self.assertEqual(metadata["usage"]["completion_tokens"], 5)
        self.assertEqual(metadata["usage"]["total_tokens"], 30)
        self.assertAlmostEqual(metadata["usage"]["cost"], 0.03)
        self.assertEqual(metadata["usage"]["prompt_tokens_details"]["cached_tokens"], 7)
        self.assertEqual(metadata["usage"]["is_byok"], False)
        self.assertAlmostEqual(metadata["cost"], 0.03)
        self.assertEqual(metadata["tool_call_count"], 1)
        self.assertEqual(len(trace), 1)

    def test_later_failure_preserves_completed_round_metadata_and_trace(self):
        first_response = tool_call_response(
            {"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12, "cost": 0.01}
        )
        spec = run_eval.model_spec_from_string("openrouter://example/model")

        with mock.patch.object(run_eval, "openrouter_request", side_effect=[first_response, TimeoutError("timed out")]), mock.patch.object(
            run_eval, "execute_tool", return_value={"evidence": [{"id": "E1"}]}
        ):
            with self.assertRaises(run_eval.ToolRoundError) as raised:
                run_eval.call_openrouter_tools(spec, {"record_dir": "/record"}, "question", 30)

        failure = raised.exception
        self.assertIsInstance(failure.cause, TimeoutError)
        self.assertEqual(failure.metadata["usage"]["total_tokens"], 12)
        self.assertAlmostEqual(failure.metadata["cost"], 0.01)
        self.assertEqual(failure.metadata["tool_rounds"], 1)
        self.assertEqual(failure.metadata["tool_call_count"], 1)
        self.assertEqual(failure.trace, [{"tool": "list_evidence", "args": {}, "result": {"evidence": [{"id": "E1"}]}}])

    def test_run_row_preserves_completed_round_metadata_and_trace(self):
        first_response = tool_call_response(
            {"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12, "cost": 0.01}
        )
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            questions = tmp_path / "questions.jsonl"
            prompt = tmp_path / "prompt.md"
            out = tmp_path / "out"
            item = {"id": "tool-item", "prompt": "Assess the record.", "mode": "tool_record", "record_dir": "/record"}
            questions.write_text(json.dumps(item) + "\n")
            prompt.write_text("Evaluate the question.")
            argv = [
                "run_eval.py",
                "--questions", str(questions),
                "--prompt", str(prompt),
                "--models", "openrouter://example/model",
                "--out", str(out),
                "--trials", "1",
                "--tool-mode", "function",
            ]
            with mock.patch.object(sys, "argv", argv), mock.patch.object(
                run_eval, "openrouter_request", side_effect=[first_response, TimeoutError("timed out")]
            ), mock.patch.object(run_eval, "execute_tool", return_value={"evidence": [{"id": "E1"}]}), mock.patch(
                "builtins.print"
            ):
                self.assertEqual(run_eval.main(), 0)

            row = json.loads((out / "raw_results.jsonl").read_text())

        self.assertEqual(row["metadata"]["error_type"], "timeout")
        self.assertEqual(row["metadata"]["usage"]["total_tokens"], 12)
        self.assertAlmostEqual(row["metadata"]["cost"], 0.01)
        self.assertEqual(row["metadata"]["tool_call_count"], 1)
        self.assertEqual(row["tool_trace"], [{"tool": "list_evidence", "args": {}, "result": {"evidence": [{"id": "E1"}]}}])


class ToolFailureScoringTests(unittest.TestCase):
    def test_boolean_confidence_fails_schema(self):
        item = {"mode": "single_turn"}
        response = {
            "answer": "A",
            "confidence": True,
            "rationale": "Supported.",
            "evidence_ids": [],
        }

        valid, error = score_eval.schema_valid(item, response)

        self.assertFalse(valid)
        self.assertEqual(error, "invalid confidence")

    def test_surrounding_text_fails_strict_json_scoring(self):
        raw = 'Answer: {"answer":"A","confidence":1,"rationale":"Supported.","evidence_ids":[]}'
        item = {
            "id": "item",
            "category": "basic_reasoning",
            "mode": "single_turn",
            "answer_type": "multiple_choice",
            "gold": "A",
            "rubric": {},
        }
        row = {
            "item_id": "item",
            "model": "example/model",
            "trial_index": 1,
            "raw_response": raw,
            "parsed_response": run_eval.parse_maybe_json(raw),
            "tool_trace": [],
            "metadata": {"error": "", "error_type": ""},
        }

        score = score_eval.score_one(item, row)
        summary = score_eval.summarize_model([score])

        self.assertIsNone(row["parsed_response"])
        self.assertFalse(score["schema_valid"])
        self.assertTrue(score["malformed_json"])
        self.assertEqual(summary["operational_metrics"]["malformed_json_count"], 1)

    def test_tool_execution_error_counts_as_tool_call_failure(self):
        item = {
            "id": "tool-item",
            "category": "tool_evidence_use",
            "mode": "tool_record",
            "allowed_tools": ["list_evidence", "read_evidence"],
            "vote_options": ["demonstrated", "not_demonstrated", "indeterminate"],
            "gold": {"vote": "demonstrated"},
            "required_citations": ["E1"],
            "rubric": {},
        }
        row = {
            "item_id": "tool-item",
            "model": "example/model",
            "trial_index": 1,
            "raw_response": "present",
            "parsed_response": {
                "vote": "demonstrated",
                "confidence": 1,
                "rationale": "The evidence supports the vote.",
                "evidence_ids": ["E1"],
            },
            "tool_trace": [
                {"tool": "list_evidence", "args": {}, "result": {"evidence": [{"id": "E1"}]}},
                {"tool": "read_evidence", "args": {"evidence_id": "E1"}, "result": {"content": "record"}},
            ],
            "metadata": {"tool_error_count": 1, "error": "", "error_type": ""},
        }

        with mock.patch.object(score_eval, "evidence_ids", return_value={"E1"}):
            score = score_eval.score_one(item, row)
        summary = score_eval.summarize_model([score])

        self.assertFalse(score["tool_valid"])
        self.assertEqual(summary["operational_metrics"]["tool_call_failure_count"], 1)

    def test_tool_trace_error_is_detected_without_metadata_count(self):
        row = {
            "metadata": {"tool_error_count": 0},
            "tool_trace": [{"tool": "read_evidence", "result": {"error": "unknown evidence id"}}],
        }

        self.assertTrue(score_eval.tool_execution_failed(row))


if __name__ == "__main__":
    unittest.main()
