#!/usr/bin/env -S uv run --no-cache --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

class ToolError(RuntimeError):
    pass

def _record_dir(record_dir: str) -> Path:
    p = Path(record_dir)
    if not p.is_absolute():
        p = ROOT / p
    p = p.resolve()
    if not p.is_dir():
        raise ToolError(f"record_dir not found: {record_dir}")
    return p

def load_manifest(record_dir: str) -> dict:
    p = _record_dir(record_dir) / "manifest.json"
    return json.loads(p.read_text())

def list_evidence(record_dir: str) -> list[dict]:
    manifest = load_manifest(record_dir)
    return [{"id": e["id"], "title": e.get("title", ""), "file": e["file"]} for e in manifest["evidence"]]

def read_evidence(record_dir: str, evidence_id: str) -> dict:
    d = _record_dir(record_dir)
    manifest = load_manifest(record_dir)
    by_id = {e["id"]: e for e in manifest["evidence"]}
    if evidence_id not in by_id:
        raise ToolError(f"unknown evidence id: {evidence_id}")
    e = by_id[evidence_id]
    path = (d / e["file"]).resolve()
    if d.resolve() not in path.parents:
        raise ToolError(f"evidence path escapes record: {evidence_id}")
    return {"id": evidence_id, "title": e.get("title", ""), "text": path.read_text()}

def stat_evidence(record_dir: str, evidence_id: str) -> dict:
    item = read_evidence(record_dir, evidence_id)
    text = item["text"]
    return {"id": evidence_id, "chars": len(text), "lines": len(text.splitlines()), "title": item["title"]}

def build_record_context(record_dir: str) -> tuple[str, list[dict]]:
    trace = [{"tool": "list_evidence", "args": {}, "result": list_evidence(record_dir)}]
    chunks = []
    for e in trace[0]["result"]:
        body = read_evidence(record_dir, e["id"])
        trace.append({"tool": "read_evidence", "args": {"evidence_id": e["id"]}, "result": {"id": body["id"], "title": body["title"], "chars": len(body["text"])}})
        chunks.append(f"[{body['id']}] {body['title']}\n{body['text'].strip()}")
    return "\n\n".join(chunks), trace

def main() -> int:
    ap = argparse.ArgumentParser(description="Local read-only evidence tools for adj model-pool evals.")
    ap.add_argument("action", choices=["list", "read", "stat", "context"])
    ap.add_argument("record_dir")
    ap.add_argument("evidence_id", nargs="?")
    args = ap.parse_args()
    if args.action == "list":
        out = list_evidence(args.record_dir)
    elif args.action == "read":
        if not args.evidence_id: raise SystemExit("read requires evidence_id")
        out = read_evidence(args.record_dir, args.evidence_id)
    elif args.action == "stat":
        if not args.evidence_id: raise SystemExit("stat requires evidence_id")
        out = stat_evidence(args.record_dir, args.evidence_id)
    else:
        context, trace = build_record_context(args.record_dir)
        out = {"context": context, "trace": trace}
    print(json.dumps(out, indent=2, sort_keys=True))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
