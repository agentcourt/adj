# Israel–Syria ADC Case Study

The [working LaTeX source](paper.tex) describes ADC, the shared model pool, jury selection, Lean procedure, and the Israel–Syria case under `adc/out/ex04-20260919-02/`.  The report remains an incomplete draft while the case runs.  No ADC report has been submitted to marXiv.

The input settings and market-rules document are under `input/`, the unified case record is under `case/`, and retained lawyer state is under `agents/`.  The case began on September 19, 2026, at 04:28:29 UTC with source revision `cfdd312` and rebuilt Go commands.  Its generated scenario permits 500 turns.

The approved configuration uses nine jurors with six required votes, two Pi lawyers using `openai/gpt-5.6-sol` at `xhigh`, Codex subscription authentication, enabled web search, the installed 97-row pool, and 25-minute model and participant turn timeouts.  ADC retains its GPT-5.4 judge and clerk and default voir dire.  The initial material contains the supplied market resolution.  The participants receive no ARB case record.

The draft quotes court filings and submitted work notes, with tool activity checked against retained process logs.  Completion and submission require a completed adjudication, its juror explanations, and factual and prose review.  The existing PDF predates this source revision and is not a submission candidate.

After the manuscript is complete, build it from this directory with the installed pdfLaTeX packages:

```bash
pdflatex -interaction=nonstopmode -halt-on-error paper.tex
pdflatex -interaction=nonstopmode -halt-on-error paper.tex
```
