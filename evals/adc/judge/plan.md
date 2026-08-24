# Judge Eval Plan

## Purpose

This plan extends the judge eval work beyond voir dire question screening.  The goal is to measure judge decisions that can terminate claims, alter the trial record, affect jury composition, or control the legal instructions that jurors receive.  The evals should use real ADC state, real Lean opportunities, real tool schemas, deterministic scoring where possible, and eval-local prompt variants when prompt iteration is needed.

The existing voir dire eval provides the implementation pattern.  It builds compact fixture rows, constructs a minimal ADC state for each row, obtains the current Lean opportunity, sends the judge prompt through the normal model path, validates the tool call, applies the decision through Lean, and writes JSONL plus summary reports.  New judge evals should reuse that shape unless the decision requires multi-turn setup.

## Priority Order

| Priority | Eval | Judge Tools | Primary Risk | Fixture Shape | Primary Score |
|---:|---|---|---|---|---|
| 1 | Rule 56 summary judgment | `decide_rule56_motion` | Granting judgment despite genuine factual dispute, credibility issue, or competing inference | Pretrial state with motion, opposition, and reply summaries | Disposition accuracy, false-grant rate |
| 2 | Rule 12 and jurisdiction dismissal | `decide_rule12_motion`, `dismiss_for_lack_of_subject_matter_jurisdiction` | Treating factual dispute as pleading failure, closing amendable cases, or missing jurisdiction defects | Filed or pretrial state with motion ground and complaint allegations | Disposition accuracy, closure correctness, amendment correctness |
| 3 | Jury instructions | `settle_jury_instructions`, `deliver_jury_instructions` | Giving argumentative, incomplete, burden-shifting, or evidence-contaminated instructions | Jury-charge state with proposed instructions and objections | Required instruction content, prohibited content, neutral wording |
| 4 | For-cause juror challenges | `decide_juror_for_cause_challenge` | Excusing jurors for lawful attitudes or retaining jurors who cannot follow law or record | Voir dire state with pending for-cause challenge and juror answers | Grant/deny accuracy, reason-tag match |
| 5 | Discovery sanctions | `decide_rule37_motion` | Awarding sanctions when opposition was justified or denying relief for clear discovery failure | Pretrial discovery state with request, response, motion, and opposition summary | Grant/deny accuracy, sanction correctness |
| 6 | Rule 11 sanctions | `decide_rule11_motion` | Punishing legitimate advocacy, ignoring safe-harbor limits, or imposing disproportionate sanctions | Filed or pretrial state with challenged filing, notice posture, motion, and correction record | Grant/deny accuracy, sanction-type correctness |
| 7 | Bench trial findings and conclusions | `add_bench_finding`, `add_bench_conclusion`, `file_bench_opinion` | Mixing facts and law, relying on unadmitted evidence, or omitting claim elements | Trial state with admitted evidence and single-claim metadata | Element coverage, evidence confinement, fact-law separation |
| 8 | Judgment and post-judgment relief | `enter_judgment`, `resolve_rule59_motion`, `resolve_rule60_motion` | Entering judgment inconsistent with verdict or granting post-judgment relief without a recognized ground | Post-verdict or judgment state with verdict, damages, and motion record | Judgment amount correctness, relief correctness |

## Shared Structure

Each eval should keep fixture rows readable and construct the full ADC state in Go.  The row should describe the legal posture, the current request, the expected tool payload, and accepted reason tags.  Complex rows can carry structured context blocks, but ordinary rows should remain compact enough for review without opening generated state JSON.

| Field | Purpose |
|---|---|
| `id` | Stable row identifier. |
| `tier` | Difficulty level, with tier 3 reserved for boundary and adversarial phrasing. |
| `case_theme` | Short factual setting for prompt context. |
| `posture` | Procedural posture needed to build the ADC state. |
| `moving_party` or `requesting_party` | Party associated with the pending request. |
| `issue_family` | Category used for summary slices. |
| `request_text` | Motion, objection, challenge, or proposed instruction text. |
| `opposition_text` | Opposing position when the decision requires one. |
| `expected_payload` | Expected fields for the judge tool call. |
| `expected_reason_tags` | Deterministic explanation tags accepted by the scorer. |
| `severity` | Weight for failures, with outcome-changing false grants weighted highest. |
| `context_notes` | Human-readable note explaining the fixture boundary. |

The runner should record the same artifacts for every judge eval.  Each result should include fixture metadata, constructed state, Lean view, Lean opportunity, prompt input, raw model response, extracted tool payload, scoring fields, and Lean acceptance.  Summary output should report total accuracy, weighted accuracy, invalid-response rate, false-positive rate, false-negative rate, and slices by issue family, party, tier, and reason tag.

## Directory Organization

