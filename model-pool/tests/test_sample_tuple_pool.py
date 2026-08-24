import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "tools" / "sample-tuple-pool.py"


def write_jsonl(path: Path, rows: list[dict]) -> None:
    path.write_text("".join(json.dumps(row) + "\n" for row in rows))


class SampleTuplePoolTests(unittest.TestCase):
    def run_sampler(
        self,
        input_path: Path,
        output_path: Path,
        pool_size: int,
        *extra: str,
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                str(input_path),
                "--out",
                str(output_path),
                "--pool-size",
                str(pool_size),
                "--seed",
                "1",
                "--without-replacement",
                "--no-dedupe-equivalent-endpoints",
                *extra,
            ],
            cwd=ROOT,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    def test_packages_personas_beside_pool(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            first = root / "first.md"
            second = root / "second.md"
            first.write_text("first persona\n")
            second.write_text("second persona\n")
            input_path = root / "clusters.jsonl"
            write_jsonl(
                input_path,
                [
                    {
                        "endpoint_variant_id": "v1",
                        "clusters": [0],
                        "persona_file": "stale-top-level.md",
                        "persona": {
                            "id": "A/B",
                            "path": str(first),
                            "file": "stale-nested-file.md",
                            "persona_file": "stale-nested-persona-file.md",
                        },
                    },
                    {
                        "endpoint_variant_id": "v2",
                        "clusters": [1],
                        "persona_file": "stale-top-level.md",
                        "persona": str(second),
                        "persona_id": "A B",
                    },
                ],
            )
            output_path = root / "output" / "pool.jsonl"
            result = self.run_sampler(input_path, output_path, 2)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = [json.loads(line) for line in output_path.read_text().splitlines()]
            self.assertEqual(len(rows), 2)
            persona_paths = [
                output_path.parent / (row["persona"]["path"] if isinstance(row["persona"], dict) else row["persona"])
                for row in rows
            ]
            for row in rows:
                self.assertNotIn("persona_file", row)
                if isinstance(row["persona"], dict):
                    self.assertNotIn("file", row["persona"])
                    self.assertNotIn("persona_file", row["persona"])
            self.assertEqual(len(set(persona_paths)), 2)
            self.assertEqual({path.name for path in persona_paths}, {"a-b.md", "a-b-2.md"})
            self.assertEqual({path.read_text() for path in persona_paths}, {"first persona\n", "second persona\n"})
            repeated = self.run_sampler(input_path, output_path, 2)
            self.assertEqual(repeated.returncode, 0, repeated.stderr)

    def test_conflicting_persona_id_fails_before_output(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            first = root / "first.md"
            second = root / "second.md"
            first.write_text("first persona\n")
            second.write_text("second persona\n")
            input_path = root / "clusters.jsonl"
            write_jsonl(
                input_path,
                [
                    {"endpoint_variant_id": "v1", "clusters": [0], "persona": {"id": "same", "path": str(first)}},
                    {"endpoint_variant_id": "v2", "clusters": [1], "persona": {"id": "same", "path": str(second)}},
                ],
            )
            output_path = root / "output" / "pool.jsonl"
            result = self.run_sampler(input_path, output_path, 2)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("files with different contents", result.stderr)
            self.assertFalse(output_path.exists())
            self.assertFalse((output_path.parent / "personas").exists())

    def test_persona_directory_symlink_is_rejected_without_external_write(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "source.md"
            source.write_text("persona\n")
            input_path = root / "clusters.jsonl"
            write_jsonl(input_path, [{"endpoint_variant_id": "v1", "clusters": [0], "persona": {"id": "a", "path": str(source)}}])
            output_dir = root / "output"
            output_dir.mkdir()
            external = root / "external"
            external.mkdir()
            sentinel = external / "sentinel"
            sentinel.write_bytes(b"unchanged\n")
            (output_dir / "personas").symlink_to(external, target_is_directory=True)

            result = self.run_sampler(input_path, output_dir / "pool.jsonl", 1)

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("symbolic link", result.stderr)
            self.assertEqual(sentinel.read_bytes(), b"unchanged\n")
            self.assertEqual(sorted(path.name for path in external.iterdir()), ["sentinel"])
            self.assertFalse((output_dir / "pool.jsonl").exists())

    def test_persona_file_symlink_is_rejected_without_external_write(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "source.md"
            source.write_text("persona\n")
            input_path = root / "clusters.jsonl"
            write_jsonl(input_path, [{"endpoint_variant_id": "v1", "clusters": [0], "persona": {"id": "a", "path": str(source)}}])
            output_dir = root / "output"
            persona_dir = output_dir / "personas"
            persona_dir.mkdir(parents=True)
            external = root / "external.md"
            external.write_bytes(b"unchanged\n")
            (persona_dir / "a.md").symlink_to(external)

            result = self.run_sampler(input_path, output_dir / "pool.jsonl", 1)

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("regular file", result.stderr)
            self.assertEqual(external.read_bytes(), b"unchanged\n")
            self.assertTrue((persona_dir / "a.md").is_symlink())
            self.assertFalse((output_dir / "pool.jsonl").exists())

    def test_persona_root_rejects_parent_escape_before_output(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            persona_root = root / "personas-source"
            persona_root.mkdir()
            external = root / "external.md"
            external.write_text("external persona\n")
            input_path = root / "clusters.jsonl"
            write_jsonl(
                input_path,
                [{"endpoint_variant_id": "v1", "clusters": [0], "persona": {"id": "a", "path": "../external.md"}}],
            )
            output_path = root / "output" / "pool.jsonl"

            result = self.run_sampler(
                input_path,
                output_path,
                1,
                "--persona-root",
                str(persona_root),
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("escapes --persona-root", result.stderr)
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())


if __name__ == "__main__":
    unittest.main()
