# Contributor Instructions

Discuss major design decisions before implementation.  Describe the available options, their consequences, and any uncertainty that affects the choice.  Keep changes within the agreed scope.

Use explicit, plain technical language in code, documentation, commit messages, and reviews.  Remove filler, sales language, slang, and unsupported claims.  State limitations and errors precisely.

Find root causes and correct them directly.  Return errors to callers rather than discarding or logging them as a substitute for handling them.  Format Go changes with `gofmt`, add focused tests, and record current design decisions and verification work in `devnotes.md`.
