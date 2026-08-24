from __future__ import annotations

import datetime as dt
import fcntl
import json
import os
import re
import shutil
import socket
import tempfile
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path, PurePosixPath
from typing import Any, Iterator


FORMAT_VERSION = 1
PENDING_FILE_PREFIX = ".run-record.tmp-"


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def canonical_path(path: Path) -> Path:
    return path.expanduser().resolve()


def safe_part(value: object) -> str:
    text = str(value or "record").lower()
    text = re.sub(r"[^a-z0-9._-]+", "-", text)
    text = re.sub(r"-+", "-", text).strip("-")
    return text[:80] or "record"


def relative_snapshot_path(value: str | Path) -> Path:
    text = Path(value).as_posix()
    pure = PurePosixPath(text)
    if pure.is_absolute() or ".." in pure.parts or not pure.parts:
        raise ValueError(f"invalid snapshot path: {value}")
    return Path(*pure.parts)


@dataclass(frozen=True)
class InputFile:
    label: str
    source_path: Path
    snapshot_path: Path

    def descriptor(self) -> dict[str, str]:
        return {
            "label": self.label,
            "source_path": str(self.source_path),
            "snapshot_path": self.snapshot_path.as_posix(),
        }


@dataclass(frozen=True)
class GeneratedInput:
    label: str
    snapshot_path: Path
    data: bytes

    def descriptor(self) -> dict[str, str]:
        return {
            "label": self.label,
            "snapshot_path": self.snapshot_path.as_posix(),
        }


def input_file(label: str, source_path: Path, snapshot_path: str | Path) -> InputFile:
    source = canonical_path(source_path)
    if not source.is_file():
        raise ValueError(f"input file does not exist: {source}")
    return InputFile(label, source, relative_snapshot_path(snapshot_path))


def generated_input(label: str, snapshot_path: str | Path, data: bytes) -> GeneratedInput:
    return GeneratedInput(label, relative_snapshot_path(snapshot_path), data)


@dataclass
class RunLock:
    run_dir: Path
    kind: str
    argv: list[str]
    file_descriptor: int

    def record_owner(self) -> None:
        owner = {
            "format_version": FORMAT_VERSION,
            "kind": self.kind,
            "pid": os.getpid(),
            "hostname": socket.gethostname(),
            "acquired_at": utc_now(),
            "argv": self.argv,
        }
        path = self.run_dir / "RUN_OWNER.json"
        atomic_write_bytes(
            path,
            (json.dumps(owner, ensure_ascii=False, allow_nan=False, indent=2, sort_keys=True) + "\n").encode("utf-8"),
        )


def _flock_acquisition_pid(file_descriptor: int) -> int | None:
    stat = os.fstat(file_descriptor)
    identity = f"{os.major(stat.st_dev):02x}:{os.minor(stat.st_dev):02x}:{stat.st_ino}"
    try:
        lines = Path("/proc/locks").read_text().splitlines()
    except OSError:
        return None
    for line in lines:
        fields = line.split()
        if len(fields) >= 6 and fields[1] == "FLOCK" and fields[5].lower() == identity.lower():
            try:
                return int(fields[4])
            except ValueError:
                return None
    return None


