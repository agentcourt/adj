# Juries

ADC constructs a jury from a court-controlled JSONL request-specification file.  Each record can identify a model endpoint, provider constraints, request settings, and a persona.  The direct runtime loads and samples these records when it prepares the candidate panel.

The case policy defines jury size, unanimity, and minimum concurrence.  The command-line overrides are `--juror-count`, `--unanimous-required`, and `--minimum-concurring`.  Lean validates the final clerk action that places this policy into the authoritative case state.

Attorneys question candidates, raise challenges for cause, and exercise available peremptory strikes under [Rule 47](ARCP.md#rule-47-selecting-jurors).  The judge rules on cause challenges, and the engine enforces candidate and strike limits.  A case policy can skip voir dire and empanel candidates randomly after setup.

The court's standard questionnaire contains eight general qualification questions covering attention, impartiality, service constraints, ability to apply instructions and the burden of proof, prior knowledge or relationships, evidence assessment, remedies, and other sources of bias.  Counsel supplies case-specific questions during voir dire, subject to judicial screening and the configured question limits.

Jurors answer selection questions and vote from the trial record during deliberation.  Failure before deliberation can permit candidate replacement, while failure during deliberation removes the juror from the eligible body.  Verdict derivation applies the recorded jury policy to the sworn jurors who remain eligible.

For an external juror, process failure, an elapsed turn deadline, and exhaustion of the invalid-decision allowance use the same failure handler.  During a questionnaire or follow-up opportunity in voir dire, the handler marks the candidate failed and adds a replacement.  During a voting opportunity, it removes the failed juror from the eligible body.  The record retains the failure cause.  An error while applying those actions ends the case.
