Think in English about what important properties (theorems) we might
want for the Lean code.  What's important, useful, and tractable?  For
a candidate theorem, how could we go above proving it?  Again, think
in English.  Would it be helpful to make careful changes to the code
to prove this property?

When writing Lean, always be very literate (in the spirit of Knuth).
First state in English the context, goals, meaning, and approach of
what you are purusing.  For a theorem or example, first describe it in
detail in English.  Be thoughful.  Include your thoughts about how the
proof should be structured. Review your work.

Only then proceed to the proof (or construction of an example) based
on the English sketch you just provided in English in the Lean file.

Be sure to use lemmas to keep proof bodies relatively concise.  You
can use "sorry" but only temporarily.

Important: Consider inspiration from math concepts like metric, norm,
measure, topolgy, group, ring, field, modudule, etc.  (But do *not*
pull in mathlib unless absolutely necessary -- and get approval
before).  If some some of structure in that spirit seems helpful, then
pursue it.  Structures like that for our procedures could be very powerful.

Your Lean files will likely have as much English as Lean code.  Make
sure that English is meaningful.