@contextmanager
def exclusive_run_lock(run_dir: Path, kind: str, argv: list[str]) -> Iterator[RunLock]:
    run_dir = canonical_path(run_dir)
    flags = os.O_RDONLY | getattr(os, "O_DIRECTORY", 0)
    file_descriptor = os.open(run_dir, flags)
    try:
        try:
            fcntl.flock(file_descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            owner_path = run_dir / "RUN_OWNER.json"
            try:
                owner = strict_json_loads(owner_path.read_bytes(), owner_path)
            except FileNotFoundError:
                owner = None
            except ValueError as owner_error:
                owner = {"metadata_error": str(owner_error)}
            acquisition_pid = _flock_acquisition_pid(file_descriptor)
            detail = {"lock_acquisition_pid": acquisition_pid, "last_recorded_owner": owner}
            raise ValueError(f"run lock is held for {run_dir}: {json.dumps(detail, sort_keys=True)}") from exc
        yield RunLock(run_dir, kind, list(argv), file_descriptor)
    finally:
        try:
            fcntl.flock(file_descriptor, fcntl.LOCK_UN)
        finally:
            os.close(file_descriptor)


def strict_json_loads(data: str | bytes, source: object) -> Any:
    def reject_constant(value: str) -> None:
        raise ValueError(f"{source}: invalid JSON constant {value}")

    def unique_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f"{source}: duplicate JSON key {key!r}")
            result[key] = value
        return result

    try:
        return json.loads(data, parse_constant=reject_constant, object_pairs_hook=unique_object)
    except UnicodeDecodeError as exc:
        raise ValueError(f"{source}: input is not UTF-8") from exc
    except json.JSONDecodeError as exc:
        raise ValueError(f"{source}: invalid JSON: {exc.msg}") from exc


def parse_jsonl_objects(data: bytes, source: Path) -> list[dict[str, Any]]:
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ValueError(f"{source}: input is not UTF-8") from exc
    rows: list[dict[str, Any]] = []
    for line_number, line in enumerate(text.splitlines(), start=1):
        if not line.strip():
            continue
        row = strict_json_loads(line, f"{source}:{line_number}")
        if not isinstance(row, dict):
            raise ValueError(f"{source}:{line_number}: expected a JSON object")
        rows.append(row)
    if not rows:
        raise ValueError(f"{source}: no JSON objects found")
    return rows


def _record_source_path(record_dir: str, source_root: Path) -> Path:
    path = Path(record_dir)
    if not path.is_absolute():
        path = source_root / path
    path = canonical_path(path)
    if not path.is_dir():
        raise ValueError(f"record directory does not exist: {path}")
    return path


def _evidence_source_path(record_dir: Path, file_name: str) -> tuple[Path, Path]:
    relative = relative_snapshot_path(file_name)
    path = canonical_path(record_dir / relative)
    root = canonical_path(record_dir)
    if path != root and root not in path.parents:
        raise ValueError(f"evidence path escapes record directory: {file_name}")
    if not path.is_file():
        raise ValueError(f"evidence file does not exist: {path}")
    return path, relative


def questions_snapshot(
    source_path: Path,
    source_root: Path,
    run_dir: Path,
    *,
    source_snapshot_path: str = "inputs/questions.source.jsonl",
    runtime_snapshot_path: str = "inputs/questions.jsonl",
    records_snapshot_root: str = "inputs/records",
) -> tuple[list[InputFile], GeneratedInput, list[dict[str, Any]]]:
    question_input = input_file("questions", source_path, source_snapshot_path)
    rows = parse_jsonl_objects(question_input.source_path.read_bytes(), question_input.source_path)
    inputs = [question_input]
    record_targets: dict[Path, Path] = {}
    runtime_rows: list[dict[str, Any]] = []
    records_root = relative_snapshot_path(records_snapshot_root)

    for row_number, source_row in enumerate(rows, start=1):
        row = dict(source_row)
        if row.get("mode") != "tool_record":
            runtime_rows.append(row)
            continue
        record_dir_value = row.get("record_dir")
        if not isinstance(record_dir_value, str) or not record_dir_value.strip():
            raise ValueError(f"{question_input.source_path}:{row_number}: tool_record item requires record_dir")
        record_source = _record_source_path(record_dir_value, source_root)
        record_target = record_targets.get(record_source)
        if record_target is None:
            item_name = safe_part(row.get("id") or f"row-{row_number}")
            record_target = records_root / f"{len(record_targets) + 1:03d}-{item_name}"
            record_targets[record_source] = record_target

            manifest_path = record_source / "manifest.json"
            manifest_input = input_file(
                f"record:{row.get('id') or row_number}:manifest",
                manifest_path,
                record_target / "manifest.json",
            )
            manifest = strict_json_loads(manifest_input.source_path.read_bytes(), manifest_path)
            if not isinstance(manifest, dict) or not isinstance(manifest.get("evidence"), list):
                raise ValueError(f"{manifest_path}: evidence must be an array")
            inputs.append(manifest_input)
            seen_evidence_paths: set[Path] = set()
            for evidence_number, evidence in enumerate(manifest["evidence"], start=1):
                if not isinstance(evidence, dict) or not isinstance(evidence.get("file"), str):
                    raise ValueError(f"{manifest_path}: evidence {evidence_number} requires a file string")
                evidence_source, evidence_relative = _evidence_source_path(record_source, evidence["file"])
                if evidence_relative in seen_evidence_paths:
                    raise ValueError(f"{manifest_path}: duplicate evidence file {evidence['file']}")
                seen_evidence_paths.add(evidence_relative)
                inputs.append(
                    input_file(
                        f"record:{row.get('id') or row_number}:evidence:{evidence.get('id') or evidence_number}",
                        evidence_source,
                        record_target / evidence_relative,
                    )
                )
        row["record_dir"] = str(canonical_path(run_dir) / record_target)
        runtime_rows.append(row)

    runtime_data = b"".join(
        (json.dumps(row, ensure_ascii=False, sort_keys=True) + "\n").encode("utf-8")
        for row in runtime_rows
    )
    return inputs, generated_input("runtime_questions", runtime_snapshot_path, runtime_data), runtime_rows


