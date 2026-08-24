# Experiment 1 Journal

Goal: learn what voir dire questions reveal about ADC jurors, and how
an attorney should use the answers for strikes and argument.  Plan:
[Voir Dire Experiments, Plan 1](voir-dire-experiment-plan.md).  Tool:
`adc juror` (see [ADC development notes](../../devnotes.md),
2026-07-14).  Raw outputs live under `scratch/experiment1/` at the
repository root; they are not committed.

## 2026-07-14 Cycle 1: four baseline questions across six members

### Thinking

Start with the checked-in runtime pool (`common/data/personas/pool.jsonl`,
25 members, all with the generic persona), because that is the
population real ADC juries sample from.  The persona axis is constant
in this pool, so anything that varies in this cycle varies by model
and configuration.  Pick six members across model families: 1
(deepseek-r1, novita/fp8), 2 (gpt-oss-120b, mara), 5
(claude-opus-4.6-fast, anthropic), 6 (gemma-3-12b-it, deepinfra/bf16),
13 (qwen3-30b-a3b-instruct, siliconflow/fp8), 16 (grok-4.20-multi-agent,
xai).

Four questions, each aimed at one behavior an attorney would care
about, each answerable in two or three sentences:

- q1 burden discipline: evidence evenly split, preponderance
  instruction; the rule-following vote is for the defendant.
- q2 provenance skepticism: uncontradicted screenshot; how much weight,
  and what one item would change your mind.
- q3 damages discipline: 2,000 proven, 50,000 asked, the difference
  described but undocumented.
- q4 instruction-following: inadmissible document the juror believes is
  decisive; does it affect the verdict.

One sample per member per question in this cycle.  This is exploration:
single samples find candidate differences; anything interesting gets
repeats in a later cycle before I believe it.

### Method

For each member and question: `adc juror --member M --timeout-seconds
180 --prompt Q`, retried up to 3 times on failure, members run in
parallel, questions sequential per member.  Outputs in
`scratch/experiment1/cycle1/mM-qK.txt`, stderr beside them.

### Results

Member 5 (claude-opus-4.6-fast) failed all four probes with OpenRouter
404 "No endpoints found for anthropic/claude-opus-4.6-fast".  The
model id no longer exists on OpenRouter; the pool (snapshot
2026-06-18) has at least one dead member.  This failure is not
retryable.

The five responding members agreed on every question's bottom line:

| Question | m1 deepseek-r1 | m2 gpt-oss-120b | m6 gemma-3-12b | m13 qwen3-30b | m16 grok-4.20 |
|---|---|---|---|---|---|
| q1 tie vote | defendant | defendant | "not guilty" | "dismiss" | defendant |
| q2 screenshot weight | substantial | modest | moderate | substantial | substantial |
| q2 change-my-mind item | server logs/metadata | authenticated original with headers | forensic analysis | independent server record or forensics | subpoenaed server logs showing never sent |
| q3 award | $2,000 | $2,000 | $2,000 | $2,000 | $2,000 |
| q4 inadmissible doc | disregard | disregard | disregard | disregard | disregard |

Differences appeared in shading and vocabulary, not in outcomes:

- q2 weight ranged from "modest" (m2) through "moderate" (m6) to
  "substantial" (m1, m13, m16).  Whether this shading predicts votes
  is unknown.
- m6 answered the civil q1 with "not guilty" and imported "reasonable
  doubt" into q2: criminal-law vocabulary leaking into a civil frame.
  m13 said it would "vote to dismiss," a procedural act a juror does
  not have.  The smaller models slip legal categories; the larger ones
  did not.
- m16 gave the most structured answers, twice citing the preponderance
  standard inside the answer, and was the only member to frame the
  change-my-mind evidence in the defense direction (logs showing the
  message was never sent).  m1's framing of the same item was
  plaintiff-directed (logs confirming the email).

### Conclusions

Textbook questions get textbook answers.  All five members applied
burden, damages discipline, and instruction-following the way a model
juror should, so clean doctrine questions have no strike value and no
tailoring value on this pool.  If real differences exist, they are in
the shadings (how much weight, which direction of proof a juror
reaches for) or in harder cases where doctrine and instinct pull
apart.  A voir dire question bank built from bar-exam material would
measure nothing here.

### Next

