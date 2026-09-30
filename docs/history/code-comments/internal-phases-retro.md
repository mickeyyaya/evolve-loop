# Comment history: `internal/phases/retro`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/retro/explanation_review_gate.go:16` — above `func validateExplanationReview(report string, req core.PhaseRequest) (advisories []string, err error) {`

```text
// validateExplanationReview applies retro's policy around the shared review
// contract (explanationdocs.ValidateReviewedHandoff). Since 2026-09-13
// (ADR-0102, operator decision) the review's shape — the section, its
// fields, the correction-todo bookkeeping, the handoff echoes and the
// citations — is advisory: the findings ride the retrospective's record as
// warnings. The error, the only blocking outcome, is the reasoning floor (a
// token Evidence; a missing or duplicated review section is no review text
// at all) and host-side defects in the handoff itself.
```

### `go/internal/phases/retro/lesson_resolution_test.go:3` — above `import (`

```text
// lesson_resolution_test.go — RED contract for where a failure lesson LIVES.
//
// retro graded itself FAIL unless hasFailureLesson found a file named
// `failure-lesson*.yaml` in the CYCLE WORKSPACE. The retro persona is instructed
// to write `.evolve/instincts/lessons/inst-LXXX-<slug>.yaml` — a different
// directory AND a different filename. The phase was graded FAIL for not producing
// an artifact its own persona is instructed never to produce.
//
// Measured on the runtime plane (where the loop actually writes; a git worktree's
// copy is empty because .evolve/instincts/lessons/* is gitignored): 600 lesson
// files, of which 135 are inst-L* and 464 are an older cycle-<N>-* convention.
// Every recent FAILING cycle produced inst-L* and no cycle-<N>-*: 1572→1, 1574→3,
// 1576→3, 1577→4. Exactly ONE file named failure-lesson* exists anywhere, and 220
// of 238 retro FAILs (92%) had a substantial retrospective and no workspace lesson.
//
// The persona's convention is the one that is load-bearing — those files are read
// back into future agents' instinctSummary — so the GATE moves to meet it, not the
// other way round. The legacy workspace shape is still accepted so cycle-1571-era
// artifacts keep passing.
```

### `go/internal/phases/retro/lesson_resolution_test.go:95` — above `name:    "a longer cycle number does not satisfy a shorter one's gate",`

```text
// DIGIT BOUNDARY (review L2). A plain prefix match lets cycle 157 be
// satisfied by cycle 1574's lesson.
```

### `go/internal/phases/retro/lesson_resolution_test.go:114` — above `name:   "legacy workspace failure-lesson*.yaml still counts",`

```text
// cycle-1571 wrote failure-lesson-cycle1571.yaml into its workspace.
// That corpus must keep passing.
```

### `go/internal/phases/retro/register_test.go:10` — above `func TestRetroSelfRegisters(t *testing.T) {`

```text
// TestRetroSelfRegisters asserts the retro phase publishes its own factory to
// the phase registry in package init() — like every other built-in phase. The
// dispatcher must not hardcode retro construction (phase-agnostic flow,
// ADR-0035/0038); it resolves the factory by name.
//
// Registration previously lived in the dispatcher (internal/cli/phasecmd);
// retro now self-registers in its own init(). This is the permanent regression
// guard for that invariant — the test does not import the dispatcher.
```

### `go/internal/phases/retro/retro.go:75` — above `func retroWorktree(req core.PhaseRequest) string {`

