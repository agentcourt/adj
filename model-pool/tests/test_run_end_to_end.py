import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "tools" / "run_end_to_end.py"


class EndToEndRunRecordTests(unittest.TestCase):
    def test_dry_run_resume_rejects_changed_input_before_writing(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            questions = temp / "questions.jsonl"
            questions.write_text(json.dumps({"id": "q1", "mode": "single_turn"}) + "\n")
            prompt = temp / "prompt.md"
            prompt.write_text("prompt one\n")
            genes = temp / "genes.json"
            genes.write_text(json.dumps(["gene one"]) + "\n")
            persona = temp / "persona.md"
            persona.write_text("persona one\n")
            command = [
                sys.executable,
                str(SCRIPT),
                "--out-root",
                str(temp / "runs"),
                "--run-id",
                "dry",
                "--questions",
                str(questions),
                "--prompt",
                str(prompt),
                "--genes",
                str(genes),
                "--persona",
                str(persona),
                "--dry-run",
            ]
            first = subprocess.run(command, cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self.assertEqual(first.returncode, 0, first.stderr)
            run_dir = temp / "runs" / "dry"
            manifest_before = (run_dir / "manifest.json").read_bytes()
            commands_before = (run_dir / "commands.jsonl").read_bytes()

            prompt.write_text("prompt two\n")
            resumed = subprocess.run(command + ["--resume"], cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self.assertNotEqual(resumed.returncode, 0)
            self.assertIn("resume source differs", resumed.stderr)
            self.assertEqual((run_dir / "manifest.json").read_bytes(), manifest_before)
            self.assertEqual((run_dir / "commands.jsonl").read_bytes(), commands_before)


if __name__ == "__main__":
    unittest.main()
