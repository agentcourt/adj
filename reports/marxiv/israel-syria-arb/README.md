# Israel–Syria ARB Case Study

[Adjudicating a Prediction-Market Resolution: An ARB Case Study of Israel's December 2024 Entry into Syria](paper.pdf) describes the completed run `ex04-20260918T144715Z`.  The report explains adj, ADC, ARB, model-pool generation, council sampling, and Lean verification before following the investigation, filings, and council decisions.  It includes quotations from lawyer work notes and filings, all eight accepted council explanations, operational failures, and limits on the findings.

The [LaTeX source](paper.tex) builds with pdfLaTeX and the installed `geometry`, `fontenc`, `inputenc`, `lmodern`, `booktabs`, `tabularx`, and `hyperref` packages.  Run both commands from this directory to resolve references:

```bash
pdflatex -interaction=nonstopmode -halt-on-error paper.tex
pdflatex -interaction=nonstopmode -halt-on-error paper.tex
```

## Case Record

The [retained case directory](../../../arb/out/ex04-20260918T144715Z/) contains the [accepted transcript](../../../arb/out/ex04-20260918T144715Z/aar-output/transcript.md), [work notes](../../../arb/out/ex04-20260918T144715Z/aar-output/work-notes.ndjson), evidence, council snapshots, sessions, logs, and research workspaces.  Git ignores that generated directory.  A fresh clone contains the report and the [example inputs](../../../examples/ex04/), but requires a separate copy of the run to inspect those execution artifacts.

The source checkout was `7016150`.  The case manifest records a different executable build identity, reproduced in the paper.  Review used the completed case's records and existing certificate-verification result.  Report preparation did not rerun the case or change runtime code, prompts, or pool entries.

marXiv accepted the PDF on September 18, 2026, as [2609.00009](http://127.0.0.1:8405/abs/2609.00009).  The [editorial review](review.md) records the decision and remarks concerning definitions and prose.  The accepted version retains those remarks for later revision.
