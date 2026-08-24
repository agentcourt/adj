# Rule 12 Evaluation Analysis

## Evaluation Coverage

The evaluation exercises `decide_rule12_motion` across eighteen filed-stage states.  Ten fixtures expect dismissal, and eight expect denial.  The grounds comprise seven failure-to-state-a-claim rows, three subject-matter-jurisdiction rows, four standing rows, two ripeness rows, and two mootness rows.

## Decision Boundaries

The pleading-sufficiency fixtures distinguish an omitted or conclusory element from a factual dispute about a well-pleaded allegation.  The jurisdiction and standing fixtures distinguish absent jurisdictional or standing facts from allegations sufficient to proceed.  The ripeness and mootness fixtures test whether the complaint presents a live dispute and whether a defect can be cured by amendment.

## Amendment and Prejudice

The fixture labels treat curable claim, jurisdiction, standing, and ripeness defects as dismissals with leave to amend.  A failure-to-state-a-claim fixture uses dismissal with prejudice when the plaintiff disclaims facts that could supply the missing element.  A mootness fixture denies leave when the complaint admits complete satisfaction before filing and seeks no remaining relief.

## Payload and Scoring

Each fixture constructs ADC state, obtains the Lean judge opportunity, executes it through the opportunity runner, and applies the resulting decision through Lean.  The scorer requires one `decide_rule12_motion` payload with motion index zero, the filed ground, a recognized disposition, and nonempty reasoning.  It compares disposition, `with_prejudice`, and `leave_to_amend`, then checks missing elements, rejected jurisdictional basis, or missing standing components when the ground requires those fields.

Outcome correctness also requires successful Lean application and an accepted runner step.  Explanation scoring uses deterministic phrase matching against each fixture's reason tags.  The summary reports accuracy, severity-weighted accuracy, invalid responses, false dismissals, false denials, posture mismatches, and slices by reason tag, issue family, ground, and tier.

## Prompt Templates

The [prompt directory](prompts/) contains two opportunity templates.  Both preserve the pleading-stage standard and specify the ground-dependent payload fields.  `candidate-v1.md` uses a general curability standard, while `candidate-v2.md` names curable claim, jurisdiction, standing, and ripeness defects, denies leave after admitted complete prefiling satisfaction, and restricts dismissal with prejudice to a plaintiff's disclaimer of curative facts.  The evaluator substitutes the production objective and fixture text before using the selected template as the opportunity objective.
