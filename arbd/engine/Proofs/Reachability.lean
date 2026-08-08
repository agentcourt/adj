import Main

namespace ArbdProofs

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

theorem stepReachableFrom_reachable
    (start target : ArbitrationState)
    (hStart : Reachable start)
    (hRun : StepReachableFrom start target) :
    Reachable target := by
  induction hRun with
  | refl =>
      exact hStart
  | step s t action _ hStep ih =>
      exact Reachable.step s t action ih hStep

end ArbdProofs
