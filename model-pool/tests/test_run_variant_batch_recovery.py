import argparse
import json
import signal
import sys
import tempfile
import unittest
from dataclasses import dataclass
from pathlib import Path
from unittest import mock


TOOLS = Path(__file__).resolve().parents[1] / "tools"
sys.path.insert(0, str(TOOLS))

import run_variant_batch as batch
from run_eval import model_spec_from_object
from run_record import PENDING_FILE_PREFIX, input_file, prepare_run_record, questions_snapshot
from score_eval import score_run


@dataclass
class BatchFixture:
    source_variants: Path
    source_questions: Path
    source_prompt: Path
    out_dir: Path
    variant: dict
    spec_path: Path
    variant_dir: Path
    log_path: Path
    saved_questions: Path
    saved_prompt: Path
    raw_rows: list[dict]
    parameters: dict


class VariantBatchRecoveryTests(unittest.TestCase):
    def create_fixture(
        self,
        base: Path,
        *,
        trials: int = 1,
        complete_trials: int | None = None,
        write_run: bool = True,
        write_scores: bool = False,
    ) -> BatchFixture:
        if complete_trials is None:
            complete_trials = trials
        source = base / "source"
        source.mkdir()
        variant = {
            "endpoint_variant_id": "variant-1",
            "openrouter_model_id": "example/model",
            "provider_name": "Provider",
            "endpoint_tag": "provider/fp8",
            "quantization": "fp8",
        }
        variants_path = source / "variants.jsonl"
        variants_path.write_text(json.dumps(variant, sort_keys=True) + "\n")
        question = {
            "id": "item-1",
            "category": "basic_human_knowledge",
            "mode": "single_turn",
            "answer_type": "multiple_choice",
            "gold": "A",
            "rubric": {"rationale_max_sentences": 3},
            "allowed_tools": [],
        }
        questions_path = source / "questions.jsonl"
        questions_path.write_text(json.dumps(question, sort_keys=True) + "\n")
        prompt_path = source / "prompt.md"
        prompt_path.write_text("Test prompt.\n")
        out_dir = base / "run"
        out_dir.mkdir()

        variants_input = input_file("variants", variants_path, "inputs/variants.jsonl")
        question_inputs, runtime_questions, _ = questions_snapshot(questions_path, batch.ROOT, out_dir)
        prompt_input = input_file("prompt", prompt_path, "inputs/prompt.md")
        parameters = {
            "trials": trials,
            "request_timeout": 5,
            "no_progress_timeout": 10,
            "variant_timeout": 20,
        }
        prepare_run_record(
            out_dir,
            kind="model-pool-variant-batch",
            parameters=parameters,
            inputs=[variants_input, *question_inputs, prompt_input],
            generated_inputs=[runtime_questions],
            resume=False,
        )

        saved_questions = out_dir / runtime_questions.snapshot_path
        saved_prompt = out_dir / prompt_input.snapshot_path
        spec_path, variant_dir, log_path = batch.variant_output_paths(out_dir, 1, variant)
        spec_path.parent.mkdir(parents=True)
        spec_path.write_text(json.dumps(variant, allow_nan=False, indent=2, sort_keys=True) + "\n")
        variant_dir.mkdir(parents=True)
        log_path.write_text("evaluation completed\n")
        normalized_spec = model_spec_from_object(variant, batch.display_path(spec_path))
        raw_rows = []
        for trial in range(1, complete_trials + 1):
            raw_rows.append(
                {
                    "item_id": "item-1",
                    "model": normalized_spec["label"],
                    "trial_index": trial,
                    "raw_response": '{"answer":"A","confidence":0.8,"rationale":"Valid.","evidence_ids":[]}',
                    "parsed_response": {
                        "answer": "A",
                        "confidence": 0.8,
                        "rationale": "Valid.",
                        "evidence_ids": [],
                    },
                    "tool_trace": [],
                    "metadata": {
                        "model_spec_label": normalized_spec["label"],
                        "openrouter_model_id": normalized_spec["openrouter_model_id"],
                        "exact_variant": normalized_spec["exact_variant"],
                        "requested_provider_constraints": normalized_spec["provider"],
                        "requested_quantization_constraints": (normalized_spec["provider"] or {}).get(
                            "quantizations"
                        ),
                        "allow_fallbacks": (normalized_spec["provider"] or {}).get("allow_fallbacks"),
                        "require_parameters": (normalized_spec["provider"] or {}).get("require_parameters"),
                        "request_parameters": normalized_spec["request"],
                        "trial_index": trial,
                        "variant_metadata": normalized_spec["variant_metadata"],
                        "created_at": "2026-01-01T00:00:00+00:00",
                        "error": "",
                        "error_type": "",
                        "elapsed_ms": 1,
                    },
                }
            )
        raw_path = variant_dir / "raw_results.jsonl"
        raw_path.write_text("".join(json.dumps(row, sort_keys=True) + "\n" for row in raw_rows))
        if write_run:
            run = {
                "run_id": variant_dir.name,
                "created_at": "2026-01-01T00:00:00+00:00",
                "models": [normalized_spec["label"]],
                "model_specs": [normalized_spec],
                "trials": trials,
                "questions": batch.display_path(saved_questions),
                "prompt": batch.display_path(saved_prompt),
                "items": ["item-1"],
                "results": raw_rows,
            }
            (variant_dir / "run.json").write_text(json.dumps(run, indent=2, sort_keys=True) + "\n")
        if write_scores:
            score_run(variant_dir, saved_questions)
        return BatchFixture(
            source_variants=variants_path,
            source_questions=questions_path,
            source_prompt=prompt_path,
            out_dir=out_dir,
            variant=variant,
            spec_path=spec_path,
            variant_dir=variant_dir,
            log_path=log_path,
            saved_questions=saved_questions,
            saved_prompt=saved_prompt,
            raw_rows=raw_rows,
            parameters=parameters,
        )

    def preflight(self, fixture: BatchFixture, *, trials: int = 1) -> dict[int, batch.RecoveredVariant]:
        return batch.preflight_batch_resume(
            variants_path=fixture.source_variants,
            out_dir=fixture.out_dir,
            questions_path=fixture.source_questions,
            prompt_path=fixture.source_prompt,
            trials=trials,
            timeout=5,
            no_progress_timeout=10,
            variant_timeout=20,
        )

    def resume_record(self, fixture: BatchFixture) -> None:
        variants_input = input_file("variants", fixture.source_variants, "inputs/variants.jsonl")
        question_inputs, runtime_questions, _ = questions_snapshot(
            fixture.source_questions,
            batch.ROOT,
            fixture.out_dir,
        )
        prompt_input = input_file("prompt", fixture.source_prompt, "inputs/prompt.md")
        prepare_run_record(
            fixture.out_dir,
            kind="model-pool-variant-batch",
            parameters=fixture.parameters,
            inputs=[variants_input, *question_inputs, prompt_input],
            generated_inputs=[runtime_questions],
            resume=True,
        )

    def execute(
        self,
        fixture: BatchFixture,
        recovered: dict[int, batch.RecoveredVariant],
        prior: dict[int, dict] | None = None,
    ) -> int:
        args = argparse.Namespace(
            trials=fixture.parameters["trials"],
            timeout=fixture.parameters["request_timeout"],
            no_progress_timeout=fixture.parameters["no_progress_timeout"],
            variant_timeout=fixture.parameters["variant_timeout"],
            run_lock_fd=-1,
        )
        return batch.execute_batch(
            args,
            fixture.out_dir,
            [fixture.variant],
            ["item-1"],
            fixture.saved_questions,
            fixture.saved_prompt,
            fixture.parameters["trials"],
            prior or {},
            recovered,
        )

    def progress_row(self, fixture: BatchFixture, *, status: str) -> dict:
        if status == "timed_out":
            run_exit_code = batch.TIMEOUT_EXIT_CODE
            timeout_kind = "no_progress"
        else:
            run_exit_code = 7
            timeout_kind = None
        return {
            "index": 1,
            **batch.variant_fields(fixture.variant),
            "variant_run_dir": batch.display_path(fixture.variant_dir),
            "run_log": batch.display_path(fixture.log_path),
            "run_exit_code": run_exit_code,
            "score_exit_code": None,
            "variant_status": status,
            "timeout_kind": timeout_kind,
            "elapsed_seconds": 1.0,
            "result_rows": 1,
        }

    def test_command_failure_and_timeout_accept_one_truncated_raw_fragment(self) -> None:
        for status in ("command_failed", "timed_out"):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as directory:
                fixture = self.create_fixture(
                    Path(directory),
                    trials=2,
                    complete_trials=1,
                    write_run=False,
                )
                with (fixture.variant_dir / "raw_results.jsonl").open("ab") as handle:
                    handle.write(b'{"item_id":"item-1","model":')
                row = self.progress_row(fixture, status=status)
                progress_path = fixture.out_dir / "progress.jsonl"
                progress_path.write_text(json.dumps(row, sort_keys=True) + "\n")

                loaded = batch.load_progress(
                    progress_path,
                    [fixture.variant],
                    fixture.out_dir,
                    ["item-1"],
                    2,
                    fixture.saved_questions,
                    fixture.saved_prompt,
                )

                self.assertEqual(loaded[1]["result_rows"], 1)

    def test_command_failure_retries_while_timeout_remains_terminal(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.create_fixture(Path(directory), trials=2, complete_trials=1, write_run=False)
            with (fixture.variant_dir / "raw_results.jsonl").open("ab") as handle:
                handle.write(b'{"item_id":"item-1","model":')
            command_row = self.progress_row(fixture, status="command_failed")
            with (
                mock.patch.object(
                    batch,
                    "run_command",
                    return_value=batch.CommandResult(7, "command_failed", None, 2.0),
                ) as evaluator,
                mock.patch.object(batch, "run_score_command") as scorer,
                mock.patch.object(batch, "send_event"),
            ):
                exit_code = self.execute(fixture, {}, {1: command_row})
            self.assertEqual(exit_code, 1)
            evaluator.assert_called_once()
            scorer.assert_not_called()

        with tempfile.TemporaryDirectory() as directory:
            fixture = self.create_fixture(Path(directory), trials=2, complete_trials=1, write_run=False)
            with (fixture.variant_dir / "raw_results.jsonl").open("ab") as handle:
                handle.write(b'{"item_id":"item-1","model":')
            timeout_row = self.progress_row(fixture, status="timed_out")
            progress_path = fixture.out_dir / "progress.jsonl"
            progress_bytes = (json.dumps(timeout_row, sort_keys=True) + "\n").encode()
            progress_path.write_bytes(progress_bytes)
            with (
                mock.patch.object(batch, "run_command") as evaluator,
                mock.patch.object(batch, "run_score_command") as scorer,
                mock.patch.object(batch, "send_event"),
            ):
                exit_code = self.execute(fixture, {}, {1: timeout_row})
            self.assertEqual(exit_code, 0)
            evaluator.assert_not_called()
            scorer.assert_not_called()
            self.assertEqual(progress_path.read_bytes(), progress_bytes)

    def test_evaluator_exit_130_requires_parent_stop_to_mean_stopped(self) -> None:
        class FinishedProcess:
            pid = 301
            returncode = batch.STOP_EXIT_CODE

            def poll(self):
                return batch.STOP_EXIT_CODE

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with (
                mock.patch.object(batch.subprocess, "Popen", return_value=FinishedProcess()),
                mock.patch.object(batch, "send_event"),
                mock.patch.object(batch, "terminate_process") as terminate,
            ):
                result = batch.run_command(
                    ["evaluator"],
                    root,
                    root / "ACTIVE_PID",
                    root / "STOP",
                    root / "raw_results.jsonl",
                    root / "run.log",
                    1,
                    "variant",
                    0,
                    1,
                    10,
                    20,
                    -1,
                )
            self.assertEqual(result.status, "command_failed")
            self.assertEqual(result.exit_code, batch.STOP_EXIT_CODE)
            terminate.assert_not_called()

        class RunningProcess:
            pid = 302
            returncode = None

            def poll(self):
                return None

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "STOP").write_text("stop\n")
            process = RunningProcess()
            with (
                mock.patch.object(batch.subprocess, "Popen", return_value=process),
                mock.patch.object(batch, "terminate_process") as terminate,
            ):
                result = batch.run_command(
                    ["evaluator"],
                    root,
                    root / "ACTIVE_PID",
                    root / "STOP",
                    root / "raw_results.jsonl",
                    root / "run.log",
                    1,
                    "variant",
                    0,
                    1,
                    10,
                    20,
                    -1,
                )
            self.assertEqual(result.status, "stopped")
            self.assertEqual(result.exit_code, batch.STOP_EXIT_CODE)
            terminate.assert_called_once_with(process)

    def test_explicit_resume_recovers_unrecorded_scored_evaluation(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.create_fixture(Path(directory), write_scores=True)
            temporary = fixture.variant_dir / f"{PENDING_FILE_PREFIX}interrupted"
            temporary.write_text("partial replacement")

            recovered = self.preflight(fixture)
            self.assertEqual(recovered[1].state, "scored")
            self.assertTrue(temporary.exists())
            self.resume_record(fixture)

            real_write_progress = batch.write_progress
            publication_checks = []

            def checked_write_progress(path, rows):
                publication_checks.append(not temporary.exists())
                real_write_progress(path, rows)

            with (
                mock.patch.object(batch, "run_command") as evaluator,
                mock.patch.object(batch, "run_score_command") as scorer,
                mock.patch.object(batch, "write_progress", side_effect=checked_write_progress),
                mock.patch.object(batch, "send_event"),
            ):
                exit_code = self.execute(fixture, recovered)

            self.assertEqual(exit_code, 0)
            evaluator.assert_not_called()
            scorer.assert_not_called()
            self.assertEqual(publication_checks, [True])
            progress = json.loads((fixture.out_dir / "progress.jsonl").read_text())
            self.assertEqual(progress["variant_status"], "scored")

    def test_run_score_command_terminates_and_reaps_after_communicate_error(self) -> None:
        class BrokenProcess:
            pid = 401
            returncode = None

            def __init__(self):
                self.wait_calls = []

            def communicate(self):
                raise RuntimeError("communicate failed")

            def poll(self):
                return None

            def wait(self, timeout=None):
                self.wait_calls.append(timeout)
                self.returncode = -signal.SIGTERM
                return self.returncode

        process = BrokenProcess()
        with (
            mock.patch.object(batch.subprocess, "Popen", return_value=process),
            mock.patch.object(batch.os, "killpg") as killpg,
            self.assertRaisesRegex(RuntimeError, "communicate failed"),
        ):
            batch.run_score_command(["scorer"], batch.ROOT, -1)

        killpg.assert_called_once_with(process.pid, signal.SIGTERM)
        self.assertEqual(process.wait_calls, [10])

    def test_unrecorded_completed_evaluation_runs_only_scoring(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.create_fixture(Path(directory), write_scores=False)
            recovered = self.preflight(fixture)
            self.assertEqual(recovered[1].state, "evaluated")
            self.resume_record(fixture)
            batch.cleanup_atomic_temporaries(fixture.out_dir)

            def score_once(command, cwd, lock_fd):
                self.assertEqual(cwd, batch.ROOT)
                self.assertEqual(lock_fd, -1)
                score_run(fixture.variant_dir, fixture.saved_questions)
                return 0, ""

            with (
                mock.patch.object(batch, "run_command") as evaluator,
                mock.patch.object(batch, "run_score_command", side_effect=score_once) as scorer,
                mock.patch.object(batch, "send_event"),
            ):
                exit_code = self.execute(fixture, recovered)

            self.assertEqual(exit_code, 0)
            evaluator.assert_not_called()
            scorer.assert_called_once()
            progress = json.loads((fixture.out_dir / "progress.jsonl").read_text())
            self.assertEqual(progress["variant_status"], "scored")

    def test_unexpected_durable_file_in_variant_run_rejects_preflight(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            fixture = self.create_fixture(Path(directory), write_scores=True)
            (fixture.variant_dir / "foreign-output.bin").write_bytes(b"unexpected")

            with self.assertRaisesRegex(ValueError, "unexpected"):
                self.preflight(fixture)


if __name__ == "__main__":
    unittest.main()
