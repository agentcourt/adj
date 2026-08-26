#!/usr/bin/env -S uv run --no-cache --script
# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "scikit-learn"]
# ///
import argparse
import csv
import datetime as dt
import json
import math
from collections import Counter
from pathlib import Path
from typing import Any

import numpy as np
from sklearn.cluster import KMeans
from sklearn.metrics import silhouette_score

ROOT = Path(__file__).resolve().parents[1]


IDENTITY_KEYS = [
    "gene_index",
    "gene",
    "persona_id",
    "persona_path",
    "variant_order",
    "combined_index",
    "endpoint_variant_id",
    "openrouter_model_id",
    "provider_name",
    "endpoint_tag",
    "quantization",
    "sample_index",
]

CSV_IDENTITY_FIELDS = [
    "gene_index",
    "gene",
    "combined_index",
    "endpoint_variant_id",
    "openrouter_model_id",
    "provider_name",
    "endpoint_tag",
    "quantization",
    "persona_id",
    "sample_index",
]


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


def row_identity(row: dict[str, Any]) -> dict[str, Any]:
    return {key: row.get(key) for key in IDENTITY_KEYS if key in row}


def variant_key(row: dict[str, Any]) -> Any:
    for key in ("endpoint_variant_id", "combined_index", "variant_order"):
        value = row.get(key)
        if value not in (None, ""):
            return value
    return None


def validate_gene_rows(
    path: Path,
    rows: list[dict[str, Any]],
    expected_rows: int | None,
    expected_variants: int | None,
    expected_samples: int | None,
    pca_dimensions: int,
) -> dict[str, Any]:
    if not rows:
        raise RuntimeError(f"{path}: no rows found")
    if expected_rows is not None and len(rows) != expected_rows:
        raise RuntimeError(f"{path}: expected {expected_rows} rows, found {len(rows)}")
    gene_indexes = {row.get("gene_index") for row in rows}
    if len(gene_indexes) != 1:
        raise RuntimeError(f"{path}: expected one gene_index, found {sorted(gene_indexes)}")
    genes = {row.get("gene") for row in rows}
    if len(genes) != 1:
        raise RuntimeError(f"{path}: expected one gene text, found {len(genes)}")

    variant_values = [variant_key(row) for row in rows]
    if any(value is None for value in variant_values):
        raise RuntimeError(f"{path}: every row must have endpoint_variant_id, combined_index, or variant_order")
    variant_counts = Counter(variant_values)
    if expected_variants is not None and len(variant_counts) != expected_variants:
        raise RuntimeError(f"{path}: expected {expected_variants} unique endpoint variants, found {len(variant_counts)}")
    if expected_samples is not None and any(count != expected_samples for count in variant_counts.values()):
        bad = {str(key): count for key, count in variant_counts.items() if count != expected_samples}
        raise RuntimeError(f"{path}: expected {expected_samples} rows per endpoint variant, bad counts {bad}")

    for index, row in enumerate(rows, start=1):
        pca = row.get("pca")
        if not isinstance(pca, list) or len(pca) != pca_dimensions:
            raise RuntimeError(f"{path}:{index}: expected pca vector length {pca_dimensions}")
        try:
            [float(value) for value in pca]
        except (TypeError, ValueError) as exc:
            raise RuntimeError(f"{path}:{index}: pca vector contains a non-number") from exc

    return {
        "gene_index": next(iter(gene_indexes)),
        "gene": next(iter(genes)),
        "row_count": len(rows),
        "unique_endpoint_variants": len(variant_counts),
        "samples_per_endpoint_variant": sorted(set(variant_counts.values())),
        "expected_rows_per_gene": expected_rows,
        "expected_variants_per_gene": expected_variants,
        "expected_samples_per_variant": expected_samples,
    }


