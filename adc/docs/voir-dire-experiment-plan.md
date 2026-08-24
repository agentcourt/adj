# Voir Dire Experiments, Plan 1

## Goal

Learn which voir dire questions tell an ADC lawyer something real
about a juror, and how to use the answers for strikes and argument.
We do not know yet what is learnable, so the first product is a map of
what varies among jurors at all.  The end product is a playbook: a
short list of judge-allowable questions with rules for reading the
answers, strike rules, and argument guidance, written as OpenClaw
attorney-instruction text and tested in full runs.

## Setting

A juror is a pool record: a model, its configuration (provider,
quantization, inference parameters), and a persona.  The current pool
is `common/data/personas/pool.jsonl` (25 records); the persona files
are in `common/etc/personas/persons/` (14).  The lawyer never sees any
of that.  The lawyer sees answers: eight standard questionnaire
answers per candidate, plus one oral question per side per candidate,
screened by the judge.  Each side gets one for-cause challenge and one
peremptory strike, the jury is 6 of 10 candidates, the verdict needs
all 6, deliberation is up to 3 written ballot rounds, and a hung jury
blocks judgment.

Two facts drive the design.  Answers come from the juror's own model,
so they carry information about model, configuration, and persona
together.  Under unanimity, one adverse juror hangs the jury, so
picking and striking jurors can decide the result more often than
wording arguments.

A probe costs one model call (`adc llm` takes a prompt and a persona
record).  Answers are noisy, so nothing is concluded from a single
sample.

## The Exploration Loop

The core of the plan is a loop, run many times, with a written log.

One cycle: pick a few pool members and a small set of questions.  Ask.
Read the answers side by side.  Write down what separated the members,
what every member answered alike, and what surprised us.  Then revise:
drop questions that separate nobody, sharpen wording that produced
mush, split questions that mixed two things, and add questions the
answers themselves suggested.  Run the next cycle with the revised
set, sometimes on new members, sometimes on the same members to see
whether an answer is stable.

Start with obvious questions (attitudes toward screenshots, expert
reports, damages, burden of proof) and expect most of them to die.
The list of behavioral dimensions is an output of many cycles, not an
input; early guesses about which traits predict decisions will be
wrong, and the log should record them being wrong.

When a question seems to separate members, check the separation
against behavior right away, inside the loop: give the separated
members a small vignette vote and see whether the answer difference
shows up as a vote or damages difference.  A question whose
separations never reach behavior dies too.

The log is the scientific record.  Each cycle records the questions
asked, the answers, the reading we took, the changes we made, and
why.  Conclusions cite cycles.  The loop stops when several cycles in
a row produce no new distinction.

Alongside the question work, and using the same loop form, probe
identity: refusal patterns, self-identification, knowledge cutoffs,
style habits.  The pool file says who each member is, so
identification accuracy is measurable.  Model-level identification is
probably easy; configuration-level identification is uncertain.

## Promotion and Confirmation

A question is promoted out of the loop when it has kept its separation
across several cycles, across wording changes, and across repeat
samples.  Promoted questions get formal scoring: their answers must
predict vignette behavior for members held out during the loop, they
must survive the judge's screening (estimated by resampling the
judge's ruling), they must add information beyond the eight standard
questionnaire answers, and they must pass an identity check: if a
question's predictive value disappears once you know which member is
answering, it recognizes the member instead of measuring a trait, and
it will fail on new jurors.  A question that fails scoring goes back
into the loop as information, not into the trash silently.

Confirmed questions then get checked against real votes.  Build a
scenario whose party and court turns are fixed payloads, so the case
walks from filing to deliberation at zero model calls with the trial
text under exact control, and only juror turns are live.  Four checks:
(a) cheap jurors against Pi jurors on identical content, since Pi
jurors have a tool loop; (b) two closings as different as the record
allows, paired votes per juror, because if closing text cannot move
votes, questions only inform strikes; (c) the question rules against
actual votes on a scripted case; (d) strike policies simulated from
the rules, then a few live seeded deliberations, including the defense
case where protecting one immovable juror forces a hung jury.

## Full Test

Write the surviving rules as attorney instructions.  Run full
`adc run` A/B on complaints the rules never saw: one side with and
without the playbook, fixed opponent.  Measure verdict rate and
damages; use work notes to tell strategy errors from writing errors.
This is the only expensive step and it runs last.

## Cost

Loop cycles are single model calls: a few members times a few
questions times repeats, per cycle.  The loop's cost control is its
own honesty: dead questions stop costing anything as soon as the log
says they are dead.  Harness deliberations cost only juror calls, a
few per vote.  The full test is sized by the effect sizes the earlier
work reports.

## Limits

Loop findings are hypotheses until the harness confirms a sample.
The interrogation format differs from the runtime format, where a
juror gets one packet and votes once.  The pool is small (25 records,
14 personas), so held-out validation is thin, and provider behavior
can drift after the pool snapshot (2026-06-18).  Rules must also hold
on new cases, which is why the full test uses unseen complaints.

## Open Questions

Is pool knowledge legitimate lawyer background, like a human lawyer's
knowledge of a venue, or an attack on juror anonymity
(`adc/docs/juries.md` treats juror provenance control as a design
goal)?  Can a scenario run combine fixed party turns with Pi jurors,
which check (a) needs?  Should the persona and complaint corpora grow
before formal scoring?  May policy settings (question budget, panel
size, rounds) vary in experiments, or do defaults hold throughout?
