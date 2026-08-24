import json
import os
import sys
import tempfile
import unittest
from pathlib import Path


TOOLS = Path(__file__).resolve().parents[1] / "tools"
sys.path.insert(0, str(TOOLS))

from run_variant_batch import active_process, display_path, load_progress, summarize_variant, variant_fields, variant_output_paths
from run_eval import model_spec_from_object
from score_eval import score_run


class VariantProgressTests(unittest.TestCase):
    def setUp(self) -> None:
        self.variant = {
            "endpoint_variant_id": "variant-1",
            "openrouter_model_id": "example/model",
            "provider_name": "Provider",
            "endpoint_tag": "provider/fp8",
            "quantization": "fp8",
        }

    def progress_row(self, out_dir: Path) -> dict:
        questions = out_dir / "questions.jsonl"
        questions.write_text(
            json.dumps(
                {
                    "id": "item-1",
                    "category": "basic_human_knowledge",
                    "mode": "single_turn",
                    "answer_type": "multiple_choice",
                    "gold": "A",
                    "rubric": {"rationale_max_sentences": 3},
                    "allowed_tools": [],
                }
            )
            + "\n"
        )
        prompt = out_dir / "prompt.md"
        prompt.write_text("test prompt\n")
        spec_path, variant_dir, log_path = variant_output_paths(out_dir, 1, self.variant)
        spec_path.parent.mkdir(parents=True, exist_ok=True)
        spec_path.write_text(json.dumps(self.variant, indent=2, sort_keys=True) + "\n")
        normalized_spec = model_spec_from_object(self.variant, display_path(spec_path))
        variant_dir.mkdir(parents=True, exist_ok=True)
        log_path.write_text("complete\n")
        raw_row = {
            "item_id": "item-1",
            "model": "variant-1",
            "trial_index": 1,
            "raw_response": '{"answer":"A","confidence":0.8,"rationale":"Valid.","evidence_ids":[]}',
            "parsed_response": {"answer": "A", "confidence": 0.8, "rationale": "Valid.", "evidence_ids": []},
            "tool_trace": [],
            "metadata": {
                "model_spec_label": normalized_spec["label"],
                "openrouter_model_id": normalized_spec["openrouter_model_id"],
                "exact_variant": normalized_spec["exact_variant"],
                "requested_provider_constraints": normalized_spec["provider"],
                "requested_quantization_constraints": (normalized_spec["provider"] or {}).get("quantizations"),
                "allow_fallbacks": (normalized_spec["provider"] or {}).get("allow_fallbacks"),
                "require_parameters": (normalized_spec["provider"] or {}).get("require_parameters"),
                "request_parameters": normalized_spec["request"],
                "trial_index": 1,
                "variant_metadata": normalized_spec["variant_metadata"],
                "error": "",
                "error_type": "",
                "elapsed_ms": 1,
            },
        }
        (variant_dir / "raw_results.jsonl").write_text(json.dumps(raw_row, sort_keys=True) + "\n")
        (variant_dir / "run.json").write_text(
            json.dumps(
                {
                    "run_id": variant_dir.name,
                    "created_at": "2026-01-01T00:00:00+00:00",
                    "models": ["variant-1"],
                    "model_specs": [normalized_spec],
                    "trials": 1,
                    "questions": str(questions),
                    "prompt": str(prompt),
                    "items": ["item-1"],
                    "results": [raw_row],
                },
                indent=2,
                sort_keys=True,
            )
            + "\n"
        )
        score_run(variant_dir, questions)
        artifact_summary = summarize_variant(variant_dir)
        return {
            "index": 1,
            **variant_fields(self.variant),
            "variant_run_dir": display_path(variant_dir),
            "run_log": display_path(log_path),
            "run_exit_code": 0,
            "score_exit_code": 0,
            "variant_status": "scored",
            **artifact_summary,
        }

    def load(self, progress: Path, out_dir: Path) -> dict[int, dict]:
        return load_progress(
            progress,
            [self.variant],
            out_dir,
            ["item-1"],
            1,
            out_dir / "questions.jsonl",
            out_dir / "prompt.md",
        )

    def test_progress_requires_exact_variant_identity(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            out_dir = Path(directory)
            progress = out_dir / "progress.jsonl"
            row = self.progress_row(out_dir)
            row["endpoint_tag"] = "other"
            progress.write_text(json.dumps(row) + "\n")
            with self.assertRaisesRegex(ValueError, "endpoint_tag"):
                self.load(progress, out_dir)

    def test_progress_rejects_duplicate_indexes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            out_dir = Path(directory)
            progress = out_dir / "progress.jsonl"
            row = self.progress_row(out_dir)
            progress.write_text(json.dumps(row) + "\n" + json.dumps(row) + "\n")
            with self.assertRaisesRegex(ValueError, "duplicate variant index"):
                self.load(progress, out_dir)

    def test_progress_rejects_duplicate_json_keys(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            out_dir = Path(directory)
            progress = out_dir / "progress.jsonl"
            row = self.progress_row(out_dir)
            encoded = json.dumps(row)
            progress.write_text(encoded[:-1] + ', "index": 1}\n')
            with self.assertRaisesRegex(ValueError, "duplicate JSON key 'index'"):
                self.load(progress, out_dir)

    def test_progress_accepts_matching_record(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            out_dir = Path(directory)
            progress = out_dir / "progress.jsonl"
            row = self.progress_row(out_dir)
            progress.write_text(json.dumps(row) + "\n")
            self.assertEqual(self.load(progress, out_dir), {1: row})

    def test_progress_rejects_changed_variant_spec(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            out_dir = Path(directory)
            progress = out_dir / "progress.jsonl"
            row = self.progress_row(out_dir)
            spec_path, _, _ = variant_output_paths(out_dir, 1, self.variant)
            spec_path.write_text("{}\n")
            progress.write_text(json.dumps(row) + "\n")
            with self.assertRaisesRegex(ValueError, "variant spec is missing or differs"):
                self.load(progress, out_dir)

    def test_progress_rejects_changed_raw_result_identity(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            out_dir = Path(directory)
            progress = out_dir / "progress.jsonl"
            row = self.progress_row(out_dir)
            variant_dir = out_dir / row["variant_run_dir"]
            (variant_dir / "raw_results.jsonl").write_text(
                json.dumps({"item_id": "wrong-item", "model": "wrong-model", "trial_index": 1}) + "\n"
            )
            progress.write_text(json.dumps(row) + "\n")
            with self.assertRaisesRegex(ValueError, "raw result"):
                self.load(progress, out_dir)

    def test_active_process_detects_current_process(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "ACTIVE_PID"
            path.write_text(f"{os.getpid()}\n")
            self.assertEqual(active_process(path), os.getpid())


if __name__ == "__main__":
    unittest.main()
