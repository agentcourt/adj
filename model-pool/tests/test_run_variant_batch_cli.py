import json
import os
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "tools" / "run_variant_batch.py"


class VariantBatchCLITests(unittest.TestCase):
    def test_new_run_and_explicit_resume(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            variants = temp / "variants.jsonl"
            variants.write_text(
                json.dumps(
                    {
                        "endpoint_variant_id": "variant-1",
                        "openrouter_model_id": "example/model",
                        "provider_name": "Provider",
                        "endpoint_tag": "provider/fp8",
                        "quantization": "fp8",
                    }
                )
                + "\n"
            )
            questions = temp / "questions.jsonl"
            questions.write_text(
                json.dumps(
                    {
                        "id": "q1",
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
            prompt = temp / "prompt.md"
            prompt.write_text("test prompt\n")
            output = temp / "run"
            calls = temp / "calls"
            bin_dir = temp / "bin"
            bin_dir.mkdir()
            fake_uv = bin_dir / "uv"
            fake_uv.write_text(
                textwrap.dedent(
                    """\
                    #!/usr/bin/env python3
                    import json
                    import os
                    import subprocess
                    import sys
                    from pathlib import Path

                    args = sys.argv[1:]
                    with Path(os.environ["FAKE_UV_CALLS"]).open("a") as handle:
                        handle.write(json.dumps(args) + "\\n")
                    if any(value.endswith("/run_eval.py") for value in args):
                        out = Path(args[args.index("--out") + 1])
                        out.mkdir(parents=True, exist_ok=True)
                        spec = json.loads(Path(args[args.index("--model-spec") + 1]).read_text())
                        question_path = Path(args[args.index("--questions") + 1])
                        prompt_path = Path(args[args.index("--prompt") + 1])
                        item = json.loads(question_path.read_text())
                        label = spec["endpoint_variant_id"]
                        identity = {
                            key: spec.get(key)
                            for key in (
                                "endpoint_variant_id",
                                "openrouter_model_id",
                                "provider_name",
                                "endpoint_tag",
                                "quantization",
                            )
                        }
                        provider = {
                            "only": [spec["endpoint_tag"]],
                            "allow_fallbacks": False,
                            "require_parameters": True,
                            "quantizations": [spec["quantization"]],
                        }
                        normalized_spec = {
                            "label": label,
                            "model": "openrouter://" + spec["openrouter_model_id"],
                            "openrouter_model_id": spec["openrouter_model_id"],
                            "provider": provider,
                            "headers": {"X-OpenRouter-Experimental-Metadata": "enabled"},
                            "request": {},
                            "variant_metadata": identity,
                            "exact_variant": True,
                            "source": args[args.index("--model-spec") + 1],
                        }
                        row = {
                            "item_id": item["id"],
                            "model": label,
                            "trial_index": 1,
                            "raw_response": '{"answer":"A","confidence":0.8,"rationale":"Valid.","evidence_ids":[]}',
                            "parsed_response": {"answer": "A", "confidence": 0.8, "rationale": "Valid.", "evidence_ids": []},
                            "tool_trace": [],
                            "metadata": {
                                "model_spec_label": label,
                                "openrouter_model_id": spec["openrouter_model_id"],
                                "exact_variant": True,
                                "requested_provider_constraints": provider,
                                "requested_quantization_constraints": [spec["quantization"]],
                                "allow_fallbacks": False,
                                "require_parameters": True,
                                "request_parameters": {},
                                "trial_index": 1,
                                "variant_metadata": identity,
                                "error": "",
                                "error_type": "",
                                "elapsed_ms": 1,
                            },
                        }
                        (out / "raw_results.jsonl").write_text(json.dumps(row, sort_keys=True) + "\\n")
                        run = {
                            "run_id": out.name,
                            "created_at": "2026-01-01T00:00:00+00:00",
                            "models": [label],
                            "model_specs": [normalized_spec],
                            "trials": 1,
                            "questions": str(question_path),
                            "prompt": str(prompt_path),
                            "items": [item["id"]],
                            "results": [row],
                        }
                        (out / "run.json").write_text(json.dumps(run, sort_keys=True) + "\\n")
                    elif any(value.endswith("/score_eval.py") for value in args):
                        raise SystemExit(subprocess.run([sys.executable, args[1], *args[2:]]).returncode)
                    else:
                        raise SystemExit(f"unexpected fake uv arguments: {args}")
                    """
                )
            )
            fake_uv.chmod(0o755)
            command = [
                sys.executable,
                str(SCRIPT),
                "--variants",
                str(variants),
                "--questions",
                str(questions),
                "--prompt",
                str(prompt),
                "--out",
                str(output),
                "--trials",
                "1",
                "--timeout",
                "1",
                "--no-progress-timeout",
                "5",
                "--variant-timeout",
                "5",
            ]
            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["FAKE_UV_CALLS"] = str(calls)

            first = subprocess.run(command, cwd=ROOT, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self.assertEqual(first.returncode, 0, first.stderr)
            self.assertEqual(len(calls.read_text().splitlines()), 2)
            manifest_before = (output / "manifest.json").read_bytes()

            repeated = subprocess.run(command, cwd=ROOT, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self.assertNotEqual(repeated.returncode, 0)
            self.assertIn("output directory is not empty", repeated.stderr)
            self.assertEqual(len(calls.read_text().splitlines()), 2)

            resumed = subprocess.run(command + ["--resume"], cwd=ROOT, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self.assertEqual(resumed.returncode, 0, resumed.stderr)
            self.assertEqual(len(calls.read_text().splitlines()), 2)
            self.assertEqual((output / "manifest.json").read_bytes(), manifest_before)


if __name__ == "__main__":
    unittest.main()
