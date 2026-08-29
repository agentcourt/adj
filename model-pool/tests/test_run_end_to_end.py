import argparse
import csv
import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "tools" / "run_end_to_end.py"
SPEC = importlib.util.spec_from_file_location("run_end_to_end", SCRIPT)
run_end_to_end = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = run_end_to_end
SPEC.loader.exec_module(run_end_to_end)


class EvaluationPartitionTests(unittest.TestCase):
    def test_provider_routes_for_one_model_remain_in_one_process(self):
        variants = [
            {"combined_index": 1, "openrouter_model_id": "model/a"},
            {"combined_index": 2, "openrouter_model_id": "model/b"},
            {"combined_index": 3, "openrouter_model_id": "model/a"},
            {"combined_index": 4, "openrouter_model_id": "model/c"},
        ]

        partitions = run_end_to_end.partition_variants_by_model(variants, 2)
        locations = {
            row["combined_index"]: partition_index
            for partition_index, rows in enumerate(partitions)
            for row in rows
        }

        self.assertEqual(locations[1], locations[3])


class ContinuationTests(unittest.TestCase):
    def test_strict_score_filter_excludes_equal_score(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            variants_path = root / "variants.jsonl"
            variants = [
                {"combined_index": 1, "openrouter_model_id": "model/a"},
                {"combined_index": 2, "openrouter_model_id": "model/b"},
            ]
            variants_path.write_text("".join(json.dumps(row) + "\n" for row in variants))
            summary_path = root / "summary.csv"
            with summary_path.open("w", newline="") as handle:
                writer = csv.DictWriter(
                    handle,
                    fieldnames=[
                        "combined_index",
                        "run_exit_code",
                        "provider_error_count",
                        "deliberation_score",
                    ],
                )
                writer.writeheader()
                writer.writerows(
                    [
                        {
                            "combined_index": 1,
                            "run_exit_code": 0,
                            "provider_error_count": 0,
                            "deliberation_score": 0.70,
                        },
                        {
                            "combined_index": 2,
                            "run_exit_code": 0,
                            "provider_error_count": 0,
                            "deliberation_score": 0.71,
                        },
                    ]
                )
            args = argparse.Namespace(
                filter_provider_error_count=0,
                filter_min_deliberation_score=0.90,
                filter_deliberation_score_gt=0.70,
            )

            out = run_end_to_end.filter_variants(args, root / "run", variants_path, summary_path)

            survivors = run_end_to_end.load_jsonl(out / "endpoint_variants.jsonl")
            self.assertEqual([row["combined_index"] for row in survivors], [2])

    def test_gene_commands_run_in_fixed_size_groups(self):
        args = argparse.Namespace(
            gene_count=5,
            gene_index=None,
            gene_processes=2,
            genes="genes.json",
            persona="persona.md",
            persona_record_path="personas/generic.md",
            samples_per_gene=5,
            timeout=120,
            embedding_model="text-embedding-3-small",
            temperature=0.7,
            top_p=1.0,
            max_tokens=512,
            completion_attempts=3,
            retry_sleep=2.0,
        )
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with mock.patch.object(run_end_to_end, "run_processes") as run_processes:
                run_end_to_end.run_genes(args, root / "run", root / "filtered")

        self.assertEqual([len(call.args[1]) for call in run_processes.call_args_list], [2, 2, 1])

    def test_gene_filter_excludes_variant_with_error_in_any_gene(self):
        args = argparse.Namespace(samples_per_gene=2)
        variants = [
            {"combined_index": 1, "openrouter_model_id": "model/a"},
            {"combined_index": 2, "openrouter_model_id": "model/b"},
        ]

        def record(index, sample, status="ok"):
            return {
                "combined_index": index,
                "sample_index": sample,
                "status": status,
                "embedding": [1.0, 2.0] if status == "ok" else None,
                "metadata": {"error_type": "provider"} if status != "ok" else {},
            }

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            filtered = root / "filtered"
            filtered.mkdir()
            (filtered / "endpoint_variants.jsonl").write_text(
                "".join(json.dumps(row) + "\n" for row in variants)
            )
            gene_dirs = {}
            gene_records = {
                0: [record(1, 1), record(1, 2), record(2, 1), record(2, 2)],
                1: [record(1, 1), record(1, 2), record(2, 1, "completion_error"), record(2, 2)],
            }
            for gene_index, rows in gene_records.items():
                inference = root / "run" / "genes" / f"gene-{gene_index}" / "inference"
                inference.mkdir(parents=True)
                (inference / "records.jsonl").write_text(
                    "".join(json.dumps(row) + "\n" for row in rows)
                )
                (inference / "summary.json").write_text(json.dumps({"records_written": 4}))
                gene_dirs[gene_index] = inference

            eligible_dir, record_paths, eligible_count = run_end_to_end.filter_gene_records(
                args, root / "run", filtered, gene_dirs
            )

            self.assertEqual(eligible_count, 1)
            self.assertEqual(
                [row["combined_index"] for row in run_end_to_end.load_jsonl(eligible_dir / "endpoint_variants.jsonl")],
                [1],
            )
            self.assertEqual(
                [row["combined_index"] for row in run_end_to_end.load_jsonl(eligible_dir / "excluded_variants.jsonl")],
                [2],
            )
            for path in record_paths.values():
                self.assertEqual([row["combined_index"] for row in run_end_to_end.load_jsonl(path)], [1, 1])


if __name__ == "__main__":
    unittest.main()
