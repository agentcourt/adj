#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import copy
import json
import os
import random
import re
import stat
import tempfile
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any


ClusterTuple = tuple[int, ...]
EquivalenceKey = str
ROOT = Path(__file__).resolve().parents[1]


def variant_record(row: dict[str, Any]) -> dict[str, Any]:
    variant = row.get("variant")
    if isinstance(variant, dict):
        return variant
    return row


def field(row: dict[str, Any], name: str) -> Any:
    if name in row:
        return row[name]
    variant = variant_record(row)
    return variant.get(name)


def text_field(row: dict[str, Any], name: str) -> str | None:
    value = field(row, name)
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def normalized_quantization(row: dict[str, Any]) -> str | None:
    value = text_field(row, "quantization")
    if value is None:
        return None
    return value.lower()


def number_field(row: dict[str, Any], name: str) -> int | float | None:
    value = field(row, name)
    if value is None or value == "":
        return None
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value) if value.is_integer() else value
    if isinstance(value, str):
        try:
            parsed = float(value)
        except ValueError:
            return None
        return int(parsed) if parsed.is_integer() else parsed
    return None


def float_field(row: dict[str, Any], name: str) -> float | None:
    value = number_field(row, name)
    if value is None:
        return None
    return float(value)


def first_float_field(row: dict[str, Any], *names: str) -> float | None:
    for name in names:
        value = float_field(row, name)
        if value is not None:
            return value
    return None


def nested_float(row: dict[str, Any], name: str, child: str) -> float | None:
    value = field(row, name)
    if not isinstance(value, dict):
        return None
    child_value = value.get(child)
    if child_value is None or child_value == "":
        return None
    if isinstance(child_value, bool):
        return None
    try:
        return float(child_value)
    except (TypeError, ValueError):
        return None


def string_list_field(row: dict[str, Any], name: str) -> list[str]:
    value = field(row, name)
    if value is None:
        return []
    if isinstance(value, str):
        text = value.strip()
        return [text] if text else []
    if not isinstance(value, list):
        return []
    out = sorted({str(item).strip() for item in value if str(item).strip()})
    return out


def equivalence_key_object(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "openrouter_model_id": text_field(row, "openrouter_model_id"),
        "endpoint_model_id": text_field(row, "endpoint_model_id"),
        "canonical_slug": text_field(row, "canonical_slug"),
        "hugging_face_id": text_field(row, "hugging_face_id"),
        "quantization": normalized_quantization(row),
        "input_modalities": string_list_field(row, "input_modalities"),
        "output_modalities": string_list_field(row, "output_modalities"),
    }


