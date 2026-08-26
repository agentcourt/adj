import csv
import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock


TOOLS = Path(__file__).resolve().parents[1] / "tools"


def load_tool(name: str):
    path = TOOLS / f"{name}.py"
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


run_variant_batch = load_tool("run_variant_batch")


def score_document() -> dict:
    operations = {metric: 0 for metric in run_variant_batch.OPERATIONAL_METRICS}
    operations["completed_count"] = 60
    operations["cost"] = 0.25
    return {
        "summary": {
            "example": {
                "deliberation_score": 0.95,
                "operational_metrics": operations,
                "dimensions": {
                    "schema_valid": 1.0,
                    "instruction_valid": 1.0,
                    "tool_valid": 1.0,
                },
            }
        }
    }


class SummaryTests(unittest.TestCase):
    def test_summary_exposes_all_strict_metrics_and_rates(self):
        with tempfile.TemporaryDirectory() as temporary:
            run_dir = Path(temporary)
            (run_dir / "raw_results.jsonl").write_text("{}\n")
            (run_dir / "scores.json").write_text(json.dumps(score_document()) + "\n")

            summary = run_variant_batch.summarize_variant(run_dir)

        self.assertEqual(summary["completed_count"], 60)
        self.assertEqual(summary["deliberation_score"], 0.95)
        self.assertEqual(summary["cost"], 0.25)
        for metric in run_variant_batch.OPERATIONAL_METRICS:
            self.assertEqual(summary[metric], 0)
        self.assertEqual(summary["schema_valid_rate"], 1.0)
        self.assertEqual(summary["instruction_valid_rate"], 1.0)
        self.assertEqual(summary["tool_valid_rate"], 1.0)

    def test_combined_index_comes_from_eligible_variant(self):
        self.assertEqual(run_variant_batch.variant_combined_index({"combined_index": 83}, 2), 83)
        self.assertEqual(run_variant_batch.variant_combined_index({}, 2), 2)

    def test_eval_command_passes_function_tool_mode(self):
        command = run_variant_batch.eval_command(
            questions_path=run_variant_batch.ROOT / "sets/core20/questions.jsonl",
            prompt="prompts/juror-single.md",
            spec_path=run_variant_batch.ROOT / "spec.json",
            variant_dir=run_variant_batch.ROOT / "out",
            trials=3,
            timeout=90,
            tool_mode="function",
        )

        self.assertEqual(command[command.index("--tool-mode") + 1], "function")
        self.assertEqual(command[command.index("--trials") + 1], "3")

    def test_precheck_rejects_known_route_and_capability_failures(self):
        reasons = run_variant_batch.precheck_reasons(
            {
                "openrouter_model_id": "example/model",
                "provider_name": "provider",
                "exact_route_ambiguous": True,
                "input_modalities": ["text"],
                "output_modalities": ["image"],
                "supported_parameters": ["max_tokens"],
            }
        )

        self.assertEqual(
            reasons,
            ["exact_route_ambiguous", "text_output_unsupported", "tools_unsupported"],
        )


class BatchIdentityTests(unittest.TestCase):
    def test_written_summary_preserves_combined_index(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            variants = root / "variants.jsonl"
            questions = root / "questions.jsonl"
            output = root / "output"
            variants.write_text(
                json.dumps(
                    {
                        "combined_index": 83,
                        "openrouter_model_id": "example/model",
                        "provider_name": "Provider",
                        "endpoint_tag": "provider",
                        "quantization": "bf16",
                    }
                )
                + "\n"
            )
            questions.write_text(json.dumps({"id": "one"}) + "\n")

            def finish_command(
                command,
                cwd,
                raw_path,
                log_path,
                expected_rows,
                variant_label,
                completed_variants,
                total_variants,
                no_progress_timeout,
                variant_timeout,
            ):
                raw_path.parent.mkdir(parents=True, exist_ok=True)
                raw_path.write_text("{}\n")
                document = score_document()
                document["summary"]["example"]["operational_metrics"]["completed_count"] = expected_rows
                (raw_path.parent / "scores.json").write_text(json.dumps(document) + "\n")
                return run_variant_batch.CommandResult(0, "finished", None, 0.1)

            completed = SimpleNamespace(returncode=0, stdout="")
            with mock.patch.object(run_variant_batch, "run_command", side_effect=finish_command), mock.patch.object(
                run_variant_batch.subprocess, "run", return_value=completed
            ), mock.patch.object(run_variant_batch, "send_event"):
                status = run_variant_batch.main(
                    [
                        "--variants",
                        str(variants),
                        "--questions",
                        str(questions),
                        "--out",
                        str(output),
                        "--trials",
                        "1",
                        "--tool-mode",
                        "function",
                    ]
                )

            with (output / "variant_summary.csv").open(newline="") as handle:
                rows = list(csv.DictReader(handle))
            batch_summary = json.loads((output / "summary.json").read_text())

        self.assertEqual(status, 0)
        self.assertEqual(rows[0]["combined_index"], "83")
        self.assertEqual(rows[0]["expected_rows"], "1")
        self.assertEqual(batch_summary["tool_mode"], "function")
        self.assertEqual(batch_summary["observed_cost"], 0.25)


if __name__ == "__main__":
    unittest.main()
