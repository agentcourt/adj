#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import json
import random
from collections import Counter
from itertools import combinations
from pathlib import Path
from typing import Any


Feature = tuple[Any, ...]


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


def single_features(row: dict[str, Any]) -> list[Feature]:
    return [("gene_cluster", gene_index, cluster) for gene_index, cluster in enumerate(row["clusters"])]


def pair_features(row: dict[str, Any]) -> list[Feature]:
    return [
        ("gene_cluster_pair", left_index, left_cluster, right_index, right_cluster)
        for (left_index, left_cluster), (right_index, right_cluster) in combinations(enumerate(row["clusters"]), 2)
    ]


def row_key(row: dict[str, Any]) -> str:
    return json.dumps({key: value for key, value in row.items() if key != "_source_row"}, sort_keys=True, ensure_ascii=False)


def endpoint_key(row: dict[str, Any]) -> str:
    endpoint_variant_id = row.get("endpoint_variant_id")
    if endpoint_variant_id:
        return str(endpoint_variant_id)
    return "|".join(
        str(row.get(key, ""))
        for key in ("openrouter_model_id", "provider_name", "endpoint_tag", "quantization")
    )


def score_row(
    row: dict[str, Any],
    single_counts: Counter[Feature],
    pair_counts: Counter[Feature],
    model_counts: Counter[str],
    provider_counts: Counter[str],
    endpoint_counts: Counter[str],
    exact_counts: Counter[str],
    pair_weight: float,
    model_penalty: float,
    provider_penalty: float,
    endpoint_penalty: float,
    duplicate_penalty: float,
) -> dict[str, Any]:
    singles = single_features(row)
    pairs = pair_features(row)
    single_score = sum(1.0 / (1.0 + single_counts[feature]) for feature in singles)
    pair_score = pair_weight * sum(1.0 / (1.0 + pair_counts[feature]) for feature in pairs)
    model_id = str(row.get("openrouter_model_id", ""))
    provider = str(row.get("provider_name", ""))
    endpoint = endpoint_key(row)
    exact = row_key(row)
    penalty = (
        model_penalty * model_counts[model_id]
        + provider_penalty * provider_counts[provider]
        + endpoint_penalty * endpoint_counts[endpoint]
        + duplicate_penalty * exact_counts[exact]
    )
    return {
        "score": single_score + pair_score - penalty,
        "single_score": single_score,
        "pair_score": pair_score,
        "penalty": penalty,
        "new_single_features": sum(1 for feature in singles if single_counts[feature] == 0),
        "new_pair_features": sum(1 for feature in pairs if pair_counts[feature] == 0),
        "model_id": model_id,
        "provider": provider,
        "endpoint": endpoint,
        "exact": exact,
    }


def clean_row(row: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in row.items() if key != "_source_row"}