def _describe_difference(expected: Any, found: Any, path: str) -> list[str]:
    if type(expected) is not type(found):
        return [f"{path}: expected {expected!r}, found {found!r}"]
    if isinstance(expected, dict):
        differences: list[str] = []
        keys = sorted(set(expected) | set(found))
        for key in keys:
            child = f"{path}.{key}"
            if key not in expected:
                differences.append(f"{child}: unexpected value {found[key]!r}")
            elif key not in found:
                differences.append(f"{child}: missing; expected {expected[key]!r}")
            else:
                differences.extend(_describe_difference(expected[key], found[key], child))
        return differences
    if isinstance(expected, list):
        if len(expected) != len(found):
            return [f"{path}: expected {len(expected)} entries, found {len(found)}"]
        differences = []
        for index, (expected_item, found_item) in enumerate(zip(expected, found, strict=True)):
            differences.extend(_describe_difference(expected_item, found_item, f"{path}[{index}]"))
        return differences
    if expected != found:
        return [f"{path}: expected {expected!r}, found {found!r}"]
    return []


def _ensure_unique_snapshot_paths(inputs: list[InputFile], generated: list[GeneratedInput]) -> None:
    paths: dict[Path, str] = {}
    for item in [*inputs, *generated]:
        prior = paths.get(item.snapshot_path)
        if prior is not None:
            raise ValueError(f"snapshot path {item.snapshot_path} is used by both {prior} and {item.label}")
        paths[item.snapshot_path] = item.label


def files_equal(first: Path, second: Path, chunk_size: int = 1024 * 1024) -> bool:
    try:
        if first.stat().st_size != second.stat().st_size:
            return False
        with first.open("rb") as first_handle, second.open("rb") as second_handle:
            while True:
                first_chunk = first_handle.read(chunk_size)
                second_chunk = second_handle.read(chunk_size)
                if first_chunk != second_chunk:
                    return False
                if not first_chunk:
                    return True
    except FileNotFoundError:
        return False


def fsync_directory(path: Path) -> None:
    descriptor = os.open(path, os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def atomic_write_bytes(destination: Path, data: bytes) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix=PENDING_FILE_PREFIX, dir=destination.parent)
    temporary = Path(name)
    try:
        with os.fdopen(descriptor, "wb", closefd=True) as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, destination)
        fsync_directory(destination.parent)
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def copy_file(source: Path, destination: Path, chunk_size: int = 1024 * 1024) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix=PENDING_FILE_PREFIX, dir=destination.parent)
    temporary = Path(name)
    try:
        with source.open("rb") as source_handle, os.fdopen(descriptor, "wb", closefd=True) as destination_handle:
            while chunk := source_handle.read(chunk_size):
                destination_handle.write(chunk)
            destination_handle.flush()
            os.fsync(destination_handle.fileno())
        os.replace(temporary, destination)
        fsync_directory(destination.parent)
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def pending_temporary_files(root: Path) -> list[Path]:
    return sorted(path for path in root.rglob(f"{PENDING_FILE_PREFIX}*") if path.is_file())


