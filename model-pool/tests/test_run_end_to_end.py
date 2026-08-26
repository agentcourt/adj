import importlib.util
import sys
import unittest
from pathlib import Path


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


if __name__ == "__main__":
    unittest.main()
