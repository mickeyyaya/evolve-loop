---
id: code-zero-comments
statement: "Code has no comments. Names, types and tests explain the code, and a design reason goes to the package note."
layer: L0
order: 10
audience: [all]
enforced_by: [go/internal/commitgate/comments_added.go]
source: "AGENTS.md invariant 10; docs/conventions/code-comments.md"
status: active
added: 2026-10-26
evidence: "AGENTS.md invariant 10 of 2026-09-30; the gate code in go/internal/commitgate/comments_added.go refuses an added comment"
---
Why: A comment drifts from the code. A name, a test or the package note stays true or fails.
