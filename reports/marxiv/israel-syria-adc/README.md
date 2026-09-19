# Israel–Syria ADC Case Study

The [working LaTeX source](paper.tex) describes ADC, the shared model pool, jury selection, Lean procedure, and the Israel–Syria case under `adc/out/ex04-20260919-03/`.  The case stopped during jury selection after the defendant exceeded its peremptory-strike deadline.  It has no verdict.  The report remains an incomplete draft, and no ADC report has been submitted to marXiv.

The input settings and market-rules document are under `input/`, the unified case record is under `case/`, and retained lawyer state is under `agents/`.  The case began on September 19, 2026, at 07:56:08 UTC with source revision `dbde6f0` and rebuilt Go commands.  Its generated scenario permits 500 turns.

The approved configuration uses nine jurors with six required votes, two Pi lawyers using `openai/gpt-5.6-sol` at `xhigh`, Codex subscription authentication, enabled web search, the installed 97-row pool, and 25-minute model and participant turn timeouts.  ADC retains its GPT-5.4 judge and clerk and default voir dire.  The initial material contains the supplied market resolution.  The participants receive no ARB case record.

The launcher exited with status 1 at 13:08:33 UTC on September 19.  The final work note identifies the defense's intended strike against J9, but its model session contains no subsequent decision submission.  The retained record ends at event 309.  No runtime, prompt, pool, or source input changed during the case.  Diagnosis and any further run remain separate from report submission.

The draft quotes court filings and submitted work notes, with tool activity checked against retained process logs.  Completion and submission require a completed adjudication, its juror explanations, and factual and prose review.  The PDF is an incomplete draft for layout review.

After the manuscript is complete, build it from this directory with the installed pdfLaTeX packages:

```bash
pdflatex -interaction=nonstopmode -halt-on-error paper.tex
pdflatex -interaction=nonstopmode -halt-on-error paper.tex
```
