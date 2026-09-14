#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# ///

from __future__ import annotations

import argparse
import csv
from pathlib import Path
import re
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


def remove_lean_comments(source: str) -> str:
    output: list[str] = []
    index = 0
    block_depth = 0
    in_string = False
    while index < len(source):
        if block_depth > 0:
            if source.startswith("/-", index):
                block_depth += 1
                output.extend("  ")
                index += 2
            elif source.startswith("-/", index):
                block_depth -= 1
                output.extend("  ")
                index += 2
            else:
                output.append("\n" if source[index] == "\n" else " ")
                index += 1
            continue

        if not in_string and source.startswith("--", index):
            line_end = source.find("\n", index)
            if line_end == -1:
                output.extend(" " * (len(source) - index))
                break
            output.extend(" " * (line_end - index))
            index = line_end
            continue
        if not in_string and source.startswith("/-", index):
            block_depth = 1
            output.extend("  ")
            index += 2
            continue

        char = source[index]
        output.append(char)
        if char == '"':
            backslashes = 0
            previous = index - 1
            while previous >= 0 and source[previous] == "\\":
                backslashes += 1
                previous -= 1
            if backslashes % 2 == 0:
                in_string = not in_string
        index += 1

    if block_depth != 0:
        raise ValueError("unterminated Lean block comment")
    return "".join(output)


def sync_proof_rows(
    proof_dir: Path,
    old_rows: list[tuple[str, str, str, str]],
) -> list[tuple[str, str, str, str]]:
    metadata: dict[tuple[str, str], tuple[str, str]] = {}
    for theorem, filename, importance, comment in old_rows:
        metadata[(filename, theorem.rsplit(".", 1)[-1])] = (importance, comment)

    paths = sorted(proof_dir.glob("*.lean"))
    if not paths:
        raise SystemExit(f"{proof_dir}: no Lean proof files found")

    rows: list[tuple[str, str, str, str]] = []
    seen: set[tuple[str, str]] = set()
    for path in paths:
        filename = f"Proofs/{path.name}"
        try:
            source = remove_lean_comments(path.read_text(encoding="utf-8"))
        except ValueError as error:
            raise SystemExit(f"{path}: {error}") from error
        namespace_match = re.search(r"(?m)^namespace[ \t]+([^ \t\r\n]+)", source)
        if namespace_match is None:
            raise SystemExit(f"{path}: missing namespace")
        namespace = namespace_match.group(1)
        for declaration_match in re.finditer(
            r"(?m)^[ \t]*(?:theorem|lemma)[ \t]+([A-Za-z_][A-Za-z0-9_'.]*)",
            source,
        ):
            name = declaration_match.group(1)
            key = (filename, name)
            if key in seen:
                raise SystemExit(f"{path}: duplicate declaration {name}")
            seen.add(key)
            importance, comment = metadata.get(key, ("", ""))
            rows.append((f"{namespace}.{name}", filename, importance, comment))
    return sorted(rows, key=lambda row: row[0])


def write_tsv(path: Path, rows: list[tuple[str, str, str, str]]) -> None:
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle, delimiter="\t", lineterminator="\n")
        for row in rows:
            fields = list(row)
            while fields and fields[-1] == "":
                fields.pop()
            writer.writerow(fields)


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
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--sync-proofs",
        type=Path,
        help="replace the catalog rows with public declarations from this directory",
    )
    parser.add_argument("tsv", nargs="?", type=Path)
    parser.add_argument("markdown", nargs="?", type=Path)
    args = parser.parse_args(argv[1:])

    project_root = Path.cwd().resolve()
    tsv_path = args.tsv or project_root / "theorems.tsv"
    md_path = args.markdown or project_root / "docs" / "theorems.md"
    rows = read_rows(tsv_path)
    if args.sync_proofs is not None:
        rows = sync_proof_rows(args.sync_proofs, rows)
    write_tsv(tsv_path, rows)
    write_md(md_path, rows)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
