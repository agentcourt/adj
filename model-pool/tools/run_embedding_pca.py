#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy"]
# ///
import argparse
import datetime as dt
import json
from pathlib import Path

import numpy as np

ROOT = Path(__file__).resolve().parents[1]


def utc_now() -> str:
    return dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def display_path(path: Path) -> str:
    if path.is_relative_to(ROOT):
        return str(path.relative_to(ROOT))
    return str(path)


def load_jsonl(path: Path) -> list[dict]:
    with path.open() as handle:
        return [json.loads(line) for line in handle if line.strip()]


def identity_fields(row: dict) -> dict:
    keys = [
        "run_id",
        "gene_index",
        "gene",
        "gene_sha256",
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
    return {key: row.get(key) for key in keys if key in row}


def main() -> int:
    parser = argparse.ArgumentParser(description="Reduce embedding records with PCA.")
    parser.add_argument("--records", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--dimensions", type=int, required=True)
    args = parser.parse_args()

    records_path = Path(args.records)
    if not records_path.is_absolute():
        records_path = ROOT / records_path
    out = Path(args.out)
    if not out.is_absolute():
        out = ROOT / out
    out.mkdir(parents=True, exist_ok=True)

    rows = load_jsonl(records_path)
    included = []
    for row in rows:
        if row.get("status") != "ok":
            continue
        embedding = row.get("embedding")
        if not isinstance(embedding, list) or not embedding:
            raise RuntimeError(f"record missing embedding: combined_index={row.get('combined_index')} sample={row.get('sample_index')}")
        included.append(row)

    if not included:
        raise RuntimeError("no ok records with embeddings found")
    dimensions = args.dimensions
    if dimensions < 1:
        raise RuntimeError("--dimensions must be positive")
    if dimensions > len(included):
        raise RuntimeError("--dimensions cannot exceed included record count")

    matrix = np.array([row["embedding"] for row in included], dtype=np.float64)
    if matrix.ndim != 2:
        raise RuntimeError("embedding matrix is not two-dimensional")
    if dimensions > matrix.shape[1]:
        raise RuntimeError("--dimensions cannot exceed embedding dimension")

    mean = matrix.mean(axis=0)
    centered = matrix - mean
    _, singular_values, vt = np.linalg.svd(centered, full_matrices=False)
    components = vt[:dimensions]
    projected = centered @ components.T
    denominator = max(matrix.shape[0] - 1, 1)
    explained_variance_all = (singular_values ** 2) / denominator
    total_variance = float(explained_variance_all.sum())
    explained_variance = explained_variance_all[:dimensions]
    if total_variance > 0:
        explained_ratio = explained_variance / total_variance
    else:
        explained_ratio = np.zeros_like(explained_variance)

    projected_path = out / "pca-records.jsonl"
    with projected_path.open("w") as handle:
        for row, vector in zip(included, projected):
            handle.write(json.dumps({**identity_fields(row), "pca": vector.tolist()}, ensure_ascii=False) + "\n")

    fit = {
        "created_at": utc_now(),
        "records_path": display_path(records_path),
        "included_records": len(included),
        "embedding_dimension": int(matrix.shape[1]),
        "pca_dimensions": dimensions,
        "centered": True,
        "mean": mean.tolist(),
        "components": components.tolist(),
        "explained_variance": explained_variance.tolist(),
        "explained_variance_ratio": explained_ratio.tolist(),
        "singular_values": singular_values[:dimensions].tolist(),
    }
    (out / "pca-fit.json").write_text(json.dumps(fit, indent=2) + "\n")

    summary = {
        "created_at": fit["created_at"],
        "records_path": fit["records_path"],
        "projected_records_path": display_path(projected_path),
        "included_records": len(included),
        "source_records": len(rows),
        "embedding_dimension": int(matrix.shape[1]),
        "pca_dimensions": dimensions,
        "explained_variance": explained_variance.tolist(),
        "explained_variance_ratio": explained_ratio.tolist(),
        "explained_variance_ratio_sum": float(explained_ratio.sum()),
    }
    (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