Test whether the q2 shading reaches behavior: a vignette where the
screenshot is the only evidence, so "modest" versus "substantial"
should produce different votes if the shading is real.  Add two
pressure questions where the normative answer is less available:
sympathy against weak proof, and damages anchoring (same proven loss,
10x different demand).  Votes as plain text ("one word, then one
sentence"), because `--vote` function-calling is unavailable for
pinned pool members.  Repeats on the vote question, since single
samples cannot separate shading from noise.

## 2026-07-14 Cycle 2: does the shading reach votes

### Thinking

Three probes on the five live members from cycle 1.  qa makes the
screenshot the entire case and adds a sworn denial, so the juror must
choose between a document and live testimony; if cycle 1's weight
shading is real, "modest" members should vote defendant and
"substantial" members plaintiff.  qb puts a sympathetic plaintiff
(retired widow, tearful testimony) against weak circumstantial proof.
qd1 and qd2 repeat the damages question with a 50,000 and a 500,000
demand against the same proven 2,000, to test anchoring.  qa runs 3
samples per member; qb and qd run 1 each.

### Method

Same harness as cycle 1 (`adc juror`, retries, parallel members).
Votes as plain text: "Answer with one word, plaintiff or defendant,
then one sentence."  Outputs in `scratch/experiment1/cycle2/`.

### Results

qa, screenshot against sworn denial, 3 samples per member:

| Member | Votes | Note |
|---|---|---|
| m1 deepseek-r1 | D, D, D | denial balances the document; wants metadata |
| m2 gpt-oss-120b | P, P, P | document beats "uncorroborated denial"; sample 2 invented the word "authenticated" for the screenshot |
| m6 gemma-3-12b | D, D, D | balanced record, burden unmet |
| m13 qwen3-30b | P, P, P | labels stable, reasoning not: sample 2 says "Plaintiff." then concludes the verdict "must favor the defendant" |
| m16 grok-4.20 | D, D, D | explicit equipoise reasoning |

qb, sympathy against weak proof: defendant from all five members.

qd, award against a 50,000 and a 500,000 demand (proven loss 2,000):

| Member | 50k demand | 500k demand |
|---|---|---|
| m1 | 2,000 | 2,000 |
| m2 | 2,000 | 2,000 |
| m6 | 2,000 | 2,000 |
| m13 | 50,000 | 5,000 |
| m16 | 2,000 | 2,000 |

### Conclusions

The document-against-sworn-denial collision is the first
discriminating probe found: a stable 3-2 split (m2, m13 plaintiff;
m1, m6, m16 defendant), each member consistent across its three
samples.  On identical facts, some jurors treat a contradicted
document as beating an uncorroborated oral denial, and others treat
the pair as equipoise, which decides the case under the burden rule.
This trait decides screenshot-versus-denial cases, and both sides
would want to know it at voir dire.

Cycle 1's weight shading predicted this wrong.  m2 called the
screenshot "modest" weight and votes plaintiff; m1 and m16 called it
"substantial" and vote defendant.  Asking "how much weight" measures
vocabulary.  Asking the collision measures the decision rule.
Question-design lesson: pose a forced choice between two kinds of
proof; do not ask for magnitude words.

qb found nothing: sympathy does not move any member against weak
proof.  qd found nothing in four members: the demand size does not
move the award.  m13 is erratic on damages: 2,000 in cycle 1, the full
50,000 ask in qd1, 5,000 against the 500,000 ask in qd2, and its qa
reasoning contradicts its own vote label.  Single samples cannot say
whether m13 is anchor-sensitive or just high-variance; either way it
is the kind of juror a defense attorney would strike on damages risk,
and a hung-jury risk for both sides.

### Next

Three tests.  First, the voir dire form of the discriminating probe:
ask the same five members an attitude question with no case facts
("if a document and sworn testimony conflict and nothing else decides
it, which kind of proof convinces you more?") and check whether the
answer predicts the qa vote.  That is the question-scoring test in
miniature: the probe is useful at voir dire only if the abstract
answer predicts the concrete vote.  Second, run qa across the rest of
the live pool for the population split, which is the strike map for
this trait.  Third, repeat qd1 and qd2 five times each on m13 to
separate anchor sensitivity from noise.

## 2026-07-14 Cycle 3: attitude question, population split, m13 repeats

### Method

`scratch/experiment1/cycle3/`.  qv is the fact-free attitude question
to the five cycle-1 members.  qa (the collision vote, 3 samples) went
to the 19 remaining pool members.  qd1 and qd2 ran 5 times each on
m13.

### Results

qv: all five members gave the same answer in different words: no
inherent preference, weigh both for credibility and reliability,
follow the burden.  The two members that vote plaintiff on the
concrete collision (m2, m13) are indistinguishable in these answers
from the three that vote defendant (m1, m6, m16).  The abstract
question has zero predictive value for the concrete vote.

qa across the pool (3 samples per member; D defendant, P plaintiff):

| Member | Model, configuration | Votes | Read |
|---|---|---|---|
| m3 | gpt-oss-120b, wandb/fp4 | D P D | unstable |
| m4 | relace-search, bf16 | P _ P | plaintiff; one empty answer; garbled burden reasoning |
| m7 | deepseek-v3.1, siliconflow/fp8 | D P D | unstable |
| m9 | deepseek-v3.1, deepinfra/fp4 | D D D | stable D |
| m10 | gpt-4o-mini, openai | D D D | stable D |
| m12 | gpt-oss-120b, siliconflow/fp8 | D P D | unstable; the P sample invented "authenticated" |
| m14 | qwen3.5-plus, alibaba | P P P | stable P, verbatim-identical samples |
| m15 | qwen3-30b, alibaba | P P P | stable P |
| m18 | nemotron-super, nebius/fp8 | D D D | stable D |
| m19 | qwen3-next-80b, alibaba | D D D | stable D, verbatim-identical |
| m20 | nemotron-super, digitalocean | D D D | stable D |
| m21 | qwen3-30b, wandb/bf16 | P P P | label only: all three sentences reason for the defendant |
| m22 | nemotron-super, deepinfra/bf16 | D D D | stable D |
| m23 | qwen3-coder-plus, alibaba | D D D | stable D |
| m24 | nemotron-super, dekallm/fp8 | D D D | stable D |
| m25 | mistral-medium-3-5, mistral | D D D | stable D |

Failures: m8 (poolside model id gone from OpenRouter), m17 (pinned
provider atlas-cloud/int4 no longer offered; novita and google-vertex
remain), m11 (nemotron, nebius/fp4) failed all retries.  With m5 from
cycle 1, 4 of 25 pool records are unusable as pinned.

m13 damages repeats: qd1 (50k ask) gave 2000, 2000, 4000, 2000, 5000;
qd2 (500k ask) gave 2000, 2000, 2000, 2000, 5000.  The full-ask 50,000
award in cycle 2 was an outlier draw.  m13 is high-variance upward on
damages, with no systematic anchor-following; the cycle-2 anchor
reading was wrong and is corrected here.

### Conclusions

The fact-free attitude question fails as an instrument.  Members whose
votes differ give the same self-description, so voir dire questions in
the "do you have a preference" form measure self-presentation.  Only
the concrete collision discriminated.  ADC's judge-screening
instruction disallows questions that "ask whether specific proof would
be enough," so the discriminating form is exactly the form at risk of
being screened out.  Finding a phrasing that discriminates and
survives screening is now the central open problem.

Pool-level split on the collision, counting stable coherent members:
12 defendant, 4 plaintiff (m2, m4, m14, m15), 5 unstable or
label-incoherent (m3, m7, m12, m13, m21).  For an attorney: a jury
drawn from this pool decides a document-against-denial case for the
defendant by default; a plaintiff whose case rests on one contested
document cannot fix that at strikes and must fix it with evidence
(authentication, corroboration) or by targeting the unstable jurors.

Configuration effects are behavioral, not just theoretical.
gpt-oss-120b is stable plaintiff at mara, unstable at wandb/fp4 and
siliconflow/fp8.  deepseek-v3.1 is unstable at siliconflow/fp8, stable
defendant at deepinfra/fp4.  qwen3-30b is coherent plaintiff at
alibaba, label-incoherent at wandb/bf16, semi-incoherent at
siliconflow/fp8.  The same weights behave differently by provider and
quantization on the same adjudication question.

Two plaintiff-voting gpt-oss samples (m2, m12) inserted the word
"authenticated" into their reasoning although the vignette never said
it.  Jurors who want to vote plaintiff on a document appear to supply
the authentication themselves.  Hypothesis for argument tailoring:
authentication language is the pivot; a closing that can say
"authenticated" with record support should flip document-skeptical
jurors, and the unstable jurors should flip on weaker authentication
language than the stable-defendant ones.

### Next

Cycle 4: (a) an experience-framed voir dire question ("in your work,
when a written record conflicted with what someone told you, which
turned out to be right more often?") to plaintiff-voters,
defendant-voters, and unstable members, to see whether any allowable
phrasing separates them; (b) the authentication-flip test: the same
collision vignette plus an independent technical report authenticating
the screenshot, on stable-defendant and unstable members, to measure
what evidence language flips whom.  Judge-screening simulation needs
tooling (`adc llm --model` sends the literal `endpoint://model` string
and fails), so it waits.

## 2026-07-14 Cycle 4: experience question and the authentication gradient

### Method

`scratch/experiment1/cycle4/`.  qe is the experience-framed voir dire
question, one sample, to three plaintiff-voters (m2, m14, m15), five
stable defendant-voters (m1, m9, m16, m19, m25), and two unstable
members (m12, m13).  qf is the collision vignette plus an independent
forensic examiner who found the message in the provider's records,
3 samples, to the five stable defendant-voters.  qg is the collision
vignette where only the plaintiff's own lawyer describes the
screenshot as authenticated, 3 samples, to the same five plus the
unstable trio (m3, m7, m12).

### Results

qe: no discrimination.  Nine of ten members gave the same answer in
different words: written records usually proved more reliable, but I
weigh both.  Plaintiff-voters and defendant-voters are
indistinguishable.  m19 refused to answer, citing a juror's duty to
ignore personal experience; that is a model tic with fingerprint
value, and it does not track the trait.  The self-reports also point
the wrong way as a group: every member says paper beats word, yet most
of the pool votes defendant when a document collides with sworn word.

qf: all five stable defendant-voters flipped to plaintiff, 15 of 15
samples.  Independent forensic authentication produces unanimity.

qg, lawyer's-word authentication (votes across 3 samples):

| Member | Bare collision (cycle 2/3) | Lawyer says "authenticated" | Forensic (qf) |
|---|---|---|---|
| m1 deepseek-r1 | D D D | D D D | P P P |
| m9 deepseek-v3.1 fp4 | D D D | D P D | P P P |
| m16 grok-4.20 | D D D | P D D | P P P |
| m19 qwen3-next | D D D | D D D | P P P |
| m25 mistral-medium | D D D | P P P | P P P |
| m3 gpt-oss fp4 | D P D | P P D | not run |
| m7 deepseek-v3.1 fp8 | D P D | D P P | not run |
| m12 gpt-oss fp8 | D P D | D P P | not run |

m1 and m19 discounted the label in their reasoning ("authenticated
solely by their own attorney").  m25 treated the label as fact ("the
authenticated email constitutes direct evidence") in all three
samples.

### Conclusions

The authentication gradient is real and monotone per member: bare
document loses 12-4, counsel's unsupported "authenticated" flips one
stable member completely (m25) and pushes the unstable trio toward
plaintiff, and forensic authentication flips everyone.  For a
plaintiff attorney the ordering of moves is: put real authentication
in the record (moots the juror question entirely); failing that, use
authentication language in argument, which moves roughly half this
pool; and know which jurors discount counsel's characterization (m1,
m19) versus absorb it (m25, m3, m7, m12).  For a defense attorney the
same table is the strike list reversed: strike the label-absorbers,
keep the provenance-strict.

Verbal voir dire questions have now failed twice (attitude form,
experience form).  Nothing a juror says about itself has predicted its
vote; only behavior on concrete hypotheticals has.  If this holds, the
voir dire slot's value is in mini-hypotheticals, and the open question
from cycle 3 stands: whether the ADC judge allows them, since the
screening instruction disallows asking "whether specific proof would
be enough."

### Next

Two candidates.  (a) The defense counter-lever: same
lawyer-label vignette plus the judge's standard reminder that lawyer
statements are not evidence, to see whether an instruction cancels the
label effect on the absorbers.  (b) The judge-screening test on the
mini-hypothetical question forms, which needs the `adc llm --model`
defect fixed (it sends the literal `endpoint://model` string) or a
judge-shaped path in `adc juror`; fixing it is a small root-cause
change in the plain-model path, pending approval.  Personas also
remain untouched: every result so far is on the generic persona, so
the persona axis is unexplored.

## 2026-07-14 Cycle 5: instruction cancellation, judge screening, and the borderline question

### Method

The `adc llm --model` defect is fixed (both `llm.go` and `juror.go`
now pass the parsed model id instead of the `endpoint://model`
string), verified live.  Three probes in
`scratch/experiment1/cycle5/`.  qh is the lawyer-label vignette plus
the judge's reminder that statements by lawyers are not evidence, 3
samples, to the four label-absorbers (m25, m3, m7, m12), the two
wobblers (m9, m16), and the two provenance-strict members (m1, m19).
The screening probe presents the engine's own screening instruction
(from the voir dire judge turn in `Main.lean`) plus a candidate
question to `openai/gpt-5`, 5 samples per question; this is a
reconstruction of the runtime judge turn, not the turn itself.  qs2
asks the borderline screening survivor as a voir dire question, 2
samples, to plaintiff-voters (m2, m14, m15), defendant-voters (m1,
m9, m16, m19, m25), and m12.

### Results

qh, instruction cancellation: every absorber returned to defendant,
12 of 12 samples (m25, m3, m7, m12), and the provenance-strict members
stayed defendant.  The instruction over-cancels in the reasoning:
m25 called the exhibit "inadmissible as hearsay" and m3 said the
screenshot "cannot be counted," although the instruction addressed
only lawyer statements.  One anomaly: m9, which was stable defendant
on the bare collision, voted plaintiff in 2 of 3 samples here, with
reasoning that ignores the instruction.  One empty grok sample (m16).

Judge screening (5 samples per question):

| Candidate question | Ruling |
|---|---|
| s1 concrete collision hypothetical (the only form that discriminates) | disallow 5/5 |
| s2 "would you require independent verification of a disputed electronic document" | allow 3/5 |
| s3 experience form (cycle 4 qe) | allow 5/5 |
| s4 attitude form (cycle 3 qv) | allow 5/5 |
| s5 questionnaire-style control | allow 5/5 |

qs2, the borderline survivor, asked of jurors: m2, m15 (plaintiff on
the bare screenshot) say yes, they would require independent
verification.  m1, m9 (defendant every time) say no fixed rule,
case-by-case.  m16, m19, m25, m12 say yes; m14 mixed.  The answer is
uncorrelated with the vote and inverted in the four members named
first: the jurors who relied on an unverified screenshot claim to
require verification, and jurors who never rely on one claim
flexibility.

### Conclusions

Verbal voir dire questions have now failed three times (attitude,
experience, requirement).  No self-report about evidence handling has
predicted voting behavior on this pool, and in several members the
self-report is inverted.  The one question form that discriminates is
the concrete hypothetical, and the simulated judge disallows it 5 of
5.  Within ADC's screening rules as written, trait extraction by
questioning does not work on this pool.

What remains for the voir dire slot, in order of promise: model
identification through allowed questions (answers carry style,
refusal tics, and determinism that identify the model family, and an
offline behavioral dossier per family then supplies what questioning
cannot), for-cause elicitation, and incoherence detection (the
qwen3-30b label-contradiction failure and verbatim-identical answers
are visible in answer form).  The diagnostic-question program and the
identification program have traded places: identification is now the
live candidate, which reverses the earlier judgment recorded in the
plan, on evidence.

The defense counter-lever is confirmed: one standard instruction
cancels the authentication-label effect on every absorber, and the
over-cancellation spillover (jurors treating the exhibit itself as
excluded) benefits the defense beyond the instruction's legal scope.
Argument guidance draft: plaintiff counsel should not rely on
characterizing exhibits as authenticated, because the counter
instruction erases the gain and can poison the exhibit; real
authentication in the record is the only durable version of the move.

### Next

(a) Identification through allowed channels: give members the eight
standard questionnaire questions plus allowed voir dire questions,
then test how well the answers identify model family against pool
ground truth.  (b) Answer-form instability check: whether
verbatim-determinism and incoherence in allowed answers predict the
unstable voters.  (c) The persona axis, still untouched.

## 2026-07-14 Cycle 6: questionnaire fingerprints and the persona axis

### Thinking

The identification test uses only the channel every lawyer gets for
free: the court's own eight-question questionnaire (text taken from
`defaultJurorQuestionnaire` in `Main.lean`).  Every live member
answers it twice.  Two samples per member support three measurements:
a determinism score (how alike a member's two answer sets are), a
form-feature scan (formatting tics, refusals, length), and an
identification task (can a classifier match a member's second answer
set to its first, and to the model family, without labels).  If
identification works on questionnaire answers alone, the lawyer needs
no special question to use an offline dossier.  The instability
question rides on the same data: the unstable voters (m3, m7, m12,
m13, m21) should show up in answer form if answer form predicts
instability.

The persona probe attaches three contrasting checked-in personas to
two models that vote stable defendant on the collision, plus generic
controls, using unpinned scratch records (the provider pin changes
behavior, so these results will not be compared against pinned
members; controls make the comparison internal).  The personas:
`c4e1a2b-0` (bookkeeper burned by a contractor's written lie, says she
cannot separate a written lie from intent and damages), `d715074-1`
(exacting Hamburg logistics supervisor), `e50e538-0` (calm, firm Cebu
service worker).  Probes: the bare collision vote, 3 samples, and the
50k damages question, 2 samples.  The interesting outcomes: does
`c4e1a2b-0` flip the collision vote or inflate damages on a
written-lie case, against the same model with the generic persona.

### Method

Questionnaire: `adc juror --member M --repeat 2` with all eight
questions in one prompt, all 21 live members, outputs
`scratch/experiment1/cycle6/mM-quest.txt`.  Personas: scratch pool
`persona-pool.jsonl` with `deepseek/deepseek-r1` and
`openai/gpt-4o-mini` crossed with the three personas plus generic,
outputs `pM-{qa,qd1}.txt` in the same directory.

### Results

Re-identification: gpt-5 received the 21 first-sample answer sets
with anonymous labels and the 21 second-sample sets shuffled, and
matched second to first.  11 of 21 exact (chance is 1 of 21).  Six of
the ten misses are swaps inside a model family: the two gpt-oss
variants swapped with each other, the two qwen3-30b variants swapped,
and two nemotron variants swapped.  Counting family-level hits, 17 of
21.  Family naming is a different story: asked to name the model
family for each set with no exemplars, gpt-5 got 2 of 21.  Matching
against exemplars works; naming does not.  A dossier system therefore
needs labeled exemplar answers per pool member, which offline probing
provides.

Determinism (difflib ratio between a member's two independent
questionnaire samples): m19, m24, and m25 produced byte-identical
answer sets (ratio 1.000), m13 0.895, down to relace at 0.078.  The
unstable-vote set (m3 .59, m7 .41, m12 .76, m13 .90, m21 .50) sits in
the middle of the range, so text determinism does not predict vote
instability: m13 is nearly deterministic in prose and unstable in
votes, m25 is fully deterministic in both.  The determinism score is
itself a strong fingerprint for the members that have it.

Personas on the collision vote: with deepseek-r1, the generic,
logistics-supervisor, and service-worker personas all vote defendant
3 of 3, and the burned-bookkeeper persona (`c4e1a2b-0`) votes
plaintiff 2 of 3, reasoning in character ("it's just more cheating").
With gpt-4o-mini, all four personas vote defendant 3 of 3.  Personas
on damages: 15 of 16 samples award exactly 2,000, including both
burned-bookkeeper samples on both models; the persona changed the
rhetoric ("cheating deserves paying back what you stole, not a
windfall") and left the number alone.  One service-worker sample on
deepseek-r1 split the difference at 5,000.

### Conclusions

The identification channel works and has a specific shape.  One
questionnaire, which every ADC lawyer sees for free, re-identifies
the member at 52 percent exact and 81 percent at family level against
offline exemplars, with confusions concentrated within families.
Votes, though, distinguish configurations that prose does not (gpt-oss
at mara votes stable plaintiff while gpt-oss at wandb/fp4 is
unstable), so a family-level dossier carries the lean but misses the
stability dimension.  The workable attorney procedure this implies:
match questionnaire answers to exemplar dossiers for the family, then
treat within-family configuration as uncertainty about stability, not
about lean.

The persona axis is real but model-dependent and behavior-specific: a
strongly biased persona flipped deepseek-r1's collision vote most of
the time, had no effect on gpt-4o-mini, and moved nobody's damages
number even when the persona text says she cannot separate the lie
from damages.  Damages discipline looks like the most
persona-resistant behavior measured so far.  Answer-form determinism
identifies members but does not flag the unstable voters, so
instability stays measurable only behaviorally.

### Next

Assemble the first playbook draft from the accumulated table: per
member, collision lean, label susceptibility, instruction compliance,
damages discipline, persona sensitivity where measured, and
identification confusability.  Then the open probes: richer
identification input (add an oral answer to the questionnaire and
remeasure), persona susceptibility across more models, and the
harness check that the isolated-vote findings survive a real
deliberation packet.

## 2026-07-14 Cycle 7: conformity, concession, and partial documentation

### Thinking

ADC deliberation is written balloting: from round 2 a juror sees the
other jurors' votes and explanations, plus the court's reminder to
consider other jurors' reasons but not surrender an honestly held
view.  A juror's response to a 5-1 majority against its own lean is
directly attorney-relevant: low conformity means a seated juror of
adverse lean is a hung-jury engine, and high conformity means one
favorable articulate juror can carry a panel.  qc1 simulates the
round-2 packet on the collision case: each stable member is told it
voted its own lean in round 1 and that the other five voted the other
way, with five short explanations, and casts a round-2 ballot.

qc2 tests the concession lever, a classic trial-practice question:
with liability proven (forensic examiner, false email), does a defense
that concedes the lie and contests only damages beat a defense that
denies everything against the evidence?  Vote plus damages number per
juror per variant.

qc3 tests the documentation threshold on damages: proven 2,000 in
receipts, plus a customer email cancelling a 1,500 order because of
the lie, plus testimony-only stress, 50,000 demand.  If members award
3,500, the rule is that one document per category unlocks that
category, which is concrete argument guidance for plaintiffs.

### Method

`scratch/experiment1/cycle7/`.  qc1: stable-defendant members (m1,
m9, m10, m16, m19, m25) face a 5-plaintiff majority; stable-plaintiff
members (m2, m14, m15) face a 5-defendant majority; 3 samples each.
qc2: variants A (deny everything) and B (concede the lie, contest
damages) to m1, m2, m9, m13, m14, m16, 2 samples each.  qc3: m1, m2,
m9, m13, m16, m25, 2 samples each.

### Results

qc1 conformity is one-directional and total.  The six
defendant-voters facing a 5-plaintiff majority held defendant, 18 of
18 samples.  The three plaintiff-voters facing a 5-defendant majority
folded to defendant, 9 of 9 samples, adopting the majority's
equipoise reasoning.  A confound limits the reading: on this case the
defendant position is the legally normative one, so the asymmetry may
be convergence to the stronger legal argument rather than to the
crowd.  The two explanations separate on a case where the plaintiff
position is normative, which is the next probe.

qc2 concession is a null.  With liability proven by a forensic
examiner, both defense postures (deny everything against the
evidence, or concede and contest damages) produced the same outcome
in every member: plaintiff verdict, 2,000 damages.  The futile denial
was not punished with higher damages, and the concession was not
rewarded.  One m9 sample wrote "Defendant 2000," another
label-number contradiction.  Damages discipline dominates closing
posture on this pool, at least where every damages category is
cleanly documented or cleanly undocumented.

qc3 confirms the documentation threshold.  With 2,000 in receipts
plus a customer email tying a 1,500 cancelled order to the lie, m1,
m2, m9, and m16 awarded exactly 3,500 in every sample; m25 split
(2,000 then 3,500); m13 awarded 2,000 twice while its own sentence
accepted the 1,500 causal link, a repeat of its accept-the-premise,
drop-the-number incoherence.  One document per damages category
unlocks that category for the stable members.

### Conclusions

For argument, the damages rule is now the firmest finding in the
program: this pool pays what the record documents, category by
category, and neither demand size (cycle 3), persona (cycle 6),
closing posture (qc2), nor counsel's characterizations (cycle 5)
move the number.  Plaintiff guidance: one document per category,
however modest, beats any amount of testimony and rhetoric; the
customer cancellation email was worth 1,500, and the stress testimony
was worth zero.  Defense guidance: with liability lost, contesting
undocumented categories is free, and conceding liability costs
nothing but buys nothing measurable in this format.

For deliberation forecasting: plaintiff-lean jurors folded and
defendant-lean jurors held, unanimous in both directions, so on a
document-against-denial case a mixed jury converges to the defense
rather than hanging.  The cycle-3 strike map overstated the
plaintiff's exposure to a hung jury and understated the convergence;
if the normativity reading holds, the real rule is that ADC juries
converge to the legally stronger position and the lawyer's task is to
be on the side of it, or to change which position is legally stronger
through the record.

### Next

Separate conformity from normativity: on the qc2 facts, where the
plaintiff position is normative, put a defendant-voting holdout
against a plaintiff majority and a plaintiff-voting holdout against a
defendant majority.  If jurors track the law rather than the crowd,
the first folds and the second holds, reversing the qc1 direction.
Also run the judge voir dire screening eval that now exists in the
repo (`adc eval judge-voir-dire`) live, and compare its rulings on
specific-evidence-sufficiency fixtures against my cycle-5
reconstruction.

## 2026-07-14 Cycle 8: convergence to law or to majority

### Thinking

Cycle 7's fold pattern (plaintiff-leaners capitulate, defendant-
leaners hold) has two explanations: jurors follow the majority, or
jurors follow the legally stronger argument, and on the bare
collision those point the same way.  The qc2 facts separate them,
because there the plaintiff position is the normative one:
independent forensic proof, unrebutted, against a bare denial.  Two
round-2 conditions on those facts complete the 2x2 that cycle 7
started.  qn1: the juror is told it voted defendant in round 1
(doubting the examiner) and faces a 5-plaintiff majority citing the
forensic record; the normativity hypothesis says it folds to
plaintiff.  qn2: the juror is told it voted plaintiff (unrebutted
forensic proof) and faces a 5-defendant majority offering the best
available weak arguments (hired expert, no eyewitness, records can be
spoofed); the normativity hypothesis says it holds plaintiff, the
conformity hypothesis says it folds.

### Method

`scratch/experiment1/cycle8/`.  Both conditions to the nine members
with known collision leans (m1, m2, m9, m10, m14, m15, m16, m19,
m25), 3 samples each, same round-2 packet format as cycle 7.

### Results

qn1 (assigned defendant, plaintiff majority, plaintiff normative):
eight of nine members abandoned the assigned defendant position and
voted plaintiff (m1, m10, m14, m15, m16, m19, m25 at 3 of 3; m2 at 2
of 3, with the one defendant vote reasoned from damages
documentation).  m9 held defendant 3 of 3, calling unrebutted forensic
proof "circumstantial" and invoking "reasonable doubt," a criminal
standard.

qn2 (assigned plaintiff, defendant majority, defendant
anti-normative): all nine members held plaintiff, 27 of 27 samples,
uniformly dismissing the majority's arguments as speculation.

The 2x2 across cycles 7 and 8 is complete.  Jurors held against a
majority twice (both times when their position was legally stronger)
and folded twice (both times when it was legally weaker), in whichever
direction.  Majority size never mattered on its own.

### Conclusions

Deliberation on this pool converges to the legally stronger position,
and majority pressure has no separate measurable effect.  The
conformity reading of cycle 7 is dead: what looked like
plaintiff-leaners folding to a crowd was jurors updating toward the
better legal argument.  The attorney consequence is large and simple:
in ADC deliberation, being on the legally stronger side of the record
is the whole game, arm-your-advocate dynamics are secondary, and the
place to win deliberation is the record, then the closing that states
the legal logic the panel will converge on.

m9 (deepseek-v3.1, deepinfra/fp4) is a different kind of juror.
Across both conditions it kept whatever position it was assigned in
round 1, 6 of 6 samples, against the majority in both directions and
against the normative answer in one.  It is anchored to its own prior
vote rather than to law or crowd.  Under 6-of-6 unanimity, a seated
self-anchored juror whose round-1 vote goes against the record is a
hung jury.  This is the single most consequential per-member fact the
program has produced: the defense wants an m9 seated in a case the
record decides for the plaintiff, and the plaintiff's protection is
either a strike (if identification can find it) or a closing strong
enough to win its round-1 vote, because after round 1 it stops
moving.

### Next

Per-member self-anchor scores: assign each stable member a
round-1 position against its own lean on the bare collision (told it
voted plaintiff, facing the defendant majority it agrees with) and
measure who stays with the assignment versus who reverts to lean and
law.  That fills the holdout-risk column of the playbook, and then
the playbook draft itself.

## 2026-07-14 Cycle 9: self-anchor scores across the pool

### Thinking

The probe is the cycle-7 qc1p packet (bare collision, assigned
round-1 plaintiff vote, five-defendant majority), where law, own
lean, and majority all point to defendant for most members.  A member
that keeps the assigned plaintiff vote there is anchored to its prior
vote against everything else, which is the m9 profile and the
hung-jury risk column.  Cycle 7 already scored m2, m14, and m15
(all folded, anchor negative).  This cycle runs the remaining live
members, 3 samples each.

### Method

`scratch/experiment1/cycle9/`, members m1, m3, m4, m6, m7, m9, m10,
m12, m13, m16, m18, m19, m20, m21, m22, m23, m24, m25, prompt
identical to cycle 7 qc1p.

### Results

Keep rate on the assigned plaintiff vote (3 samples; cycle 7 scores
for m2, m14, m15 included for the full table):

| Keep 3/3 | Keep 2/3 | Revert to defendant 3/3 |
|---|---|---|
| m4 relace, m7 deepseek-v3.1 fp8, m9 deepseek-v3.1 fp4, m10 gpt-4o-mini | m21 qwen3-30b bf16, m25 mistral | m1, m2, m3, m6, m12, m13, m14, m15, m16, m18, m19, m20, m22, m23, m24 |

m9 has now kept its assigned round-1 position in all four tested
conditions, 12 of 12 samples, including against unrebutted forensic
proof.  m10 kept its assignment in both collision-case directions
(held assigned defendant in cycle 7, holds assigned plaintiff here)
but folded to the normative side on the forensic case, so it anchors
when the record is close and follows the record when it is decisive.
m7 keeps the assignment here despite unstable bare-collision votes.
m4's keep is confounded with its own plaintiff lean, but cycle 7
showed lean alone does not survive the defendant majority (m2, m14,
m15 folded), so m4's persistence still marks holdout risk.

### Conclusions

The holdout-risk set is {m4, m7, m9, m10, m21, m25}: six of 21 live
members.  A six-seat jury drawn from 21 members with six risky ones
contains at least one risky juror about nine times in ten
(1 - C(15,6)/C(21,6) = 0.908), and each side holds one for-cause
challenge and one peremptory strike, so strikes cannot clear the
risk.  The consequence under 6-of-6 unanimity: both sides should
treat round 1 as the verdict, because anchored jurors freeze there,
and everyone else converges to the record's stronger side.  The
asymmetry favors the defense: a juror frozen against the record
produces a hung jury, and a hung jury blocks judgment, which is a
defense outcome.  A frozen plaintiff-voter cannot force a plaintiff
verdict, only the same hung jury.

### Next

The playbook draft, then the three open validity items: harness
calibration of probe votes against full deliberation packets,
cross-case transfer of every finding beyond the misrepresentation
fact family, and persona effects across more models.
