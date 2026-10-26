---
id: docs-through-docs-write
statement: "Write each document through the docs-write skill. The new text passes evolve docs ste-lint --strict."
layer: L0
order: 11
audience: [all]
enforced_by: [go/internal/stelint]
source: "AGENTS.md invariant 11; memory feedback_docs_follow_docs_conventions, feedback_docs_write_skill_and_three_approaches and feedback_ste100_docs_and_replies"
status: active
added: 2026-10-26
evidence: "ADR-0132: documents are written through one skill"
---
Why: One skill gives each document its type, its home and its reasoning chain. The lint keeps the text simple and checkable.
