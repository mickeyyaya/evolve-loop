---
id: code-zero-comments
statement: "MANDATORY: code has no comments, except tool-read lines and the one package doc that docgo requires. A reason goes to the package note."
layer: L0
order: 12
audience: [all]
enforced_by: [go/internal/commitgate/comments_added.go]
source: "AGENTS.md invariant 10; docs/conventions/code-comments.md; operator 2026-10-27"
status: active
added: 2026-10-26
evidence: "AGENTS.md invariant 10 of 2026-09-30; the gate code in go/internal/commitgate/comments_added.go refuses an added comment; console report of 2026-10-27: the TDD predicate TestC2061_004 of cycle 2061 tested a Go package doc comment, so no build could pass it"
---
Why: A comment drifts from the code. A name, a test or the package note stays true or fails.

Each agent must obey this rule. It applies to each phase and to each family.

- **No comment, with two exceptions.** This includes a line comment, a block comment and a Go doc comment. The exceptions are a marker that a tool reads, for example `// acs-predicate:`, and the one package doc that `go/acs/regression/docgo` requires.
- **The package note is a file.** "Package note" means the file of the package under `docs/architecture/packages/`, for example `internal-tokenusage.md`. It is not a Go doc comment.
- **A test or a criterion never needs a comment.** A scout criterion, a TDD predicate and an audit finding never ask for a comment in code. To check that a reason is recorded, read the package note file.

Never write, ask for or test any other comment.
