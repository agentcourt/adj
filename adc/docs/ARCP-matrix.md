# ARCP Implementation Matrix

This matrix maps the normative [Agent Rules of Civil Procedure (ARCP)](ARCP.md) to current engine behavior, with [Federal Rules of Civil Procedure (FRCP): U.S. Courts](https://www.uscourts.gov/rules-policies/current-rules-practice-procedure/federal-rules-civil-procedure) as the external baseline.

Status labels:

- `Done`: explicit executable support exists.
- `Partial`: some executable support exists, but coverage is limited.
- `Gap`: no direct executable support.
- `Abrogated`: matches FRCP/ARCP abrogated status.

Roadmap labels:

- `Soon`: near-term roadmap.
- `Later`: distant roadmap.
- `TBD`: likely not addressed anytime soon.

## Coverage summary

The current matrix marks 95 rules or rule groups.  Coverage is uneven and still early in many FRCP areas.

| Status | Count |
|---|---:|
| Done | 16 |
| Partial | 24 |
| Gap | 51 |
| Abrogated | 4 |

Most open work remains in discovery depth, deposition stack, jury-instruction workflows, and post-judgment procedural detail.

| Rule | Status | Roadmap | Implementation notes |
|---|---|---|---|
| 1 | Partial | Later | System intent and procedural scope are present, but Rule 1 is not separately enforced as an executable check. |
| 2 | Partial | Later | Single civil action model exists; no separate executable Rule 2 gate. |
| 3 | Done |  | `file_complaint` action. |
| 4 | Gap | Later | No summons workflow. |
| 4.1 | Gap | TBD | No separate process-serving workflow. |
| 5 | Partial | Soon | Filing/docketing exists; full service mechanics are not implemented. |
| 5.1 | Gap | TBD | No constitutional-challenge notice/certification flow. |
| 6 | Partial | Soon | Time-window logic exists for Rule 59/60 timing; general extension/time-paper regime is incomplete. |
| 7 | Partial | Soon | Pleadings/motions actions exist; full Rule 7 form regime is not complete. |
| 7.1 | Gap | TBD | No disclosure-statement workflow under Rule 7.1. |
| 8 | Partial | Soon | Complaint/answer structure exists, but full pleading doctrine is not modeled. |
| 9 | Gap | TBD | Special pleading matters are not modeled. |
| 10 | Partial | Later | Structured payload fields exist, but no full pleading-format enforcement. |
| 11 | Done |  | Safe-harbor notice, withdrawal/correction, motion, and sanctions flow. |
| 12 | Done |  | Motion, opposition, reply, and judicial decision path. |
| 13 | Gap | TBD | Counterclaims/crossclaims not modeled. |
| 14 | Gap | TBD | Third-party practice not modeled. |
| 15 | Partial | Soon | `file_amended_complaint` exists; supplemental pleadings are not complete. |
| 16 | Partial | Soon | Pretrial management exists via phases and mode resolution; not full scheduling-order practice. |
| 17 | Gap | TBD | Capacity/real-party logic not modeled. |
| 18 | Gap | TBD | Joinder of claims not modeled. |
| 19 | Gap | TBD | Required joinder not modeled. |
| 20 | Gap | TBD | Permissive joinder not modeled. |
| 21 | Gap | TBD | Misjoinder/nonjoinder remedies not modeled. |
| 22 | Gap | TBD | Interpleader not modeled. |
| 23 | Gap | TBD | Class actions not modeled. |
| 23.1 | Gap | TBD | Derivative actions not modeled. |
| 23.2 | Gap | TBD | Unincorporated association actions not modeled. |
| 24 | Gap | TBD | Intervention not modeled. |
| 25 | Gap | TBD | Substitution of parties not modeled. |
| 26 | Partial | Soon | Initial disclosures and local-rule discovery limits exist; full Rule 26 suite is incomplete. |
| 27 | Gap | Later | Depositions to perpetuate testimony not modeled. |
| 28 | Gap | Later | Deposition officer rules not modeled. |
| 29 | Gap | Later | Stipulated discovery-procedure variation not modeled. |
| 30 | Gap | Later | Oral depositions not modeled. |
| 31 | Gap | Later | Written-question depositions not modeled. |
| 32 | Gap | Later | Deposition-use rules not modeled. |
| 33 | Done |  | Interrogatories service/response flow with limits. |
| 34 | Done |  | Requests for production service/response flow with limits. |
| 35 | Gap | TBD | Physical/mental examinations not modeled. |
| 36 | Done |  | Requests for admission service/response flow with limits. |
| 37 | Done |  | Discovery dispute motion and sanctions decision flow. |
| 38 | Done |  | Jury demand tracking. |
| 39 | Done |  | Jury/bench trial mode resolution. |
| 40 | Partial | Later | Trial progression exists via phase advancement; no full calendaring layer. |
| 41 | Partial | Soon | Settlement path exists; full dismissal mechanisms are incomplete. |
| 42 | Gap | TBD | Consolidation/separate-trial procedure not modeled. |
| 43 | Partial | Soon | Trial presentations and argument exist; witness/testimony regime is limited. |
| 44 | Gap | TBD | Official-record proof rules not modeled. |
| 44.1 | Gap | TBD | Foreign-law determination not modeled. |
| 45 | Gap | TBD | Subpoenas not modeled. |
| 46 | Partial | Later | `object_to_evidence` action exists; full preservation/error practice is limited. |
| 47 | Done |  | Voir dire questions, cause challenges, peremptory strikes, jury swearing. |
| 48 | Done |  | Jury configuration, verdict votes, failed-juror threshold adjustment, polling, hung declaration. |
| 49 | Partial | Soon | General verdict with interrogatories action exists; full Rule 49 practice is limited. |
| 50 | Gap | Later | JMOL/new-trial conditional structure not modeled. |
| 51 | Gap | Soon | Jury instruction/objection package not modeled. |
| 52 | Partial | Soon | Bench findings/conclusions/opinion exist; full Rule 52 practice is limited. |
| 53 | Gap | TBD | Masters not modeled. |
| 54 | Partial | Later | Judgment evidence exist; costs/multi-claim judgment details are limited. |
| 55 | Done |  | Default entry and default judgment. |
| 56 | Done |  | Motion, opposition, reply, and judicial decision path. |
| 57 | Gap | TBD | Declaratory judgment process not modeled separately. |
| 58 | Done |  | `enter_judgment` action with trial-path validation gates. |
| 59 | Partial | Later | Motion filing and timeliness validation exist; full adjudication/remedy logic is limited. |
| 60 | Partial | Later | Motion filing/timeliness and resolution path exist; full doctrinal detail is limited. |
| 61 | Gap | TBD | Harmless-error doctrine not modeled. |
| 62 | Gap | Later | Stay pending enforcement not modeled. |
| 63 | Gap | TBD | Judge inability/substitution procedure not modeled. |
| 64 | Gap | TBD | Seizure remedies not modeled. |
| 65 | Gap | TBD | Injunction/TRO procedures not modeled. |
| 65.1 | Gap | TBD | Surety proceedings not modeled. |
| 66 | Gap | TBD | Receivers not modeled. |
| 67 | Gap | TBD | Deposit into court not modeled. |
| 68 | Gap | TBD | Offer-of-judgment procedure not modeled. |
| 69 | Gap | TBD | Execution procedure not modeled. |
| 70 | Gap | TBD | Specific-act enforcement not modeled. |
| 71 | Gap | TBD | Nonparty enforcement not modeled. |
| 71.1 | Gap | TBD | Condemnation proceedings not modeled. |
| 72 | Gap | TBD | Magistrate pretrial order workflow not modeled. |
| 73 | Gap | TBD | Magistrate consent trial/appeal workflow not modeled. |
| 74 | Abrogated |  | Matches FRCP status. |
| 75 | Abrogated |  | Matches FRCP status. |
| 76 | Abrogated |  | Matches FRCP status. |
| 77 | Partial | Later | Clerk/judge operational role exists; formal Rule 77 clerk-business detail is incomplete. |
| 78 | Partial | Later | Motion practice supports brief-based decision flow; full hearing/submission regime is limited. |
| 79 | Done |  | Docket/record persistence plus PACER-style listing/fetch. |
| 80 | Partial | Later | Transcript generation exists; evidentiary Rule 80 framing is limited. |
| 81 | Gap | TBD | Special applicability/removal treatment not modeled. |
| 82 | Gap | TBD | Jurisdiction/venue boundaries not modeled. |
| 83 | Done |  | Local-rule limits and judge overrides. |
| 84 | Abrogated |  | Matches FRCP status. |
| 85 | Partial |  | Title present as normative text; no executable behavior. |
| 86 | Gap | TBD | Effective-date transition regime not modeled. |
| 87 | Partial | Soon | Agent-specific judicial directives exist in spirit (local overrides/orders), not as a dedicated Rule 87 engine. |

## Priority gaps for ARCP fidelity

High-value missing areas for near-term ARCP completeness:

1. Rule 51 jury-instruction workflow.
2. Rule 26(e) supplementation and integration with Rule 37 consequences.
3. Protective-order enforcement in discovery/PACER visibility.
4. Deposition stack (Rules 30-32) if discovery realism is a near-term goal.

## References

- [Agent Rules of Civil Procedure (ARCP)](ARCP.md)
- [Federal Rules of Civil Procedure (FRCP): U.S. Courts](https://www.uscourts.gov/rules-policies/current-rules-practice-procedure/federal-rules-civil-procedure)