```text
// retroWorktree resolves the working directory this retro launch is dispatched
// against.
//
// A LIVE provisioned worktree passes through verbatim — a fallback that fired
// unconditionally would strand every normal retro in an empty scratch dir with
// no repo. The fallback exists for one window only: under a fleet supervisor the
// bridge drivers REFUSE a launch whose working dir does not clear the guard —
// empty is refused as errWorktreeRequired (bridge/driver_tmux_repl.go:27) and a
// non-existent path is refused at gobridge.IsDir (driver_tmux_repl.go:123,
// ExitBadFlags, stderr only, no error return) — instead of falling back to the
// process cwd. Either way a lane whose worktree was torn down (or never
// provisioned after exhausted retries) loses its retrospective entirely — a
// failure in the failure-handler. Retro is read-mostly and Evaluate-archetype,
// so a disposable cwd under the workspace it already owns clears the guard.
//
// The condition is the guard's own predicate, not a string shape: a torn-down
// fleet lane hands retro a NON-EMPTY but stale path (cs.ActiveWorktree), and
// testing only for "" would pass it straight into the refusal. Both no-worktree
// shapes are one contract (cycle-1278; the empty half alone was cycle-1270).
//
// The two shapes deliberately NOT used: the shared main tree (req.ProjectRoot —
// refuted by PR #400; worktree is the write-authority predicate) and the
// dispatching process cwd (the exact leak the guard closes). With no owned
// workspace there is nowhere safe to mint, so this returns "" and the bridge
// decides exactly as it does today — never a fabricated path. The guard itself is
// untouched: the fix is supplied by the phase.
```

### `go/internal/phases/retro/retro.go:155` — above `var prof profiles.Profile`

```text
// CLI resolution chain: EVOLVE_CLI > profile.cli > claude-tmux — matching
// BaseRunner (runner.go). This hand-rolled runner had regressed to the
// cycle-107 class (EVOLVE_CLI-or-hardcoded, profile.cli ignored), which
// made the 2026-08-26 deep-tier sol arrangement's flagship flip —
// retrospective, ~40% of deep dispatch volume — dead on arrival until
// review caught it against the dispatched BridgeRequest.
```

### `go/internal/phases/retro/retro.go:210` — above `fence := treefence.Begin(ctx, bridgeReq.Worktree, req.WorktreeReadOnly)`

```text
// Read-only worktree fence (ADR-0097): retro calls the bridge itself, so it
// holds the same fence the shared runner does — the tree it hands to the
// retry envelope (tdd re-entry, salvage) is the tree it was given.
```

### `go/internal/phases/retro/retro.go:350` — above `func matchesCycleLesson(name, prefix string) bool {`

```text
// matchesCycleLesson requires a NON-DIGIT delimiter after the cycle number. A
// plain prefix match would let cycle 157's gate be satisfied by
// inst-L1574a-<slug>.yaml, and cycle 1's by any lesson whose number starts with 1
// — re-opening the rubber-stamp hole the cycle-scoping exists to close.
```

### `go/internal/phases/retro/retro.go:383` — above `entries, err := os.ReadDir(ws)`

```text
// Legacy: cycle-1571-era runs wrote failure-lesson*.yaml into the workspace.
```

### `go/internal/phases/retro/retro.go:400` — above `func init() {`

```text
// init self-registers the retro phase factory with the phase registry, like
// every other built-in phase. The subprocess dispatcher (internal/cli/phasecmd)
// resolves phases by name and never constructs retro directly — keeping the
// flow phase-agnostic (ADR-0035/0038). The factory builds the default
// project-rooted bridge + prompts loader (Model "auto"), matching the prior
// phasecmd wiring byte-for-byte (cmdutil.NewPromptsLoader was a duplicate of
// prompts.NewForProject).
```

### `go/internal/phases/retro/retro_chain_test.go:38` — above `func TestRun_TimeoutOnThePrimaryCLIFallsBackThroughTheChain(t *testing.T) {`

```text
// TestRun_TimeoutOnThePrimaryCLIFallsBackThroughTheChain reproduces the seal
// lanes 1676/1677 met (2026-09-14): the retro's own launch on codex exited 81
// after 1800 s and the cycle sealed FAIL with no disposition. Through the
// chain-walking bridge handle the composition root now hands it, the same
// retro falls back to the profile's next CLI and PASSes — one CLI's timeout
// costs one attempt, not the cycle.
```

### `go/internal/phases/retro/retro_compaction_amplified_test.go:3` — above `import (`

