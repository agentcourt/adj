import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


TOOL_PATH = Path(__file__).resolve().parents[1] / "tools" / "run_first_gene_inference_embeddings.py"


def load_tool():
    name = "run_first_gene_inference_embeddings_persona_record_test"
    spec = importlib.util.spec_from_file_location(name, TOOL_PATH)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


tool = load_tool()


class PersonaRecordPathTests(unittest.TestCase):
    def run_generator(self, persona_record_path=None):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        variants = root / "variants.jsonl"
        genes = root / "genes.json"
        persona = root / "source-persona.md"
        out = root / "out"
        variants.write_text(
            json.dumps(
                {
                    "combined_index": 1,
                    "endpoint_variant_id": "variant-1",
                    "openrouter_model_id": "example/model",
                    "provider_name": "Example",
                    "endpoint_tag": "example-endpoint",
                    "quantization": "unknown",
                }
            )
            + "\n"
        )
        genes.write_text(json.dumps(["Example gene"]) + "\n")
        persona.write_text("Persona text used for the request.\n")
        argv = [
            "run_first_gene_inference_embeddings.py",
            "--out",
            str(out),
            "--variants",
            str(variants),
            "--genes",
            str(genes),
            "--persona",
            str(persona),
            "--samples",
            "1",
            "--completion-attempts",
            "1",
        ]
        if persona_record_path is not None:
            argv.extend(["--persona-record-path", persona_record_path])

        seen_personas = []

        def complete(_spec, prompt_persona, _gene, _params, _timeout, _attempts, _retry_sleep):
            seen_personas.append(prompt_persona)
            return "response", {}

        with mock.patch.object(sys, "argv", argv), mock.patch.object(
            tool, "load_openrouter_key", return_value="key"
        ), mock.patch.object(tool, "load_openai_key", return_value="key"), mock.patch.object(
            tool, "spec_from_variant", return_value={}
        ), mock.patch.object(tool, "openrouter_completion", side_effect=complete), mock.patch.object(
            tool, "embedding_request", return_value=([0.25, 0.75], {})
        ), mock.patch.object(tool, "hydrate_posthoc_generation_metadata"), mock.patch("builtins.print"):
            result = tool.main()

        record = json.loads((out / "records.jsonl").read_text())
        summary = json.loads((out / "summary.json").read_text())
        return result, record, summary, seen_personas, persona

    def test_record_override_does_not_change_prompt_source(self):
        result, record, summary, seen_personas, _ = self.run_generator("  personas/generic.md  ")

        self.assertEqual(result, 0)
        self.assertEqual(seen_personas, ["Persona text used for the request."])
        self.assertEqual(record["persona_path"], "personas/generic.md")
        self.assertEqual(summary["persona_path"], "personas/generic.md")

    def test_omitted_override_preserves_source_path_recording(self):
        result, record, summary, _, persona = self.run_generator()
        expected = tool.display_path(persona)

        self.assertEqual(result, 0)
        self.assertEqual(record["persona_path"], expected)
        self.assertEqual(summary["persona_path"], expected)

    def test_empty_record_path_is_rejected_before_credentials(self):
        argv = [
            "run_first_gene_inference_embeddings.py",
            "--out",
            "unused",
            "--persona-record-path",
            "   ",
        ]
        with mock.patch.object(sys, "argv", argv):
            with self.assertRaisesRegex(RuntimeError, "--persona-record-path must not be empty"):
                tool.main()


if __name__ == "__main__":
    unittest.main()