def pending_durable_files(root: Path) -> list[Path]:
    return sorted(
        path
        for path in root.rglob("*")
        if path.is_file() and not path.name.startswith(PENDING_FILE_PREFIX)
    )


def reject_unexpected_pending_files(
    pending: Path,
    inputs: list[InputFile],
    generated: list[GeneratedInput],
) -> None:
    allowed = {pending / "manifest.json"}
    allowed.update(pending / item.snapshot_path for item in [*inputs, *generated])
    for path in pending.rglob("*"):
        if not path.is_file() or path in allowed or path.name.startswith(PENDING_FILE_PREFIX):
            continue
        raise ValueError(f"unexpected durable file in pending run record: {path}")


def validate_snapshot_trees(record_root: Path, snapshot_paths: list[Path]) -> None:
    expected = {record_root / path for path in snapshot_paths}
    top_levels = {path.parts[0] for path in snapshot_paths}
    for top_level in top_levels:
        root = record_root / top_level
        if not root.exists():
            continue
        if root.is_symlink() or not root.is_dir():
            raise ValueError(f"snapshot root is not a real directory: {root}")
        for path in root.rglob("*"):
            if path.is_symlink():
                raise ValueError(f"snapshot path is a symbolic link: {path}")
            if path.is_file() and path.name.startswith(PENDING_FILE_PREFIX):
                continue
            if path.is_file() and path not in expected:
                raise ValueError(f"unexpected snapshot file: {path}")
            if not path.is_file() and not path.is_dir():
                raise ValueError(f"unexpected snapshot entry: {path}")


def _load_matching_manifest(manifest_path: Path, expected: dict[str, Any], run_dir: Path) -> dict[str, Any]:
    try:
        manifest = strict_json_loads(manifest_path.read_bytes(), manifest_path)
    except FileNotFoundError as exc:
        raise ValueError(f"resume manifest does not exist: {manifest_path}") from exc
    if not isinstance(manifest, dict):
        raise ValueError(f"resume manifest must be a JSON object: {manifest_path}")
    expected_keys = set(expected) | {"created_at", "run_dir"}
    if set(manifest) != expected_keys:
        missing = sorted(expected_keys - set(manifest))
        extra = sorted(set(manifest) - expected_keys)
        raise ValueError(f"resume manifest fields differ: missing={missing}, unexpected={extra}")
    if not isinstance(manifest["created_at"], str) or not manifest["created_at"]:
        raise ValueError("resume manifest created_at must be a nonempty string")
    if manifest["run_dir"] != str(run_dir):
        raise ValueError(f"resume manifest run_dir is {manifest['run_dir']!r}; expected {str(run_dir)!r}")
    found = {key: manifest.get(key) for key in expected}
    differences = _describe_difference(expected, found, "manifest")
    if differences:
        raise ValueError("resume parameters or input locations differ:\n" + "\n".join(differences))
    return manifest


def _snapshot_candidate(run_dir: Path, pending: Path, relative: Path) -> Path:
    published = run_dir / relative
    staged = pending / relative
    if published.is_file():
        if staged.is_file() and not files_equal(published, staged):
            raise ValueError(f"published and pending snapshots differ: {published} and {staged}")
        return published
    return staged


