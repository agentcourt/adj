import Proofs.Samples

open ArbProofs

/-
This file proves the first facts that the rest of the arbitration engine relies
on.

Initialization is the point where a draft state becomes a live case.  If that
step does not enforce the policy correctly, the rest of the procedure is not
well-defined.  The theorems in this file therefore answer four direct
questions.

First, does the policy validator reject an impossible vote threshold?

Second, does initialization reject a council list whose size disagrees with the
policy?

Third, when initialization succeeds, does it open an active case, reset the
round counter, and store the proposition?

Fourth, does it reseat the whole council instead of preserving stale input
statuses?

These questions are narrow, but they are foundational.  Later proofs about
phase order and deliberation assume exactly these initialization effects.
-/

/--
The policy validator rejects a decision threshold that exceeds the council
size.

This is the smallest arithmetic sanity check that the policy must enforce.  If
the threshold can exceed the number of council seats, the engine could begin in
a state that can never produce a decision.

The proof uses one explicit policy with three seats and a four-vote threshold.
That sample is enough because the theorem is about the exact rejection message
for this exact invalid policy.
-/
theorem validatePolicy_rejects_threshold_above_council_size :
    policyErrorMessage (validatePolicy invalidThresholdPolicy) =
      "policy.required_votes_for_decision exceeds council_size" := by
  native_decide

/--
The policy validator rejects a decision threshold that is not a strict
majority of the council.

This arithmetic condition rules out the only policy shapes under which both
substantive outcomes can simultaneously meet the configured threshold.  The
neutrality theorem for vote-flipping depends on that exclusion.
-/
theorem validatePolicy_rejects_non_strict_majority_threshold :
    policyErrorMessage (validatePolicy nonStrictMajorityPolicy) =
      "policy.required_votes_for_decision must be a strict majority of council_size" := by
  native_decide

/--
Initialization rejects a council list whose length does not match the policy.

This theorem proves that council size is not treated as a loose hint.  The
engine checks the supplied list against `policy.council_size` before it opens
the case.

The proof changes only one fact from the standard initialization request: it
supplies two members where the policy requires three.  Everything else remains
valid, so the failure is about council length and nothing else.
-/
theorem initializeCase_requires_matching_council_size :
    initErrorMessage
      (initializeCase
        { initRequest with council_members := mixedStatusCouncil.take 2 }) =
      "council_members length does not match policy.council_size" := by
  native_decide

/--
Initialization rejects an empty council member identifier.

Council-member actions use `member_id` as the identity key.  An empty key
cannot identify a council seat, so initialization rejects the case before any
live state exists.
-/
theorem initializeCase_rejects_empty_member_id :
    initErrorMessage
      (initializeCase
        { initRequest with
            council_members :=
              [ sampleMember "" "m1" "p1" "seated"
              , sampleMember "C2" "m2" "p2" "seated"
              , sampleMember "C3" "m3" "p3" "seated"
              ] }) =
      "council_members contain empty or untrimmed member_id" := by
  native_decide

/--
Initialization rejects a council member identifier with surrounding space.

Runtime actions trim submitted member identifiers before lookup.  Initialization
therefore requires stored identifiers to be canonical, so a listed council seat
can be reached by the same identifier later.
-/
theorem initializeCase_rejects_untrimmed_member_id :
    initErrorMessage
      (initializeCase
        { initRequest with
            council_members :=
              [ sampleMember " C1 " "m1" "p1" "seated"
              , sampleMember "C2" "m2" "p2" "seated"
              , sampleMember "C3" "m3" "p3" "seated"
              ] }) =
      "council_members contain empty or untrimmed member_id" := by
  native_decide

/--
Initialization rejects duplicate council member identifiers.

The engine uses `member_id` as the identity key for voting, removal, and
opportunity selection.  Initialization therefore has to reject a council list
that reuses the same identifier for multiple seats.
-/
theorem initializeCase_rejects_duplicate_member_ids :
    initErrorMessage
      (initializeCase
        { initRequest with
            council_members :=
              [ sampleMember "C1" "m1" "p1" "timed_out"
              , sampleMember "C1" "m2" "p2" "seated"
              , sampleMember "C3" "m3" "p3" "excused"
              ] }) =
      "council_members contain duplicate member_id" := by
  native_decide

/--
Successful initialization sets the live starting point of the case.

The engine should do more than return success.  It should produce a specific
starting state: the case is active, the phase is `openings`, the deliberation
round is `1`, the proposition is stored, and `state_version` increases by one.

The proof checks exactly those fields on the standard sample request.  That is
enough for the later phase and deliberation proofs, which use the initialized
sample state as their starting point.
-/
theorem initializeCase_sets_core_case_fields :
    stateStatus (initializeCase initRequest) = "active" ∧
      statePhase (initializeCase initRequest) = "openings" ∧
      stateRound (initializeCase initRequest) = 1 ∧
      stateVersion (initializeCase initRequest) = baseState.state_version + 1 ∧
      stateProposition (initializeCase initRequest) = initRequest.proposition := by
  native_decide

/--
Successful initialization reseats every council member and clears any prior
resolution.

This theorem addresses a subtle but important point.  The input council list in
the sample request contains mixed statuses: one timed out, one seated, one
excused.  Initialization should not preserve those statuses in the live case.
It should normalize the case to a newly seated council.

The theorem therefore checks three things together: the council size is the
policy size, every status becomes `seated`, and the resolution field is empty.
Those three facts capture the meaning of a fresh live case.
-/
theorem initializeCase_reseats_every_council_member :
    stateCouncilSize (initializeCase initRequest) = samplePolicy.council_size ∧
      stateAllCouncilStatusesAre "seated" (initializeCase initRequest) = true ∧
      stateResolution (initializeCase initRequest) = "" := by
  native_decide
