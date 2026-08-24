import contextlib
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


TOOLS = Path(__file__).resolve().parents[1] / "tools"
sys.path.insert(0, str(TOOLS))

import model_inventory


class ModelInventoryResumeTests(unittest.TestCase):
    def setUp(self) -> None:
        self.catalog = {
            "data": [
                {"id": "author/first"},
                {"id": "author/second"},
            ]
        }
        self.endpoint_bodies = {
            "author/first": {
                "data": {
                    "id": "author/first",
                    "endpoints": [
                        {
                            "provider_name": "First Provider",
                            "name": "First endpoint",
                            "tag": "first/fp8",
                            "quantization": "fp8",
                        }
                    ],
                }
            },
            "author/second": {
                "data": {
                    "id": "author/second",
                    "endpoints": [
                        {
                            "provider_name": "Second Provider",
                            "name": "Second endpoint",
                            "tag": "second",
                            "quantization": None,
                        }
                    ],
                }
            },
        }

    def command(self, root: Path, *extra: str) -> list[str]:
        return [
            "--out-root",
            str(root),
            "--run-id",
            "inventory",
            "--model-id",
            "author/first",
            "--model-id",
            "author/second",
            *extra,
        ]

    def run_partial(self, root: Path) -> tuple[int, list[str]]:
        calls: list[str] = []

        def api_get(path: str, _key: str, _timeout: int, _retries: int):
            calls.append(path)
            if path == "/models":
                return self.catalog
            if path.endswith("/first/endpoints"):
                return self.endpoint_bodies["author/first"]
            raise model_inventory.OpenRouterError("temporary endpoint failure")

        with (
            mock.patch.object(model_inventory, "load_openrouter_key", return_value="test-key"),
            mock.patch.object(model_inventory, "api_get", side_effect=api_get),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            code = model_inventory.main(self.command(root))
        return code, calls

    def test_resume_reuses_recorded_query_and_fetches_only_missing_model(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, first_calls = self.run_partial(root)
            self.assertEqual(code, 1)
            self.assertEqual(
                first_calls,
                ["/models", "/models/author/first/endpoints", "/models/author/second/endpoints"],
            )
            run_dir = root / "inventory"
            manifest_before = (run_dir / "manifest.json").read_bytes()
            first_raw = run_dir / "raw" / "endpoints" / model_inventory.endpoint_raw_filename("author/first")
            first_raw_before = first_raw.read_bytes()

            resumed_calls: list[str] = []

            def resumed_api_get(path: str, _key: str, _timeout: int, _retries: int):
                resumed_calls.append(path)
                if path.endswith("/second/endpoints"):
                    return self.endpoint_bodies["author/second"]
                raise AssertionError(f"resume made an unexpected request: {path}")

            with (
                mock.patch.object(model_inventory, "load_openrouter_key", return_value="test-key"),
                mock.patch.object(model_inventory, "api_get", side_effect=resumed_api_get),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                resumed_code = model_inventory.main(self.command(root, "--resume"))

            self.assertEqual(resumed_code, 0)
            self.assertEqual(resumed_calls, ["/models/author/second/endpoints"])
            self.assertEqual((run_dir / "manifest.json").read_bytes(), manifest_before)
            self.assertEqual(first_raw.read_bytes(), first_raw_before)
            variants = [json.loads(line) for line in (run_dir / "endpoint_variants.jsonl").read_text().splitlines()]
            self.assertEqual([row["openrouter_model_id"] for row in variants], ["author/first", "author/second"])
            summary = json.loads((run_dir / "summary.json").read_text())
            self.assertTrue(summary["complete"])
            self.assertEqual(summary["reused_endpoint_fetch_count"], 1)
            self.assertEqual(summary["new_endpoint_fetch_count"], 1)

    def test_resume_rejects_changed_recorded_endpoint_before_request(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, _ = self.run_partial(root)
            self.assertEqual(code, 1)
            run_dir = root / "inventory"
            first_raw = run_dir / "raw" / "endpoints" / model_inventory.endpoint_raw_filename("author/first")
            original = first_raw.read_bytes()
            changed = original.replace(b"First Provider", b"Other Provider", 1)
            self.assertNotEqual(changed, original)
            self.assertEqual(len(changed), len(original))
            first_raw.write_bytes(changed)

            with (
                mock.patch.object(model_inventory, "load_openrouter_key", return_value="test-key"),
                mock.patch.object(model_inventory, "api_get") as api_get,
                contextlib.redirect_stdout(io.StringIO()),
                self.assertRaisesRegex(SystemExit, "differs from its saved snapshot"),
            ):
                model_inventory.main(self.command(root, "--resume"))
            api_get.assert_not_called()

    def test_resume_rejects_changed_selection_before_request(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, _ = self.run_partial(root)
            self.assertEqual(code, 1)
            command = [
                "--out-root",
                str(root),
                "--run-id",
                "inventory",
                "--model-id",
                "author/first",
                "--resume",
            ]
            with (
                mock.patch.object(model_inventory, "load_openrouter_key", return_value="test-key"),
                mock.patch.object(model_inventory, "api_get") as api_get,
                contextlib.redirect_stdout(io.StringIO()),
                self.assertRaisesRegex(SystemExit, "selection_parameters"),
            ):
                model_inventory.main(command)
            api_get.assert_not_called()

    def test_read_only_resume_validation_returns_recovery_state_without_writes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, _ = self.run_partial(root)
            self.assertEqual(code, 1)
            run_dir = root / "inventory"
            second_name = model_inventory.endpoint_raw_filename("author/second")
            partial = run_dir / "raw" / "endpoints" / f"{second_name}.tmp"
            partial.write_bytes(b"partial endpoint response")
            before = {
                path.relative_to(run_dir): path.read_bytes()
                for path in run_dir.rglob("*")
                if path.is_file()
            }

            with (
                mock.patch.object(model_inventory, "api_get") as api_get,
                mock.patch.object(model_inventory, "atomic_write") as atomic_write,
            ):
                state = model_inventory.validate_inventory_resume(
                    run_dir,
                    run_id="inventory",
                    parameters={
                        "model_ids": ["author/first", "author/second"],
                        "sample_models": None,
                        "sample_seed": None,
                    },
                )

            api_get.assert_not_called()
            atomic_write.assert_not_called()
            self.assertEqual([row["model_id"] for row in state.selected_records], ["author/first", "author/second"])
            self.assertEqual(set(state.completed), {"author/first"})
            self.assertEqual(state.partial_paths, (partial,))
            after = {
                path.relative_to(run_dir): path.read_bytes()
                for path in run_dir.rglob("*")
                if path.is_file()
            }
            self.assertEqual(after, before)

    def test_read_only_resume_validation_rejects_endpoint_identity_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, _ = self.run_partial(root)
            self.assertEqual(code, 1)
            run_dir = root / "inventory"
            first_name = model_inventory.endpoint_raw_filename("author/first")
            snapshot = run_dir / "inputs" / "endpoints" / first_name
            raw = run_dir / "raw" / "endpoints" / first_name
            body = json.loads(snapshot.read_text())
            body["data"]["id"] = "author/second"
            changed = model_inventory.pretty_json(body).encode()
            snapshot.write_bytes(changed)
            raw.write_bytes(changed)

            with (
                mock.patch.object(model_inventory, "api_get") as api_get,
                self.assertRaisesRegex(ValueError, "payload id"),
            ):
                model_inventory.validate_inventory_resume(
                    run_dir,
                    run_id="inventory",
                    parameters={
                        "model_ids": ["author/first", "author/second"],
                        "sample_models": None,
                        "sample_seed": None,
                    },
                )
            api_get.assert_not_called()

    def test_sampled_selection_records_and_resumes_one_selected_model(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            calls: list[str] = []

            def api_get(path: str, _key: str, _timeout: int, _retries: int):
                calls.append(path)
                if path == "/models":
                    return self.catalog
                for model_id, body in self.endpoint_bodies.items():
                    if path.endswith(f"/{model_id.split('/', 1)[1]}/endpoints"):
                        return body
                raise AssertionError(f"unexpected request: {path}")

            command = [
                "--out-root",
                str(root),
                "--run-id",
                "inventory",
                "--sample-models",
                "1",
                "--sample-seed",
                "7",
            ]
            with (
                mock.patch.object(model_inventory, "load_openrouter_key", return_value="test-key"),
                mock.patch.object(model_inventory, "api_get", side_effect=api_get),
                contextlib.redirect_stdout(io.StringIO()),
            ):
                code = model_inventory.main(command)
            self.assertEqual(code, 0)
            self.assertEqual(len(calls), 2)
            manifest = json.loads((root / "inventory" / "manifest.json").read_text())
            self.assertEqual(
                manifest["selection_parameters"],
                {"model_ids": None, "sample_models": 1, "sample_seed": 7},
            )
            self.assertEqual(len(manifest["selected_models"]), 1)

            with (
                mock.patch.object(model_inventory, "load_openrouter_key") as load_key,
                mock.patch.object(model_inventory, "api_get") as resumed_api_get,
                contextlib.redirect_stdout(io.StringIO()),
            ):
                resumed_code = model_inventory.main([*command, "--resume"])
            self.assertEqual(resumed_code, 0)
            load_key.assert_not_called()
            resumed_api_get.assert_not_called()

    def test_unexpected_endpoint_exception_aborts_without_error_summary(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)

            def api_get(path: str, _key: str, _timeout: int, _retries: int):
                if path == "/models":
                    return self.catalog
                raise RuntimeError("unexpected implementation failure")

            command = [
                "--out-root",
                str(root),
                "--run-id",
                "inventory",
                "--model-id",
                "author/first",
            ]
            with (
                mock.patch.object(model_inventory, "load_openrouter_key", return_value="test-key"),
                mock.patch.object(model_inventory, "api_get", side_effect=api_get),
                contextlib.redirect_stdout(io.StringIO()),
                self.assertRaisesRegex(RuntimeError, "unexpected implementation failure"),
            ):
                model_inventory.main(command)
            self.assertFalse((root / "inventory" / "summary.json").exists())


if __name__ == "__main__":
    unittest.main()
