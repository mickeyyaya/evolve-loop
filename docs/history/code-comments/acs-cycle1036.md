# Comment history: `acs/cycle1036`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1036/predicates_test.go:3` — above `package cycle1036`

```text
// Package cycle1036 materialises the cycle-1036 acceptance criteria for this
// fleet lane's sole item, retro-role-gate-lessons-write-allowance (triage top_n
// slug identical). Per R9.3 no predicate here binds to any other lane's items.
//
// Defect (from scout-report.md). go/internal/guards/role.go's doc comment
// promises a `learn/retrospective: workspace_path + .evolve/lessons/**`
// allowance that Decide() never implements. PhaseRetro ("retro") is a valid
// cs.Phase but is NOT a core.WorktreePhase, so the role guard grants it only the
// WorkspacePath allowance — a retro-phase Edit/Write under the real lesson
// corpus path `<repoRoot>/.evolve/instincts/lessons/` is default-denied
// (role.go fallthrough). The doc comment also names the wrong path
// (`.evolve/lessons/**` vs the real `.evolve/instincts/lessons/`, per
// go/internal/research/kb.go:75). Fix: add a retro-phase branch allowing writes
// under `<repoRoot>/.evolve/instincts/lessons/`, evaluated STRICTLY AFTER the
// IsProtectedSurface deny (so a crafted lessons path cannot smuggle a
// control-plane edit), and correct the doc comment.
//
// Predicate strategy. These predicates EXERCISE THE SYSTEM UNDER TEST: 001-004
// construct the real guards.Role via NewRole and call Decide() with a synthetic
// core.Storage whose CycleState has Phase=="retro" and a canonical
// WorkspacePath (`<root>/.evolve/runs/cycle-1036`). The guard derives the lesson
// corpus dir from that canonical WorkspacePath (its repoRoot ancestor +
// `.evolve/instincts/lessons`), so no real filesystem is touched — Decide is
// pure path arithmetic (isUnderDir/filepath.Rel + IsProtectedSurface). This is
// NOT a source-grep proxy (cycle-85 ban): the assertions are on Decide's
// returned GuardDecision. Only 005 asserts on source text, and it carries the
// `// acs-predicate: config-check` waiver because the doc-comment correction is
// an inherent documentation-text criterion with no runtime code path (the same
// waiver cycle-1029/cycle-943 used for inherent doc criteria).
//
// Adversarial axes (adversarial-testing SKILL §6):
//   - positive  : 001 retro + lessons path → Allow.
//   - negative  : 002 retro + non-lessons/non-workspace path → Deny (proves the
//     fix does not over-broaden — a no-op that blanket-allows retro fails here).
//   - edge/smuggle: 003 retro + a path that is BOTH under the lessons dir AND a
//     protected surface (`…/.evolve/instincts/lessons/.evolve/policy.json`) →
//     Deny + Alarm (proves the allowance is checked after IsProtectedSurface).
//   - edge/traversal: 004 retro + `…/.evolve/instincts/lessons/../../etc/passwd`
//     (escapes the lessons dir after cleaning) → Deny (proves clean containment,
//     not a naive prefix match).
//
// RED today:
//   - 001 fails: retro is not a WorktreePhase, so Decide returns Allow:false for
//     the lessons path (the missing branch).
//   - 005 fails: role.go's doc comment still reads `.evolve/lessons/**` and lacks
//     the corrected `.evolve/instincts/lessons` path.
//   - 002/003/004 are PRE-EXISTING GREEN guard tests (they assert behaviour the
//     fix must PRESERVE): they lock in that the new allowance does not
//     over-broaden, does not bypass the control-plane deny, and does not leak via
//     path traversal.
```