Judge evals are organized by governing procedural rule and evaluated behavior under `evals/adc/judge/`.  The earlier ADC-local judge layout worked for the first Rule 47 eval, but it did not scale once Rule 56, Rule 12, Rule 51, sanctions, bench-trial, and post-judgment evals existed.  The top-level judge directory holds cross-rule planning and shared conventions, while rule-specific fixture sets, prompt candidates, and analysis documents live under a behavior directory.

The current structure is:

```text
evals/adc/judge/
  README.md
  plan.md
  rules/
    rule47/
      voir-dire-question/
        analysis.md
        fixtures.jsonl
        hard-fixtures.jsonl
        prompts/
      for-cause-challenge/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule56/
      summary-judgment/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule12/
      dismissal-jurisdiction/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule51/
      jury-instructions/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule52/
      bench-opinion/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule58/
      judgment-entry/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule60/
      relief-from-judgment/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule37/
      discovery-sanctions/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
    rule11/
      sanctions/
        analysis.md
        fixtures.jsonl
        plan.md
        prompts/
```

Rule 47 covers voir dire and jury selection in ADC's ARCP rules.  The voir dire question-screening fixtures, hard fixtures, analysis, and prompt candidates live under `rules/rule47/voir-dire-question/`.  The for-cause challenge eval also lives under Rule 47 because it tests whether a juror should be excused for good cause after voir dire answers.

These directories exist:

| Eval Area | Directory |
|---|---|
| Voir dire question screening | `rules/rule47/voir-dire-question/` |
| For-cause juror challenges | `rules/rule47/for-cause-challenge/` |
| Jury instructions | `rules/rule51/jury-instructions/` |
| Rule 56 summary judgment | `rules/rule56/summary-judgment/` |
| Rule 12 dismissal and jurisdiction | `rules/rule12/dismissal-jurisdiction/` |
| Rule 37 discovery sanctions | `rules/rule37/discovery-sanctions/` |
| Rule 11 sanctions | `rules/rule11/sanctions/` |
| Bench findings and conclusions | `rules/rule52/bench-opinion/` |
| Judgment entry | `rules/rule58/judgment-entry/` |
| Rule 60 relief from judgment | `rules/rule60/relief-from-judgment/` |

These areas have no directory and no fixtures.  Each would take the rule number of the decision it tests:

| Eval Area | Rule |
|---|---|
| Default and default judgment | Rule 55 |
| Post-trial relief | Rule 59 |
| Stays and bonds | Rule 62 |
| Protective orders | Rule 26 |

Some judge evals will cross rule boundaries.  Judgment and post-judgment relief should split into Rule 58, Rule 59, and Rule 60 once the fixtures become concrete.  ADC-specific judge powers that do not map cleanly to a federal rule should either use the closest practice rule, such as Rule 83 for local-rule overrides, or live under `evals/adc/judge/adc-specific/` when the eval mainly tests ADC policy rather than a procedural rule.

CLI defaults and documentation should follow these suite paths.  Generated report data should remain under ignored `evals/out/adc/judge/` directories.  Committed rule directories should contain fixtures, prompt candidates, plans, and analysis rather than generated report data.

## Rule 56 Summary Judgment

Rule 56 should be the next eval.  A false grant can terminate a claim before trial, and the judge prompt already warns against resolving disputed facts, credibility questions, and competing inferences.  The eval should test whether that warning works under concrete motion records rather than abstract instructions.

Fixture categories should include clean no-dispute grants, clean denials, element-specific failures, unsupported damages theories, credibility disputes, competing document interpretations, authentication disputes, and movant statements that sound strong but depend on inference.  The scorer should treat a false grant as more severe than a false denial because it removes trial access.  Explanation tags should include `no_genuine_dispute`, `missing_element`, `credibility_dispute`, `competing_inference`, `unsupported_damages`, `authentication_dispute`, and `movant_burden_not_met`.

The expected payload is small: `disposition` and `reasoning`.  If partial grants become common, rows should identify the claim element or issue resolved and the remaining issue.  The initial version can score `granted`, `denied`, and `partial` against fixture labels before adding finer-grained partial-judgment scoring.

## Rule 12 And Jurisdiction

Rule 12 should follow Rule 56 because it tests a different boundary: pleading sufficiency rather than evidentiary sufficiency.  The highest-risk error is closing a case because the judge disbelieves allegations or imports facts outside the pleading.  Jurisdiction dismissal belongs in the same eval family because ADC has both motion-driven Rule 12 dismissal and court-driven subject-matter jurisdiction screening.

Fixture categories should include missing element, conclusory allegation, factual allegation that must be accepted at the pleading stage, jurisdictional amount defect, missing citizenship allegation, no standing, traceability defect, redressability defect, and amendable pleading defect.  Scoring should cover disposition, ground, closure status, `leave_to_amend`, `with_prejudice`, and any required jurisdiction field.  False dismissal with prejudice should carry the highest weight.

