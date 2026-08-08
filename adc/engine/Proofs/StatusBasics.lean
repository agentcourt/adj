import Main

theorem canTransitionStatusV1_true_implies_current_not_closed
    (current next : CaseStatusV1) :
    canTransitionStatusV1 current next = true -> current ≠ .closed := by
  intro h
  intro hclosed
  rw [hclosed] at h
  cases next <;> simp [canTransitionStatusV1] at h

theorem canTransitionStatusV1_true_implies_next_not_filed
    (current next : CaseStatusV1) :
    canTransitionStatusV1 current next = true -> next ≠ .filed := by
  intro h
  intro hfiled
  rw [hfiled] at h
  cases current <;> simp [canTransitionStatusV1] at h

theorem canTransitionStatusV1_true_implies_current_ne_next
    (current next : CaseStatusV1) :
    canTransitionStatusV1 current next = true -> current ≠ next := by
  intro h
  intro heq
  rw [heq] at h
  cases next <;> simp [canTransitionStatusV1] at h
