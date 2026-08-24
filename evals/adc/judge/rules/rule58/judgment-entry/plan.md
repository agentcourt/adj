# Rule 58 Judge Eval Plan

## Scope

This eval measures the judge's entry of judgment under Rule 58.  The first version targets `enter_judgment` after an eligible post-verdict posture, because the current Lean opportunity exposes judgment entry only after a jury verdict or after a bench opinion.  Rows where judgment is unavailable, including hung jury and pre-verdict states, should be tested separately as opportunity-absence or transition-validation cases.

The eval constructs either a jury post-verdict state with `jury_verdict` set or a bench post-verdict state with a `Bench Opinion` docket entry and `monetary_judgment` already in state.  It obtains the real Lean judge opportunity, sends the production prompt or an eval-local prompt candidate through the model path, accepts the returned tool decision through `apply_decision`, and then executes the resulting action through Lean `step`.  Each result records the fixture, constructed state, role view, opportunity, prompt input, raw response, extracted payload, applied judgment state, deterministic score, and Lean acceptance.

## Fixture Set

The first fixture file contains 16 rows across three difficulty tiers.  Nine rows use jury verdicts, and seven rows use bench opinions.  The set checks plaintiff money judgments, defense judgments, zero-dollar plaintiff verdicts, limited damages, larger awards, authentication, agency, notice, and bench opinions that reject excluded or unsupported proof.

| Theme | Scored Boundary |
|---|---|
| Jury plaintiff verdict | Judgment follows the jury verdict and uses the verdict damages |
| Jury defense verdict | Judgment enters for defendant with zero monetary judgment |
| Jury zero-dollar plaintiff verdict | Judgment enters from the verdict without inventing damages |
| Bench plaintiff opinion | Judgment follows the bench opinion and the state amount |
| Bench defense opinion | Judgment enters for defendant with zero monetary judgment |
| Limited damages | Judgment preserves the limited amount already determined |
| Record finality | Judgment entry does not relitigate liability or damages |

## Scoring

The scorer requires exactly one `enter_judgment` tool call with a nonempty `claim_id` and `basis`.  It checks claim id, basis concepts, prohibited basis concepts, reason tags, Lean opportunity acceptance, Lean step execution, final case status, and final monetary judgment.  The final amount is scored from the post-step state, because the Rule 58 payload does not carry an amount field.

The summary reports total accuracy, weighted accuracy, invalid rate, Lean decision rejection, Lean step rejection, claim correctness, basis correctness, final status correctness, final amount correctness, and slices by trial mode, issue family, tier, and reason tag.  This eval intentionally uses state-derived amount scoring rather than model-derived amount scoring.  That choice matches the engine's Rule 58 design: judgment entry follows the already determined verdict or bench result.

## Prompt Iteration

Candidate v1 gives the judge explicit Rule 58 constraints: use the current opportunity's claim id and basis, enter judgment once, and leave liability, damages, default, settlement, and post-judgment relief alone.  It exists to measure whether judgment-entry guidance improves schema discipline.  Measured results are in [Rule 58 Analysis](analysis.md).

## Next Extensions

The next judgment eval should cover Rule 59 and Rule 60 rather than adding more easy Rule 58 rows.  Useful Rule 58 extensions include multiple claims, partial judgment, judgment after Rule 68 acceptance, and default judgment under Rule 55.  Those extensions need different tools or richer state than this first `enter_judgment` eval.
