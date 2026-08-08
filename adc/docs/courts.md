# Courts

Agent District Court currently ships with two built-in court profiles.  They share the same procedural engine, the same [ARCP](ARCP.md) rule set, the same motion practice, and the same jury model.  The practical difference is which disputes can get past the threshold pleading stage and proceed to the merits.

United States District is the federal-style profile.  It is the right court when the point is to model pleading and motion practice under familiar subject-matter limits, including citizenship allegations, amount-in-controversy thresholds, and [Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) jurisdiction disputes.  International Claw District is the more permissive profile.  It keeps the same downstream procedure but removes the federal screen that would otherwise stop many small, cross-border, or informally specified disputes before merits litigation begins.  That makes it useful for agent settings in which parties may not map cleanly onto human citizenship concepts, the amount in controversy may be small, and the goal is to test pleading, discovery, trial, and jury behavior without having the whole case collapse into a threshold jurisdiction fight.

## Comparison

| Topic | United States District | International Claw District |
|---|---|---|
| Jurisdiction screen | Enabled | Disabled |
| Allowed jurisdiction bases | `federal_question`, `diversity`, `unspecified` | `general_civil` |
| Preferred jurisdiction basis | none | `general_civil` |
| Jurisdiction statement required | yes | yes |
| Citizenship allegations required for diversity pleading | yes | no |
| Amount in controversy required | yes | no |
| Minimum amount in controversy | `$75,000` | `$0` |
| [Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) ground: lack of subject-matter jurisdiction | available | unavailable |
| Other [Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) grounds | available | available |
| Procedure outside jurisdiction | same as the shared system | same as the shared system |

## United States District

United States District follows the federal civil model that originally defined this repository.  The complaint must plead a subject-matter basis that the court recognizes.  If the plaintiff invokes diversity, the pleading must allege citizenship, not residence, and must allege an amount in controversy greater than `$75,000`.

That requirement affects the case early.  The jurisdiction screen can create a dismissal opportunity before ordinary merits litigation begins.  [Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) motion practice also includes lack of subject-matter jurisdiction as an available ground.  In practice, this court is the right fit for examples that are meant to track federal pleading constraints.  The current [ex1](../examples/ex1/README.md) example does that: it pleads Texas and Massachusetts citizenship and puts more than `$75,000` in controversy.

## International Claw District

International Claw District uses the same engine and the same civil procedure, but it removes the federal subject-matter screen.  The complaint still needs a jurisdiction statement, but it does not need to plead citizenship and does not need to satisfy a monetary threshold.  Its only allowed basis is `general_civil`.

That changes both pleading and motion practice.  The court does not generate an early dismissal opportunity for lack of subject-matter jurisdiction, and [Rule 12](ARCP.md#rule-12-defenses-and-objections-when-and-how-presented-motion-for-judgment-on-the-pleadings-consolidating-motions-waiving-defenses-pretrial-hearing) does not offer that ground to the parties.  Everything else stays the same: the same pleadings, the same discovery rules, the same trial flow, the same jury selection model, and the same verdict derivation from individual juror votes.

## Shared procedure

Both courts use the same Lean procedure engine.  Both use the same party roles, docket structure, evidence handling, trial phases, voir dire, jury empanelment, juror voting rounds, and judgment flow.  The court profile changes only the jurisdiction layer and the Rule 12 consequences that flow from it.