The eval should include paired rows where the same factual theme appears at Rule 12 and Rule 56.  That pairing tests whether the judge applies the different procedural standard instead of using a general sense that the case is weak.  It also gives prompt iteration a clear target if the judge collapses pleading and evidence standards.

## Jury Instructions

Jury-instruction evals should test neutrality, completeness, and legal accuracy.  ADC gives parties `propose_jury_instruction` and `object_jury_instruction`, then the judge uses `settle_jury_instructions` and `deliver_jury_instructions`.  The eval should build a charge posture with party proposals and objections, then ask the judge to settle or deliver instructions.

Fixture categories should include burden shifting, missing element, argumentative phrasing, instruction that assumes disputed facts, limiting instruction, damages instruction, authentication instruction, adverse-inference request, and instruction based on excluded evidence.  The scorer should combine required-substring checks, prohibited-substring checks, and reason tags.  Model-graded instruction quality should wait for a separate decision because it would change the trust boundary.

The first version should score `settle_jury_instructions` summaries rather than full final charges.  Once the summary scorer works, a `deliver_jury_instructions` eval can check complete instruction text.  The delivered-charge eval should verify that the text does not include party names as credibility signals, lawyer argument, or statements that a particular exhibit proves an element.

## For-Cause Juror Challenges

For-cause challenge evaluation is the natural extension of voir dire question screening.  The existing eval decides whether a lawyer may ask a question; this eval decides whether the juror's answer requires removal.  The judge must distinguish lawful unfavorable attitudes from inability to follow instructions, fixed bias, or refusal to decide from the record.

Fixture categories should include expressed inability to be impartial, refusal to apply burden of proof, fixed damages floor or ceiling, inability to evaluate digital evidence, sympathy bias, financial hardship, language or attention limitation, rehabilitation by later answer, and strategic challenge based only on unfavorable lawful views.  Scoring should cover `granted`, `ruling_reason`, and whether the right `challenge_id` and `juror_id` were used.  False denials should receive high weight when the juror expressly refuses to follow law; false grants should receive high weight when the juror merely expresses a lawful concern that voir dire can address.

This eval can reuse most of the voir dire state builder.  The fixture setup differs because it needs an answered exchange and a pending `for_cause_challenges` entry.  The result analysis should report failures by juror-answer family rather than question family.

## Discovery Sanctions

Rule 37 decisions test procedural enforcement.  The judge must distinguish a real failure to respond from a justified objection, proportionality dispute, harmless delay, or motion that seeks fees without a proper predicate.  The payload also tests sanction constraints because `fees` require a positive amount and denied motions cannot include sanctions.

Fixture categories should include no response, evasive response, justified privilege objection, overbroad request, proportionality objection, late supplementation, failure to obey a discovery order, harmless delay, and fee-shifting without grant.  Scoring should cover grant or denial, sanction type, sanction amount, and order text.  Invalid payloads should be reported separately because sanction decisions have more schema-dependent failure modes than voir dire rulings.

The first Rule 37 eval should keep the discovery record synthetic but structured.  The state builder can create one discovery request set, one response summary, and one Rule 37 motion docket entry.  Later versions can use real discovery artifacts when the discovery system needs deeper testing.

## Rule 11 Sanctions

Rule 11 evals should test restraint as much as enforcement.  A judge that grants sanctions for weak but nonfrivolous advocacy creates a serious failure, while a judge that denies sanctions for plainly unsupported factual content misses a different enforcement function.  The eval should include safe-harbor posture because Rule 11 decision quality depends on whether the challenged filing was withdrawn or corrected.

Fixture categories should include frivolous legal contention, no evidentiary support, improper purpose, reasonable extension argument, factual contention likely to have support after discovery, corrected filing after safe-harbor notice, discovery filing wrongly challenged under Rule 11, and sanction proportionality.  Scoring should cover grant or denial, sanction type, sanction amount, sanction detail, and reasoning.  False grants against plausible advocacy should carry high severity.

The first version should avoid complicated monetary sanctions.  It can focus on `none`, `admonition`, `non_monetary_directive`, and clear fee-shift cases.  Monetary penalties can enter a hard set once the grant/deny boundary is stable.

## Bench Trial Findings And Conclusions

Bench-trial evals should test whether the judge writes findings from admitted evidence and conclusions from law.  The risk is not only wrong winner selection; the judge can also mix facts and law, cite unadmitted material, omit claim elements, or enter findings that conflict with the record.  This eval should be built after the motion and instruction evals because it requires richer trial-state construction.