def equivalence_key(row: dict[str, Any]) -> EquivalenceKey:
    return json.dumps(equivalence_key_object(row), ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def price_field(row: dict[str, Any], name: str) -> float | None:
    value = float_field(row, name)
    if value is not None:
        return value
    pricing = field(row, "model_pricing")
    if isinstance(pricing, dict):
        raw = pricing.get(name.removeprefix("pricing_"))
        if raw not in (None, ""):
            try:
                return float(raw)
            except (TypeError, ValueError):
                return None
    return None


def ranking_value(value: float | None, missing: float) -> float:
    return missing if value is None else value


def capacity_value(row: dict[str, Any], *names: str) -> float:
    values = [float(value) for name in names if (value := number_field(row, name)) is not None]
    if not values:
        return -1
    return max(values)


def representative_sort_key(row: dict[str, Any]) -> tuple[Any, ...]:
    provider_errors = ranking_value(first_float_field(row, "filter_provider_error_count", "provider_error_count"), 0)
    deliberation_score = ranking_value(first_float_field(row, "filter_deliberation_score", "deliberation_score"), -1)
    schema_violations = ranking_value(float_field(row, "schema_violation_count"), 0)
    timeouts = ranking_value(float_field(row, "timeout_count"), 0)
    context_errors = ranking_value(float_field(row, "context_limit_error_count"), 0)
    context_capacity = capacity_value(row, "context_length", "model_context_length")
    prompt_capacity = capacity_value(row, "max_prompt_tokens")
    completion_capacity = capacity_value(row, "max_completion_tokens")
    uptime_30m = ranking_value(nested_float(row, "uptime_last_30m", "value"), -1)
    if uptime_30m == -1:
        uptime_30m = ranking_value(float_field(row, "uptime_last_30m"), -1)
    uptime_1d = ranking_value(float_field(row, "uptime_last_1d"), -1)
    latency_p50 = ranking_value(nested_float(row, "latency_last_30m", "p50"), float("inf"))
    prompt_price = ranking_value(price_field(row, "pricing_prompt"), float("inf"))
    completion_price = ranking_value(price_field(row, "pricing_completion"), float("inf"))
    endpoint_variant_id = str(field(row, "endpoint_variant_id") or "")
    endpoint_tag = str(field(row, "endpoint_tag") or "")
    provider_name = str(field(row, "provider_name") or "")
    return (
        provider_errors,
        -deliberation_score,
        schema_violations,
        timeouts,
        context_errors,
        -context_capacity,
        -prompt_capacity,
        -completion_capacity,
        -uptime_30m,
        -uptime_1d,
        latency_p50,
        prompt_price + completion_price,
        endpoint_variant_id,
        endpoint_tag,
        provider_name,
        row["_source_row"],
    )


def endpoint_summary(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "source_row": row.get("_source_row"),
        "endpoint_variant_id": field(row, "endpoint_variant_id"),
        "openrouter_model_id": field(row, "openrouter_model_id"),
        "canonical_slug": field(row, "canonical_slug"),
        "hugging_face_id": field(row, "hugging_face_id"),
        "provider_name": field(row, "provider_name"),
        "endpoint_name": field(row, "endpoint_name"),
        "endpoint_tag": field(row, "endpoint_tag"),
        "quantization": field(row, "quantization"),
        "context_length": field(row, "context_length"),
        "max_prompt_tokens": field(row, "max_prompt_tokens"),
        "max_completion_tokens": field(row, "max_completion_tokens"),
        "supported_parameters": string_list_field(row, "supported_parameters"),
        "filter_provider_error_count": field(row, "filter_provider_error_count"),
        "filter_deliberation_score": field(row, "filter_deliberation_score"),
        "provider_error_count": field(row, "provider_error_count"),
        "deliberation_score": field(row, "deliberation_score"),
        "schema_violation_count": field(row, "schema_violation_count"),
        "timeout_count": field(row, "timeout_count"),
        "context_limit_error_count": field(row, "context_limit_error_count"),
        "uptime_last_30m": field(row, "uptime_last_30m"),
        "uptime_last_1d": field(row, "uptime_last_1d"),
        "latency_last_30m": field(row, "latency_last_30m"),
        "pricing_prompt": field(row, "pricing_prompt"),
        "pricing_completion": field(row, "pricing_completion"),
        "clusters": row.get("clusters"),
    }


def annotate_equivalence(rows: list[dict[str, Any]]) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    grouped: dict[EquivalenceKey, list[dict[str, Any]]] = defaultdict(list)
    key_objects: dict[EquivalenceKey, dict[str, Any]] = {}
    for row in rows:
        key = equivalence_key(row)
        grouped[key].append(row)
        key_objects[key] = equivalence_key_object(row)

    representatives: list[dict[str, Any]] = []
    records: list[dict[str, Any]] = []
    for key in sorted(grouped):
        members = sorted(grouped[key], key=representative_sort_key)
        representative = members[0]
        endpoints = sorted(
            (endpoint_summary(member) for member in members),
            key=lambda item: (
                str(item.get("endpoint_variant_id") or ""),
                str(item.get("provider_name") or ""),
                str(item.get("endpoint_tag") or ""),
                int(item.get("source_row") or 0),
            ),
        )
        for member in members:
            member["_equivalence_key"] = key_objects[key]
            member["_equivalence_class_size"] = len(members)
            member["_equivalent_endpoints"] = endpoints
            member["_representative_source_row"] = representative["_source_row"]
            member["_representative_endpoint_variant_id"] = field(representative, "endpoint_variant_id")
        representatives.append(representative)
        records.append({
            "equivalence_key": key_objects[key],
            "equivalence_class_size": len(members),
            "representative_source_row": representative["_source_row"],
            "representative_endpoint_variant_id": field(representative, "endpoint_variant_id"),
            "representative_provider_name": field(representative, "provider_name"),
            "representative_endpoint_tag": field(representative, "endpoint_tag"),
            "equivalent_endpoints": endpoints,
        })
    representatives.sort(key=lambda row: row["_source_row"])
    return representatives, records


def load_jsonl(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    with path.open() as handle:
        for line_num, line in enumerate(handle, start=1):
            line = line.strip()
            if not line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError as exc:
                raise RuntimeError(f"{path}:{line_num}: invalid JSON") from exc
            if not isinstance(row, dict):
                raise RuntimeError(f"{path}:{line_num}: expected JSON object")
            row["_source_row"] = len(rows) + 1
            rows.append(row)
    if not rows:
        raise RuntimeError(f"{path}: no rows found")
    return rows


def validate_rows(rows: list[dict[str, Any]]) -> int:
    cluster_count: int | None = None
    for index, row in enumerate(rows, start=1):
        clusters = row.get("clusters")
        if not isinstance(clusters, list) or not clusters:
            raise RuntimeError(f"row {index}: clusters must be a non-empty array")
        if not all(isinstance(value, int) for value in clusters):
            raise RuntimeError(f"row {index}: clusters must contain only integers")
        if cluster_count is None:
            cluster_count = len(clusters)
        elif len(clusters) != cluster_count:
            raise RuntimeError(f"row {index}: clusters length {len(clusters)} differs from expected {cluster_count}")
    assert cluster_count is not None
    return cluster_count


def group_by_tuple(rows: list[dict[str, Any]]) -> dict[ClusterTuple, list[dict[str, Any]]]:
    grouped: dict[ClusterTuple, list[dict[str, Any]]] = defaultdict(list)
    for row in rows:
        grouped[tuple(row["clusters"])].append(row)
    return dict(grouped)


def clean_row(row: dict[str, Any]) -> dict[str, Any]:
    hidden = {
        "_source_row",
        "_equivalence_key",
        "_equivalence_class_size",
        "_equivalent_endpoints",
        "_representative_source_row",
        "_representative_endpoint_variant_id",
    }
    out = {key: value for key, value in row.items() if key not in hidden}
    if "_equivalence_key" in row:
        out["equivalence_key"] = row["_equivalence_key"]
        out["equivalence_class_size"] = row["_equivalence_class_size"]
        out["representative_source_row"] = row["_representative_source_row"]
        out["representative_endpoint_variant_id"] = row["_representative_endpoint_variant_id"]
        out["equivalent_endpoints"] = row["_equivalent_endpoints"]
    return out


def endpoint_key(row: dict[str, Any]) -> str:
    endpoint_variant_id = row.get("endpoint_variant_id")
    if endpoint_variant_id:
        return str(endpoint_variant_id)
    return "|".join(
        str(row.get(key, ""))
        for key in ("openrouter_model_id", "provider_name", "endpoint_tag", "quantization")
    )


def safe_persona_name(value: str) -> str:
    text = value.lower()
    text = re.sub(r"[^a-z0-9._-]+", "-", text)
    text = re.sub(r"-+", "-", text).strip("-.")
    return text[:80] or "persona"


def persona_fields(row: dict[str, Any]) -> tuple[str, str]:
    persona = row.get("persona")
    if isinstance(persona, dict):
        persona_id = str(persona.get("id") or row.get("persona_id") or "").strip()
        reference = str(persona.get("path") or persona.get("file") or persona.get("persona_file") or "").strip()
    elif isinstance(persona, str):
        reference = persona.strip()
        persona_id = str(row.get("persona_id") or Path(reference).stem).strip()
    else:
        reference = str(row.get("persona_file") or "").strip()
        persona_id = str(row.get("persona_id") or Path(reference).stem).strip()
    if not persona_id:
        raise RuntimeError("selected row has no persona id")
    if not reference:
        raise RuntimeError(f"persona {persona_id!r} has no file path")
    return persona_id, reference


def resolve_persona_path(reference: str, input_path: Path, persona_root: Path | None) -> Path:
    path = Path(reference)
    if persona_root is not None:
        root = persona_root.expanduser().resolve()
        candidate = path.expanduser().resolve() if path.is_absolute() else (root / path).resolve()
        if candidate != root and root not in candidate.parents:
            raise RuntimeError(f"persona path escapes --persona-root: {reference!r}")
        candidates = [candidate]
    elif path.is_absolute():
        candidates = [path]
    else:
        candidates = [ROOT / path, input_path.parent / path, ROOT.parent / "common" / "etc" / path]
    seen: set[Path] = set()
    for candidate in candidates:
        candidate = candidate.expanduser().resolve()
        if candidate in seen:
            continue
        seen.add(candidate)
        if candidate.is_file():
            return candidate
    rendered = ", ".join(str(path) for path in seen)
    raise RuntimeError(f"persona file does not exist for {reference!r}; checked {rendered}")


def absolute_without_symlink_resolution(path: Path) -> Path:
    return Path(os.path.abspath(path))


def validate_real_directory_path(path: Path, *, allow_missing: bool) -> None:
    absolute = absolute_without_symlink_resolution(path)
    current = Path(absolute.anchor)
    missing = False
    for part in absolute.parts[1:]:
        current /= part
        if missing:
            continue
        try:
            metadata = os.lstat(current)
        except FileNotFoundError:
            if not allow_missing:
                raise RuntimeError(f"directory does not exist: {current}")
            missing = True
            continue
        if stat.S_ISLNK(metadata.st_mode):
            raise RuntimeError(f"directory path contains a symbolic link: {current}")
        if not stat.S_ISDIR(metadata.st_mode):
            raise RuntimeError(f"directory path component is not a directory: {current}")


def ensure_real_directory(path: Path) -> Path:
    absolute = absolute_without_symlink_resolution(path)
    current = Path(absolute.anchor)
    for part in absolute.parts[1:]:
        current /= part
        try:
            metadata = os.lstat(current)
        except FileNotFoundError:
            try:
                os.mkdir(current)
            except FileExistsError:
                pass
            metadata = os.lstat(current)
        if stat.S_ISLNK(metadata.st_mode):
            raise RuntimeError(f"directory path contains a symbolic link: {current}")
        if not stat.S_ISDIR(metadata.st_mode):
            raise RuntimeError(f"directory path component is not a directory: {current}")
    return absolute


def read_regular_file_without_symlinks(path: Path) -> bytes:
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    try:
        descriptor = os.open(path, flags)
    except FileNotFoundError:
        raise
    except OSError as exc:
        raise RuntimeError(f"persona destination is not a regular file: {path}: {exc}") from exc
    try:
        metadata = os.fstat(descriptor)
        if not stat.S_ISREG(metadata.st_mode):
            raise RuntimeError(f"persona destination is not a regular file: {path}")
        chunks: list[bytes] = []
        while chunk := os.read(descriptor, 1024 * 1024):
            chunks.append(chunk)
        return b"".join(chunks)
    finally:
        os.close(descriptor)


def publish_persona_file(path: Path, data: bytes) -> None:
    try:
        found = read_regular_file_without_symlinks(path)
    except FileNotFoundError:
        found = None
    if found is not None:
        if found != data:
            raise RuntimeError(f"persona destination contains different contents: {path}")
        return

    directory = ensure_real_directory(path.parent)
    descriptor, temporary_name = tempfile.mkstemp(prefix=f".{path.name}.pending-", dir=directory)
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb", closefd=True) as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        try:
            os.link(temporary, path, follow_symlinks=False)
        except FileExistsError:
            found = read_regular_file_without_symlinks(path)
            if found != data:
                raise RuntimeError(f"persona destination contains different contents: {path}")
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def package_personas(
    selected_rows: list[dict[str, Any]],
    input_path: Path,
    output_path: Path,
    persona_root: Path | None,
) -> tuple[list[dict[str, Any]], dict[Path, bytes]]:
    by_id: dict[str, tuple[bytes, set[Path]]] = {}
    row_personas: list[str] = []
    for row in selected_rows:
        persona_id, reference = persona_fields(row)
        source = resolve_persona_path(reference, input_path, persona_root)
        data = source.read_bytes()
        if not data.strip():
            raise RuntimeError(f"persona file is empty: {source}")
        prior = by_id.get(persona_id)
        if prior is not None and prior[0] != data:
            sources = sorted(str(path) for path in prior[1] | {source})
            raise RuntimeError(f"persona id {persona_id!r} refers to files with different contents: {', '.join(sources)}")
        if prior is None:
            by_id[persona_id] = (data, {source})
        else:
            prior[1].add(source)
        row_personas.append(persona_id)

    filenames: dict[str, str] = {}
    occupied: set[str] = set()
    for persona_id in sorted(by_id):
        _, sources = by_id[persona_id]
        source = min(sources, key=lambda path: str(path))
        suffix = source.suffix.lower() or ".txt"
        base = safe_persona_name(persona_id)
        candidate = f"{base}{suffix}"
        number = 2
        while candidate in occupied:
            candidate = f"{base}-{number}{suffix}"
            number += 1
        occupied.add(candidate)
        filenames[persona_id] = candidate

    persona_dir = output_path.parent / "personas"
    validate_real_directory_path(persona_dir, allow_missing=True)
    files: dict[Path, bytes] = {}
    for persona_id, (data, _) in by_id.items():
        destination = persona_dir / filenames[persona_id]
        try:
            found = read_regular_file_without_symlinks(destination)
        except FileNotFoundError:
            found = None
        if found is not None and found != data:
            raise RuntimeError(f"persona destination contains different contents: {destination}")
        files[destination] = data

    output_rows: list[dict[str, Any]] = []
    for row, persona_id in zip(selected_rows, row_personas, strict=True):
        output_row = copy.deepcopy(clean_row(row))
        packaged_path = (Path("personas") / filenames[persona_id]).as_posix()
        persona = output_row.get("persona")
        if isinstance(persona, dict):
            persona["path"] = packaged_path
            persona.pop("file", None)
            persona.pop("persona_file", None)
            output_row.pop("persona_file", None)
        elif isinstance(persona, str):
            output_row["persona"] = packaged_path
            output_row.pop("persona_file", None)
        else:
            output_row["persona_file"] = packaged_path
        output_rows.append(output_row)
    return output_rows, files


def main() -> int:
    parser = argparse.ArgumentParser(description="Sample rows by uniformly selected cluster-assignment tuples.")
    parser.add_argument("input", type=Path, help="Input variant-persona cluster JSONL file")
    parser.add_argument("--out", required=True, type=Path, help="Output sampled JSONL file")
    parser.add_argument("--diagnostics-out", type=Path, help="Optional JSONL diagnostics file")
    parser.add_argument("--equivalence-out", type=Path, help="Optional JSONL file of equivalent endpoint groups")
    parser.add_argument(
        "--no-dedupe-equivalent-endpoints",
        action="store_true",
        help="Keep provider endpoints distinct instead of sampling one representative per equivalent model configuration",
    )
    parser.add_argument(
        "--without-replacement",
        action="store_true",
        help="Emit each row from the sampling frame at most once. Fails if --pool-size exceeds the frame size.",
    )
    parser.add_argument("--pool-size", type=int, default=20, help="Number of rows to emit. Default: %(default)s")
    parser.add_argument("--seed", type=int, help="Optional deterministic random seed")
    parser.add_argument(
        "--persona-root",
        type=Path,
        help="Base directory for relative persona paths. Defaults include model-pool/, the input directory, and common/etc/.",
    )
    args = parser.parse_args()

    args.input = args.input.expanduser()
    args.out = args.out.expanduser()
    if args.diagnostics_out is not None:
        args.diagnostics_out = args.diagnostics_out.expanduser()
    if args.equivalence_out is not None:
        args.equivalence_out = args.equivalence_out.expanduser()
    if args.persona_root is not None:
        args.persona_root = args.persona_root.expanduser()

    if args.pool_size <= 0:
        raise RuntimeError("--pool-size must be positive")

    rows = load_jsonl(args.input)
    cluster_count = validate_rows(rows)
    representatives, equivalence_records = annotate_equivalence(rows)
    sample_rows = rows if args.no_dedupe_equivalent_endpoints else representatives
    grouped = group_by_tuple(sample_rows)
    tuples = sorted(grouped)
    if args.without_replacement and args.pool_size > len(sample_rows):
        raise RuntimeError(
            f"--pool-size={args.pool_size} exceeds sampling frame rows "
            f"({len(sample_rows)}) with --without-replacement"
        )
    available_grouped = {cluster_tuple: list(candidates) for cluster_tuple, candidates in grouped.items()}
    available_tuples = sorted(available_grouped)
    rng = random.Random(args.seed)

    tuple_counts: Counter[ClusterTuple] = Counter()
    row_counts: Counter[int] = Counter()
    model_counts: Counter[str] = Counter()
    provider_counts: Counter[str] = Counter()
    endpoint_counts: Counter[str] = Counter()

    selected_rows: list[dict[str, Any]] = []
    diagnostics: list[dict[str, Any]] = []
    console_lines: list[str] = []
    for step in range(1, args.pool_size + 1):
        if args.without_replacement:
            cluster_tuple = available_tuples[rng.randrange(len(available_tuples))]
            candidates = available_grouped[cluster_tuple]
            available_before = len(candidates)
            row = candidates.pop(rng.randrange(len(candidates)))
            if not candidates:
                available_tuples.remove(cluster_tuple)
        else:
            cluster_tuple = tuples[rng.randrange(len(tuples))]
            candidates = grouped[cluster_tuple]
            available_before = len(candidates)
            row = candidates[rng.randrange(len(candidates))]

        tuple_counts[cluster_tuple] += 1
        row_counts[row["_source_row"]] += 1
        model_id = str(row.get("openrouter_model_id", ""))
        provider_name = str(row.get("provider_name", ""))
        endpoint = endpoint_key(row)
        model_counts[model_id] += 1
        provider_counts[provider_name] += 1
        endpoint_counts[endpoint] += 1
        selected_rows.append(row)

        console_lines.append(
            f"{step}: tuple={list(cluster_tuple)} tuple_size={len(grouped[cluster_tuple])} "
            f"available_before={available_before} "
            f"tuple_count={tuple_counts[cluster_tuple]} row={row['_source_row']} "
            f"row_count={row_counts[row['_source_row']]} model={model_id} "
            f"provider={provider_name} endpoint={row.get('endpoint_tag', '')} "
            f"quantization={row.get('quantization', '')}"
        )
        diagnostics.append({
            "step": step,
            "cluster_tuple": list(cluster_tuple),
            "cluster_tuple_count": tuple_counts[cluster_tuple],
            "cluster_tuple_size": len(grouped[cluster_tuple]),
            "cluster_tuple_available_before": available_before,
            "source_row": row["_source_row"],
            "source_row_count": row_counts[row["_source_row"]],
            "openrouter_model_id": model_id,
            "provider_name": provider_name,
            "endpoint_tag": row.get("endpoint_tag"),
            "quantization": row.get("quantization"),
            "endpoint_identifier": endpoint,
            "equivalence_key": row.get("_equivalence_key"),
            "equivalence_class_size": row.get("_equivalence_class_size", 1),
            "representative_source_row": row.get("_representative_source_row", row["_source_row"]),
            "representative_endpoint_variant_id": row.get("_representative_endpoint_variant_id", row.get("endpoint_variant_id")),
            "equivalent_endpoints": row.get("_equivalent_endpoints", [endpoint_summary(row)]),
            "without_replacement": args.without_replacement,
        })

    persona_root = args.persona_root
    if persona_root is not None and not persona_root.is_absolute():
        persona_root = ROOT / persona_root
    output_rows, persona_files = package_personas(selected_rows, args.input, args.out, persona_root)
    output_paths = [path for path in (args.out, args.diagnostics_out, args.equivalence_out) if path is not None]
    if len({path.expanduser().resolve() for path in output_paths}) != len(output_paths):
        raise RuntimeError("output, diagnostics, and equivalence paths must be distinct")
    persona_destinations = {path.expanduser().resolve() for path in persona_files}
    conflict = persona_destinations & {path.expanduser().resolve() for path in output_paths}
    if conflict:
        raise RuntimeError(f"persona destination conflicts with an output path: {min(conflict, key=str)}")

    for destination, data in persona_files.items():
        publish_persona_file(destination, data)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as output_handle:
        for row in output_rows:
            output_handle.write(json.dumps(row, ensure_ascii=False) + "\n")
    if args.diagnostics_out:
        args.diagnostics_out.parent.mkdir(parents=True, exist_ok=True)
        with args.diagnostics_out.open("w") as diagnostics_handle:
            for record in diagnostics:
                diagnostics_handle.write(json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n")
    if args.equivalence_out:
        args.equivalence_out.parent.mkdir(parents=True, exist_ok=True)
        with args.equivalence_out.open("w") as equivalence_handle:
            for record in equivalence_records:
                equivalence_handle.write(json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n")
    for line in console_lines:
        print(line)

    print(
        f"summary: input_rows={len(rows)} deduped_rows={len(sample_rows)} "
        f"equivalence_classes={len(equivalence_records)} gene_count={cluster_count} "
        f"unique_tuples={len(tuples)} without_replacement={args.without_replacement} "
        f"emitted={args.pool_size} "
        f"emitted_unique_tuples={len(tuple_counts)} unique_rows={len(row_counts)} "
        f"unique_models={len(model_counts)} unique_providers={len(provider_counts)} "
        f"unique_endpoints={len(endpoint_counts)}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