def cluster_gene(matrix: np.ndarray, min_k: int, max_k: int) -> tuple[np.ndarray, dict[str, Any]]:
    rows = matrix.shape[0]
    if rows < min_k + 1:
        return np.zeros(rows, dtype=np.int64), {
            "status": "fallback",
            "reason": "not_enough_rows",
            "chosen_k": 1,
            "candidate_scores": [],
            "centers": [[float(value) for value in matrix.mean(axis=0)]],
        }

    candidate_scores: list[dict[str, Any]] = []
    best_score: float | None = None
    best_labels: np.ndarray | None = None
    best_centers: np.ndarray | None = None
    best_k: int | None = None

    for k in range(min_k, min(max_k, rows - 1) + 1):
        model = KMeans(n_clusters=k, n_init=10, random_state=0)
        labels = model.fit_predict(matrix)
        unique_labels = sorted({int(value) for value in labels})
        if len(unique_labels) < 2:
            candidate_scores.append({
                "k": k,
                "status": "skipped",
                "reason": "single_label",
                "unique_labels": unique_labels,
            })
            continue
        score = float(silhouette_score(matrix, labels))
        if not math.isfinite(score):
            raise ValueError(f"silhouette score is not finite for k={k}: {score}")
        candidate_scores.append({
            "k": k,
            "status": "scored",
            "silhouette_score": score,
            "unique_labels": unique_labels,
        })
        if best_score is None or score > best_score:
            best_score = score
            best_labels = labels.astype(np.int64, copy=False)
            best_centers = model.cluster_centers_
            best_k = k

    if best_labels is None or best_centers is None or best_k is None:
        return np.zeros(rows, dtype=np.int64), {
            "status": "fallback",
            "reason": "no_valid_silhouette_score",
            "chosen_k": 1,
            "candidate_scores": candidate_scores,
            "centers": [[float(value) for value in matrix.mean(axis=0)]],
        }

    return best_labels, {
        "status": "clustered",
        "chosen_k": best_k,
        "chosen_silhouette_score": best_score,
        "candidate_scores": candidate_scores,
        "centers": best_centers.tolist(),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Cluster per-gene PCA rows.")
    parser.add_argument("--pca-records", action="append", required=True, help="Per-gene pca-records.jsonl path. Repeat once per gene.")
    parser.add_argument("--out", required=True)
    parser.add_argument("--expected-rows-per-gene", type=int)
    parser.add_argument("--expected-variants-per-gene", type=int)
    parser.add_argument("--expected-samples-per-variant", type=int)
    parser.add_argument("--pca-dimensions", type=int, default=3)
    parser.add_argument("--min-k", type=int, default=3)
    parser.add_argument("--max-k", type=int, default=10)
    args = parser.parse_args()

    if args.pca_dimensions < 1:
        raise RuntimeError("--pca-dimensions must be positive")
    if args.expected_rows_per_gene is not None and args.expected_rows_per_gene < 1:
        raise RuntimeError("--expected-rows-per-gene must be positive")
    if args.expected_variants_per_gene is not None and args.expected_variants_per_gene < 1:
        raise RuntimeError("--expected-variants-per-gene must be positive")
    if args.expected_samples_per_variant is not None and args.expected_samples_per_variant < 1:
        raise RuntimeError("--expected-samples-per-variant must be positive")
    if args.min_k < 2:
        raise RuntimeError("--min-k must be at least 2")
    if args.max_k < args.min_k:
        raise RuntimeError("--max-k must be greater than or equal to --min-k")

    out = resolve_path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    created_at = utc_now()

    all_clustered_rows: list[dict[str, Any]] = []
    gene_summaries: list[dict[str, Any]] = []
    fits: dict[str, Any] = {}
    seen_gene_indexes: set[Any] = set()

    for path_text in args.pca_records:
        path = resolve_path(path_text)
        rows = load_jsonl(path)
        validation = validate_gene_rows(
            path,
            rows,
            args.expected_rows_per_gene,
            args.expected_variants_per_gene,
            args.expected_samples_per_variant,
            args.pca_dimensions,
        )
        gene_index = validation["gene_index"]
        if gene_index in seen_gene_indexes:
            raise RuntimeError(f"duplicate gene_index in inputs: {gene_index}")
        seen_gene_indexes.add(gene_index)

        matrix = np.array([row["pca"] for row in rows], dtype=np.float64)
        labels, fit = cluster_gene(matrix, args.min_k, args.max_k)
        cluster_counts = Counter(int(value) for value in labels)
        for row, label in zip(rows, labels, strict=True):
            all_clustered_rows.append({
                **row_identity(row),
                "source_pca_path": display_path(path),
                "pca": [float(value) for value in row["pca"]],
                "cluster": int(label),
            })

        key = str(gene_index)
        gene_summaries.append({
            **validation,
            "source_pca_path": display_path(path),
            "status": fit["status"],
            "chosen_k": fit["chosen_k"],
            "chosen_silhouette_score": fit.get("chosen_silhouette_score"),
            "cluster_counts": {str(key): cluster_counts[key] for key in sorted(cluster_counts)},
            "candidate_scores": fit["candidate_scores"],
        })
        fits[key] = {
            "gene_index": gene_index,
            "gene": validation["gene"],
            "source_pca_path": display_path(path),
            **fit,
        }

    clusters_path = out / "clusters.jsonl"
    with clusters_path.open("w") as handle:
        for row in all_clustered_rows:
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")

    csv_path = out / "clusters.csv"
    csv_fields = [*CSV_IDENTITY_FIELDS, *[f"pc{index}" for index in range(1, args.pca_dimensions + 1)], "cluster"]
    with csv_path.open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=csv_fields)
        writer.writeheader()
        for row in all_clustered_rows:
            pca = row["pca"]
            csv_row = {
                "gene_index": row.get("gene_index"),
                "gene": row.get("gene"),
                "combined_index": row.get("combined_index"),
                "endpoint_variant_id": row.get("endpoint_variant_id"),
                "openrouter_model_id": row.get("openrouter_model_id"),
                "provider_name": row.get("provider_name"),
                "endpoint_tag": row.get("endpoint_tag"),
                "quantization": row.get("quantization"),
                "persona_id": row.get("persona_id"),
                "sample_index": row.get("sample_index"),
                "cluster": row.get("cluster"),
            }
            for index, value in enumerate(pca, start=1):
                csv_row[f"pc{index}"] = value
            writer.writerow(csv_row)

    fit_path = out / "cluster-fit.json"
    fit_path.write_text(json.dumps({
        "created_at": created_at,
        "pca_dimensions": args.pca_dimensions,
        "min_k": args.min_k,
        "max_k": args.max_k,
        "genes": fits,
    }, indent=2, ensure_ascii=False) + "\n")

    summary = {
        "created_at": created_at,
        "output_dir": display_path(out),
        "clusters_jsonl": display_path(clusters_path),
        "clusters_csv": display_path(csv_path),
        "cluster_fit": display_path(fit_path),
        "gene_count": len(gene_summaries),
        "rows_written": len(all_clustered_rows),
        "expected_rows_per_gene": args.expected_rows_per_gene,
        "expected_variants_per_gene": args.expected_variants_per_gene,
        "expected_samples_per_variant": args.expected_samples_per_variant,
        "pca_dimensions": args.pca_dimensions,
        "min_k": args.min_k,
        "max_k": args.max_k,
        "genes": sorted(gene_summaries, key=lambda item: item["gene_index"]),
    }
    (out / "summary.json").write_text(json.dumps(summary, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps(summary, sort_keys=True, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