```text
// retro_compaction_amplified_test.go — Adversarial amplification for cycle-421
// retro-phase-compaction-wiring (behavioral gaps in retro package).
//
// Probes gaps NOT covered by retro_compaction_test.go (3 tests: ConfigHasCompactPrompts,
// CompactEnabled_StripsBody, CompactDisabled_BodyIdentical):
//
//   - CompactPrompts=true must NOT break the PASS verdict short-circuit.
//     Existing TestRun_PreviousPASS_SKIPPEDWithoutBridgeCall uses default Config
//     (CompactPrompts=false). This amplifier probes whether wiring the flag accidentally
//     removes the guard that skips bridge calls for PASS verdicts.
//   - CompactEnabled produces a strictly shorter prompt than CompactDisabled.
//     C421_005 / TestRetroPhase_CompactEnabled_StripsBody verifies the tail is ABSENT
//     (content check). This test adds a SIZE assertion: if the tail is stripped, the
//     total prompt length must be strictly less (guards against "absent in content
//     search but still injected elsewhere" bugs).
```

### `go/internal/phases/retro/retro_compaction_test.go:12` — above `func TestRetroPhase_ConfigHasCompactPrompts(t *testing.T) {`

```text
// retro_compaction_test.go — RED contract for cycle-421 task retro-phase-compaction-wiring.
//
// RED state (before Builder):
//   - retro.Config has no CompactPrompts bool field → reflect FieldByName returns zero Value → FAIL
//   - retro.go line 80 loads agent.Body raw (no StripOnDemandSections call) → bridge gets unstripped body → FAIL
//   - TestRetroPhase_CompactDisabled_BodyIdentical: current behavior is no-strip → pre-existing GREEN
//
// Builder must:
//   1. Add CompactPrompts bool to retro.Config.
//   2. In retro.go, after Agent() load, call StripOnDemandSections(agent.Body) when p.compactPrompts.
//   3. In cmd_cycle.go:358, pass CompactPrompts: cfg.CompactPrompts to retro.New.
```

### `go/internal/phases/retro/retro_dispatch_cli_test.go:3` — above `import (`

```text
// retro_dispatch_cli_test.go — the cycle-107 class, pinned at the DISPATCH
// level this time: retro's hand-rolled runner consulted only EVOLVE_CLI and
// hardcoded claude-tmux, so editing retrospective.json's cli had no effect —
// the 2026-08-26 deep-tier sol arrangement's flagship flip (retro = ~40% of
// deep dispatch volume) was dead on arrival until review reproduced it
// against the dispatched BridgeRequest. These pins assert the REQUEST the
// bridge receives, never the JSON file.
```

### `go/internal/phases/retro/retro_fleet_dispatch_test.go:3` — above `import (`

```text
// retro_fleet_dispatch_test.go — cycle-1270 Task 2
// (`retro-fleet-worktree-dispatch`), the item's literal acceptance criterion:
// a fleet-mode test proving retro's dispatch carries the LANE worktree.
//
// The root cause is already fixed upstream (worktree-provisioning-retry,
// PR #401 / a497ffe1); what stayed open is the regression test, and the gap it
// must close is a JOIN, not another half. retro_worktree_fallback_test.go
// proves retro mints something; bridge's driver_tmux_repl_workdir_test.go
// proves the guard refuses nothing. Neither proves the value that TRAVELS
// between them satisfies the guard's own predicate.
//
// A fabricated path would satisfy "non-empty" and still strand the lane at the
// guard's isDir() check — which is why this asserts on the directory, not on
// the string.
```

### `go/internal/phases/retro/retro_stale_worktree_test.go:3` — above `import (`

```text
// retro_stale_worktree_test.go — cycle-1278 `retro-fleet-stale-worktree-fallback`,
// the verified-open half of the cycle-1255 CRITICAL that the 1255→1268→1270→1272
// salvage chain progressively narrowed to the EMPTY-worktree shape and then
// declared closed (CHANGELOG, 68322bdf).
//
// The gap: retroWorktree substitutes the scratch cwd only when req.Worktree == "".
// A torn-down fleet lane hands it a NON-EMPTY but STALE path (cs.ActiveWorktree is
// never cleared on lane teardown, cyclerun.go:456/471), which passes through
// verbatim and is then refused by the bridge guard's isDir() check
// (driver_tmux_repl.go:123-126, ExitBadFlags, stderr only) — the lane loses its
// retrospective entirely. A failure in the failure-handler.
//
// The invariant these tests pin is the guard's own predicate, not a string shape:
// under fleet mode retro must never hand the bridge a non-empty path that fails
// isDir(). The three input shapes (empty / existing / stale) are one contract;
// TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate already covers
// the empty one, so these cover the other two plus the non-fleet passthrough that
// an over-broad fix would silently break.
```

