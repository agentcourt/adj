# Evidence Handling

This note documents the evidence custody used by `aard case`.  The runtime stores admitted bytes, assigns record identifiers, records provenance, enforces custody limits, and provides bounded reads.  The [AARD rules](ARAP.md) define when a party may submit or offer that material.

## Record Model

An `evidence_id` identifies one record item and follows `ev_<sha-prefix>_<slug>`, where the prefix is the first 12 characters of the stored SHA-256.  The full canonical digest and byte size bind that identifier to the admitted bytes.  Local paths and content-addressed storage names remain custody details rather than record citations.

The engine receives an immutable initial evidence catalog during case initialization.  Submitted evidence enters a separate ordered list through accepted `submit_evidence` actions, and filings cite either class through `offered_evidence`.  Arguments, rebuttals, and surrebuttals may submit and offer evidence, while openings and closings may read admitted material but cannot add it.

Derived evidence carries either all three lineage fields or none: `parent_evidence_id`, `parent_sha256`, and `derivation_method`.  The runtime resolves the parent from the initial catalog or an earlier submission and supplies its verified digest.  Lean rejects an unknown parent, a mismatched digest, self-parenting, incomplete lineage, and a parent absent from the action's source state.

## Initial Capture

Automatic selection scans the complaint directory, skips directories, and excludes names ending in `~`.  It excludes the configured complaint by file identity and the exact names `.gitignore`, `README.md`, `complaint.md`, `situation.md`, `sign.sh`, `confession.sig`, and `samantha_private.pem`.  Explicit `--file` values replace that scan, and every selected input must be a regular file.  The runtime checks that the path and opened descriptor refer to the same file, reads and hashes that descriptor, rewinds it for publication, and rejects replacement, size drift, or digest drift.

The runtime publishes each verified file into the evidence store before Lean initialization.  It then derives the initial catalog from the verified descriptor results, including identifier, full digest, and size.  An initial file may have zero bytes, while an admitted submission must have a positive size.  Council sampling begins after the catalog exists, and the private API begins after Lean accepts initialization.

## Runtime Storage

Each run writes evidence records below `--out-dir`.  The shared manifest schema is `aar.evidence-manifest.v0`, which AARD retains for compatibility with the sibling evidence format.  The Lean state remains schema `v1` and carries its initial catalog plus accepted submitted-evidence metadata.

| Path | Contents |
|---|---|
| `evidence-manifest.json` | Shared manifest schema, creation time, count, and sorted evidence metadata. |
| `evidence-store/<first-two-sha256-characters>/<sha256>` | Immutable content-addressed bytes. |
| `submitted-evidence/` | Copies of accepted lawyer submissions. |
| `events.ndjson` | Process events, including successful Lawyer and Council evidence reads. |
| `run.json` | Final result with case-file, submitted-evidence, and evidence metadata. |
| `state.json` | Final engine state and record commitments. |

Manifest entries include `evidence_id`, SHA-256, size, MIME type, storage name, creation time, admissibility status, visibility, and readability.  Optional fields carry title, original name, source description or URL, retrieval time, submitting role and phase, relevance, and lineage.  An initial item has `case_packet` status, while an accepted lawyer submission has `submitted_evidence` status.

## Role API Operations

The Lawyer API publishes the operation list allowed for each opportunity.  Each request body has a finite byte limit and must contain one JSON value, while operation arguments reject unknown fields.  Participant argument errors return `tool_failed`, and storage or engine faults return `runtime_failure`.

| Operation | Behavior |
|---|---|
| `list_evidence` | Returns visible evidence metadata without file bytes. |
| `stat_evidence` | Returns one item's metadata, allowed operations, and remaining limits. |
| `read_evidence_range` | Returns a bounded range as base64 after verifying the complete stored file. |
| `submit_evidence` | Submits a small source item from `content` or `content_base64`. |
| `begin_evidence_upload` | Starts a sequential chunked upload without admitting evidence. |
| `write_evidence_chunk` | Writes the next bounded base64 chunk at the required offset. |
| `commit_evidence_upload` | Verifies the completed upload and enters the shared admission path. |