def _validate_pending_root_record(
    run_dir: Path,
    pending: Path,
    expected: dict[str, Any],
    inputs: list[InputFile],
    generated: list[GeneratedInput],
    *,
    allow_missing: bool = False,
) -> None:
    _load_matching_manifest(pending / "manifest.json", expected, run_dir)
    reject_unexpected_pending_files(pending, inputs, generated)
    for item in inputs:
        snapshot = _snapshot_candidate(run_dir, pending, item.snapshot_path)
        if not snapshot.is_file():
            if allow_missing:
                continue
            raise ValueError(f"pending input snapshot does not exist: {snapshot}")
        if not files_equal(item.source_path, snapshot):
            raise ValueError(f"resume source differs from saved snapshot: {item.source_path}")
    for item in generated:
        snapshot = _snapshot_candidate(run_dir, pending, item.snapshot_path)
        try:
            saved = snapshot.read_bytes()
        except FileNotFoundError as exc:
            if allow_missing:
                continue
            raise ValueError(f"pending generated input snapshot does not exist: {snapshot}") from exc
        if saved != item.data:
            raise ValueError(f"generated input differs from saved snapshot: {snapshot}")


def _complete_pending_root_record(
    run_dir: Path,
    pending: Path,
    expected: dict[str, Any],
    inputs: list[InputFile],
    generated: list[GeneratedInput],
) -> None:
    _validate_pending_root_record(
        run_dir,
        pending,
        expected,
        inputs,
        generated,
        allow_missing=True,
    )
    for temporary in pending_temporary_files(pending):
        temporary.unlink()
    for item in inputs:
        if not (run_dir / item.snapshot_path).is_file() and not (pending / item.snapshot_path).is_file():
            destination = pending / item.snapshot_path
            copy_file(item.source_path, destination)
    for item in generated:
        if not (run_dir / item.snapshot_path).is_file() and not (pending / item.snapshot_path).is_file():
            destination = pending / item.snapshot_path
            atomic_write_bytes(destination, item.data)


def _publish_pending_root_record(
    run_dir: Path,
    pending: Path,
    expected: dict[str, Any],
    inputs: list[InputFile],
    generated: list[GeneratedInput],
) -> None:
    _validate_pending_root_record(run_dir, pending, expected, inputs, generated)
    for item in [*inputs, *generated]:
        source = pending / item.snapshot_path
        destination = run_dir / item.snapshot_path
        if destination.is_file():
            continue
        if not source.is_file():
            raise ValueError(f"pending snapshot does not exist: {source}")
        destination.parent.mkdir(parents=True, exist_ok=True)
        os.rename(source, destination)
        fsync_directory(destination.parent)
    os.rename(pending / "manifest.json", run_dir / "manifest.json")
    fsync_directory(run_dir)
    shutil.rmtree(pending)


def _create_pending_root_record(
    run_dir: Path,
    expected: dict[str, Any],
    inputs: list[InputFile],
    generated: list[GeneratedInput],
) -> Path:
    pending = Path(tempfile.mkdtemp(prefix=".run-record.pending-", dir=run_dir))
    try:
        manifest = {**expected, "created_at": utc_now(), "run_dir": str(run_dir)}
        atomic_write_bytes(
            pending / "manifest.json",
            (json.dumps(manifest, ensure_ascii=False, allow_nan=False, indent=2, sort_keys=True) + "\n").encode("utf-8"),
        )
        for item in inputs:
            destination = pending / item.snapshot_path
            copy_file(item.source_path, destination)
        for item in generated:
            destination = pending / item.snapshot_path
            atomic_write_bytes(destination, item.data)
        return pending
    except Exception:
        shutil.rmtree(pending)
        raise


