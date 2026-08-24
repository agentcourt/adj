import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


TOOLS = Path(__file__).resolve().parents[1] / "tools"
sys.path.insert(0, str(TOOLS))

import run_record
from run_record import PENDING_FILE_PREFIX, input_file, prepare_run_record, questions_snapshot, validate_run_record_inputs
from tool_server import build_record_context


class RunRecordTests(unittest.TestCase):
    def tree_bytes(self, root: Path) -> dict[str, bytes]:
        return {
            str(path.relative_to(root)): path.read_bytes()
            for path in sorted(root.rglob("*"))
            if path.is_file()
        }

    def make_questions(self, root: Path) -> tuple[Path, Path]:
        record = root / "record"
        record.mkdir()
        evidence = record / "E1.md"
        evidence.write_text("record evidence\n")
        (record / "manifest.json").write_text(
            json.dumps({"evidence": [{"id": "E1", "file": "E1.md"}]}) + "\n"
        )
        questions = root / "questions.jsonl"
        questions.write_text(
            json.dumps({"id": "q1", "mode": "tool_record", "record_dir": "record"}) + "\n"
        )
        return questions, evidence

    def test_snapshots_questions_and_rejects_changed_source_before_writing(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run_dir = root / "run"
            questions, evidence = self.make_questions(root)
            inputs, runtime, rows = questions_snapshot(questions, root, run_dir)
            prepare_run_record(
                run_dir,
                kind="test",
                parameters={"trials": 1},
                inputs=inputs,
                generated_inputs=[runtime],
                resume=False,
            )
            copied_record = Path(rows[0]["record_dir"])
            self.assertEqual((copied_record / "E1.md").read_bytes(), evidence.read_bytes())
            self.assertEqual((copied_record / "manifest.json").read_bytes(), (root / "record" / "manifest.json").read_bytes())
            context, trace = build_record_context(str(copied_record))
            self.assertIn("record evidence", context)
            self.assertEqual([entry["tool"] for entry in trace], ["list_evidence", "read_evidence"])

            sentinel = run_dir / "sentinel"
            sentinel.write_text("unchanged\n")
            manifest_before = (run_dir / "manifest.json").read_bytes()
            runtime_before = (run_dir / "inputs" / "questions.jsonl").read_bytes()
            evidence.write_text("changed evidence\n")
            changed_inputs, changed_runtime, _ = questions_snapshot(questions, root, run_dir)
            with self.assertRaisesRegex(ValueError, "resume source differs"):
                prepare_run_record(
                    run_dir,
                    kind="test",
                    parameters={"trials": 1},
                    inputs=changed_inputs,
                    generated_inputs=[changed_runtime],
                    resume=True,
                )
            self.assertEqual((run_dir / "manifest.json").read_bytes(), manifest_before)
            self.assertEqual((run_dir / "inputs" / "questions.jsonl").read_bytes(), runtime_before)
            self.assertEqual(sentinel.read_text(), "unchanged\n")

    def test_resume_rejects_parameter_change(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run_dir = root / "run"
            questions, _ = self.make_questions(root)
            inputs, runtime, _ = questions_snapshot(questions, root, run_dir)
            prepare_run_record(
                run_dir,
                kind="test",
                parameters={"trials": 1},
                inputs=inputs,
                generated_inputs=[runtime],
                resume=False,
            )
            with self.assertRaisesRegex(ValueError, "manifest.parameters.trials"):
                prepare_run_record(
                    run_dir,
                    kind="test",
                    parameters={"trials": 2},
                    inputs=inputs,
                    generated_inputs=[runtime],
                    resume=True,
                )

    def test_new_run_rejects_nonempty_output(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run_dir = root / "run"
            run_dir.mkdir()
            (run_dir / "existing").write_text("data\n")
            questions, _ = self.make_questions(root)
            inputs, runtime, _ = questions_snapshot(questions, root, run_dir)
            with self.assertRaisesRegex(ValueError, "output directory is not empty"):
                prepare_run_record(
                    run_dir,
                    kind="test",
                    parameters={},
                    inputs=inputs,
                    generated_inputs=[runtime],
                    resume=False,
                )

    def test_interrupted_snapshot_rejects_changed_captured_source_without_writing(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run_dir = root / "run"
            run_dir.mkdir()
            first = root / "first.txt"
            second = root / "second.txt"
            first.write_text("first bytes\n")
            second.write_text("second bytes\n")
            inputs = [
                input_file("first", first, "inputs/first.txt"),
                input_file("second", second, "inputs/second.txt"),
            ]
            original_copy = run_record.copy_file
            calls = 0

            def interrupt_second(source: Path, destination: Path, chunk_size: int = 1024 * 1024) -> None:
                nonlocal calls
                calls += 1
                if calls == 2:
                    raise KeyboardInterrupt
                original_copy(source, destination, chunk_size)

            with mock.patch.object(run_record, "copy_file", side_effect=interrupt_second):
                with self.assertRaises(KeyboardInterrupt):
                    prepare_run_record(run_dir, kind="test", parameters={}, inputs=inputs, resume=False)
            first.write_text("other bytes\n")
            before = self.tree_bytes(run_dir)
            with self.assertRaisesRegex(ValueError, "resume source differs"):
                validate_run_record_inputs(run_dir, kind="test", parameters={}, inputs=inputs)
            self.assertEqual(self.tree_bytes(run_dir), before)

    def test_interrupted_temporary_snapshot_is_recovered(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run_dir = root / "run"
            run_dir.mkdir()
            source = root / "source.txt"
            source.write_text("source bytes\n")
            inputs = [input_file("source", source, "inputs/source.txt")]

            def interrupt_copy(_source: Path, destination: Path, _chunk_size: int = 1024 * 1024) -> None:
                destination.parent.mkdir(parents=True, exist_ok=True)
                (destination.parent / f"{PENDING_FILE_PREFIX}partial").write_bytes(b"partial")
                raise KeyboardInterrupt

            with mock.patch.object(run_record, "copy_file", side_effect=interrupt_copy):
                with self.assertRaises(KeyboardInterrupt):
                    prepare_run_record(run_dir, kind="test", parameters={}, inputs=inputs, resume=False)
            validate_run_record_inputs(run_dir, kind="test", parameters={}, inputs=inputs)
            prepare_run_record(run_dir, kind="test", parameters={}, inputs=inputs, resume=True)
            self.assertEqual((run_dir / "inputs" / "source.txt").read_bytes(), source.read_bytes())
            self.assertFalse(list(run_dir.rglob(f"{PENDING_FILE_PREFIX}*")))


if __name__ == "__main__":
    unittest.main()
