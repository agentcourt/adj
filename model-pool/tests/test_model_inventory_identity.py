import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


TOOL_PATH = Path(__file__).resolve().parents[1] / "tools" / "model_inventory.py"
SPEC = importlib.util.spec_from_file_location("model_inventory_identity_test", TOOL_PATH)
model_inventory = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = model_inventory
SPEC.loader.exec_module(model_inventory)


def catalog_model(model_id):
    return {
        "id": model_id,
        "links": {"details": "/api/v1/models/example/model/endpoints"},
        "architecture": {"input_modalities": ["text"], "output_modalities": ["text"]},
    }


class ModelInventoryIdentityTests(unittest.TestCase):
    def test_catalog_alias_retains_its_request_model_id(self):
        catalog = {
            "data": [
                catalog_model("example/model"),
                catalog_model("example/model:free"),
            ]
        }
        endpoint_response = {
            "data": {
                "id": "example/model",
                "endpoints": [
                    {
                        "provider_name": "Provider",
                        "tag": "provider",
                        "model_id": "example/model",
                        "quantization": None,
                    }
                ],
            }
        }

        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(
            model_inventory, "load_openrouter_key", return_value="key"
        ), mock.patch.object(
            model_inventory,
            "api_get",
            side_effect=[catalog, endpoint_response, endpoint_response],
        ):
            status = model_inventory.main(
                ["--out-root", temporary, "--run-id", "inventory"]
            )
            rows = [
                json.loads(line)
                for line in (Path(temporary) / "inventory" / "endpoint_variants.jsonl").read_text().splitlines()
            ]

        self.assertEqual(status, 0)
        self.assertEqual(
            [row["openrouter_model_id"] for row in rows],
            ["example/model", "example/model:free"],
        )
        self.assertEqual(
            [row["endpoint_model_id"] for row in rows],
            ["example/model", "example/model"],
        )
        self.assertEqual(len({row["endpoint_variant_id"] for row in rows}), 2)

    def test_duplicate_catalog_rows_are_retained_and_marked_ambiguous(self):
        catalog = {"data": [catalog_model("example/model")]}
        endpoint_response = {
            "data": {
                "id": "example/model",
                "endpoints": [
                    {
                        "provider_name": "Provider",
                        "tag": "provider",
                        "model_id": "example/model",
                        "quantization": "bf16",
                        "supported_parameters": ["tools"],
                    },
                    {
                        "provider_name": "Provider",
                        "tag": "provider",
                        "model_id": "example/model",
                        "quantization": "bf16",
                        "supported_parameters": [],
                    },
                ],
            }
        }

        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(
            model_inventory, "load_openrouter_key", return_value="key"
        ), mock.patch.object(model_inventory, "api_get", side_effect=[catalog, endpoint_response]):
            status = model_inventory.main(
                ["--out-root", temporary, "--run-id", "inventory"]
            )
            rows = [
                json.loads(line)
                for line in (Path(temporary) / "inventory" / "endpoint_variants.jsonl").read_text().splitlines()
            ]
            summary = json.loads(
                (Path(temporary) / "inventory" / "summary.json").read_text()
            )

        route_id = "openrouter:example/model@provider#bf16"
        self.assertEqual(status, 0)
        self.assertEqual([row["endpoint_route_id"] for row in rows], [route_id, route_id])
        self.assertEqual(
            [row["endpoint_variant_id"] for row in rows],
            [f"{route_id}~catalog-row-0", f"{route_id}~catalog-row-1"],
        )
        self.assertEqual([row["exact_route_ambiguous"] for row in rows], [True, True])
        self.assertEqual([row["exact_route_row_count"] for row in rows], [2, 2])
        self.assertEqual(summary["ambiguous_exact_route_count"], 1)
        self.assertEqual(summary["ambiguous_exact_route_row_count"], 2)


if __name__ == "__main__":
    unittest.main()
