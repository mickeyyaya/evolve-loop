---
id: read-before-call
statement: "Read a module and list its real exports before you call or import it. Never invent an API."
layer: L0
order: 18
audience: [all]
enforced_by: []
source: "AGENTS.md core rule 8; memory feedback_read_before_implement and feedback_verify_named_proofs_exist"
status: active
added: 2026-10-26
evidence: "on 2026-09-30 ADR-0105 named a proof test that did not exist (memory feedback_verify_named_proofs_exist)"
---
Why: An invented API compiles in the mind and fails in the build.
