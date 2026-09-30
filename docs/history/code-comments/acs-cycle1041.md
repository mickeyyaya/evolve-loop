# Comment history: `acs/cycle1041`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1041/predicates_test.go:3` — above `package cycle1041`

```text
// Package cycle1041 materialises the cycle-1041 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - retro-role-gate-lessons-write-allowance
//
// The defect. go/internal/guards/role.go:11-16 documents a per-phase allowance
// "learn/retrospective: workspace_path + .evolve/lessons/**", but Decide()
// (role.go:30-91) has exactly two allow branches for a non-always-safe,
// non-protected path: under cs.WorkspacePath, and (for WorktreePhase only) under
// cs.ActiveWorktree. There is NO branch keyed on the retro phase, so a retro
// write to the lessons directory falls through to the terminal deny. The
// documented allowance does not exist in code.
//
// Phase-name correction carried by these predicates. The doc comment names
// "learn/retrospective", but the canonical runtime value of CycleState.Phase is
// the string "retro" (go/internal/cyclestate/phase.go:18, PhaseRetro). A fix
// gated on the doc comment's wording would compile, pass a wording-shaped test,
// and STILL never fire in production. Predicate 001 therefore drives the guard
// with the real runtime value.
//
// Predicate strategy — every predicate CONSTRUCTS the guard under test and calls
// Decide(), asserting on the returned decision. None greps role.go's source
// (the cycle-85 degenerate-predicate ban): a predicate that merely looked for
// the string ".evolve/instincts/lessons" in role.go would pass on a comment.
//
//   - 001 is the crux (currently RED): phase=retro writing
//     <root>/.evolve/instincts/lessons/<name>.yaml must be ALLOWED.
//   - 002 is the scoping negative: under the SAME phase and the SAME root, a
//     path OUTSIDE lessons/** must stay DENIED. It is also the structural
//     control for 001 — if the temp root were an always-safe prefix (/tmp/**),
//     making 001 pass for the wrong reason, 002 fails loudly.
//   - 003 is the phase negative: a non-retro phase writing the very same lessons
//     path must stay DENIED, so the fix cannot be a blanket path allowance.
//   - 004 is the anti-gaming precedence pin (expected pre-existing GREEN): the
//     new allowance must be placed AFTER the ADR-0064 control-plane check, so a
//     retro phase still cannot edit a protected surface by routing through it.
```

### `go/acs/cycle1041/predicates_test.go:155` — above `func TestC1041_004_ControlPlanePrecedenceSurvivesRetroAllowance(t *testing.T) {`

```text
// TestC1041_004_ControlPlanePrecedenceSurvivesRetroAllowance pins the ordering
// invariant (ADR-0064): the new retro branch must sit AFTER the
// IsProtectedSurface check, so a retro phase still cannot edit the gate that
// grades its own cycle. Expected pre-existing GREEN — it is the regression pin
// that fails if the fix is inserted above the integrity boundary.
```
