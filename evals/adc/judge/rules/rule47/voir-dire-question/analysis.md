# Rule 47 Judge Eval Analysis

These results predate the evaluator's restoration to the current ADC runtime.  The old harness issued one provider response, passed its proposed action through Lean `apply_decision`, and stopped before several production turn stages.  Current comparisons require a new run whose summary identifies `production` or `counterfactual_model` execution mode.

## Voir Dire Question Screening

The voir dire question eval covers `decide_voir_dire_question`.  The baseline fixture set contains 60 voir dire questions across allowed screening questions and prohibited precommitment, sufficiency, assumed-fact, merits-argument, and inadmissible-material questions.  The hard fixture set adds 30 tier-3 boundary questions focused on damages ranges, digital-evidence sufficiency, limiting-instruction phrasing with embedded disputed facts, missing-witness sufficiency, insurance references, and “could you still find” formulations.

The prompt-iteration path uses eval-local Markdown templates under [Judge Prompt Candidates](prompts/) and leaves the production Lean opportunity text unchanged.  Each live run records the prompt source, copied prompt text, results JSONL, and summary JSON under `evals/out/adc/judge/`, which is generated local data and should not be committed.  The scorer uses deterministic reason tags, so explanation scores measure whether the ruling reason names an accepted category rather than whether a separate model agrees with the explanation.

### Results

| Prompt | Fixture Set | Correct | Reason Matches | False Allows | False Disallows | Invalid | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|
| production | baseline 60 | 60 | 60 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | baseline 60 | 60 | 55 | 0 | 0 | 0 | 1.000 |
| candidate-v2 | baseline 60 | 59 | 58 | 1 | 0 | 0 | 0.973 |
| candidate-v3 | baseline 60 | 60 | 59 | 0 | 0 | 0 | 1.000 |
| production | hard 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | hard 30 | 29 | 27 | 1 | 0 | 0 | 0.952 |
| candidate-v2 | hard 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |
| candidate-v3 | hard 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |

### Findings

The production opportunity prompt remains the best measured prompt.  It scored perfectly on both fixture sets after the deterministic scorer accepted the singular phrase “limiting instruction” as instruction-following wording.  That scorer adjustment did not change any ruling outcome; it corrected a vocabulary gap in explanation matching.

The hard fixture set was useful because it separated candidates that the baseline alone could not distinguish.  Candidate v1 preserved the baseline ruling outcomes, but it encouraged generic explanation language such as “class of evidence” and then failed the hard set by allowing a prohibited damages-range comfort question.  The false allow was `jvdh-002`: “If liability is proven, would you be comfortable returning a damages number between $80,000 and $120,000?”

Candidate v2 fixed the hard-set behavior but failed the original baseline on the same legal boundary.  It allowed `jvd-053`, a tier-3 damages-precommitment question asking whether the candidate would be comfortable returning a damages number in the $80,000 to $120,000 range if liability were proven.  That result shows why the original baseline should remain fixed while new hard sets test candidate prompts against additional failure clusters.

Candidate v3 directly addresses the damages-number cluster.  It distinguishes permissible questions about fixed damages bias or proof discipline from prohibited questions asking whether the candidate would be comfortable with, willing to return, able to award, or inclined to reject a named amount, range, minimum, maximum, or nominal-damages result.  Candidate v3 removed the false-allow cluster across both fixture sets, but it still lost one baseline reason match by classifying an attention question as digital or documentary evidence.

## For-Cause Challenges

The for-cause eval covers `decide_juror_for_cause_challenge`.  The fixture set contains 16 pending challenges across fixed bias, follow-law refusal, damages precommitment, digital-evidence refusal, relationship interest, sympathy bias, language or attention limitations, hardship, lawful attitudes, and rehabilitation.  Each row builds a real ADC voir dire state with an answered exchange and a pending challenge, then runs the judge through the Lean opportunity and tool schema.  The scorer checks the challenge identifiers, the grant or denial decision, the reason tags, and Lean acceptance.

### Results

| Prompt | Run | Correct | Reason Matches | False Grants | False Denials | Invalid | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|
| production | dry 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| production | live 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | dry 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | live 16 | 16 | 16 | 0 | 0 | 0 | 1.000 |

### Findings

Production made no outcome errors on the first for-cause set.  The initial live production summary reported 14 explanation matches because the scorer did not recognize ordinary wording for rehabilitation and lawful preference.  Rescoring fixed those deterministic vocabulary gaps by accepting phrases such as “rehabilitation,” “assurance,” “follow the instructions,” “general preference,” and “not disqualifying.”  The payloads were valid, and Lean accepted every returned ruling.

Candidate v1 also made no outcome errors.  The candidate prompt states the central for-cause boundary more directly, but the current fixture set does not show an improvement over production.  Its value is as an eval-local reference for future hard rows, especially rows that mix awkward first answers, later assurances, and party attempts to convert lawful skepticism into cause.

## Recommendation

Do not move any Rule 47 candidate prompt into production based on these runs.  Production scored 90 correct voir dire rulings across the combined measured sets and 16 correct for-cause rulings on the first challenge set, with no false allows, false disallows, false grants, false denials, or invalid responses.  Candidate v3 is the strongest eval-local candidate for voir dire question screening, and candidate v1 remains a reference prompt for future for-cause challenge rows.  A harder fixture set should create the next prompt decision point.

## Next Work

Future fixture additions should follow the hard-set pattern.  Add boundary pairs that differ by one legal feature, preserve the original baseline for comparison, and run candidates against both sets before considering production changes.  The next useful voir dire clusters are questions that mix proper instruction-following language with disputed facts, and questions that turn a proper evidence-category screen into a specific sufficiency commitment.  The next useful for-cause clusters are rehabilitation after damaging first answers, hedged willingness to follow limiting instructions, and challenges based on unpopular but lawful attitudes toward damages, corporations, or digital evidence.