def prepare_run_record(
    run_dir: Path,
    *,
    kind: str,
    parameters: dict[str, Any],
    inputs: list[InputFile],
    generated_inputs: list[GeneratedInput] | None = None,
    resume: bool,
) -> None:
    run_dir = canonical_path(run_dir)
    generated = list(generated_inputs or [])
    _ensure_unique_snapshot_paths(inputs, generated)
    expected = {
        "format_version": FORMAT_VERSION,
        "kind": kind,
        "parameters": parameters,
        "inputs": [item.descriptor() for item in inputs],
        "generated_inputs": [item.descriptor() for item in generated],
    }
    try:
        json.dumps(expected, ensure_ascii=False, allow_nan=False, sort_keys=True)
    except (TypeError, ValueError) as exc:
        raise ValueError(f"run parameters are not valid JSON: {exc}") from exc
    manifest_path = run_dir / "manifest.json"

    if resume:
        if not run_dir.is_dir():
            raise ValueError(f"resume directory does not exist: {run_dir}")
        pending_records = sorted(
            path
            for path in run_dir.iterdir()
            if path.name.startswith(".run-record.pending-") and path.is_dir() and not path.is_symlink()
        )
        if len(pending_records) > 1:
            raise ValueError(f"multiple pending root records exist in {run_dir}")
        if not manifest_path.is_file():
            if not pending_records:
                raise ValueError(f"resume manifest does not exist: {manifest_path}")
            pending = pending_records[0]
            pending_manifest = pending / "manifest.json"
            if pending_manifest.is_file():
                _complete_pending_root_record(run_dir, pending, expected, inputs, generated)
                _publish_pending_root_record(run_dir, pending, expected, inputs, generated)
            else:
                unexpected = [
                    path
                    for path in run_dir.iterdir()
                    if path != pending
                ]
                if unexpected:
                    raise ValueError(
                        f"incomplete pending root record has unexpected published entries: {unexpected}"
                    )
                if pending_durable_files(pending):
                    raise ValueError(
                        f"manifestless pending root record contains captured input data: {pending}"
                    )
                shutil.rmtree(pending)
                replacement = _create_pending_root_record(run_dir, expected, inputs, generated)
                _publish_pending_root_record(run_dir, replacement, expected, inputs, generated)
        _load_matching_manifest(manifest_path, expected, run_dir)
        for item in inputs:
            snapshot = run_dir / item.snapshot_path
            if not snapshot.is_file():
                raise ValueError(f"resume input snapshot does not exist: {snapshot}")
            if not files_equal(item.source_path, snapshot):
                raise ValueError(f"resume source differs from saved snapshot: {item.source_path}")
        for item in generated:
            snapshot = run_dir / item.snapshot_path
            try:
                saved = snapshot.read_bytes()
            except FileNotFoundError as exc:
                raise ValueError(f"generated input snapshot does not exist: {snapshot}") from exc
            if saved != item.data:
                raise ValueError(f"generated input differs from saved snapshot: {snapshot}")
        validate_snapshot_trees(run_dir, [item.snapshot_path for item in [*inputs, *generated]])
        for pending in pending_records:
            if pending.exists():
                shutil.rmtree(pending)
        return

    if run_dir.exists() and any(run_dir.iterdir()):
        raise ValueError(f"output directory is not empty: {run_dir}; use --resume for the recorded run")
    run_dir.mkdir(parents=True, exist_ok=True)
    pending = _create_pending_root_record(run_dir, expected, inputs, generated)
    _publish_pending_root_record(run_dir, pending, expected, inputs, generated)


def validate_run_record_inputs(
    run_dir: Path,
    *,
    kind: str,
    parameters: dict[str, Any],
    inputs: list[InputFile],
    generated_inputs: list[GeneratedInput] | None = None,
    recorded_run_dir: Path | None = None,
) -> None:
    run_dir = canonical_path(run_dir)
    expected_recorded_dir = canonical_path(recorded_run_dir) if recorded_run_dir is not None else run_dir
    generated = list(generated_inputs or [])
    _ensure_unique_snapshot_paths(inputs, generated)
    expected = {
        "format_version": FORMAT_VERSION,
        "kind": kind,
        "parameters": parameters,
        "inputs": [item.descriptor() for item in inputs],
        "generated_inputs": [item.descriptor() for item in generated],
    }
    manifest_path = run_dir / "manifest.json"
    if manifest_path.is_file():
        _load_matching_manifest(manifest_path, expected, expected_recorded_dir)
        for item in inputs:
            snapshot = run_dir / item.snapshot_path
            if not snapshot.is_file():
                raise ValueError(f"resume input snapshot does not exist: {snapshot}")
            if not files_equal(item.source_path, snapshot):
                raise ValueError(f"resume source differs from saved snapshot: {item.source_path}")
        for item in generated:
            snapshot = run_dir / item.snapshot_path
            try:
                saved = snapshot.read_bytes()
            except FileNotFoundError as exc:
                raise ValueError(f"generated input snapshot does not exist: {snapshot}") from exc
            if saved != item.data:
                raise ValueError(f"generated input differs from saved snapshot: {snapshot}")
        validate_snapshot_trees(run_dir, [item.snapshot_path for item in [*inputs, *generated]])
        return
    if recorded_run_dir is not None:
        raise ValueError(f"saved run manifest does not exist: {manifest_path}")
    pending_records = sorted(
        path
        for path in run_dir.iterdir()
        if path.name.startswith(".run-record.pending-") and path.is_dir() and not path.is_symlink()
    )
    if len(pending_records) != 1:
        raise ValueError(f"resume manifest does not exist and exactly one pending root record was expected: {run_dir}")
    pending = pending_records[0]
    if not (pending / "manifest.json").is_file():
        if pending_durable_files(pending):
            raise ValueError(f"manifestless pending root record contains captured input data: {pending}")
        return
    _validate_pending_root_record(
        run_dir,
        pending,
        expected,
        inputs,
        generated,
        allow_missing=True,
    )


