---
id: dev-on-branches
statement: "Do all development on a branch in a worktree, never on main. Main is the integration branch only."
layer: L0
order: 9
audience: [all]
enforced_by: []
source: "memory feedback_dev_on_branches_never_main"
status: active
added: 2026-10-26
evidence: "the operator rule of 2026-06-18 (memory feedback_dev_on_branches_never_main)"
---
Why: Main receives only reviewed merges. A direct change skips the review and the CI of a PR.
