# Evidence Handling

This note documents the AAR runtime evidence layer used by `aar case`.

## Model

AAR owns record custody.  It stores record bytes, assigns stable evidence identifiers, records provenance metadata, enforces policy limits, and logs successful participant range reads.  Attorneys and council members inspect evidence through media-agnostic methods, while AAR does not parse, render, OCR, transcribe, extract, or execute evidence formats.  The stored bytes define the record object independently of a participant's source path.

`evidence_id` identifies an evidence item in the record.  AAR derives it from the stored SHA-256 and a normalized source name; local paths, workspace paths, and content-addressed storage paths have no record identity.  Filings cite visible `evidence_id` values in `offered_evidence`.

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

`evidence-store/` is content-addressed by SHA-256.  AAR opens each regular initial case file once, hashes and publishes from that descriptor, and constructs the file view and Lean `evidence_catalog` from the stored copy before Lean initializes the case.  Each catalog, submission, and lineage SHA-256 commitment uses exactly 64 lowercase hexadecimal characters.  The runtime sorts the catalog by evidence identifier before initialization, and repeated identical bytes may share one stored object.  `evidence-manifest.json` records the AAR view of each visible evidence:

- `evidence_id`
- `sha256`
- `size_bytes`
- `mime_type`
- `storage_name`
- `created_at`
- `admissibility_status`
- `record_visibility`
- optional title, original filename, provenance, parent evidence, derivation, and readability fields

Initial case materials are registered as `case_packet` evidence.  Accepted attorney submissions are registered as `submitted_evidence` evidence.  For derived evidence, the public submission tools accept `parent_evidence_id` and `derivation_method` together, while a public call containing `parent_sha256` fails.  AAR resolves the named parent's committed SHA-256 and records all three nonempty strings in the evidence metadata and internal Lean action.  An internal action without a parent omits all three lineage members.

## Lawyer API tools

AAR exposes read-only evidence tools through the HTTP Lawyer API in every active lawyer phase.  Evidence-submission tools are available during arguments, rebuttals, and surrebuttals.

- `get_case` returns the visible arbitration record.
- `list_evidence` lists visible evidence metadata. It returns metadata only, not bytes.
- `stat_evidence` returns metadata and remaining limits for one evidence item.
- `read_evidence_range` returns a bounded byte range as base64.  It never mutates the record.  Successful Lawyer reads are logged as `evidence_read` events; Observer reads are not logged.
- `submit_evidence` submits small source evidence in one JSON request using `content` or `content_base64`.
- `begin_evidence_upload`, `write_evidence_chunk`, and `commit_evidence_upload` submit larger source evidence by chunks.
- `submit_decision` submits the legal act for the current opportunity.

## Council API tools

When `aar case` runs with `--council-backend councilapi`, council members use read-only evidence operations during deliberation.  The Council HTTP API exposes these operations, and its range-read limits apply to those external clients.  The direct backend instead places the rendered record in the model prompt, including readable bodies for offered exhibits.  Direct council calls do not use Council API range reads.

- `get_case` returns the visible arbitration record.
- `list_evidence` lists visible evidence metadata.
- `stat_evidence` returns metadata, allowed read operations, and remaining limits for one evidence.
- `read_evidence_range` returns a bounded byte range as base64 and logs an `evidence_read` event with role `council`.
- `submit_council_vote` submits the council member's vote through the same Lean `submit_council_vote` transition used by the direct council path.

Council members do not receive upload, submit-evidence, or lawyer-decision methods.  AAR does not grant council members a path to introduce new facts.  The procedural boundary is: lawyers build the record; council members examine the admitted record.

## Chunked upload methods

Chunked upload is for evidence too large or unsuitable for single-request `submit_evidence`.

- `begin_evidence_upload` starts an upload session. It requires title, MIME type, expected size, relevance, and either source URL or source description. Nothing is admitted at this step.
- `write_evidence_chunk` writes one base64 chunk at the next expected offset. Chunks must be sequential. The runtime enforces chunk and total upload limits.
- `commit_evidence_upload` verifies size and SHA-256, evaluates the Lean `submit_evidence` transition, publishes the evidence bytes and candidate manifest, records the accepted transition, and returns `evidence_id`.