def format_log(step: int, row: dict[str, Any], metrics: dict[str, Any], uncovered_single: int) -> str:
    return (
        f"{step}: row={row['_source_row']} score={metrics['score']:.6f} "
        f"single={metrics['single_score']:.6f} pair={metrics['pair_score']:.6f} "
        f"penalty={metrics['penalty']:.6f} new_single={metrics['new_single_features']} "
        f"new_pair={metrics['new_pair_features']} uncovered_single={uncovered_single} "
        f"model={metrics['model_id']} provider={metrics['provider']} "
        f"endpoint={row.get('endpoint_tag', '')} quantization={row.get('quantization', '')} "
        f"clusters={row['clusters']}"
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Greedily sample a cluster-diverse variant/persona pool.")
    parser.add_argument("input", type=Path, help="Input variant-persona cluster JSONL file")
    parser.add_argument("--out", required=True, type=Path, help="Output sampled JSONL file")
    parser.add_argument("--diagnostics-out", type=Path, help="Optional JSONL diagnostics file")
    parser.add_argument("--pool-size", type=int, default=20, help="Number of rows to emit. Default: %(default)s")
    parser.add_argument("--seed", type=int, help="Optional deterministic random seed")
    parser.add_argument("--pair-weight", type=float, default=0.0, help="Weight for pairwise cluster coverage. Default: %(default)s")
    parser.add_argument("--model-penalty", type=float, default=0.35, help="Penalty per prior selected model id")
    parser.add_argument("--provider-penalty", type=float, default=0.15, help="Penalty per prior selected provider")
    parser.add_argument("--endpoint-penalty", type=float, default=0.50, help="Penalty per prior selected endpoint variant")
    parser.add_argument("--duplicate-penalty", type=float, default=3.0, help="Penalty per exact duplicate row")
    parser.add_argument("--top-k-random", type=int, default=1, help="Randomly choose among the top K scored rows. Default: %(default)s")
    args = parser.parse_args()

    if args.pool_size <= 0:
        raise RuntimeError("--pool-size must be positive")
    if args.top_k_random <= 0:
        raise RuntimeError("--top-k-random must be positive")
    for name in ("pair_weight", "model_penalty", "provider_penalty", "endpoint_penalty", "duplicate_penalty"):
        if getattr(args, name) < 0:
            raise RuntimeError(f"--{name.replace('_', '-')} must be non-negative")

    rows = load_jsonl(args.input)
    validate_rows(rows)
    all_single_features = {feature for row in rows for feature in single_features(row)}
    rng = random.Random(args.seed)

    single_counts: Counter[Feature] = Counter()
    pair_counts: Counter[Feature] = Counter()
    model_counts: Counter[str] = Counter()
    provider_counts: Counter[str] = Counter()
    endpoint_counts: Counter[str] = Counter()
    exact_counts: Counter[str] = Counter()
    selected_source_rows: set[int] = set()

    args.out.parent.mkdir(parents=True, exist_ok=True)
    diagnostics_handle = None
    if args.diagnostics_out:
        args.diagnostics_out.parent.mkdir(parents=True, exist_ok=True)
        diagnostics_handle = args.diagnostics_out.open("w")

    try:
        with args.out.open("w") as output_handle:
            for step in range(1, args.pool_size + 1):
                unused_rows = [row for row in rows if row["_source_row"] not in selected_source_rows]
                candidates = unused_rows if unused_rows else rows
                scored = [
                    (score_row(
                        row,
                        single_counts,
                        pair_counts,
                        model_counts,
                        provider_counts,
                        endpoint_counts,
                        exact_counts,
                        args.pair_weight,
                        args.model_penalty,
                        args.provider_penalty,
                        args.endpoint_penalty,
                        args.duplicate_penalty,
                    ), row)
                    for row in candidates
                ]
                scored.sort(key=lambda item: (-item[0]["score"], item[1]["_source_row"]))
                choice_window = scored[: min(args.top_k_random, len(scored))]
                metrics, row = rng.choice(choice_window)

                for feature in single_features(row):
                    single_counts[feature] += 1
                for feature in pair_features(row):
                    pair_counts[feature] += 1
                model_counts[metrics["model_id"]] += 1
                provider_counts[metrics["provider"]] += 1
                endpoint_counts[metrics["endpoint"]] += 1
                exact_counts[metrics["exact"]] += 1
                selected_source_rows.add(row["_source_row"])

                uncovered_single = len([feature for feature in all_single_features if single_counts[feature] == 0])
                print(format_log(step, row, metrics, uncovered_single))
                output_handle.write(json.dumps(clean_row(row), ensure_ascii=False) + "\n")

                if diagnostics_handle:
                    diagnostics_handle.write(json.dumps({
                        "step": step,
                        "source_row": row["_source_row"],
                        "score": metrics["score"],
                        "single_score": metrics["single_score"],
                        "pair_score": metrics["pair_score"],
                        "penalty": metrics["penalty"],
                        "new_single_features": metrics["new_single_features"],
                        "new_pair_features": metrics["new_pair_features"],
                        "uncovered_single_features": uncovered_single,
                        "openrouter_model_id": metrics["model_id"],
                        "provider_name": metrics["provider"],
                        "endpoint_tag": row.get("endpoint_tag"),
                        "quantization": row.get("quantization"),
                        "endpoint_identifier": metrics["endpoint"],
                        "clusters": row["clusters"],
                        "candidate_count": len(candidates),
                        "top_k_random": min(args.top_k_random, len(scored)),
                    }, ensure_ascii=False, sort_keys=True) + "\n")
    finally:
        if diagnostics_handle:
            diagnostics_handle.close()

    print(
        f"summary: emitted={args.pool_size} unique_rows={len(selected_source_rows)} "
        f"covered_single={len(all_single_features) - sum(1 for feature in all_single_features if single_counts[feature] == 0)}/{len(all_single_features)} "
        f"unique_models={len(model_counts)} unique_providers={len(provider_counts)} unique_endpoints={len(endpoint_counts)}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
