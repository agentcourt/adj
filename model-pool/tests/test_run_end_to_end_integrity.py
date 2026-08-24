# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "scikit-learn"]
# ///
import csv
import json
import subprocess
import sys
import tempfile
import unittest
from collections import Counter
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools"
sys.path.insert(0, str(TOOLS))

import run_end_to_end as end_to_end


def write_json(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def write_jsonl(path: Path, rows: list[dict]) -> None:
    path.write_text(
        "".join(json.dumps(row, ensure_ascii=False) + "\n" for row in rows),
        encoding="utf-8",
    )


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def load_jsonl(path: Path) -> list[dict]:
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line]


class EndToEndIntegrityTests(unittest.TestCase):
    def run_tool(self, script: str, *arguments: str) -> None:
        result = subprocess.run(
            [sys.executable, str(TOOLS / script), *arguments],
            cwd=ROOT,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def assert_cluster_validation(self, expected: bool, arguments: tuple) -> None:
        payload = {
            "out_dir": str(arguments[0]),
            "expected_genes": arguments[1],
            "expected_rows_per_gene": arguments[2],
            "expected_variants_per_gene": arguments[3],
            "expected_samples_per_variant": arguments[4],
            "min_k": arguments[5],
            "max_k": arguments[6],
            "pca_sources": [str(path) for path in arguments[7]],
            "expected": expected,
        }
        code = """
import json
import sys
from pathlib import Path

sys.path.insert(0, sys.argv[1])
import run_end_to_end

payload = json.loads(sys.argv[2])
actual = run_end_to_end.clusters_complete(
    Path(payload["out_dir"]),
    payload["expected_genes"],
    payload["expected_rows_per_gene"],
    payload["expected_variants_per_gene"],
    payload["expected_samples_per_variant"],
    payload["min_k"],
    payload["max_k"],
    [Path(value) for value in payload["pca_sources"]],
)
print(json.dumps({"actual": actual, "expected": payload["expected"]}))
raise SystemExit(0 if actual is payload["expected"] else 1)
"""
        result = subprocess.run(
            [
                sys.executable,
                "-c",
                code,
                str(TOOLS),
                json.dumps(payload),
            ],
            cwd=ROOT,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        self.assertEqual(result.returncode, 0, result.stderr or result.stdout)

    def test_pca_validator_rejects_consistently_reordered_axes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            records_path = temp / "gene-records.jsonl"
            out_dir = temp / "pca"
            embeddings = [
                [0.0, 0.0, 0.0],
                [2.0, 0.0, 0.0],
                [0.0, 1.0, 0.5],
                [3.0, 2.0, 4.0],
            ]
            rows = [
                {
                    "run_id": "gene-run",
                    "gene_index": 0,
                    "gene": "Analyze the evidence.",
                    "gene_sha256": "gene-digest",
                    "persona_id": "persona-1",
                    "persona_path": "personas/persona-1.md",
                    "variant_order": index // 2,
                    "combined_index": index // 2 + 1,
                    "endpoint_variant_id": f"endpoint-{index // 2 + 1}",
                    "openrouter_model_id": "provider/model",
                    "provider_name": "provider",
                    "endpoint_tag": "tag",
                    "quantization": "fp16",
                    "sample_index": index % 2,
                    "status": "ok",
                    "embedding": embedding,
                }
                for index, embedding in enumerate(embeddings)
            ]
            write_jsonl(records_path, rows)

            self.run_tool(
                "run_embedding_pca.py",
                "--records",
                str(records_path),
                "--out",
                str(out_dir),
                "--dimensions",
                "2",
            )
            self.assertTrue(end_to_end.pca_complete(out_dir, 2, records_path))

            fit_path = out_dir / "pca-fit.json"
            summary_path = out_dir / "summary.json"
            projected_path = out_dir / "pca-records.jsonl"
            fit = load_json(fit_path)
            summary = load_json(summary_path)
            projected = load_jsonl(projected_path)
            fit["components"] = [fit["components"][1], fit["components"][0]]
            for field in ("explained_variance", "explained_variance_ratio", "singular_values"):
                fit[field] = [fit[field][1], fit[field][0]]
            for field in ("explained_variance", "explained_variance_ratio"):
                summary[field] = [summary[field][1], summary[field][0]]
            for row in projected:
                row["pca"] = [row["pca"][1], row["pca"][0]]
            write_json(fit_path, fit)
            write_json(summary_path, summary)
            write_jsonl(projected_path, projected)

            self.assertFalse(end_to_end.pca_complete(out_dir, 2, records_path))

    def test_cluster_validator_rejects_consistent_label_permutation(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            pca_path = temp / "pca-records.jsonl"
            out_dir = temp / "clusters"
            points = [
                [0.0, 0.0],
                [0.1, 0.2],
                [5.0, 5.0],
                [5.2, 5.1],
                [10.0, 0.0],
                [10.2, 0.1],
            ]
            rows = [
                {
                    "run_id": "pca-run",
                    "gene_index": 0,
                    "gene": "Analyze the evidence.",
                    "gene_sha256": "gene-digest",
                    "persona_id": "persona-1",
                    "persona_path": "personas/persona-1.md",
                    "variant_order": index // 2,
                    "combined_index": index // 2 + 1,
                    "endpoint_variant_id": f"endpoint-{index // 2 + 1}",
                    "openrouter_model_id": "provider/model",
                    "provider_name": "provider",
                    "endpoint_tag": "tag",
                    "quantization": "fp16",
                    "sample_index": index % 2,
                    "pca": point,
                }
                for index, point in enumerate(points)
            ]
            write_jsonl(pca_path, rows)

            self.run_tool(
                "run_gene_pca_clustering.py",
                "--pca-records",
                str(pca_path),
                "--out",
                str(out_dir),
                "--expected-rows-per-gene",
                "6",
                "--expected-variants-per-gene",
                "3",
                "--expected-samples-per-variant",
                "2",
                "--pca-dimensions",
                "2",
                "--min-k",
                "2",
                "--max-k",
                "3",
            )
            validator_arguments = (out_dir, 1, 6, 3, 2, 2, 3, [pca_path])
            self.assert_cluster_validation(True, validator_arguments)

            records_path = out_dir / "clusters.jsonl"
            fit_path = out_dir / "cluster-fit.json"
            summary_path = out_dir / "summary.json"
            csv_path = out_dir / "clusters.csv"
            clustered = load_jsonl(records_path)
            labels = sorted({row["cluster"] for row in clustered})
            self.assertGreaterEqual(len(labels), 2)
            mapping = dict(zip(labels, reversed(labels), strict=True))
            for row in clustered:
                row["cluster"] = mapping[row["cluster"]]

            fit = load_json(fit_path)
            gene_fit = fit["genes"]["0"]
            old_centers = gene_fit["centers"]
            new_centers = [None] * len(old_centers)
            for old_label, new_label in mapping.items():
                new_centers[new_label] = old_centers[old_label]
            gene_fit["centers"] = new_centers

            summary = load_json(summary_path)
            counts = Counter(row["cluster"] for row in clustered)
            summary["genes"][0]["cluster_counts"] = {
                str(label): counts[label] for label in sorted(counts)
            }

            with csv_path.open(newline="") as handle:
                reader = csv.DictReader(handle)
                fieldnames = reader.fieldnames
                csv_rows = list(reader)
            self.assertIsNotNone(fieldnames)
            for row in csv_rows:
                row["cluster"] = str(mapping[int(row["cluster"])])
            with csv_path.open("w", newline="") as handle:
                writer = csv.DictWriter(handle, fieldnames=fieldnames)
                writer.writeheader()
                writer.writerows(csv_rows)
            write_jsonl(records_path, clustered)
            write_json(fit_path, fit)
            write_json(summary_path, summary)

            self.assert_cluster_validation(False, validator_arguments)

    def test_atomic_command_append_preserves_prior_log_and_validation_rejects_bad_logs(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            commands_path = temp / "commands.jsonl"
            prior = {
                "at": "2026-01-01T00:00:00Z",
                "stage": "inventory",
                "cmd": ["inventory"],
                "cwd": str(ROOT),
                "dry_run": False,
                "stdout_path": None,
            }
            write_jsonl(commands_path, [prior])
            prior_bytes = commands_path.read_bytes()
            next_record = {
                "at": "2026-01-01T00:00:01Z",
                "stage": "inventory",
                "cmd": ["inventory"],
                "exit_code": 0,
                "tail": [],
            }
            with mock.patch.object(
                end_to_end,
                "atomic_write_bytes",
                side_effect=OSError("injected atomic-write failure"),
            ):
                with self.assertRaisesRegex(OSError, "injected atomic-write failure"):
                    end_to_end.append_command_record(commands_path, next_record)
            self.assertEqual(commands_path.read_bytes(), prior_bytes)
            self.assertEqual(end_to_end.validate_command_log(commands_path), [prior])

            malformed_path = temp / "malformed.jsonl"
            write_jsonl(malformed_path, [{**prior, "unknown": True}])
            with self.assertRaisesRegex(ValueError, "fields differ"):
                end_to_end.validate_command_log(malformed_path)

            truncated_path = temp / "truncated.jsonl"
            truncated_path.write_text('{"at":"2026-01-01T00:00:00Z"', encoding="utf-8")
            with self.assertRaises(ValueError):
                end_to_end.validate_command_log(truncated_path)

    def test_filter_validator_binds_specs_and_full_survivor_summary(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            source_path = temp / "endpoint-variants.jsonl"
            eval_summary_path = temp / "variant-summary.csv"
            source_specs_dir = temp / "source-specs"
            out_dir = temp / "filtered"
            copied_specs_dir = out_dir / "specs"
            source_specs_dir.mkdir()
            copied_specs_dir.mkdir(parents=True)

            source_row = {
                "combined_index": 1,
                "endpoint_variant_id": "endpoint-1",
                "openrouter_model_id": "provider/model",
                "provider_name": "provider",
                "endpoint_tag": "tag",
                "quantization": "fp16",
                "context_length": 8192,
            }
            write_jsonl(source_path, [source_row])
            eval_fields = [
                "combined_index",
                "run_exit_code",
                "provider_error_count",
                "deliberation_score",
                "variant_status",
                "timeout_kind",
                "variant_run_dir",
                "elapsed_seconds",
            ]
            eval_row = {
                "combined_index": "1",
                "run_exit_code": "0",
                "provider_error_count": "0",
                "deliberation_score": "0.9",
                "variant_status": "scored",
                "timeout_kind": "",
                "variant_run_dir": "/runs/variant-1",
                "elapsed_seconds": "12.5",
            }
            with eval_summary_path.open("w", newline="") as handle:
                writer = csv.DictWriter(handle, fieldnames=eval_fields)
                writer.writeheader()
                writer.writerow(eval_row)

            spec_name = "01-provider-model.json"
            source_spec = source_specs_dir / spec_name
            copied_spec = copied_specs_dir / spec_name
            source_spec.write_text('{"model":"provider/model"}\n', encoding="utf-8")
            copied_spec.write_bytes(source_spec.read_bytes())

            survivor = {
                **source_row,
                "filter_provider_error_count": 0,
                "filter_deliberation_score": 0.9,
            }
            survivor_summary = {
                **eval_row,
                "combined_index": 1,
                "provider_error_count": 0,
                "deliberation_score": 0.9,
            }
            manifest = {
                "combined_index": 1,
                "endpoint_variant_id": "endpoint-1",
                "openrouter_model_id": "provider/model",
                "provider_name": "provider",
                "endpoint_tag": "tag",
                "quantization": "fp16",
                "run_dir": "/runs/variant-1",
            }
            write_jsonl(out_dir / "endpoint_variants.jsonl", [survivor])
            write_jsonl(out_dir / "variant_summary.jsonl", [survivor_summary])
            write_jsonl(out_dir / "manifest.jsonl", [manifest])
            write_jsonl(out_dir / "removed_variants.jsonl", [])
            filtered_fields = [
                "combined_index",
                "openrouter_model_id",
                "provider_name",
                "endpoint_tag",
                "quantization",
                "endpoint_variant_id",
                "filter_provider_error_count",
                "filter_deliberation_score",
            ]
            with (out_dir / "endpoint_variants.csv").open("w", newline="") as handle:
                writer = csv.DictWriter(handle, fieldnames=filtered_fields, extrasaction="ignore")
                writer.writeheader()
                writer.writerow(survivor)
            write_json(
                out_dir / "summary.json",
                {
                    "created_at": "2026-01-01T00:00:00Z",
                    "source_variant_file": end_to_end.display_path(source_path),
                    "source_eval_summary_file": end_to_end.display_path(eval_summary_path),
                    "source_specs_dir": end_to_end.display_path(source_specs_dir),
                    "filter_criteria": {
                        "provider_error_count": 0,
                        "deliberation_score_minimum": 0.5,
                    },
                    "total_variants": 1,
                    "survivor_count": 1,
                    "survivor_combined_indexes": [1],
                    "removed_count": 0,
                    "removed_variant_indexes": [],
                    "outputs": [
                        "endpoint_variants.jsonl",
                        "endpoint_variants.csv",
                        "variant_summary.jsonl",
                        "manifest.jsonl",
                        "removed_variants.jsonl",
                        "specs/*.json",
                        "summary.json",
                    ],
                },
            )
            validator_arguments = (
                out_dir,
                source_path,
                eval_summary_path,
                source_specs_dir,
                0,
                0.5,
            )
            self.assertTrue(end_to_end.filter_complete(*validator_arguments))

            copied_spec.write_text('{"model":"different/model"}\n', encoding="utf-8")
            self.assertFalse(end_to_end.filter_complete(*validator_arguments))
            copied_spec.write_bytes(source_spec.read_bytes())

            survivor_summary["elapsed_seconds"] = "13.5"
            write_jsonl(out_dir / "variant_summary.jsonl", [survivor_summary])
            self.assertFalse(end_to_end.filter_complete(*validator_arguments))


if __name__ == "__main__":
    unittest.main()
