# Comment history: `acs/cycle943`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle943/predicates_test.go:3` — above `package cycle943`

```text
// Package cycle943 materializes the cycle-943 acceptance criteria for this
// fleet lane's sole inbox item, overlay-injection-dormant-wire-fable-deep,
// which the Scout re-scoped into three committed (## top_n) tasks after finding
// the phase-runner→prompt half already wired:
//
//	T1 skilloverlay-compact-md-first-materialization
//	T2 skill-overlay-dispatch-observability-log
//	T3 wire-overlay-resolution-non-phase-dispatch-sites
//
// EVERY behavioral predicate below EXERCISES THE SYSTEM UNDER TEST — it calls
// skilloverlay.Materialize on a real on-disk skill dir, or drives subagent.Run /
// retro.Phase.Run with a capturing fake core.Bridge and asserts on the
// core.BridgeRequest the dispatcher actually built. None assert solely on source
// text (the cycle-85 degenerate-predicate failure mode is avoided); the single
// doc-comment predicate carries the `// acs-predicate: config-check` waiver
// because a "this feature is still deferred" comment that has gone stale is an
// inherent text criterion, not a code path.
//
// RED today:
//   - T1 predicates fail on assertion: Materialize reads SKILL.md
//     unconditionally, so a dir with a distinct COMPACT.md yields the SKILL body.
//   - T2 predicates fail on COMPILE: runner.FormatSkillOverlayLog does not exist
//     yet. The Builder adds it (a pure formatter the dispatch closure calls).
//   - T3 predicates fail on assertion: subagent.Run and retro.Phase.Run build
//     their BridgeRequest with an empty Skills field today.
//
// SUT CONTRACT the Builder must satisfy WITHOUT modifying this file
// (full prose in test-report.md § Handoff to Builder):
//
//	skilloverlay.Materialize(skillsDir, names):
//	    prefer <skillsDir>/<name>/COMPACT.md when present+non-empty;
//	    fall back to SKILL.md when COMPACT.md is absent; unchanged otherwise.
//
//	runner.FormatSkillOverlayLog(phase string, skills []string, tier string) string:
//	    a pure formatter returning a line that contains "phase=<phase>",
//	    "skill-overlays=[<comma-joined skills>]", and "tier=<tier>". The
//	    per-attempt dispatch closure in runner.go emits it via log.Diag().Infof.
//
//	subagent.Run / retro.Phase.Run:
//	    resolve overlays for the launch's tier (Policy.ResolveOverlays on an
//	    OverlayDispatch built from the launch's phase/cli/model/tier) and set the
//	    resolved NAMES on core.BridgeRequest.Skills — matching runner.go. The
//	    tier→skill mapping stays SOLELY in policy.compiledDefaultOverlays (no new
//	    Go literal). A deep/top-tier launch attaches "fable"; a balanced launch
//	    attaches nothing.
```
