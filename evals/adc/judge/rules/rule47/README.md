# Rule 47

Rule 47 contains separate suites for `decide_voir_dire_question` and `decide_juror_for_cause_challenge`.  The question-screening suite has sixty baseline fixtures and thirty hard fixtures, while the for-cause suite has sixteen fixtures.  Their scorers check the ruling, required identifiers, reason tags, invalid payloads, and Lean acceptance.

## Suites

| Suite | Analysis | Plan |
| --- | --- | --- |
| [Voir Dire Question Screening](voir-dire-question/README.md) | [Rule 47 Voir Dire Analysis](voir-dire-question/analysis.md) | [Judge Eval Plan](../../plan.md) |
| [For-Cause Challenges](for-cause-challenge/README.md) | [Rule 47 For-Cause Analysis](for-cause-challenge/analysis.md) | [For-Cause Plan](for-cause-challenge/plan.md) |
