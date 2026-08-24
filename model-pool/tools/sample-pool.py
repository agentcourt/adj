#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import json
import random
from pathlib import Path
from typing import Any


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


def sample_one(rows: list[dict[str, Any]], gene_count: int, rng: random.Random) -> tuple[dict[str, Any], str]:
    survivors = rows
    unvisited_genes = list(range(gene_count))
    trace: list[str] = []

    while unvisited_genes:
        gene_position = rng.randrange(len(unvisited_genes))
        gene_index = unvisited_genes.pop(gene_position)
        present_clusters = sorted({row["clusters"][gene_index] for row in survivors})
        if not present_clusters:
            raise RuntimeError(f"gene {gene_index}: no clusters among survivors")
        cluster = present_clusters[rng.randrange(len(present_clusters))]
        survivors = [row for row in survivors if row["clusters"][gene_index] == cluster]
        if not survivors:
            raise RuntimeError(f"gene {gene_index}, cluster {cluster}: no surviving rows")
        trace.append(f"{cluster}: {len(survivors)}")

    return survivors[rng.randrange(len(survivors))], ", ".join(trace)


def main() -> int:
    parser = argparse.ArgumentParser(description="Sample a clustered variant/persona pool.")
    parser.add_argument("input", type=Path, help="Input variant-persona cluster JSONL file")
    parser.add_argument("--out", required=True, type=Path, help="Output sampled JSONL file")
    parser.add_argument("--pool-size", type=int, default=20, help="Number of rows to emit. Default: %(default)s")
    parser.add_argument("--seed", type=int, help="Optional deterministic random seed")
    args = parser.parse_args()

    if args.pool_size <= 0:
        raise RuntimeError("--pool-size must be positive")

    rows = load_jsonl(args.input)
    gene_count = validate_rows(rows)
    rng = random.Random(args.seed)

    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w") as handle:
        for _ in range(args.pool_size):
            row, trace = sample_one(rows, gene_count, rng)
            print(trace)
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
