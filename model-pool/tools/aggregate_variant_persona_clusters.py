#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import datetime as dt
import json
import math
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def resolve_path(path_text: str) -> Path:
    path = Path(path_text)
    if path.is_absolute():
        return path
    return ROOT / path


def display_path(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def load_jsonl(path: Path) -> list[dict[str, Any]]:
    with path.open() as handle:
        return [json.loads(line) for line in handle if line.strip()]


def load_fit(path: Path) -> dict[str, Any]:
    payload = json.loads(path.read_text())
    genes = payload.get("genes")
    if not isinstance(genes, dict):
        raise RuntimeError(f"{path}: expected genes object")
    return genes


def distance(a: list[float], b: list[float]) -> float:
    return math.sqrt(sum((float(x) - float(y)) ** 2 for x, y in zip(a, b, strict=True)))


def choose_cluster(rows: list[dict[str, Any]], fit: dict[str, Any]) -> dict[str, Any]:
    counts = Counter(int(row["cluster"]) for row in rows)
    most_common = counts.most_common()
    best_count = most_common[0][1]
    winners = sorted(cluster for cluster, count in counts.items() if count == best_count)
    if len(winners) == 1:
        return {
            "cluster": winners[0],
            "method": "unanimous" if best_count == len(rows) else "majority",
            "sample_clusters": [int(row["cluster"]) for row in sorted(rows, key=lambda item: item["sample_index"])],
            "cluster_counts": {str(key): counts[key] for key in sorted(counts)},
            "tie_break_sample_index": None,
        }

    centers = fit.get("centers")
    if not isinstance(centers, list):
        raise RuntimeError("cluster fit is missing centers for tie break")
    candidates = []
    for row in rows:
        cluster = int(row["cluster"])
        if cluster not in winners:
            continue
        center = centers[cluster]
        candidates.append((
            distance(row["pca"], center),
            int(row["sample_index"]),
            cluster,
        ))
    candidates.sort()
    _, sample_index, cluster = candidates[0]
    return {
        "cluster": cluster,
        "method": "center_distance_tie_break",
        "sample_clusters": [int(row["cluster"]) for row in sorted(rows, key=lambda item: item["sample_index"])],
        "cluster_counts": {str(key): counts[key] for key in sorted(counts)},
        "tie_break_sample_index": sample_index,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Aggregate sample-level clusters into variant/persona cluster vectors.")
    parser.add_argument("--clusters", required=True)
    parser.add_argument("--cluster-fit", required=True)
    parser.add_argument("--variants", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--expected-samples-per-gene", type=int)
    parser.add_argument("--allow-missing-gene-samples", action="store_true")
    args = parser.parse_args()

    if args.expected_samples_per_gene is not None and args.expected_samples_per_gene < 1:
        raise RuntimeError("--expected-samples-per-gene must be positive")

    clusters_path = resolve_path(args.clusters)
    fit_path = resolve_path(args.cluster_fit)
    variants_path = resolve_path(args.variants)
    out = resolve_path(args.out)
    out.mkdir(parents=True, exist_ok=True)

    cluster_rows = load_jsonl(clusters_path)
    variants = {
        row["endpoint_variant_id"]: row
        for row in load_jsonl(variants_path)
    }
    fits = load_fit(fit_path)

    grouped: dict[tuple[str, str], dict[int, list[dict[str, Any]]]] = defaultdict(lambda: defaultdict(list))
    for row in cluster_rows:
        grouped[(row["endpoint_variant_id"], row["persona_id"])][int(row["gene_index"])].append(row)

    output_rows: list[dict[str, Any]] = []
    method_counts: Counter[str] = Counter()
    skipped_variants: list[dict[str, Any]] = []
    expected_genes = sorted({int(row["gene_index"]) for row in cluster_rows})
    for endpoint_variant_id, persona_id in sorted(grouped):
        by_gene = grouped[(endpoint_variant_id, persona_id)]
        present_genes = sorted(by_gene)
        if present_genes != expected_genes:
            missing_genes = [gene_index for gene_index in expected_genes if gene_index not in by_gene]
            if args.allow_missing_gene_samples:
                skipped_variants.append(
                    {
                        "endpoint_variant_id": endpoint_variant_id,
                        "persona_id": persona_id,
                        "missing_gene_indexes": missing_genes,
                        "present_gene_indexes": present_genes,
                        "reason": "missing_gene_samples",
                    }
                )
                continue
            raise RuntimeError(f"{endpoint_variant_id},{persona_id}: missing gene indexes")
        variant = variants.get(endpoint_variant_id)
        if variant is None:
            raise RuntimeError(f"{endpoint_variant_id}: missing variant record")
        exemplar = by_gene[expected_genes[0]][0]
        clusters: list[int] = []
        cluster_details: list[dict[str, Any]] = []
        for gene_index in expected_genes:
            rows = sorted(by_gene[gene_index], key=lambda item: item["sample_index"])
            if args.expected_samples_per_gene is not None and len(rows) != args.expected_samples_per_gene:
                raise RuntimeError(
                    f"{endpoint_variant_id},{persona_id},gene {gene_index}: "
                    f"expected {args.expected_samples_per_gene} sample rows, found {len(rows)}"
                )
            fit = fits.get(str(gene_index))
            if fit is None:
                raise RuntimeError(f"gene {gene_index}: missing fit")
            chosen = choose_cluster(rows, fit)
            method_counts[chosen["method"]] += 1
            clusters.append(chosen["cluster"])
            cluster_details.append({
                "gene_index": gene_index,
                "gene": rows[0]["gene"],
                **chosen,
            })
        output_rows.append({
            "endpoint_variant_id": endpoint_variant_id,
            "combined_index": variant.get("combined_index", exemplar.get("combined_index")),
            "openrouter_model_id": variant.get("openrouter_model_id", exemplar.get("openrouter_model_id")),
            "provider_name": variant.get("provider_name", exemplar.get("provider_name")),
            "endpoint_tag": variant.get("endpoint_tag", exemplar.get("endpoint_tag")),
            "quantization": variant.get("quantization", exemplar.get("quantization")),
            "persona": {
                "id": persona_id,
                "path": exemplar.get("persona_path"),
            },
            "clusters": clusters,
            "cluster_details": cluster_details,
            "variant": variant,
        })
    if not output_rows:
        raise RuntimeError("no variant/persona rows produced")

    jsonl_path = out / "variant-persona-clusters.jsonl"
    with jsonl_path.open("w") as handle:
        for row in output_rows:
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")

    json_path = out / "variant-persona-clusters.json"
    json_path.write_text(json.dumps(output_rows, indent=2, ensure_ascii=False) + "\n")

    summary = {
        "created_at": utc_now(),
        "clusters_path": display_path(clusters_path),
        "cluster_fit_path": display_path(fit_path),
        "variants_path": display_path(variants_path),
        "jsonl_path": display_path(jsonl_path),
        "json_path": display_path(json_path),
        "variant_persona_rows": len(output_rows),
        "gene_indexes": expected_genes,
        "clusters_per_row": len(expected_genes),
        "input_cluster_rows": len(cluster_rows),
        "skipped_variant_count": len(skipped_variants),
        "skipped_variants": skipped_variants,
        "aggregation_method_counts": {key: method_counts[key] for key in sorted(method_counts)},
        "expected_samples_per_gene": args.expected_samples_per_gene,
        "allow_missing_gene_samples": args.allow_missing_gene_samples,
    }
    (out / "summary.json").write_text(json.dumps(summary, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps(summary, sort_keys=True, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
