---
id: builder-worktree-only
statement: "The builder writes only inside the worktree that the runtime provisions. An agent never runs git worktree."
layer: L0
order: 6
audience: [all]
enforced_by: [go/internal/guards/role.go]
source: "AGENTS.md invariant 4"
status: active
added: 2026-10-26
evidence: "AGENTS.md invariant 4; go/internal/guards/role.go denies a write outside the worktree"
---
Why: The worktree is the isolation of a build. A write outside it reaches other cycles.