Lawyer reads are available in every merits phase, while submissions are limited to arguments, rebuttals, and surrebuttals.  Council API members may list, inspect, and read evidence during their deliberation opportunity.  Observer calls may perform the same bounded inspection, but successful Observer reads do not consume participant read budgets or create `evidence_read` events.

## Verified Reads

A Lawyer or Council read reserves count and byte capacity while holding the case mutex, then releases the mutex for file input.  The runtime opens one regular-file descriptor, confirms path and descriptor identity, hashes and sizes the complete file while collecting the requested range, and reacquires the mutex to check that the opportunity remains current.  A successful read commits the reservation and writes `evidence_read`, while a failed or stale read restores the reservation when applicable.

Observer reads use the same descriptor verification without participant budget accounting or read events.  Path replacement, a nonregular file, size drift, and digest drift produce request-scoped `runtime_failure` responses for an Observer.  Those Observer storage faults do not end the active case.

## Submission and Publication

Direct and chunked submissions converge on one admission path.  The runtime validates a candidate, obtains an accepted Lean transition, publishes the immutable store object and submitted copy, writes a candidate evidence manifest, rechecks the turn deadline, and commits state, evidence registries, and the replay action together under the case mutex.  A filing can offer that evidence only after this commit because Lean checks offers against the filing action's source state.

Public evidence submission requires an identifier absent from the initial and submitted-evidence registries.  Separate records containing identical bytes may reuse the same content-addressed store object, while their record identifiers remain distinct.  A duplicate submitted `evidence_id` or conflicting digest, size, storage, or provenance causes rejection.  Errors before the in-memory commit preserve the prior state, while a later event-write failure can report an error after state and replay action commit.

The publication sequence has no multi-file crash journal.  A machine stop can therefore leave files that the committed state and manifest do not reference.  Recovery requires inspection of the output packet before any manual removal.

## Policy Limits

Four policy fields bound evidence admission and use.  Lean enforces `max_submitted_evidence_bytes` on each submission and `max_exhibit_bytes` on every offered item, while Go applies the same limits before custody publication.  Go also enforces the smaller transport-specific direct and upload limits.

| Field | Scope |
|---|---|
| `max_submitted_evidence_bytes` | Maximum bytes in one admitted submitted item. |
| `max_exhibit_bytes` | Maximum bytes in one item offered by a filing. |
| `max_direct_submitted_evidence_bytes` | Maximum bytes carried by one direct JSON or base64 submission. |
| `max_evidence_upload_bytes` | Maximum bytes admitted through one chunked upload. |

Upload chunks also obey `max_evidence_chunk_bytes`.  Range reads obey per-call, per-opportunity count, and per-opportunity byte limits.  Technical-report counts and UTF-8 title and summary byte limits apply to arguments, rebuttals, and surrebuttals in both Go and Lean.

## Inspection

Packet inspection should compare record identifiers, full digests, sizes, lineage, and the offered references in each filing.  A derived item should name the expected parent identifier and digest, and its parent should occur in the initial catalog or earlier submitted evidence.  The event sequence should place `submitted_evidence` before the filing that first offers that item.

```bash
AARD_OUTPUT=out/example
jq '.evidence | length' "$AARD_OUTPUT/run.json"
jq '{schema_version,evidence_count}' "$AARD_OUTPUT/evidence-manifest.json"
jq '.evidence[] | {evidence_id,sha256,size_bytes,parent_evidence_id,parent_sha256,derivation_method}' "$AARD_OUTPUT/evidence-manifest.json"
rg -n 'evidence_read|evidence_materialized|submitted_evidence' "$AARD_OUTPUT/events.ndjson"
```

The stored bytes require a separate custody check from replay-certificate verification.  `aard verify-certificate` hashes JSON states and replays engine actions but does not rehash `evidence-store/`.  The [verification guide](verification.md) states that boundary and the formal facts proved for accepted Lean certificates.
