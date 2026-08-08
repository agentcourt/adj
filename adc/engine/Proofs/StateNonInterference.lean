import Main

theorem updateCase_preserves_schema_version
    (s : CourtState) (c : CaseState) :
    (updateCase s c).schema_version = s.schema_version := by
  simp [updateCase, clearPassedOpportunities, bumpStateVersion]

theorem updateCase_preserves_court_name
    (s : CourtState) (c : CaseState) :
    (updateCase s c).court_name = s.court_name := by
  simp [updateCase, clearPassedOpportunities, bumpStateVersion]

theorem updateCase_preserves_policy
    (s : CourtState) (c : CaseState) :
    (updateCase s c).policy = s.policy := by
  simp [updateCase, clearPassedOpportunities, bumpStateVersion]

theorem appendDocket_preserves_status
    (c : CaseState) (title desc : String) :
    (appendDocket c title desc).status = c.status := by
  simp [appendDocket]

theorem appendDocket_preserves_trial_mode
    (c : CaseState) (title desc : String) :
    (appendDocket c title desc).trial_mode = c.trial_mode := by
  simp [appendDocket]

theorem appendDocket_preserves_phase
    (c : CaseState) (title desc : String) :
    (appendDocket c title desc).phase = c.phase := by
  simp [appendDocket]

theorem appendDocket_preserves_jury_verdict
    (c : CaseState) (title desc : String) :
    (appendDocket c title desc).jury_verdict = c.jury_verdict := by
  simp [appendDocket]

theorem appendDocket_preserves_hung_jury
    (c : CaseState) (title desc : String) :
    (appendDocket c title desc).hung_jury = c.hung_jury := by
  simp [appendDocket]

theorem appendTrace_preserves_status
    (c : CaseState) (action outcome : String) (citations : List String) :
    (appendTrace c action outcome citations).status = c.status := by
  simp [appendTrace]

theorem appendTrace_preserves_trial_mode
    (c : CaseState) (action outcome : String) (citations : List String) :
    (appendTrace c action outcome citations).trial_mode = c.trial_mode := by
  simp [appendTrace]

theorem appendTrace_preserves_phase
    (c : CaseState) (action outcome : String) (citations : List String) :
    (appendTrace c action outcome citations).phase = c.phase := by
  simp [appendTrace]

theorem appendTrace_preserves_jury_verdict
    (c : CaseState) (action outcome : String) (citations : List String) :
    (appendTrace c action outcome citations).jury_verdict = c.jury_verdict := by
  simp [appendTrace]

theorem appendTrace_preserves_hung_jury
    (c : CaseState) (action outcome : String) (citations : List String) :
    (appendTrace c action outcome citations).hung_jury = c.hung_jury := by
  simp [appendTrace]
