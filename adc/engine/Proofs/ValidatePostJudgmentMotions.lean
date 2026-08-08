import Main

theorem validateRule59Timing_of_isRule59Timely_error
    (judgmentDate filedAt : String)
    (msg : String)
    (h : isRule59Timely judgmentDate filedAt = .error msg) :
    validateRule59Timing judgmentDate filedAt = .error msg := by
  unfold validateRule59Timing
  simp [h]

theorem validateRule59Timing_of_isRule59Timely_true
    (judgmentDate filedAt : String)
    (h : isRule59Timely judgmentDate filedAt = .ok true) :
    validateRule59Timing judgmentDate filedAt = .ok () := by
  unfold validateRule59Timing
  simp [h]

theorem validateRule59Timing_of_isRule59Timely_false
    (judgmentDate filedAt : String)
    (h : isRule59Timely judgmentDate filedAt = .ok false) :
    validateRule59Timing judgmentDate filedAt = .error "rule 59 motion is untimely" := by
  unfold validateRule59Timing
  simp [h]

theorem validateRule60Timing_of_isRule60Timely_error
    (judgmentDate filedAt ground : String)
    (msg : String)
    (h : isRule60Timely judgmentDate filedAt ground = .error msg) :
    validateRule60Timing judgmentDate filedAt ground = .error msg := by
  unfold validateRule60Timing
  simp [h]

theorem validateRule60Timing_of_isRule60Timely_true
    (judgmentDate filedAt ground : String)
    (h : isRule60Timely judgmentDate filedAt ground = .ok true) :
    validateRule60Timing judgmentDate filedAt ground = .ok () := by
  unfold validateRule60Timing
  simp [h]

theorem validateRule60Timing_of_isRule60Timely_false
    (judgmentDate filedAt ground : String)
    (h : isRule60Timely judgmentDate filedAt ground = .ok false) :
    validateRule60Timing judgmentDate filedAt ground = .error "rule 60(b)(1)-(3) motion is untimely" := by
  unfold validateRule60Timing
  simp [h]

theorem validateRule59Timing_ok_implies_isRule59Timely_true
    (judgmentDate filedAt : String)
    (hOk : validateRule59Timing judgmentDate filedAt = .ok ()) :
    isRule59Timely judgmentDate filedAt = .ok true := by
  unfold validateRule59Timing at hOk
  cases hTimely : isRule59Timely judgmentDate filedAt with
  | error msg =>
      simp [hTimely] at hOk
  | ok timely =>
      cases timely <;> simp [hTimely] at hOk ⊢

theorem validateRule60Timing_ok_implies_isRule60Timely_true
    (judgmentDate filedAt ground : String)
    (hOk : validateRule60Timing judgmentDate filedAt ground = .ok ()) :
    isRule60Timely judgmentDate filedAt ground = .ok true := by
  unfold validateRule60Timing at hOk
  cases hTimely : isRule60Timely judgmentDate filedAt ground with
  | error msg =>
      simp [hTimely] at hOk
  | ok timely =>
      cases timely <;> simp [hTimely] at hOk ⊢