def publish_run_record(
    run_dir: Path,
    *,
    kind: str,
    parameters: dict[str, Any],
    inputs: list[InputFile],
) -> None:
    run_dir = canonical_path(run_dir)
    parent = run_dir.parent
    parent.mkdir(parents=True, exist_ok=True)
    if run_dir.exists():
        raise ValueError(f"published run record already exists: {run_dir}")
    pending = Path(tempfile.mkdtemp(prefix=f".{run_dir.name}.pending-", dir=parent))
    try:
        prepare_run_record(
            pending,
            kind=kind,
            parameters=parameters,
            inputs=inputs,
            resume=False,
        )
        manifest_path = pending / "manifest.json"
        manifest = strict_json_loads(manifest_path.read_bytes(), manifest_path)
        if not isinstance(manifest, dict):
            raise ValueError(f"pending run manifest must be a JSON object: {manifest_path}")
        manifest["run_dir"] = str(run_dir)
        atomic_write_bytes(
            manifest_path,
            (json.dumps(manifest, ensure_ascii=False, allow_nan=False, indent=2, sort_keys=True) + "\n").encode("utf-8"),
        )
        os.rename(pending, run_dir)
        fsync_directory(parent)
    finally:
        if pending.exists():
            shutil.rmtree(pending)


def validate_saved_run_record(run_dir: Path, *, recorded_run_dir: Path | None = None) -> dict[str, Any]:
    run_dir = canonical_path(run_dir)
    expected_recorded_dir = canonical_path(recorded_run_dir) if recorded_run_dir is not None else run_dir
    manifest_path = run_dir / "manifest.json"
    try:
        manifest = strict_json_loads(manifest_path.read_bytes(), manifest_path)
    except FileNotFoundError as exc:
        raise ValueError(f"saved run manifest does not exist: {manifest_path}") from exc
    if not isinstance(manifest, dict):
        raise ValueError(f"saved run manifest must be a JSON object: {manifest_path}")
    required = {
        "format_version",
        "kind",
        "parameters",
        "inputs",
        "generated_inputs",
        "created_at",
        "run_dir",
    }
    if set(manifest) != required:
        raise ValueError(f"saved run manifest fields differ: {manifest_path}")
    if manifest.get("format_version") != FORMAT_VERSION:
        raise ValueError(f"saved run manifest format differs: {manifest_path}")
    if manifest.get("run_dir") != str(expected_recorded_dir):
        raise ValueError(f"saved run manifest directory differs: {manifest_path}")
    if manifest.get("generated_inputs") != []:
        raise ValueError(f"saved stage record cannot contain generated inputs: {manifest_path}")
    descriptors = manifest.get("inputs")
    if not isinstance(descriptors, list) or not descriptors:
        raise ValueError(f"saved stage record inputs must be a nonempty array: {manifest_path}")
    seen_snapshots: set[Path] = set()
    for index, descriptor in enumerate(descriptors, start=1):
        if not isinstance(descriptor, dict) or set(descriptor) != {"label", "source_path", "snapshot_path"}:
            raise ValueError(f"{manifest_path}: invalid input descriptor {index}")
        if not isinstance(descriptor["label"], str) or not descriptor["label"]:
            raise ValueError(f"{manifest_path}: invalid input label {index}")
        if not isinstance(descriptor["source_path"], str) or not Path(descriptor["source_path"]).is_absolute():
            raise ValueError(f"{manifest_path}: source path {index} must be absolute")
        snapshot_relative = relative_snapshot_path(descriptor["snapshot_path"])
        if snapshot_relative in seen_snapshots:
            raise ValueError(f"{manifest_path}: duplicate snapshot path {snapshot_relative}")
        seen_snapshots.add(snapshot_relative)
        source = canonical_path(Path(descriptor["source_path"]))
        snapshot = run_dir / snapshot_relative
        if not source.is_file():
            raise ValueError(f"saved stage source does not exist: {source}")
        if not snapshot.is_file():
            raise ValueError(f"saved stage snapshot does not exist: {snapshot}")
        if not files_equal(source, snapshot):
            raise ValueError(f"saved stage source differs from snapshot: {source}")
    validate_snapshot_trees(
        run_dir,
        [relative_snapshot_path(descriptor["snapshot_path"]) for descriptor in descriptors],
    )
    return manifest


