# How to prove things

Here's how to go about thinking up theorems and proving them.

First consider what sort of properties would be meaningful to
demonstrate.  Review the code, the rules (ARCP), limits, overall
goals, etc. But don't be too grandiose.  Much better to start with
small, tractable, tactical results, which hopefully are useful on
their own but can also play a role in larger results later.  We don't
want proofs for their own sake.  We want at least a little meaning.  As
you build up proofs, be more aggressive in pursuing non-trivial, deep
results.

Do not be too afraid to modify existing Lean code to make a proof
easier; however, be cautious and be ready to backtrack.  If you break
other proofs or any (other) tests, that's a serious situation. In
those situations, you should consider if the tests themselves are
incorrect.  You can't assume anything is really authoritative.  Give
strong deference to the rules (ARCP), but even they can be adjusted if
appropriate.  When in doubt about what must give, ask. You should also
feel free to note a problem and defer/abandon that particular
little effort for now.  You'll likely run into these sort of obstacles
frequently.  Don't give up on a proof too soon. Work at the proof
while holding everything else fixed.  Maybe work pretty hard; iterate
a lot. But step back occasionally when facing real obstacles.

It's okay to have a `sorry` very temporarily, but be *very*
cautious. It's much better to avoid sorries completely in order to
avoid a lot of effort which ultimately can't really be used.  Probably
better to proceed without ever using a `sorry`.

Once exception: You can leave a `sorry` either a critical but
indefinitely deferred work in progress.  Ask for approval before doing
that.

Consider: When you have a theorem in mind, *first* think about it in
English. Then think in English about how you might go about proving
it.  Consider what lemmas and theorems -- existing or not -- might be
useful. Comment on the use of induction or recursion or whatever.
Only then start to formalize the theorem in Lean and begin trying to
prove it.  Update your English description of the theorem and proof
approach as you make progress.  Also keep notes about how this
particular effort went.  These notes should discuss obstacles,
opportunities, and any other observations relevant to your attempts at
proving the theorem.  A narrative style is appropriate.  Leave notes
for your future self to use in other tasks.

Here's the desired template for Lean theorems and proofs.

```Lean
/--

DESCRIPTION OF THEOREM

DISCUSSION OF PROOF PLAN

-/

THEOREM WITH PROOF

/--
AFTER-THE-FACT NOTES AND NARRATIVE ABOUT COMING UP WITH THE PROOF
-/
```

*It's critical to follow this literate style, with the comment above
in the code itself.*  All of these comments should be detailed, clear,
and helpful.  You are writing for an expert audience.
