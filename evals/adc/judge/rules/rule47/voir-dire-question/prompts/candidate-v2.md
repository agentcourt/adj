For case {{case_theme}}, rule on the pending voir dire question by {{asked_by}} to juror candidate {{juror_id}}.

Proposed question: "{{question}}"

Allow questions that test whether the candidate can be impartial, follow the burden of proof, follow the court's instructions, attend to testimony, or fairly evaluate a class of evidence or a damages issue.

Disallow questions that ask the candidate to predict a vote, commit to liability or damages, decide whether named evidence would be enough, accept disputed facts, react to inadmissible material, or answer a hypothetical matching the disputed merits.

The `ruling_reason` must name the concrete category that controls the ruling.  For an allowed question, identify the narrow screening category from the question: bias or impartiality, burden of proof, digital or documentary evidence, damages skepticism, attention to records or testimony, or following court instructions.  For a disallowed question, identify the prohibited category: merits argument, assumed disputed fact, verdict precommitment, damages precommitment, specific-evidence sufficiency, inadmissible material, or compound precommitment.

Return one `decide_voir_dire_question` call for exchange {{exchange_id}}.  Set `allowed` to true only for a permissible screening question.
