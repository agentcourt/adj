# Israel–Syria ADC Case Study

The [technical report](paper.pdf) follows the completed ADC proceeding in chronological order.  It explains the adj procedures, model-pool generation and sampling, Lean verification, evidence collection, discovery, voir dire, trial, instructions, and judgment.  Quotations include submitted work notes, filings, judicial reasons, and every accepted juror explanation.

The case used two Pi lawyers with GPT-5.6-sol at xhigh reasoning, Codex subscription authentication, and web search.  Nine jurors were sworn from the installed 97-row pool, with six votes required.  The result was seven plaintiff votes, one defendant vote, and one juror process that exited without voting.  ADC entered judgment that the proposition was demonstrated, with zero damages.  Replay verified 227 transitions.

The retained run is under `adc/out/ex04-20260919-04/`.  It began on September 19, 2026, at 14:32:57 UTC and completed at 16:36:36 UTC.  The source revision at launch was `87de350`, with executables built from unchanged runtime source at `dbde6f0`.  The running processes retained their original configuration when the separate Pi court-submission error limit was added.

The [main source](paper.tex) includes the [trial and jury narrative](proceeding-detail.tex), [assessment and execution findings](assessment.tex), and [record excerpts](record-excerpts.tex).  The appendices reproduce selected discovery responses, the complete final charge, and all eight accepted vote explanations.  Generated case records remain outside version control.

The [review record](review.md) identifies the checks completed before submission and records editorial status.  The report distinguishes source statements, party admissions, attorney inferences, and the author's observations.  It reports limits on exhibit admission, ballot isolation, source sufficiency, accounting, and model reliability.

Build from this directory with the installed pdfLaTeX packages.  A further pass may be needed after the table of contents changes:

    pdflatex -interaction=nonstopmode -halt-on-error paper.tex
    pdflatex -interaction=nonstopmode -halt-on-error paper.tex

The 36-page report is accepted as version 5 of [marXiv:2609.00015](http://127.0.0.1:8405/abs/2609.00015).  The [final editorial decision](http://127.0.0.1:8405/status/513c4a934831) returned “No remarks.”  The source and PDF in this directory match that submission.
