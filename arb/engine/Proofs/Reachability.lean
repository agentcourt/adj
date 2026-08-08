import Main

namespace ArbProofs

/-
This file defines the shared language for global procedure theorems.

The first proof batch used concrete sample states.  The next batch needs a
notion of all states that the engine can actually produce.  That is the purpose
of `Reachable`.

`Reachable s` means that `s` arose from one successful initialization, followed
by zero or more successful `step` transitions.  The definition does not talk
about the Go runner.  It talks only about the Lean engine's own transition
functions.  That is the right level for the global invariants, because the
claims are about what the engine permits, not about one caller.

The same file also defines the sequence predicates used by the fairness
theorems.  The bilateral phases have a specific order: plaintiff first, then
defendant.  The optional phases have a specific side: plaintiff for rebuttal,
defendant for surrebuttal.  The sequence predicates say exactly that.
-/

inductive Reachable : ArbitrationState → Prop where
  | init (req : InitializeCaseRequest) (s : ArbitrationState)
      (h : initializeCase req = .ok s) : Reachable s
  | step (s t : ArbitrationState) (action : CourtAction)
      (hs : Reachable s)
      (h : step { state := s, action := action } = .ok t) : Reachable t

inductive StepReachableFrom (start : ArbitrationState) : ArbitrationState → Prop where
  | refl : StepReachableFrom start start
  | step (s t : ArbitrationState) (action : CourtAction)
      (hs : StepReachableFrom start s)
      (h : step { state := s, action := action } = .ok t) : StepReachableFrom start t

theorem stepCore_ok_of_step_ok
    (s t : ArbitrationState)
    (action : CourtAction)
    (hStep : step { state := s, action := action } = .ok t) :
    stepCore { state := s, action := action } = .ok t := by
  unfold step at hStep
  by_cases hClosed : s.case.status = "closed"
  · simp [hClosed] at hStep
  · by_cases hFailed : s.case.status = "failed"
    · simp [hFailed] at hStep
    · simpa [hClosed, hFailed] using hStep

def bilateralStarted (phase : String) : List Filing → Prop
  | [] => True
  | [p] => p.phase = phase ∧ p.role = "plaintiff"
  | _ => False

def bilateralComplete (phase : String) : List Filing → Prop
  | [p, d] => p.phase = phase ∧ p.role = "plaintiff" ∧ d.phase = phase ∧ d.role = "defendant"
  | _ => False

def plaintiffOptionalSequence (phase : String) : List Filing → Prop
  | [] => True
  | [p] => p.phase = phase ∧ p.role = "plaintiff"
  | _ => False

def defendantOptionalSequence (phase : String) : List Filing → Prop
  | [] => True
  | [d] => d.phase = phase ∧ d.role = "defendant"
  | _ => False

def phaseShape (c : ArbitrationCase) : Prop :=
  match c.phase with
  | "openings" =>
      bilateralStarted "openings" c.openings ∧
        c.arguments = [] ∧ c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = []
  | "arguments" =>
      bilateralComplete "openings" c.openings ∧
        bilateralStarted "arguments" c.arguments ∧
        c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = []
  | "rebuttals" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        c.rebuttals = [] ∧ c.surrebuttals = [] ∧ c.closings = []
  | "surrebuttals" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        c.surrebuttals = [] ∧ c.closings = []
  | "closings" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
        bilateralStarted "closings" c.closings
  | "deliberation" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
        bilateralComplete "closings" c.closings
  | "closed" =>
      bilateralComplete "openings" c.openings ∧
        bilateralComplete "arguments" c.arguments ∧
        plaintiffOptionalSequence "rebuttals" c.rebuttals ∧
        defendantOptionalSequence "surrebuttals" c.surrebuttals ∧
        bilateralComplete "closings" c.closings
  | _ => False

def filingCount (xs : List Filing) (role : String) : Nat :=
  (xs.filter (fun item => item.role = role)).length

def offeredCount (xs : List OfferedEvidence) (role : String) : Nat :=
  (xs.filter (fun item => item.role = role)).length

def reportCount (xs : List TechnicalReport) (role : String) : Nat :=
  (xs.filter (fun item => item.role = role)).length

def materialLimitsRespected (s : ArbitrationState) : Prop :=
  offeredCount s.case.offered_evidence "plaintiff" ≤ s.policy.max_exhibits_per_side ∧
    offeredCount s.case.offered_evidence "defendant" ≤ s.policy.max_exhibits_per_side ∧
    reportCount s.case.technical_reports "plaintiff" ≤ s.policy.max_reports_per_side ∧
    reportCount s.case.technical_reports "defendant" ≤ s.policy.max_reports_per_side

def proceduralParity (c : ArbitrationCase) : Prop :=
  filingCount c.openings "plaintiff" ≤ 1 ∧
    filingCount c.openings "defendant" ≤ 1 ∧
    filingCount c.openings "defendant" ≤ filingCount c.openings "plaintiff" ∧
    filingCount c.arguments "plaintiff" ≤ 1 ∧
    filingCount c.arguments "defendant" ≤ 1 ∧
    filingCount c.arguments "defendant" ≤ filingCount c.arguments "plaintiff" ∧
    filingCount c.closings "plaintiff" ≤ 1 ∧
    filingCount c.closings "defendant" ≤ 1 ∧
    filingCount c.closings "defendant" ≤ filingCount c.closings "plaintiff" ∧
    filingCount c.rebuttals "plaintiff" ≤ 1 ∧
    filingCount c.rebuttals "defendant" = 0 ∧
    filingCount c.surrebuttals "plaintiff" = 0 ∧
    filingCount c.surrebuttals "defendant" ≤ 1

