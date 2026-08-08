# Evidence Handling

This note documents the AARD runtime evidence layer used by `aard case`.

## Model

AARD owns record custody. It stores admitted bytes, assigns stable evidence identifiers, records provenance metadata, enforces policy limits, and logs access. Attorneys and later council agents inspect evidence through media-agnostic methods. AARD does not parse, render, OCR, transcribe, extract, execute, or otherwise interpret evidence formats.

`evidence_id` is the record identity for an evidence item. It is deterministic from the stored SHA-256 and a normalized source name. It is not a local path, workspace path, or content-addressed storage path. Filings cite visible `evidence_id` values in `offered_evidence`.

Local paths, workspace paths, and content-addressed storage names are implementation details.  Use `evidence_id` plus SHA-256 when exact byte custody is at issue.

## Runtime storage

Each run writes evidence state under `--out-dir`:

```text
evidence-manifest.json
evidence-store/<sha-prefix>/<sha256>
submitted-evidence/
events.ndjson
run.json
state.json
```

`evidence-store/` is content-addressed by SHA-256. Repeated identical bytes may share the same stored object. `evidence-manifest.json` records the AARD view of each visible evidence:

- `evidence_id`
- `sha256`
- `size_bytes`
- `mime_type`
- `storage_name`
- `created_at`
- `admissibility_status`
- `record_visibility`
- optional title, original filename, provenance, parent evidence, derivation, and readability fields

Initial case materials are registered as `case_packet` evidence. Accepted attorney submissions are registered as `submitted_evidence` evidence.

## Lawyer API Methods

AARD exposes evidence operations through the Lawyer API.  The case process supplies the current operation list for each opportunity.  Evidence reads are available in openings, arguments, rebuttals, surrebuttals, and closings, while evidence submission is available in arguments, rebuttals, and surrebuttals.

- `get_case` returns the visible arbitration record.
- `list_evidence` lists visible evidence metadata. It returns metadata only, not bytes.
- `stat_evidence` returns metadata, allowed operations, and remaining limits for one evidence item.
- `read_evidence_range` returns a bounded byte range as base64. It never mutates the record. Successful reads are logged as `evidence_read` events.
- `submit_evidence` submits small source evidence in one JSON request using `content` or `content_base64`.
- `submit_decision` submits the legal act for the current opportunity.
- `case_status` reports the current case phase and active turn.
- `send_work_notes` records off-record lawyer work notes for outside analysis.


## Chunked upload methods

Chunked upload is for evidence too large or unsuitable for single-request `submit_evidence`.

- `begin_evidence_upload` starts an upload session. It requires title, MIME type, expected size, relevance, and either source URL or source description. Nothing is admitted at this step.
- `write_evidence_chunk` writes one base64 chunk at the next expected offset. Chunks must be sequential. The runtime enforces chunk and total upload limits.
- `commit_evidence_upload` verifies size and SHA-256, admits the evidence through the Lean `submit_evidence` state transition, moves the uploaded bytes into `submitted-evidence/`, registers the evidence in `evidence-store/`, and returns `evidence_id`.

A failed or incomplete upload session is not evidence. A completed upload becomes record evidence only after commit succeeds and the Lean engine accepts the corresponding `submit_evidence` action.

## Policy limits

The policy has three evidence-size limits:

- `max_submitted_evidence_bytes` is the authoritative record limit enforced by the Lean engine for each submitted evidence.
- `max_exhibit_bytes` caps an offered evidence item. The default matches `max_submitted_evidence_bytes` so chunked evidence accepted into the record can be offered as an exhibit.
- `max_direct_submitted_evidence_bytes` is the smaller direct JSON/base64 limit for `submit_evidence`.
- `max_evidence_upload_bytes` is the chunked-upload limit. It must not exceed `max_submitted_evidence_bytes`.

Evidence read policy:

- `max_evidence_chunk_bytes` caps each uploaded chunk.
- `max_evidence_read_bytes` caps each evidence range read.
- `max_evidence_reads_per_opportunity` caps read count per opportunity.
- `max_evidence_read_bytes_per_opportunity` caps returned evidence bytes per opportunity.

The runtime rejects invalid policies at startup. Evidence access is enforced server-side by phase. Evidence reads are allowed throughout the lawyer merits sequence, and evidence submissions are allowed during arguments, rebuttals, and surrebuttals.

## Custody invariants

The implementation must preserve these invariants:

1. AARD stores exact bytes before exposing an item as accepted evidence.
2. `evidence_id` and SHA-256 identify record bytes. Paths do not.
3. Upload commit does not bypass the Lean `submit_evidence` transition.
4. `offered_evidence` uses visible `evidence_id` values.
5. Evidence reads are logged.
6. AARD remains media-agnostic. Agents examine bytes with their own tools.

## Inspection checklist

After a run that uses submitted evidence:

```bash
jq '.evidence | length' "$out_dir/run.json"
jq '.evidence_count' "$out_dir/evidence-manifest.json"
jq '.evidence[] | {evidence_id,sha256,size_bytes,mime_type,admissibility_status}' "$out_dir/evidence-manifest.json"
grep -n 'evidence_read\|evidence_materialized\|submitted_evidence' "$out_dir/events.ndjson"
```

For each important exhibit, verify that:

- the `offered_evidence` entry uses a visible `evidence_id`;
- the corresponding evidence has the expected SHA-256 and size;
- any derived evidence names its source evidence and derivation method;
- the attorney's filing distinguishes source evidence from analysis or work product.
