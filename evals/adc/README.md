# ADC Evals

`evals/adc/` contains the checked-in assets for ADC behavior evals.  [Judge Evals](judge/README.md) are organized by procedural rule and judge action.  Every suite contains fixtures, prompt candidates, and analysis.  Nine suites contain a local `plan.md`, while Rule 47 voir dire uses the cross-rule [Judge Eval Plan](judge/plan.md).

Runner code that depends on ADC runtime types lives under `adc/runtime/eval`, and `adc eval` exposes the registered commands.  Each run writes `results.jsonl` and `summary.json` beneath its selected output directory.  Generated ADC eval output belongs under `evals/out/adc/`.

## Layout

| Path | Contents |
| --- | --- |
| [Judge Evals](judge/README.md) | Judge behavior suites, grouped by rule and behavior. |
| `../out/adc/` | Ignored generated output from ADC behavior eval runs. |