/-
Some later theorems need to talk about what part of a case remains fixed once
initialization succeeds.

The proposition is the issue being adjudicated.  The policy is the governing
procedure for that run.  The council-member identifiers determine who the
decision makers are, even though later steps may update council-member status.

Those three components form the case frame.
-/

def councilMemberIds (members : List CouncilMember) : List String :=
  members.map (fun member => member.member_id)

def caseFrameMatches
    (proposition : String)
    (policy : ArbitrationPolicy)
    (memberIds : List String)
    (s : ArbitrationState) : Prop :=
  s.case.proposition = proposition ∧
    s.policy = policy ∧
    councilMemberIds s.case.council_members = memberIds

theorem filingCount_nil (role : String) :
    filingCount [] role = 0 := by
  simp [filingCount]

theorem offeredCount_nil (role : String) :
    offeredCount [] role = 0 := by
  simp [offeredCount]

theorem reportCount_nil (role : String) :
    reportCount [] role = 0 := by
  simp [reportCount]

theorem filingCount_single (phase role target : String) (text : String) :
    filingCount [{ phase := phase, role := role, text := text }] target =
      (if role = target then 1 else 0) := by
  by_cases h : role = target
  · simp [filingCount, h]
  · simp [filingCount, h]

theorem filingCount_append (xs ys : List Filing) (role : String) :
    filingCount (xs ++ ys) role = filingCount xs role + filingCount ys role := by
  simp [filingCount, List.filter_append]

theorem offeredCount_append (xs ys : List OfferedEvidence) (role : String) :
    offeredCount (xs ++ ys) role = offeredCount xs role + offeredCount ys role := by
  simp [offeredCount, List.filter_append]

theorem reportCount_append (xs ys : List TechnicalReport) (role : String) :
    reportCount (xs ++ ys) role = reportCount xs role + reportCount ys role := by
  simp [reportCount, List.filter_append]

/-
The engine code and the proof layer count by two different definitions.

The executable functions in `Main.lean` use `foldl`.  The proof predicates in
this directory use `filter` and `length`.  They measure the same quantity.
These lemmas let later proofs move between the executable guard logic and the
global invariants without carrying that translation by hand each time.
-/

theorem filingCountForRole_foldl
    (items : List Filing)
    (role : String)
    (acc : Nat) :
    List.foldl (fun total item => if item.role = role then total + 1 else total) acc items =
      acc + filingCount items role := by
  induction items generalizing acc with
  | nil =>
      simp [filingCount]
  | cons head tail ih =>
      by_cases hRole : head.role = role
      · simp [filingCount, hRole, ih, Nat.add_assoc, Nat.add_comm]
      · simp [filingCount, hRole, ih]

theorem filingCountForRole_eq_filingCount (items : List Filing) (role : String) :
    filingCountForRole items role = filingCount items role := by
  unfold filingCountForRole
  simpa using filingCountForRole_foldl items role 0

theorem offeredEvidenceCountForRole_foldl
    (items : List OfferedEvidence)
    (role : String)
    (acc : Nat) :
    List.foldl (fun total item => if item.role = role then total + 1 else total) acc items =
      acc + offeredCount items role := by
  induction items generalizing acc with
  | nil =>
      simp [offeredCount]
  | cons head tail ih =>
      by_cases hRole : head.role = role
      · simp [offeredCount, hRole, ih, Nat.add_assoc, Nat.add_comm]
      · simp [offeredCount, hRole, ih]

theorem offeredEvidenceCountForRole_eq_offeredCount
    (items : List OfferedEvidence)
    (role : String) :
    offeredEvidenceCountForRole items role = offeredCount items role := by
  unfold offeredEvidenceCountForRole
  simpa using offeredEvidenceCountForRole_foldl items role 0

theorem technicalReportCountForRole_foldl
    (items : List TechnicalReport)
    (role : String)
    (acc : Nat) :
    List.foldl (fun total item => if item.role = role then total + 1 else total) acc items =
      acc + reportCount items role := by
  induction items generalizing acc with
  | nil =>
      simp [reportCount]
  | cons head tail ih =>
      by_cases hRole : head.role = role
      · simp [reportCount, hRole, ih, Nat.add_assoc, Nat.add_comm]
      · simp [reportCount, hRole, ih]

theorem technicalReportCountForRole_eq_reportCount
    (items : List TechnicalReport)
    (role : String) :
    technicalReportCountForRole items role = reportCount items role := by
  unfold technicalReportCountForRole
  simpa using technicalReportCountForRole_foldl items role 0

theorem councilMemberIds_status_update
    (members : List CouncilMember)
    (memberId status : String) :
    councilMemberIds
      (members.map (fun member =>
        if member.member_id = memberId then
          { member with status := status }
        else
          member)) =
      councilMemberIds members := by
  unfold councilMemberIds
  induction members with
  | nil =>
      simp
  | cons head tail ih =>
      by_cases h : head.member_id = memberId
      · simp [h, ih]
      · simp [h, ih]

end ArbProofs