### `go/internal/phases/retro/retro_test.go:164` — above `if fb.gotReq.ArtifactPath != filepath.Join(ws, "retrospective-report.md") {`

```text
// cycle-187 AC-5/AC-6: retro must poll the file the evolve-retrospective
// agent actually writes — "retrospective-report.md" — not the stale
// "retrospective.md" the runner used before. The mismatch made the bridge
// time out on every retro invocation (Scout Gap B).
```

### `go/internal/phases/retro/retro_test.go:235` — above `got := core.VerdictPASS`

```text
// ADR-0102: a missing review is a missing reasoning (FAIL); a
// complete review is clean (PASS, no advisories).
```

### `go/internal/phases/retro/retro_test.go:753` — above `func TestRun_PreviousFAIL_LostHandoffAdvisoryRidesTheRecord(t *testing.T) {`

```text
// ADR-0102: the review's shape is advisory in retro too — the phase refreshes
// the handoff itself (this fixture has no snapshot, so the finding is the
// missing-handoff one) and every finding rides the retrospective's record as a
// warning while the verdict is untouched.
```

### `go/internal/phases/retro/retro_test.go:808` — above `func TestValidateExplanationReview_BookkeepingFindingsAreAdvisory(t *testing.T) {`

```text
// ADR-0102: retro's bookkeeping findings — an absent Correction todo, a
// duplicated field — are advisories, never a block.
```

### `go/internal/phases/retro/retro_test.go:842` — above `func TestValidateExplanationReview_DuplicateSectionIsAdvisoryButFailsTheFloor(t *testing.T) {`

```text
// ADR-0102: a retrospective with two review sections is a parser finding
// (advisory) with no attributable review text — the reasoning floor fails it.
```

### `go/internal/phases/retro/retro_test.go:853` — above `func TestValidateExplanationReview_NeedsCorrectionTodoRulesAreAdvisoryAndHostDefectsAreLoud(t *testing.T) {`

```text
// ADR-0102 in retro: NEEDS_CORRECTION with "none" or with an unbacked todo is
// advisory; a host-side handoff defect still fails loudly through the gate.
```

### `go/internal/phases/retro/retro_worktree_fallback_test.go:1` — above `package retro`

```text
// RED contract for cycle-1255 task `retro-fleet-worktree-empty-fallback`.
//
// The window: when a fleet lane's worktree is gone (post-teardown) or was never
// provisioned (exhausted retries), retro dispatches with req.Worktree == "". The
// bridge's fleet guard (errWorktreeRequired, driver_tmux_repl.go:27) then refuses
// the launch and the knowledge-capture phase degrades to FAIL-not-adopted every
// time this window is hit.
//
// The fix retro must make: supply its OWN disposable cwd under the workspace it
// already owns (the applyScratchCwd precedent, scratch_cwd.go:22) so the launch
// carries a real, isolated, writable directory. Retro is read-mostly and
// Evaluate-archetype, so a scratch cwd is sufficient for it.
//
// Anti-goals these tests pin, because the naive fixes are the dangerous ones:
//   - NEVER point retro at the shared main tree (req.ProjectRoot) — PR #400 was
//     refuted on exactly that; worktree is the write-authority predicate.
//   - NEVER fall through to the dispatching process's cwd — that is the very
//     leak the fleet guard exists to close.
//   - NEVER widen the bridge guard itself (covered by the untouched
//     driver_tmux_repl_workdir_test.go fleet-refusal tests, which must stay green).
//   - NEVER clobber a real worktree when one WAS provisioned.
```

### `go/internal/phases/retro/retro_worktree_fallback_test.go:76` — above `func TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd(t *testing.T) {`

```text
// TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd — AC2, the NEGATIVE axis.
// The two shapes that would satisfy AC1's "non-empty" letter while reintroducing
// the exact defect the guard exists to prevent: pointing at the shared main tree
// (the refuted PR #400 pattern) or at the dispatching process's cwd.
```