Fixture categories should include admitted-document proof, conflicting testimony, missing causation, damages proof gap, credibility explanation, unadmitted exhibit reference, and conclusion that omits an element.  Scoring should check element coverage, admitted-evidence confinement, fact-law separation, and final judgment consistency.  The first version can score structured findings and conclusions before scoring full bench opinions.

The first implemented version scores `file_bench_opinion`, because that is the judge opportunity currently exposed at `status=trial`, `trial_mode=bench`, and `phase=verdict_return`.  It checks winner selection, amount, required element reasoning, prohibited reliance on excluded proof, fact-law-judgment separation, and Lean acceptance.  A later version can add a two-step runner for separate findings and conclusions if the Lean opportunity sequence exposes `add_bench_finding` and `add_bench_conclusion` before the filed opinion.

## Judgment And Post-Judgment Relief

Judgment and post-judgment evals should verify that the judge respects the existing state rather than relitigating the case.  `enter_judgment` should follow the verdict or bench findings and use the amount determined by state.  Rule 59 and Rule 60 decisions should test recognized grounds rather than general dissatisfaction with the result.

Fixture categories should include judgment after plaintiff verdict, judgment after defense verdict, damages mismatch, judgment before verdict, Rule 59 new-trial request based on weight of evidence, Rule 59 legal error, Rule 60 mistake, Rule 60 newly discovered evidence, untimely Rule 60 ground, and Rule 60 request that repeats trial arguments.  Scoring should cover tool choice, relief grant or denial, amount correctness, status transition, and reason tag.  False grants of post-judgment relief should receive high weight because they disturb a completed result.

This eval can use deterministic state fixtures more than free-form text.  Many judgment failures are state-consistency failures, and the Lean engine already enforces some of them.  The model eval should focus on whether the judge tries the correct tool and payload before Lean validation catches impossible moves.

## Remaining Work

| Work | Condition |
|---|---|
| A `deliver_jury_instructions` eval under `rules/rule51/`, scoring full charge text for structure, neutral wording, and consistency with the settled summary. | The `settle_jury_instructions` scorer holds up on harder fixtures. |
| A Rule 59 post-trial relief suite. | Lean exposes a grant-or-deny Rule 59 opportunity.  The path observed during planning is deterministic denial, which gives prompt iteration nothing to move. |
| Suites for default judgment, stays and bonds, and protective orders. | State builders exist for the postures those decisions need. |
| Harder Rule 60 fixtures. | Independent of the above; the current set does not separate production from candidate v1. |

## Suites

Each suite's `analysis.md` is the single record of what its runs measured, including which prompt candidate the evidence favors.  This table gives the fixture count and candidate count so the size of a suite is visible without opening it.

| Suite | Fixtures | Candidates | Results |
|---|---:|---:|---|
| [Rule 11 sanctions](rules/rule11/sanctions/README.md) | 16 | 2 | [Analysis](rules/rule11/sanctions/analysis.md) |
| [Rule 12 dismissal and jurisdiction](rules/rule12/dismissal-jurisdiction/README.md) | 18 | 2 | [Analysis](rules/rule12/dismissal-jurisdiction/analysis.md) |
| [Rule 37 discovery sanctions](rules/rule37/discovery-sanctions/README.md) | 16 | 2 | [Analysis](rules/rule37/discovery-sanctions/analysis.md) |
| [Rule 47 voir dire question screening](rules/rule47/voir-dire-question/README.md) | 60 plus 30 hard | 3 | [Analysis](rules/rule47/voir-dire-question/analysis.md) |
| [Rule 47 for-cause challenges](rules/rule47/for-cause-challenge/README.md) | 16 | 1 | [Analysis](rules/rule47/for-cause-challenge/analysis.md) |
| [Rule 51 jury instructions](rules/rule51/jury-instructions/README.md) | 16 | 1 | [Analysis](rules/rule51/jury-instructions/analysis.md) |
| [Rule 52 bench opinion](rules/rule52/bench-opinion/README.md) | 16 | 1 | [Analysis](rules/rule52/bench-opinion/analysis.md) |
| [Rule 56 summary judgment](rules/rule56/summary-judgment/README.md) | 30 | 2 | [Analysis](rules/rule56/summary-judgment/analysis.md) |
| [Rule 58 judgment entry](rules/rule58/judgment-entry/README.md) | 16 | 1 | [Analysis](rules/rule58/judgment-entry/analysis.md) |
| [Rule 60 relief from judgment](rules/rule60/relief-from-judgment/README.md) | 16 | 1 | [Analysis](rules/rule60/relief-from-judgment/analysis.md) |

## Reporting Rules

Generated reports should stay under `evals/out/adc/judge/`, which is ignored.  Committed materials should include fixture sets, prompt candidates, analysis documents, and runner code.  Each analysis document should state whether a candidate prompt beats production, preserves production behavior, or exposes a failure cluster that needs more fixtures.
