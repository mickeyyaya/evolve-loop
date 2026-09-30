# Comment history: `acs/cycle1325`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1325/predicates_test.go:3` — above `package cycle1325`

```text
// Package cycle1325 materializes the cycle-1325 acceptance criteria for the
// TWO fleet-scoped tasks this lane owns that are NOT the boundary-refresh
// wiring gap (that one — auto-refresh-binary-at-boundary — is covered by a
// plain package-main test, cmd/evolve/cmd_loop_wave_boundaryrefresh_wiring_test.go,
// per R9.3: predicates bind only to this lane's top_n tasks, and that gap
// lives entirely inside package main, which an external acs package cannot
// import).
//
// Task B — wire-scout-carryforward-filter (scout-report.md Task B).
// Task C — mint-profile-driver-suffix (scout-report.md Task C).
//
// ARCHITECTURE CORRECTION (Task C, cycle-644 reachability-probe obligation):
// scout-report.md's targetFiles named go/internal/core/phase_advisor.go:1009
// as the fix site (`Dispatch: phaseconfig.Dispatch{CLI: e.Mint.CLI, ...}`).
// Compiler-probed here (temp import of internal/bridge from a
// zzz_import_probe_test.go dropped into internal/core, `go vet
// ./internal/core/...`): internal/bridge already imports internal/core
// (driver_claudep.go, engine.go, and ~a dozen *_test.go files) — so
// internal/core importing internal/bridge is a PROVEN import cycle
// ("import cycle not allowed in test"), not merely a plausible one. Freezing
// a predicate that pins bridge.DriverFor inside phase_advisor.go would
// reproduce cycle-644 exactly (an unsatisfiable AC baked into a frozen
// test). The fix instead lands at go/internal/phaseregistrar/registrar.go's
// Register — the actual mint choke point every minted phase (including
// advisor mints) already funnels through for its dispatch.cli validation
// (the existing "phase has no dispatch.cli; refusing to mint a driverless
// profile stub" guard, right next to where this task's guard belongs).
// phaseregistrar already imports internal/core (for core.Bridge/
// core.PhaseRunner) and compiler-probed clean for an ADDITIONAL import of
// internal/bridge (no cycle: bridge does not import phaseregistrar).
//
// Predicate strategy (cycle-85 degenerate-predicate ban: every predicate
// below exercises the real system under test, never a source-grep):
//
//	C1325_001 (Task C, positive)  — Register on a bare-CLI ("claude") config
//	  persists a profile whose cli field is the RESOLVED driver name
//	  ("claude-tmux"), proving the mint-time projection actually ran (not
//	  merely that dispatch tolerates the bare name elsewhere).
//	C1325_002 (Task C, negative)  — Register on an UNRESOLVABLE CLI name
//	  (neither a registered driver nor a known bare alias) is REJECTED
//	  loudly (non-nil error, nothing persisted) — never silently passed
//	  through to disk to fail hours later at the next loop launch's
//	  preflight halt (the batch-15 incident this task exists to close).
//	C1325_003 (Task C, edge/no-regression) — an ALREADY-resolved driver name
//	  ("claude-tmux") persists unchanged; resolution is idempotent, not a
//	  second, incompatible clamp on top of the existing base-name
//	  AllowedCLIs clamp (TestRegister_CLIWithDriverSuffix_ClampedByBase,
//	  registrar_test.go, pre-existing GREEN, left untouched by this task).
//	C1325_004 (Task B, positive+negative, the cycle-826 repro) — a real git
//	  fixture with three orphan cycle-* branches (superseded/ancestor,
//	  genuine 3-way conflict, clean/landable) proves
//	  triage.CarryforwardCandidatesSection surfaces ONLY the landable one —
//	  reproducing the exact cd409fed(ancestor)/b270ee11(orphan-but-here-
//	  conflicting) mis-selection the inbox item roots.
//	C1325_005 (Task B, edge) — no local cycle-* branches at all renders "".
//	C1325_006 (Task B, WIRING PROOF) — triage.ComposePrompt's real
//	  production body actually calls CarryforwardCandidatesSection (AST
//	  caller-proof, cycle-968 precedent) so the filter is reachable from the
//	  real triage prompt, not a second inert oracle sitting beside it.
//
// Adversarial diversity: negative (002, unresolvable-CLI rejection; 004's
// conflict+superseded exclusions), edge (003 idempotence; 005 empty-repo),
// semantic (004 distinguishes THREE distinct verdicts, not one restated).
```

### `go/acs/cycle1325/predicates_test.go:168` — above `func TestRegister_UnresolvableCLI_RejectedNotPersisted(t *testing.T) {`

```text
// C1325_002 (negative): a CLI name that is neither a registered driver nor a
// known bare alias (bareDriverMap) must be REJECTED at mint time — never
// silently persisted to fail hours later at the next loop launch's
// preflight halt (the batch-15 incident).
```

### `go/acs/cycle1325/predicates_test.go:224` — above `func carryforwardFixture(t *testing.T) (dir string) {`

```text
// carryforwardFixture builds a real repo reproducing the cycle-826 incident
// exactly: an ancestor-superseded orphan (cycle-100, mirrors cd409fed —
// already landed on main), a genuinely conflicting orphan (cycle-200,
// mirrors the auditor's real cherry-pick-dry-run conflicts a bare
// merge-tree-clean read would have missed), and a clean, not-yet-landed,
// landable orphan (cycle-300) that must be the ONLY one surfaced.
```

### `go/acs/cycle1325/predicates_test.go:242` — above `runGit(t, dir, "checkout", "-q", "-b", "cycle-100")`

```text
// cycle-100: superseded — its change is merged straight into main, so it
// becomes a pure ancestor (git merge-base --is-ancestor succeeds).
```

### `go/acs/cycle1325/predicates_test.go:252` — above `runGit(t, dir, "tag", "orig-base", "HEAD~1")`

```text
// cycle-200: forked BEFORE the c100 merge, diverges on the same line main
// keeps editing — a genuine, unresolvable 3-way conflict.
```

### `go/acs/cycle1325/predicates_test.go:266` — above `runGit(t, dir, "checkout", "-q", "-b", "cycle-300", "main")`

```text
// cycle-300: clean, disjoint, not-yet-landed — the one true candidate.
```

### `go/acs/cycle1325/predicates_test.go:277` — above `func TestCarryforwardCandidatesSection_ExcludesSupersededAndConflicting(t *testing.T) {`

```text
// C1325_004: the cycle-826 repro — only cycle-300 (landable) is surfaced;
// cycle-100 (superseded/ancestor) and cycle-200 (real conflict) are BOTH
// excluded. A bare `git merge-tree`-clean-only read would have wrongly kept
// cycle-100 (merge-tree of an already-merged branch reads clean) — this
// predicate fails against that degenerate implementation and only passes
// against the real CarryforwardCandidateLandable-backed filter.
```

### `go/acs/cycle1325/predicates_test.go:320` — above `func TestCarryforwardCandidatesSection_WiredIntoComposePrompt(t *testing.T) {`

```text
// C1325_006 (WIRING PROOF — kills a second inert oracle) — triage.go's real
// ComposePrompt body must call CarryforwardCandidatesSection, so the
// deterministic filter is actually consulted during the phase's own
// candidate selection, not merely built and left beside it (the exact
// "second, uninvoked oracle" gap the inbox item's [VERIFY 2026-07-23] note
// names). Structural (AST) caller-proof, paired with the behavioral
// predicates above — cycle-968 TestClassifyFleetRebaseCandidate_WiredIntoRecoverFromShipError
// precedent.
// acs-predicate: config-check — caller-existence is an inherent source-
// structure check; CarryforwardCandidatesSection's own filtering behavior is
// proven by C1325_004/005 above.
```
