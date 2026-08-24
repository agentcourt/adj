# ADC Evals

`evals/adc/` contains behavior eval assets for ADC actors.  [Judge Evals](judge/README.md) live under `judge/`, organized by procedural rule and evaluated behavior.  Future lawyer and juror evals should use the same split between committed fixtures and ignored run output.

Committed eval directories hold fixtures, prompt candidates, plans, and analysis.  Runner code that depends on ADC runtime types stays under `adc/runtime/eval`.  Generated ADC eval output belongs under `evals/out/adc/`.

## Layout

| Path | Contents |
| --- | --- |
| [Judge Evals](judge/README.md) | Judge behavior suites, grouped by rule and behavior. |
| `../out/adc/` | Ignored generated output from ADC behavior eval runs. |
