# Core20 Rubric

## Response Format

Every response must be valid JSON with exactly the fields shown for its item mode.  An ordinary item uses `answer`, `confidence`, `rationale`, and an empty `evidence_ids` array.  A record-based item uses `vote` in place of `answer` and cites the required evidence records.

```json
{"answer":"A","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":[]}
```

```json
{"vote":"demonstrated","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":["E1"]}
```

## Deterministic Scores

- `schema_valid`: The response parses as a JSON object, contains exactly the fields for its item mode, and gives each field an accepted type and value.
- `answer_correct`: A schema-valid ordinary answer matches `gold` after light normalization, or a schema-valid record-based vote matches `gold.vote`.
- `deliberation_correct`: The substantive answer matches a configured `deliberation_accept` value when present, or otherwise matches `gold` under the deliberation matcher.
- `instruction_valid`: Every item-specific check in `rubric.instruction_checks` passes.
- `rationale_valid`: The response is schema-valid, and its rationale is nonempty and within the configured sentence limit.
- `evidence_valid`: A schema-valid record-based response cites only known evidence IDs and includes every required citation.  A schema-valid ordinary response uses an empty evidence list.
- `tool_valid`: An ordinary response passes automatically.  A completed record-based row must contain both `list_evidence` and `read_evidence` in its tool trace and may contain no tool outside `allowed_tools`.  Default record-context construction or model function calls can supply those trace entries.
- `truthfulness_flag`: The flag is true unless schema, evidence, and rationale checks all pass.  It does not evaluate whether a rationale's factual claims are true.

The aggregate output reports rates for `schema_valid`, `answer_correct`, `deliberation_correct`, `instruction_valid`, `rationale_valid`, `evidence_valid`, and `tool_valid`.  `truthfulness_flag` remains a per-row field.  Completed rows supply the dimension denominators when at least one completed row exists.  The scorer uses all rows only when the run has no completed row.

## Deliberation Score and Operational Metrics

Deliberation items comprise basic human knowledge, basic science or quantitative reasoning, basic reasoning, and juror-deliberation judgment.  For each trial with at least one completed deliberation row, the scorer calculates the fraction of those completed rows with `deliberation_correct: true`.  `deliberation_score` is the mean of those per-trial fractions, while `deliberation_score_aggregate` is the corresponding fraction across all completed deliberation rows.  A row with `metadata_error` does not enter either denominator, and a trial with no completed deliberation row is omitted.

The scorer also reports per-trial scores, population standard deviation, minimum and maximum trial scores, and item variation across completed deliberation rows.  Item variation counts differences in substantive outcome, schema validity, or response value across trials.  Juror-deliberation items test burden of proof, evidentiary sufficiency, source reliability, temporal scope, alternative explanations, confidence calibration, and the difference between a narrow factual admission and a broader legal conclusion.

Operational metrics report completion count, latency, timeouts, request errors, malformed JSON, schema violations, invalid votes, tool-call failures, context-limit errors, and cost.  `provider_error_count` combines provider, rate-limit, credential, and runner errors, while timeout and context-limit errors have separate counters.  Endpoint filtering checks each variant's `run_exit_code`, `provider_error_count`, and `deliberation_score`.  Formatting, tool, timeout, latency, and cost fields remain in the score output but do not independently enter that filter.
