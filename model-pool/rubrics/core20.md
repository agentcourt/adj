# core20 rubric

## Response contract

Every response must be valid JSON. Ordinary items use:

```json
{"answer":"A","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":[]}
```

Tool/evidence items use:

```json
{"vote":"demonstrated","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":["E1"]}
```

## Deterministic scores

- `schema_valid`: response parses as JSON and contains exactly the required fields for the item type.
- `answer_correct`: multiple-choice and short-answer items match `gold` after light normalization. Tool/evidence items match `gold.vote`.
- `instruction_valid`: item-specific checks in `rubric.instruction_checks` pass.
- `rationale_valid`: rationale respects the configured sentence limit and is non-empty.
- `evidence_valid`: cited evidence ids exist in the fixture and required citations are present. Non-tool items must not cite evidence.
- `tool_valid`: for completed tool/evidence items, the runner supplied record context through the local evidence layer or the model used the required evidence tools, with no tools outside `allowed_tools`.
- `truthfulness_flag`: true when deterministic checks detect malformed output, invalid evidence use, or rationale-format failure. It is a review flag, not a semantic contradiction detector in v0.

A v0 aggregate should report these dimensions separately. Do not collapse them into one score when comparing council or juror models.


## Deliberation score and operational metrics

`deliberation_score` is computed only over deliberation items: basic human knowledge, basic science/quantitative reasoning, basic reasoning, and juror-deliberation judgment. It is the mean of per-trial fractions of items with `deliberation_correct = true`. `deliberation_correct` measures the substantive answer and is separate from strict schema compliance. The scorer also reports per-trial scores, standard deviation, minimum score, and per-item variation across trials.

Juror-deliberation items test whether a model applies the record like a careful juror: it respects burden of proof, avoids unsupported inferences, weighs contemporaneous records against weaker recollections, matches the proposition's temporal scope, considers alternative explanations, ignores irrelevant reputation evidence, calibrates confidence, and distinguishes a narrow factual admission from broader legal effect.

The scorer reports operational metrics separately and excludes them from `deliberation_score`. They include latency, timeouts, provider errors, malformed JSON, schema violations, invalid votes, tool-call failures, context-limit errors, and cost.

Use operational metrics as filters or eligibility gates. Use `deliberation_score` as the first quantitative quality measure among models that satisfy the required operational filters.
