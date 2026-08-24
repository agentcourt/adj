#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import json
import random
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any


ClusterTuple = tuple[int, ...]
EquivalenceKey = str


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
    args = parser.parse_args()

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

    args.out.parent.mkdir(parents=True, exist_ok=True)
    diagnostics_handle = None
    if args.diagnostics_out:
        args.diagnostics_out.parent.mkdir(parents=True, exist_ok=True)
        diagnostics_handle = args.diagnostics_out.open("w")
    if args.equivalence_out:
        args.equivalence_out.parent.mkdir(parents=True, exist_ok=True)
        with args.equivalence_out.open("w") as equivalence_handle:
            for record in equivalence_records:
                equivalence_handle.write(json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n")

    try:
        with args.out.open("w") as output_handle:
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

                print(
                    f"{step}: tuple={list(cluster_tuple)} tuple_size={len(grouped[cluster_tuple])} "
                    f"available_before={available_before} "
                    f"tuple_count={tuple_counts[cluster_tuple]} row={row['_source_row']} "
                    f"row_count={row_counts[row['_source_row']]} model={model_id} "
                    f"provider={provider_name} endpoint={row.get('endpoint_tag', '')} "
                    f"quantization={row.get('quantization', '')}"
                )
                output_handle.write(json.dumps(clean_row(row), ensure_ascii=False) + "\n")

                if diagnostics_handle:
                    diagnostics_handle.write(json.dumps({
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
                    }, ensure_ascii=False, sort_keys=True) + "\n")
    finally:
        if diagnostics_handle:
            diagnostics_handle.close()

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
