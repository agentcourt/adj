import argparse
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools"
sys.path.insert(0, str(TOOLS))

import run_end_to_end as end_to_end


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def write_jsonl(path: Path, rows: list[dict]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        "".join(json.dumps(row, ensure_ascii=False) + "\n" for row in rows),
        encoding="utf-8",
    )


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def load_jsonl(path: Path) -> list[dict]:
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line]


def tree_snapshot(root: Path) -> tuple[tuple[str, ...], dict[str, bytes]]:
    directories = tuple(
        str(path.relative_to(root))
        for path in sorted(root.rglob("*"))
        if path.is_dir()
    )
    files = {
        str(path.relative_to(root)): path.read_bytes()
        for path in sorted(root.rglob("*"))
        if path.is_file()
    }
    return directories, files


class EndToEndResumeTests(unittest.TestCase):
    def run_tool(self, script: str, *arguments: str) -> None:
        result = subprocess.run(
            [sys.executable, str(TOOLS / script), *arguments],
            cwd=ROOT,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_aggregate_validator_rejects_consistent_alternate_result(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            clusters_path = temp / "clusters.jsonl"
            fit_path = temp / "cluster-fit.json"
            variants_path = temp / "variants.jsonl"
            out_dir = temp / "aggregate"
            variant = {
                "endpoint_variant_id": "variant-1",
                "combined_index": 1,
                "openrouter_model_id": "provider/model",
                "provider_name": "provider",
                "endpoint_tag": "tag",
                "quantization": "fp16",
            }
            write_jsonl(variants_path, [variant])
            cluster_rows = [
                {
                    **variant,
                    "persona_id": "persona-1",
                    "persona_path": "personas/persona-1.md",
                    "gene_index": gene_index,
                    "gene": f"gene {gene_index}",
                    "sample_index": sample_index,
                    "cluster": cluster,
                    "pca": pca,
                }
                for gene_index, samples in (
                    (0, [(0, [0.0, 0.0]), (0, [0.2, 0.1])]),
                    (1, [(0, [0.1, 0.0]), (1, [10.0, 10.0])]),
                )
                for sample_index, (cluster, pca) in enumerate(samples)
            ]
            write_jsonl(clusters_path, cluster_rows)
            write_json(
                fit_path,
                {
                    "genes": {
                        "0": {"centers": [[0.0, 0.0], [10.0, 10.0]]},
                        "1": {"centers": [[0.0, 0.0], [10.0, 10.0]]},
                    }
                },
            )

            self.run_tool(
                "aggregate_variant_persona_clusters.py",
                "--clusters",
                str(clusters_path),
                "--cluster-fit",
                str(fit_path),
                "--variants",
                str(variants_path),
                "--out",
                str(out_dir),
                "--expected-samples-per-gene",
                "2",
            )
            validator_arguments = (
                out_dir,
                clusters_path,
                fit_path,
                variants_path,
                2,
                False,
            )
            self.assertTrue(end_to_end.aggregate_complete(*validator_arguments))

            records_path = out_dir / "variant-persona-clusters.jsonl"
            json_path = out_dir / "variant-persona-clusters.json"
            summary_path = out_dir / "summary.json"
            rows = load_jsonl(records_path)
            self.assertEqual(rows[0]["cluster_details"][1]["method"], "center_distance_tie_break")
            rows[0]["clusters"][1] = 0
            rows[0]["cluster_details"][1] = {
                "gene_index": 1,
                "gene": "gene 1",
                "cluster": 0,
                "method": "unanimous",
                "sample_clusters": [0, 0],
                "cluster_counts": {"0": 2},
                "tie_break_sample_index": None,
            }
            summary = load_json(summary_path)
            summary["aggregation_method_counts"] = {"unanimous": 2}
            write_jsonl(records_path, rows)
            write_json(json_path, rows)
            write_json(summary_path, summary)

            self.assertFalse(end_to_end.aggregate_complete(*validator_arguments))

    def test_pool_validator_rejects_consistent_alternate_representative(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            source_path = temp / "variant-persona-clusters.jsonl"
            persona_path = temp / "persona.md"
            out_dir = temp / "pool"
            persona_path.write_text("persona text\n", encoding="utf-8")
            common = {
                "openrouter_model_id": "provider/model",
                "quantization": "fp16",
                "clusters": [0, 1],
                "persona": {"id": "persona-1", "path": str(persona_path)},
            }
            source_rows = [
                {
                    **common,
                    "endpoint_variant_id": "variant-1",
                    "provider_name": "alpha",
                    "endpoint_tag": "alpha-tag",
                },
                {
                    **common,
                    "endpoint_variant_id": "variant-2",
                    "provider_name": "beta",
                    "endpoint_tag": "beta-tag",
                },
            ]
            write_jsonl(source_path, source_rows)
            self.run_tool(
                "sample-tuple-pool.py",
                str(source_path),
                "--out",
                str(out_dir / "pool.jsonl"),
                "--diagnostics-out",
                str(out_dir / "diagnostics.jsonl"),
                "--equivalence-out",
                str(out_dir / "equivalence.jsonl"),
                "--pool-size",
                "3",
                "--seed",
                "7",
            )
            validator_arguments = (out_dir, 3, persona_path, source_path, 7, False)
            self.assertTrue(end_to_end.pool_complete(*validator_arguments))

            pool_path = out_dir / "pool.jsonl"
            diagnostics_path = out_dir / "diagnostics.jsonl"
            equivalence_path = out_dir / "equivalence.jsonl"
            pool_rows = load_jsonl(pool_path)
            diagnostics = load_jsonl(diagnostics_path)
            equivalence = load_jsonl(equivalence_path)
            self.assertTrue(all(row["endpoint_variant_id"] == "variant-1" for row in pool_rows))
            self.assertEqual(equivalence[0]["representative_endpoint_variant_id"], "variant-1")

            equivalence[0].update(
                {
                    "representative_source_row": 2,
                    "representative_endpoint_variant_id": "variant-2",
                    "representative_provider_name": "beta",
                    "representative_endpoint_tag": "beta-tag",
                }
            )
            for row in pool_rows:
                row.update(
                    {
                        "endpoint_variant_id": "variant-2",
                        "provider_name": "beta",
                        "endpoint_tag": "beta-tag",
                        "representative_source_row": 2,
                        "representative_endpoint_variant_id": "variant-2",
                    }
                )
            for index, diagnostic in enumerate(diagnostics, start=1):
                diagnostic.update(
                    {
                        "source_row": 2,
                        "source_row_count": index,
                        "provider_name": "beta",
                        "endpoint_tag": "beta-tag",
                        "endpoint_identifier": "variant-2",
                        "representative_source_row": 2,
                        "representative_endpoint_variant_id": "variant-2",
                    }
                )
            write_jsonl(pool_path, pool_rows)
            write_jsonl(diagnostics_path, diagnostics)
            write_jsonl(equivalence_path, equivalence)

            self.assertFalse(end_to_end.pool_complete(*validator_arguments))

    def test_pending_provider_validation_precedes_finalization_and_later_failure_is_read_only(self) -> None:
        args = argparse.Namespace(gene_index=[], gene_count=0)
        with tempfile.TemporaryDirectory() as directory:
            run_dir = Path(directory) / "valid"
            pending_path = run_dir / "stage-records" / ".inventory.pending-valid"
            final_path = run_dir / "stage-records" / "inventory"
            write_json(pending_path / "manifest.json", {"run_dir": str(pending_path.resolve())})
            with mock.patch.object(
                end_to_end,
                "validate_pending_saved_run_record",
                return_value=True,
            ):
                pending = end_to_end.preflight_stage_records(run_dir)

            def validate_pending(
                _args: argparse.Namespace,
                _run_dir: Path,
                stage: str,
                record_path: Path,
                _pca_dimensions: int | None,
            ) -> None:
                self.assertEqual(stage, "inventory")
                self.assertEqual(record_path, pending_path)
                self.assertTrue(pending_path.is_dir())
                self.assertFalse(final_path.exists())

            with mock.patch.object(
                end_to_end,
                "validate_completed_stage_semantics",
                side_effect=validate_pending,
            ) as semantic_validation:
                completed = end_to_end.preflight_completed_stages(args, run_dir, pending)
            semantic_validation.assert_called_once()
            self.assertEqual(completed, {"inventory": pending_path})
            self.assertTrue(pending_path.is_dir())
            self.assertFalse(final_path.exists())

            interrupted = end_to_end.finalize_pending_stage_records(run_dir, pending)
            self.assertEqual(interrupted, set())
            self.assertFalse(pending_path.exists())
            self.assertTrue(final_path.is_dir())
            self.assertEqual(load_json(final_path / "manifest.json")["run_dir"], str(final_path.resolve()))

        with tempfile.TemporaryDirectory() as directory:
            run_dir = Path(directory) / "invalid-later"
            pending_path = run_dir / "stage-records" / ".inventory.pending-invalid-later"
            eval_record = run_dir / "stage-records" / "eval"
            write_json(pending_path / "manifest.json", {"run_dir": str(pending_path.resolve())})
            (pending_path / ".run-record.tmp-stage").write_bytes(b"pending temporary bytes\n")
            write_json(eval_record / "manifest.json", {"stage": "eval"})
            (run_dir / ".run-record.tmp-root").write_bytes(b"root temporary bytes\n")
            write_json(run_dir / "RUN_OWNER.json", {"pid": 123})
            (run_dir / "commands.jsonl").write_bytes(b"command bytes\n")
            (run_dir / "filtered").mkdir()
            (run_dir / "filtered" / "partial.jsonl").write_bytes(b"local output bytes\n")
            before = tree_snapshot(run_dir)
            required_files = {
                ".run-record.tmp-root",
                "RUN_OWNER.json",
                "commands.jsonl",
                "filtered/partial.jsonl",
                "stage-records/.inventory.pending-invalid-later/.run-record.tmp-stage",
                "stage-records/.inventory.pending-invalid-later/manifest.json",
            }
            self.assertTrue(required_files.issubset(before[1]))

            with (
                mock.patch.object(
                    end_to_end,
                    "validate_pending_saved_run_record",
                    return_value=True,
                ),
                mock.patch.object(end_to_end, "validate_saved_run_record"),
            ):
                pending = end_to_end.preflight_stage_records(run_dir)

            stages: list[str] = []

            def fail_later(
                _args: argparse.Namespace,
                _run_dir: Path,
                stage: str,
                record_path: Path,
                _pca_dimensions: int | None,
            ) -> None:
                stages.append(stage)
                if stage == "inventory":
                    self.assertEqual(record_path, pending_path)
                    return
                raise ValueError("injected invalid eval semantics")

            with (
                mock.patch.object(
                    end_to_end,
                    "validate_completed_stage_semantics",
                    side_effect=fail_later,
                ),
                mock.patch.object(
                    end_to_end,
                    "finalize_pending_stage_records",
                    wraps=end_to_end.finalize_pending_stage_records,
                ) as finalize,
            ):
                with self.assertRaisesRegex(ValueError, "invalid eval semantics"):
                    completed = end_to_end.preflight_completed_stages(args, run_dir, pending)
                    end_to_end.finalize_pending_stage_records(run_dir, pending)
                    self.fail(f"unexpected completed stages: {completed}")
            self.assertEqual(stages, ["inventory", "eval"])
            finalize.assert_not_called()
            self.assertEqual(tree_snapshot(run_dir), before)

    def test_interrupted_local_stage_is_quarantined_only_after_preflight(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            run_dir = Path(directory) / "run"
            pending_path = run_dir / "stage-records" / ".filter.pending-interrupted"
            output_path = run_dir / "filtered"
            write_json(pending_path / "manifest.json", {"run_dir": str(pending_path.resolve())})
            (output_path / "partial.jsonl").parent.mkdir(parents=True)
            (output_path / "partial.jsonl").write_bytes(b"partial filter output\n")
            args = argparse.Namespace(gene_index=[], gene_count=0)
            before = tree_snapshot(run_dir)

            with mock.patch.object(
                end_to_end,
                "validate_pending_saved_run_record",
                return_value=True,
            ):
                pending = end_to_end.preflight_stage_records(run_dir)
            completed = end_to_end.preflight_completed_stages(args, run_dir, pending)
            self.assertEqual(completed, {})
            self.assertEqual(tree_snapshot(run_dir), before)
            self.assertTrue(pending_path.is_dir())
            self.assertTrue(output_path.is_dir())

            interrupted = end_to_end.finalize_pending_stage_records(run_dir, pending)
            self.assertEqual(interrupted, {"filter"})
            self.assertFalse(pending_path.exists())
            self.assertFalse(end_to_end.stage_record_dir(run_dir, "filter").exists())
            args.interrupted_stage_records = interrupted
            end_to_end.quarantine_interrupted_local_stages(args, run_dir, completed)

            self.assertFalse(output_path.exists())
            destinations = list((run_dir / "interrupted-local-stages").glob("filter-*"))
            self.assertEqual(len(destinations), 1)
            self.assertEqual((destinations[0] / "partial.jsonl").read_bytes(), b"partial filter output\n")


if __name__ == "__main__":
    unittest.main()