A failed or incomplete upload session is not evidence.  A completed upload becomes record evidence only after the Lean engine accepts the corresponding action and the candidate evidence manifest has replaced the prior manifest.  A publication failure leaves the prior Lean state and replay-action list unchanged.  The lower-level publication operation can reuse matching content-addressed files, while a publication error returned through the Lawyer API is a `runtime_failure` that ends the active turn and case run without consuming a participant attempt.

## Policy limits

The policy has four evidence-size limits:

- `max_exhibit_bytes` caps each initial or submitted item cited in `offered_evidence`.
- `max_submitted_evidence_bytes` is the authoritative record limit enforced by the Lean engine for each submitted evidence.
- `max_direct_submitted_evidence_bytes` is the smaller direct JSON/base64 limit for `submit_evidence`.
- `max_evidence_upload_bytes` is the chunked-upload limit. It must not exceed `max_submitted_evidence_bytes`.

Lawyer `/do` request bodies reserve `max_response_bytes` for the JSON envelope and metadata, then add the base64-encoded length of `max_direct_submitted_evidence_bytes`.  A `content_base64` value at the direct evidence limit therefore fits with a bounded non-content request.  Council API request bodies remain limited to `max_response_bytes` because council members cannot submit evidence.

`max_report_title_bytes` and `max_report_summary_bytes` cap the UTF-8 encodings of the trimmed title and summary stored in each technical report.  The Go runtime checks these limits before submitting a filing, and the Lean engine checks them at the state-transition boundary.  The Lean engine also resolves every offered evidence identifier against the immutable initial catalog or an earlier submitted item before accepting the filing.

Evidence read policy:

- `max_evidence_chunk_bytes` caps each uploaded chunk.
- `max_evidence_read_bytes` caps each evidence range read.
- `max_evidence_reads_per_opportunity` caps read count per opportunity.
- `max_evidence_read_bytes_per_opportunity` caps returned evidence bytes per opportunity.

The runtime rejects invalid policies at startup. Evidence reads are available in every active lawyer phase. Evidence submission is allowed only during arguments, rebuttals, and surrebuttals.

## Custody invariants

The implementation must preserve these invariants:

1. AAR stores and verifies exact bytes before exposing an item as accepted evidence.
2. `evidence_id`, SHA-256, and size identify record bytes.  Paths do not.
3. The initial Lean state commits to every initial evidence identifier, canonical SHA-256, and size.
4. A submitted identifier cannot collide with the initial catalog or an earlier submission.
5. A derived submission names an initial or earlier submitted parent and repeats that parent's committed SHA-256.
6. Upload commit does not bypass the Lean `submit_evidence` transition.
7. `offered_evidence` uses a catalog or earlier submitted `evidence_id` whose committed size fits `max_exhibit_bytes`.
8. Successful Lawyer and Council range reads are logged.  Observer range reads are not logged.
9. AAR remains media-agnostic.  Agents examine bytes with their own tools.
10. Council evidence access is read-only and narrower than attorney access.

Lean proves relationships among identifiers, canonical SHA-256 strings, sizes, lineage fields, filings, and the immutable catalog recorded in engine state.  `RecordIntegrity` checks the accumulated offered-evidence list against the catalog and complete submitted-evidence list in a state, establishing final-state referential closure.  `MeritsOfferChronology` follows the replay source states and establishes that every merits offer referred to the catalog or a submission already present when the filing occurred.  The Go runtime establishes that each stored file has the recorded hash and size when it creates or reads the custody record.  Replay verification checks the committed metadata and transition history but does not rehash `evidence-store/`; byte-level inspection therefore requires the manifest and stored files as well as the certificate.

## Inspection checklist

After a run that uses submitted evidence:

```bash
jq '.evidence | length' "$out_dir/run.json"
jq '.evidence_count' "$out_dir/evidence-manifest.json"
jq '.evidence[] | {evidence_id,sha256,size_bytes,mime_type,admissibility_status}' "$out_dir/evidence-manifest.json"
grep -n 'evidence_read\|submitted_evidence' "$out_dir/events.ndjson"
```

For each important exhibit, verify that:

- the `offered_evidence` entry uses a visible `evidence_id`;
- the corresponding evidence has the expected SHA-256 and size;
- any derived evidence names its source evidence and derivation method;
- the attorney's filing distinguishes source evidence from analysis or work product.
