#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# ///

from __future__ import annotations

import csv
from pathlib import Path
import sys


def read_rows(path: Path) -> list[tuple[str, str, str, str]]:
    rows: list[tuple[str, str, str, str]] = []
    with path.open(newline="", encoding="utf-8") as handle:
        reader = csv.reader(handle, delimiter="\t")
        for raw in reader:
            if not raw:
                continue
            if len(raw) > 4:
                raise SystemExit(f"{path}: expected at most 4 columns, got {len(raw)}")
            while len(raw) < 4:
                raw.append("")
            theorem, filename, importance, comment = raw
            rows.append((theorem, filename, importance, comment))
    return sorted(rows, key=lambda row: row[0])


def write_tsv(path: Path, rows: list[tuple[str, str, str, str]]) -> None:
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle, delimiter="\t", lineterminator="\n")
        writer.writerows(rows)


def escape_cell(text: str) -> str:
    return text.replace("|", r"\|")


def write_md(path: Path, rows: list[tuple[str, str, str, str]]) -> None:
    with path.open("w", encoding="utf-8") as handle:
        handle.write("# Theorems\n\n")
        handle.write("| Theorem | File | Importance | Comment |\n")
        handle.write("|---|---|---|---|\n")
        for theorem, filename, importance, comment in rows:
            handle.write(
                f"| `{escape_cell(theorem)}` | `{escape_cell(filename)}` | "
                f"{escape_cell(importance)} | {escape_cell(comment)} |\n"
            )


def main(argv: list[str]) -> int:
    project_root = Path.cwd().resolve()
    tsv_path = Path(argv[1]) if len(argv) > 1 else project_root / "theorems.tsv"
    md_path = Path(argv[2]) if len(argv) > 2 else project_root / "docs" / "theorems.md"
    rows = read_rows(tsv_path)
    write_tsv(tsv_path, rows)
    write_md(md_path, rows)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
