import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


TOOLS = Path(__file__).resolve().parents[1] / "tools"
sys.path.insert(0, str(TOOLS))

import run_first_gene_inference_embeddings as gene


def tree_bytes(root: Path) -> dict[str, bytes]:
    return {
        str(path.relative_to(root)): path.read_bytes()
        for path in sorted(root.rglob("*"))
        if path.is_file()
    }


class GeneResumeTests(unittest.TestCase):
    def sources(self, root: Path) -> tuple[Path, Path, Path, Path]:
        variants = root / "variants.jsonl"
        variants.write_text(
            json.dumps(
                {
                    "combined_index": 1,
                    "endpoint_variant_id": "variant-1",
                    "openrouter_model_id": "example/model",
                    "provider_name": "Provider",
                    "endpoint_tag": "provider/fp8",
                    "quantization": "fp8",
                    "supported_parameters": ["temperature", "top_p", "max_tokens"],
                }
            )
            + "\n"
        )
        genes = root / "genes.json"
        genes.write_text(json.dumps(["Analyze the issue."]) + "\n")
        persona = root / "persona.md"
        persona.write_text("Analyze carefully.\n")
        return variants, genes, persona, root / "run"

    def argv(self, variants: Path, genes: Path, persona: Path, out: Path, *, resume: bool = False) -> list[str]:
        argv = [
            "--variants",
            str(variants),
            "--genes",
            str(genes),
            "--persona",
            str(persona),
            "--out",
            str(out),
            "--samples",
            "1",
            "--gene-index",
            "0",
            "--completion-attempts",
            "1",
            "--retry-sleep",
            "0",
        ]
        if resume:
            argv.append("--resume")
        return argv

    def first_embedding_failure(self, variants: Path, genes: Path, persona: Path, out: Path) -> None:
        with (
            mock.patch.object(gene, "load_openrouter_key", return_value="openrouter-key"),
            mock.patch.object(gene, "load_openai_key", return_value="openai-key"),
            mock.patch.object(gene, "openrouter_completion", return_value=("completed response", {"runner": "test"})),
            mock.patch.object(gene, "embedding_request", side_effect=gene.EmbeddingServiceError("embedding failed")),
            mock.patch.object(gene, "hydrate_posthoc_generation_metadata", return_value=None),
        ):
            self.assertEqual(gene.main(self.argv(variants, genes, persona, out)), 0)

    def test_embedding_error_resume_does_not_repeat_completion(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            variants, genes, persona, out = self.sources(root)
            self.first_embedding_failure(variants, genes, persona, out)
            completion = mock.Mock(side_effect=AssertionError("completion must not repeat"))
            with (
                mock.patch.object(gene, "load_openrouter_key", return_value=None),
                mock.patch.object(gene, "load_openai_key", return_value="openai-key"),
                mock.patch.object(gene, "openrouter_completion", completion),
                mock.patch.object(gene, "embedding_request", return_value=([1.0, 2.0], {"embedding_model": "test"})),
                mock.patch.object(gene, "hydrate_posthoc_generation_metadata", return_value=None),
            ):
                self.assertEqual(gene.main(self.argv(variants, genes, persona, out, resume=True)), 0)
            self.assertEqual(completion.call_count, 0)
            row = json.loads((out / "records.jsonl").read_text())
            self.assertEqual(row["status"], "ok")
            self.assertEqual(row["response_text"], "completed response")
            self.assertEqual(row["embedding"], [1.0, 2.0])

    def test_row_transaction_recovers_without_provider_request(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            variants, genes, persona, out = self.sources(root)
            self.first_embedding_failure(variants, genes, persona, out)
            with (
                mock.patch.object(gene, "load_openrouter_key", return_value=None),
                mock.patch.object(gene, "load_openai_key", return_value="openai-key"),
                mock.patch.object(gene, "embedding_request", return_value=([1.0], {"embedding_model": "test"})),
                mock.patch.object(gene, "hydrate_posthoc_generation_metadata", return_value=None),
                mock.patch.object(gene, "write_current_records", side_effect=RuntimeError("interrupted publication")),
            ):
                with self.assertRaisesRegex(RuntimeError, "interrupted publication"):
                    gene.main(self.argv(variants, genes, persona, out, resume=True))
            self.assertEqual(len(list((out / "row-transactions").glob("*.json"))), 1)

            with (
                mock.patch.object(gene, "load_openrouter_key", return_value=None),
                mock.patch.object(gene, "load_openai_key", return_value=None),
                mock.patch.object(gene, "openrouter_completion", side_effect=AssertionError("completion repeated")),
                mock.patch.object(gene, "embedding_request", side_effect=AssertionError("embedding repeated")),
                mock.patch.object(gene, "hydrate_posthoc_generation_metadata", return_value=None),
            ):
                self.assertEqual(gene.main(self.argv(variants, genes, persona, out, resume=True)), 0)
            self.assertFalse(list((out / "row-transactions").glob("*.json")))
            self.assertEqual(json.loads((out / "records.jsonl").read_text())["status"], "ok")

    def test_tampered_row_rejects_before_resume_write(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            variants, genes, persona, out = self.sources(root)
            self.first_embedding_failure(variants, genes, persona, out)
            row_path = next((out / "row-records").glob("*.json"))
            row = json.loads(row_path.read_text())
            row["provider_name"] = "Changed"
            row_path.write_text(json.dumps(row, sort_keys=True) + "\n")
            before = tree_bytes(out)
            with self.assertRaisesRegex(ValueError, "provider_name"):
                gene.main(self.argv(variants, genes, persona, out, resume=True))
            self.assertEqual(tree_bytes(out), before)

    def test_unexpected_completion_exception_propagates(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            variants, genes, persona, out = self.sources(root)
            with (
                mock.patch.object(gene, "load_openrouter_key", return_value="openrouter-key"),
                mock.patch.object(gene, "load_openai_key", return_value="openai-key"),
                mock.patch.object(gene, "openrouter_completion", side_effect=RuntimeError("programmer failure")),
            ):
                with self.assertRaisesRegex(RuntimeError, "programmer failure"):
                    gene.main(self.argv(variants, genes, persona, out))
            self.assertFalse((out / "records.jsonl").exists())

    def test_empty_completion_is_an_expected_service_error(self) -> None:
        with (
            mock.patch.object(gene, "openrouter_request", return_value={}),
            mock.patch.object(gene, "response_meta", return_value=("   ", {}, None)),
        ):
            with self.assertRaisesRegex(gene.CompletionServiceError, "contained no text"):
                gene.openrouter_completion_once(
                    {"openrouter_model_id": "example/model", "provider": None, "headers": {}},
                    "persona",
                    "gene",
                    {},
                    1,
                )

    def test_nonfinite_embedding_is_rejected_before_publication(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            variants, genes, persona, out = self.sources(root)
            with (
                mock.patch.object(gene, "load_openrouter_key", return_value="openrouter-key"),
                mock.patch.object(gene, "load_openai_key", return_value="openai-key"),
                mock.patch.object(gene, "openrouter_completion", return_value=("completed response", {"runner": "test"})),
                mock.patch.object(gene, "embedding_request", return_value=([float("nan")], {"embedding_model": "test"})),
            ):
                with self.assertRaisesRegex(ValueError, "nonnumeric"):
                    gene.main(self.argv(variants, genes, persona, out))
            self.assertFalse((out / "records.jsonl").exists())

    def test_nonfinite_request_argument_fails_before_output_creation(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            variants, genes, persona, out = self.sources(root)
            argv = self.argv(variants, genes, persona, out)
            argv.extend(["--top-p", "nan"])
            with self.assertRaisesRegex(RuntimeError, "--top-p"):
                gene.main(argv)
            self.assertFalse(out.exists())


if __name__ == "__main__":
    unittest.main()
