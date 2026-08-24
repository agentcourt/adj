# Evals

`evals/` contains fixtures, prompt candidates, suite plans, and analysis for adjudication behavior evals.  Go runners under `adc/runtime/eval` construct the fixture state and execute the production opportunity and state transition.  Model-backed turns include correction and decision-validation stages, while deterministic turns execute the action supplied by the opportunity.  The checked-in suites cover [ADC judge behavior](adc/judge/README.md).

Committed suite directories hold their inputs and documentation.  Generated `results.jsonl` and `summary.json` files belong under `out/`, which Git ignores except for `out/.gitkeep`.  [Model Pool](../model-pool/README.md) evaluates provider endpoints and constructs juror and council pools.

## Contents

| Path | Contents |
| --- | --- |
| [ADC Evals](adc/README.md) | ADC behavior evals, grouped by actor. |
| `out/` | Ignored generated output from eval runs. |
