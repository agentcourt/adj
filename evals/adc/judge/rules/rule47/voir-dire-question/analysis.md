# Rule 47 Voir Dire Evaluation Analysis

## Evaluation Coverage

The evaluation exercises `decide_voir_dire_question` against two fixture files.  The baseline file contains sixty rows across three difficulty tiers, evenly divided between allowed and disallowed questions.  The hard file contains thirty tier-3 rows, also evenly divided between allowed and disallowed questions.

Allowed families test bias or impartiality, burden of proof, digital or documentary evidence, damages skepticism, attention, and willingness to follow instructions.  Disallowed families test liability or damages precommitment, specific-evidence sufficiency, assumed disputed facts, merits argument, inadmissible material, and compound precommitment.  The hard rows place permissible screening language beside questions that ask the candidate to accept evidence, predict a result, or commit to a damages amount.

## Decision Boundaries

A permissible question asks whether the candidate can remain impartial, apply a legal standard, follow instructions, attend to the record, or evaluate a general category of evidence.  A prohibited question asks the candidate to decide disputed facts, predict a verdict, commit to liability or damages, treat named evidence as sufficient, or react to inadmissible material.  The close fixtures test whether conditional phrasing or a proper introductory clause conceals a commitment about the merits.

Damages fixtures distinguish fixed bias or willingness to follow damages instructions from comfort with a named award, range, floor, cap, or nominal result.  Evidence fixtures distinguish openness to an evidence category from a commitment that named evidence would establish a disputed proposition.  Instruction fixtures distinguish willingness to follow a limiting instruction from acceptance of the disputed fact embedded in the proposed question.

## Payload and Scoring

Each fixture constructs ADC voir dire state with one proposed exchange, obtains the Lean judge opportunity, executes it through the opportunity runner, and applies the ruling through Lean.  The scorer requires one `decide_voir_dire_question` payload with exchange id `vx-1`, the fixture's juror id, a Boolean `allowed` field, and nonempty `ruling_reason`.  A mismatch in a required identifier or a malformed ruling makes the response invalid.

Outcome correctness compares `allowed` with the fixture label and also requires successful Lean application and an accepted runner step.  Explanation scoring uses deterministic phrase matching against each fixture's reason tags.  The summary reports accuracy, severity-weighted accuracy, invalid responses, false allows, false disallows, and slices by reason tag, question family, tier, and asking party.

## Prompt Templates

The [prompt directory](prompts/) contains three templates.  `candidate-v1.md` states the allowed and prohibited categories, `candidate-v2.md` adds a controlled vocabulary for the ruling reason, and `candidate-v3.md` specifies the damages-commitment boundary.  The evaluator substitutes case theme, asking party, juror id, proposed question, and required exchange id before using the selected template as the opportunity objective.

## Execution and Prompt Selection

The command resolves the configured court and model, then gives the resulting response client and ADC state to the opportunity runner.  The runner uses the objective supplied by the judge opportunity by default.  A selected template replaces that objective after fixture substitution.  The evaluator writes per-fixture records to `results.jsonl` and aggregate metrics to `summary.json`.

```bash
adc eval judge-voir-dire \
  --fixtures evals/adc/judge/rules/rule47/voir-dire-question/hard-fixtures.jsonl \
  --opportunity-prompt-file evals/adc/judge/rules/rule47/voir-dire-question/prompts/candidate-v3.md \
  --opportunity-prompt-name candidate-v3 \
  --out-dir evals/out/adc/judge/voir-dire-hard-candidate-v3
```
