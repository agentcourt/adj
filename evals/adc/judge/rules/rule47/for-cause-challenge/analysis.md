# Rule 47 For-Cause Evaluation Analysis

## Evaluation Coverage

The evaluation exercises `decide_juror_for_cause_challenge` across sixteen voir dire states.  Nine fixtures expect the challenge to be granted, and seven expect it to be denied.  The issue families cover refusal to follow law, fixed bias, lawful attitudes, damages commitments, digital-evidence refusal, direct or remote relationships, sympathy, rehabilitation, language or attention limits, and hardship.

## Decision Boundaries

The fixtures distinguish inability to decide impartially or follow the court's instructions from inconvenience, skepticism, preference, and remote relationships.  They test fixed views about liability, proof, evidence categories, and damages against answers showing a willingness to apply the law.  Rehabilitation fixtures require the ruling to consider the completed answer record rather than isolate an awkward statement from a credible assurance.

## Payload and Scoring

Each fixture constructs ADC state with one answered voir dire exchange and one pending for-cause challenge, obtains the Lean judge opportunity, executes it through the opportunity runner, and applies the ruling through Lean.  The scorer requires one `decide_juror_for_cause_challenge` payload with challenge id `fc-1`, the fixture's juror and challenging party, a Boolean `granted` field, and nonempty `ruling_reason`.  A mismatch in any required identifier makes the response invalid.

Outcome correctness compares the grant decision with the fixture label and also requires successful Lean application and an accepted runner step.  Explanation scoring uses deterministic phrase matching against each fixture's reason tags.  The summary reports accuracy, severity-weighted accuracy, invalid responses, false grants, false denials, and slices by reason tag, issue family, tier, and challenging party.

## Prompt Template

The [prompt candidate](prompts/candidate-v1.md) states the categories that support or defeat a for-cause challenge.  It directs the judge to use the required identifiers from the opportunity constraints and to explain the decisive record fact.  The evaluator substitutes the production objective and fixture record before using the template as the opportunity objective.
