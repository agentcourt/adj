import Proofs.NoStuck
import Proofs.DeliberationSummaryCore

namespace ArbProofs

/-
This file adds the reachable wrappers for the lower deliberation-summary core.

`Proofs.DeliberationSummaryCore` now carries the summary definition, the
case-level correspondence with `currentResolution?`, and the local council
arithmetic that does not depend on later reachability layers.  The remaining
theorems here are the genuinely reachable ones: vote-count bounds from
reachable council integrity, inherited positivity of the decision threshold,
and the seated-count bound from the fixed council roster.
-/

/--
Reachable states bound the summary's current-round vote count by the seated
council count.
-/
theorem reachable_deliberationSummary_vote_count_le_seated_count
    (s : ArbitrationState)
    (hs : Reachable s) :
    (deliberationSummary s).current_round_vote_count ≤
      (deliberationSummary s).seated_count := by
  have hUnique : councilIdsUnique s.case := reachable_councilIdsUnique s hs
  have hIntegrity : councilVoteIntegrity s.case := reachable_councilVoteIntegrity s hs
  have hLen :
      (currentRoundVoteIds s.case).length ≤ (seatedCouncilMemberIds s.case).length :=
    list_length_le_of_nodup_subset
      hIntegrity.1
      (seatedCouncilMemberIds_nodup s.case hUnique)
      (currentRoundVoteIds_subset_seatedCouncilMemberIds s.case hIntegrity)
  simpa [deliberationSummary, deliberationSummaryForCase, currentRoundVoteIds,
    seatedCouncilMemberIds, seatedCouncilMemberCount, councilMemberIds] using hLen

/--
Reachable states carry a positive decision threshold, and the summary inherits
that fact without changing its meaning.
-/
theorem reachable_deliberationSummary_required_votes_positive
    (s : ArbitrationState)
    (hs : Reachable s) :
    0 < (deliberationSummary s).required_votes := by
  simpa [deliberationSummary, deliberationSummaryForCase] using
    reachable_required_votes_positive s hs

/--
Reachable states bound the summary's seated count by the configured council
size.
-/
theorem reachable_deliberationSummary_seated_count_le_council_size
    (s : ArbitrationState)
    (hs : Reachable s) :
    (deliberationSummary s).seated_count ≤ s.policy.council_size := by
  simpa [deliberationSummary, deliberationSummaryForCase] using
    reachable_seatedCouncilMemberCount_le_councilSize s hs

end ArbProofs
