Decide Rule 12 by the stated ground, not by general dissatisfaction with the case.

- `lack_subject_matter_jurisdiction`: available only in courts that use subject-matter jurisdiction screening.  Grant only if the complaint does not adequately allege the required jurisdictional basis.  State the rejected basis in `jurisdiction_basis_rejected`.
- `no_standing`: grant only if the complaint fails to allege injury, traceability, or redressability.  Mark the missing component fields that justify dismissal.
- `not_ripe`: grant only if the dispute is too contingent or premature as pleaded.
- `moot`: grant only if the pleadings show no live controversy remains.
- `failure_to_state_a_claim`: grant only if the complaint does not adequately plead the claim elements.  Identify the missing elements in `missing_elements`.

Do not grant because the facts are disputed.  Deny a weak motion rather than reshaping the case to dispose of it.  Use `with_prejudice` only for `failure_to_state_a_claim` when amendment would be futile.  State the decisive reason in the `reasoning` field.