def validate_pending_saved_run_record(pending_dir: Path, final_dir: Path) -> bool:
    pending_dir = canonical_path(pending_dir)
    final_dir = canonical_path(final_dir)
    if (pending_dir / "manifest.json").is_file():
        manifest_path = pending_dir / "manifest.json"
        manifest = strict_json_loads(manifest_path.read_bytes(), manifest_path)
        if not isinstance(manifest, dict):
            raise ValueError(f"pending stage manifest must be a JSON object: {manifest_path}")
        recorded = manifest.get("run_dir")
        if recorded == str(pending_dir):
            validate_saved_run_record(pending_dir)
        elif recorded == str(final_dir):
            validate_saved_run_record(pending_dir, recorded_run_dir=final_dir)
        else:
            raise ValueError(f"pending stage manifest directory differs: {manifest_path}")
        return True

    nested = sorted(
        path
        for path in pending_dir.iterdir()
        if path.name.startswith(".run-record.pending-") and path.is_dir() and not path.is_symlink()
    )
    if len(nested) != 1:
        if any(pending_dir.iterdir()):
            raise ValueError(f"partial stage record has no unique pending root record: {pending_dir}")
        return False
    manifest_path = nested[0] / "manifest.json"
    if not manifest_path.is_file():
        if pending_durable_files(nested[0]):
            raise ValueError(f"manifestless pending stage record contains captured data: {nested[0]}")
        return False
    manifest = strict_json_loads(manifest_path.read_bytes(), manifest_path)
    if not isinstance(manifest, dict):
        raise ValueError(f"pending stage manifest must be a JSON object: {manifest_path}")
    if manifest.get("run_dir") != str(pending_dir):
        raise ValueError(f"pending stage root manifest directory differs: {manifest_path}")
    if manifest.get("generated_inputs") != []:
        raise ValueError(f"pending stage record cannot contain generated inputs: {manifest_path}")
    descriptors = manifest.get("inputs")
    if not isinstance(descriptors, list) or not descriptors:
        raise ValueError(f"pending stage record inputs must be a nonempty array: {manifest_path}")
    for index, descriptor in enumerate(descriptors, start=1):
        if not isinstance(descriptor, dict) or set(descriptor) != {"label", "source_path", "snapshot_path"}:
            raise ValueError(f"{manifest_path}: invalid input descriptor {index}")
        source = canonical_path(Path(descriptor["source_path"]))
        snapshot = nested[0] / relative_snapshot_path(descriptor["snapshot_path"])
        if snapshot.exists() and (not snapshot.is_file() or not files_equal(source, snapshot)):
            raise ValueError(f"pending stage source differs from captured snapshot: {source}")
    return False
