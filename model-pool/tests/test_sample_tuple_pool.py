import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "tools" / "sample-tuple-pool.py"
SPEC = importlib.util.spec_from_file_location("sample_tuple_pool", SCRIPT)
sample_tuple_pool = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = sample_tuple_pool
SPEC.loader.exec_module(sample_tuple_pool)


def row(name, model_id, clusters):
    return {
        "name": name,
        "openrouter_model_id": model_id,
        "endpoint_model_id": name,
        "endpoint_variant_id": name,
        "provider_name": "provider",
        "clusters": clusters,
    }


def write_jsonl(path, rows):
    path.write_text("".join(json.dumps(item) + "\n" for item in rows))


def read_jsonl(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


class FixedRandom:
    def __init__(self, values):
        self.values = iter(values)
        self.stops = []

    def randrange(self, stop):
        self.stops.append(stop)
        value = next(self.values)
        if value >= stop:
            raise AssertionError(f"fixed random value {value} is outside range({stop})")
        return value


class SampleTuplePoolTests(unittest.TestCase):
    def run_sampler(self, input_path, output_path, *options):
        argv = [
            "sample-tuple-pool.py",
            str(input_path),
            "--out",
            str(output_path),
            *options,
        ]
        with mock.patch.object(sys, "argv", argv), mock.patch("builtins.print"):
            return sample_tuple_pool.main()

    def test_without_replacement_emits_each_sampling_row_once(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            directory = Path(temp_dir)
            input_path = directory / "input.jsonl"
            output_path = directory / "output.jsonl"
            rows = [
                row("a", "model-a", [0]),
                row("b", "model-b", [0]),
                row("c", "model-c", [1]),
            ]
            write_jsonl(input_path, rows)

            self.run_sampler(
                input_path,
                output_path,
                "--pool-size",
                "3",
                "--seed",
                "7",
                "--without-replacement",
                "--no-dedupe-equivalent-endpoints",
            )

            sampled = read_jsonl(output_path)
            self.assertEqual({item["name"] for item in sampled}, {"a", "b", "c"})

    def test_one_per_model_keeps_tuple_selection_uniform_over_available_tuples(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            directory = Path(temp_dir)
            input_path = directory / "input.jsonl"
            output_path = directory / "output.jsonl"
            diagnostics_path = directory / "diagnostics.jsonl"
            write_jsonl(
                input_path,
                [
                    row("a0", "model-a", [0]),
                    row("b0", "model-b", [0]),
                    row("a1", "model-a", [1]),
                    row("c2", "model-c", [2]),
                ],
            )
            fixed_random = FixedRandom([1, 0, 0, 0, 0, 0])

            with mock.patch.object(sample_tuple_pool.random, "Random", return_value=fixed_random):
                self.run_sampler(
                    input_path,
                    output_path,
                    "--diagnostics-out",
                    str(diagnostics_path),
                    "--pool-size",
                    "3",
                    "--one-per-model",
                    "--no-dedupe-equivalent-endpoints",
                )

            sampled = read_jsonl(output_path)
            diagnostics = read_jsonl(diagnostics_path)
            self.assertEqual([item["openrouter_model_id"] for item in sampled], ["model-a", "model-b", "model-c"])
            self.assertEqual(fixed_random.stops, [3, 1, 2, 1, 1, 1])
            self.assertTrue(all(item["one_per_model"] for item in diagnostics))

    def test_capacity_failures_do_not_open_outputs(self):
        cases = [
            ("without-replacement", [row("a", "model-a", [0]), row("b", "model-b", [1])], ["--without-replacement"]),
            (
                "one-per-model",
                [row("a0", "model-a", [0]), row("a1", "model-a", [1]), row("b", "model-b", [2])],
                ["--one-per-model"],
            ),
        ]
        for name, rows, option in cases:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temp_dir:
                directory = Path(temp_dir)
                input_path = directory / "input.jsonl"
                output_path = directory / "output.jsonl"
                diagnostics_path = directory / "diagnostics.jsonl"
                equivalence_path = directory / "equivalence.jsonl"
                write_jsonl(input_path, rows)
                for path in (output_path, diagnostics_path, equivalence_path):
                    path.write_text("unchanged\n")

                with self.assertRaises(RuntimeError):
                    self.run_sampler(
                        input_path,
                        output_path,
                        "--diagnostics-out",
                        str(diagnostics_path),
                        "--equivalence-out",
                        str(equivalence_path),
                        "--pool-size",
                        "3",
                        "--no-dedupe-equivalent-endpoints",
                        *option,
                    )

                for path in (output_path, diagnostics_path, equivalence_path):
                    self.assertEqual(path.read_text(), "unchanged\n")


if __name__ == "__main__":
    unittest.main()
