import Proofs.Samples

open ArbdProofs

theorem validatePolicy_requires_judgment_standard :
    policyErrorMessage (validatePolicy invalidBlankStandardPolicy) =
      "policy.judgment_standard is required" := by
  native_decide

theorem initializeCase_requires_matching_council_size :
    initErrorMessage
      (initializeCase
        { initRequest with council_members := mixedStatusCouncil.take 2 }) =
      "council_members length does not match policy.council_size" := by
  native_decide

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

theorem initializeCase_sets_core_case_fields :
    stateStatus (initializeCase initRequest) = "active" ∧
      statePhase (initializeCase initRequest) = "openings" ∧
      stateRound (initializeCase initRequest) = 1 ∧
      stateVersion (initializeCase initRequest) = baseState.state_version + 1 ∧
      stateQuestion (initializeCase initRequest) = initRequest.question := by
  native_decide

theorem initializeCase_reseats_every_council_member :
    stateCouncilSize (initializeCase initRequest) = samplePolicy.council_size ∧
      stateAllCouncilStatusesAre "seated" (initializeCase initRequest) = true ∧
      stateAnswerCount (initializeCase initRequest) = 0 := by
  native_decide
