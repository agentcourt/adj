# Evals

`evals/` holds behavior evals for the adjudication systems: fixture sets that put a system actor in a controlled state, prompt candidates for controlled comparisons, and analysis records from completed runs.  The runners are Go code inside the system runtimes and execute the production opportunity, reference-tool, correction, validation, and state-transition path.  [ADC Evals](adc/README.md) is the current actor tree, and [Judge Evals](adc/judge/README.md) is its current suite family.

Committed eval directories hold fixtures, prompt candidates, plans, and analysis.  Generated run output belongs under `out/`, which is ignored except for `out/.gitkeep`.  Model and provider-endpoint selection is a separate concern and lives in [Model Pool](../model-pool/README.md); it builds the juror and council pools that live runs draw from rather than measuring actor behavior.

## Contents

| Path | Contents |
| --- | --- |
| [adc/](adc/README.md) | ADC behavior evals, grouped by actor. |
| `out/` | Ignored generated output from eval runs. |
