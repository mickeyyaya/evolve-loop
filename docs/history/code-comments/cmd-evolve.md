# Comment history: `cmd/evolve`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/cmd/evolve/cli_health_canary.go:44` — above `if ctx.Err() != nil {`

```text
// A cancelled probe is not evidence: once the loop's interrupt is in, a
// smoke test that returns "not a wall" would clear a bench that is still
// walled (F20 review). The canary's one cancellation disposition is to
// touch no bench.
```

### `go/cmd/evolve/cli_health_canary_cancel_test.go:12` — above `func TestCanary_CancelledContextTouchesNoBench(t *testing.T) {`

```text
// 2026-09-15 (F20 review): once the canary's probe derives from the loop's
// context, an operator interrupt makes the live smoke test return "not a
// wall" — and the canary's default arm CLEARS the expired bench, promoting a
// family that is still walled and lying about why. A cancelled probe is not
// evidence: the canary touches no bench once the context is done.
```

### `go/cmd/evolve/cli_reachability_check.go:1` — above `package main`

```text
// cli_reachability_check.go wires reachabilityprobe.BuildImportGraph/
// CheckCallSite (landed cycle-1226) into a callable `evolve reachability
// check-pin` subcommand. Before this file, the library had zero non-test
// callers repo-wide, so the cycle-644 failure mode it detects — freezing a
// doNotModifyTests:true structural test pin that is an unbuildable import
// cycle — remained fully reproducible: nothing in the CLI ever ran the check.
```

### `go/cmd/evolve/cli_reachability_check_test.go:1` — above `package main`

```text
// cli_reachability_check_test.go pins the `evolve reachability check-pin`
// wiring gap: reachabilityprobe.BuildImportGraph/CheckCallSite (cycle-1226)
// has zero non-test callers repo-wide, so cycle-644's failure mode (freezing
// a doNotModifyTests:true structural test pin that is an unbuildable import
// cycle) remains fully reproducible today. This test drives the real
// production caller — runReachability, the function wired into the
// dispatcher `commands` table (registry_test asserts the wiring separately)
// — against fixture Go modules built from the REAL toolchain, not a
// hand-built literal ImportGraph (that round-trip is already covered by
// go/acs/cycle1226; this cycle's job is proving the CLI seam exists at all).
```

### `go/cmd/evolve/cli_reachability_check_test.go:39` — above `func writeFixtureModule(t *testing.T, storageImportsCore bool) (root, corePkg, storagePkg string) {`

```text
// writeFixtureModule creates a throwaway Go module on disk with two leaf
// packages, "core" and "storage". When storageImportsCore is true, storage's
// source imports core (a real, toolchain-verifiable edge) — the exact
// cycle-644 shape when core is later pinned to call into storage. When
// false, the two packages are mutually unaware (the false-positive guard
// case). Returns the module root and the two packages' full import paths.
```

### `go/cmd/evolve/cli_reachability_check_test.go:77` — above `func TestReachabilityCheckPin_CyclicFixture_DetectsViolation(t *testing.T) {`

```text
// TestReachabilityCheckPin_CyclicFixture_DetectsViolation is the primary
// positive case (the cycle-644 shape): storage really does import core, so
// pinning core.UpdateStateMap( inside a storage-package file would create an
// import cycle. runReachability must exit non-zero and print a message that
// matches reachabilityprobe.Violation.Error()'s format (names the referenced
// symbol, the pinning package, and "import cycle").
```

### `go/cmd/evolve/cli_usage_probe.go:25` — above `func runPreWaveProbes(ctx context.Context, projectRoot, evolveDir string, env map[string]string, stderr io.Writer) error…`

```text
// runPreWaveProbes is the ONE pre-wave probe protocol both runners follow
// (the loop's prepareIteration and the campaign's BeforeWave hook): the
// CLI-health canary first (clear benches that lifted, re-bench walls), then
// the proactive usage probe (bench families already capped). Both take the
// runner's interrupt context; the returned error is that context's, so a
// caller can stop before dispatching a wave that would be cancelled at spawn
// (F20: "wave 2: 0/2 lanes ok" after the boundary SIGINT).
```

### `go/cmd/evolve/cmd_acs.go:35` — above `func suiteProjectRoot(evolveDir string, cycle int, root string) string {`

```text
// suiteProjectRoot resolves the plane root whose `.evolve/` holds the cycle's
// runtime state (cycle-1434 ADR-0072 halt). The invocation already names it:
// --evolve-dir under the caller's cwd. The anchor is the kernel-owned
// cycle-state.json for THIS cycle — the one file only the orchestrator mints.
// A bare runs/cycle-N directory is NOT proof: `evolve acs run` MkdirAll-mints
// exactly that path under its own cwd, so an agent invoking it from inside a
// cycle worktree would otherwise consecrate the WORKTREE as the plane (review
// HIGH). File presence is the whole proof — its field values don't matter
// here, so an empty active_worktree can never demote a real plane to the git
// fallback. That git common-dir walk (mainProjectRoot) remains only for
// invocations with no plane evolveDir (the issue-#12 shape) — it resolves the
// OWNING repo, which is wrong precisely when the plane is itself a linked
// worktree (all linked worktrees share one common dir, so the walk skips the
// plane and lands on the console checkout).
```

### `go/cmd/evolve/cmd_acs.go:59` — above `func mainProjectRoot(dir string) string {`

```text
// mainProjectRoot resolves the MAIN project root from dir, following a git worktree
// back to its main checkout via --git-common-dir (whose parent is the main root).
// Predicates read `.evolve/` runtime data from there, so the suite must point
// EVOLVE_PROJECT_ROOT at it even when invoked from a worktree (issue #12). Falls
// back to dir when git resolution is unavailable.
```

### `go/cmd/evolve/cmd_acs.go:98` — above `func runACSSuite(args []string, stdout, stderr io.Writer) int {`

```text
// runACSSuite implements `evolve acs suite --cycle N [--root .] [--evolve-dir .evolve]`.
// It runs the Go predicate lane (current cycle + regression + redteam scopes,
// each `go test -tags acs`) and writes acs-verdict.json (EGPS v11; ADR-0042,
// successor to the bash run-acs-suite.sh of ADR-0025). Exit 2 when any predicate
// is RED, 0 when all green, 1 on a hard error (e.g. a predicate package that
// fails to compile — never a silent PASS).
```

### `go/cmd/evolve/cmd_acs.go:124` — above `if root == "." {`

```text
// Auto-resolve suite root from kernel-owned cycle-state.json when --root
// was not explicitly overridden from the default (mode 5, ADR-0025).
```

### `go/cmd/evolve/cmd_acs_adversarial_cycle230_test.go:10` — above `func writeCycleStateN(t *testing.T, evolveDir string, cycle int, body string) {`

```text
// Cycle-230 test-amplification adversarial tests for task acs-suite-root-autosolve.
// Written from spec only (no implementation read) — anti-bias isolation.
//
// Coverage gaps addressed:
//   - JSON null and type-mismatch for active_worktree: graceful fallback to ""
//   - Path values with spaces/special chars: resolver must return verbatim
//   - Wrong cycle number passed: resolver constructs path from cycle param; if
//     the state file for the requested cycle doesn't exist, it must return ""
```

### `go/cmd/evolve/cmd_acs_adversarial_cycle230_test.go:19` — above `func writeCycleStateN(t *testing.T, evolveDir string, cycle int, body string) {`

```text
// writeCycleStateN writes a cycle-state.json for an arbitrary cycle number.
// Unlike writeCycleState (TDD helper hardcoded to cycle-230), this respects the
// cycle parameter — needed to test that the resolver uses the cycle param correctly.
```

### `go/cmd/evolve/cmd_acs_adversarial_cycle230_test.go:81` — above `func TestACSSuiteRootAutosolve_WrongCycle_Amp(t *testing.T) {`

```text
// TestACSSuiteRootAutosolve_WrongCycle_Amp: the resolver constructs its file path
// from the cycle parameter. When cycle=230 is requested but only cycle=229's
// state file exists, the resolver must return "" (file-not-found fallback), NOT
// bleed the cycle-229 active_worktree value.
```

### `go/cmd/evolve/cmd_acs_adversarial_cycle230_test.go:87` — above `writeCycleStateN(t, evolveDir, 229, '{"active_worktree":"/tmp/worktrees/cycle-229"}')`

```text
// Write state only for cycle 229 — cycle 230 has no file
```

### `go/cmd/evolve/cmd_acs_projectroot_test.go:3` — above `import (`

```text
// cmd_acs_projectroot_test.go — RED contract for the cycle-1434 ADR-0072 halt
// (auto-filed P0): `evolve acs suite` derived EVOLVE_PROJECT_ROOT via
// mainProjectRoot (git --git-common-dir parent), which resolves to the OWNING
// repo. Correct when a cycle worktree's owner IS the plane; wrong the moment
// the plane is itself a linked worktree — all linked worktrees share one
// common dir, so the derivation skipped the plane and landed on the console
// checkout, whose .evolve has none of this cycle's run state. Three predicates
// red'd against the wrong state root while the audit phase's correct-root run
// was 8/8 green, and the CLI-written artifact won.
//
// The invocation already names the correct plane: --evolve-dir (default
// ".evolve" under the caller's cwd, which the audit persona pins to the plane
// root). suiteProjectRoot anchors on it whenever it actually holds this
// cycle's run — kernel-owned proof it is the plane — and falls back to the
// git derivation for invocations with no plane evolveDir (issue #12 shape).
```

### `go/cmd/evolve/cmd_acs_projectroot_test.go:44` — above `plane := filepath.Join(base, "plane")`

```text
// The plane is a LINKED worktree of the console repo (the live topology:
// evolve-loop-runtime is a worktree of evolve-loop) and holds the runtime
// state for cycle 7.
```

### `go/cmd/evolve/cmd_acs_test.go:9` — above `func writeCycleState(t *testing.T, evolveDir string, cycle int, body string) {`

```text
// Cycle-230 task acs-suite-root-autosolve (mode 5 of
// user-phase-persona-resolution): `evolve acs suite` must resolve its suite
// root from the kernel-owned cycle state instead of trusting the caller's cwd,
// eliminating the LLM-topology nondeterminism that false-FAILed cycles 226-227.
//
// Contract under test: resolveACSSuiteRoot(evolveDir string, cycle int) string
//   - reads <evolveDir>/runs/cycle-<N>/cycle-state.json
//   - returns its non-empty "active_worktree" value
//   - returns "" when the file is absent, malformed, or the field is empty
//     (caller then falls back to the --root flag default)
//
// NOTE (TDD): the cycle param is int (not string as sketched in
// scout-report.md Build Plan §3) — it must match the existing --cycle
// flag.IntVar in runACSSuite; a string param would force lossy round-trips.
//
// DO NOT MODIFY (builder contract): implement resolveACSSuiteRoot in
// cmd_acs.go and wire it into runACSSuite when --root is not explicitly set.
```

### `go/cmd/evolve/cmd_acs_test.go:31` — above `_ = cycle`

```text
// path is fixed to cycle-230 fixtures in these tests
```

### `go/cmd/evolve/cmd_branches.go:1` — above `package main`

```text
// `evolve branches` audits and prunes stale orphan `cycle-*` branches, giving
// the two cycle-962 core exports their first live production caller
// (core.PruneSupersededOrphans + core.CarryforwardCandidateLandable were shipped
// fully-tested but callerless — the inert-API gap this cycle closes).
//
// Subcommands (mirrors runWorktree's dispatch shape):
//
//	audit  — read-only. Per local cycle-* branch prints
//	         `<ref> superseded=<t|f> landable=<t|f>`, dispatching to BOTH core
//	         functions. Never deletes.
//	prune  — walks the same refs. Default is dry-run: superseded refs are
//	         reported `would-prune`, nothing is deleted. With --dry-run=false a
//	         superseded ref is deleted (`pruned`) ONLY when hasOpenPR reports
//	         false — honoring verify_remote_pr_before_branch_delete.
```

### `go/cmd/evolve/cmd_branches_test.go:3` — above `import (`

```text
// cmd_branches_test.go — TDD red-first unit contract for the `evolve branches`
// subcommand (cycle-969, wire-carryforward-prune-cli). These tests call
// runBranches directly against a real temp git repo (no remote), so they run in
// the normal suite (`go test ./cmd/evolve/...`) with no `acs` build tag and no
// binary build — the fast red/green loop the Builder codes against. The
// worktree-gating, durable predicates live in go/acs/cycle969/predicates_test.go.
//
// RED before the Builder acts: runBranches is undefined, so this file fails to
// COMPILE — the whole package test reds for the right reason (the SUT is absent).
//
// The Builder must NOT modify this file; it adds go/cmd/evolve/cmd_branches.go
// (runBranches) and the registry.go row. Contract mirrored from the ACS package:
//   - audit  → read-only; per branch prints `superseded=<t|f> landable=<t|f>`
//              (dispatching to core.PruneSupersededOrphans AND
//              core.CarryforwardCandidateLandable).
//   - prune  → default dry-run (deletes nothing, flags `would-prune`);
//              --dry-run=false deletes each superseded ref whose hasOpenPR is
//              false. With no remote configured hasOpenPR MUST degrade to
//              (false, nil) (verify_remote_pr_before_branch_delete).
```

### `go/cmd/evolve/cmd_branches_test.go:49` — above `func brFixture(t *testing.T) string {`

```text
// brFixture builds a remote-less repo on `main` with a superseded (ancestor)
// cycle-100 and a divergent-clean cycle-200 — same shape as the ACS fixture.
```

### `go/cmd/evolve/cmd_bridge_engine_roots_test.go:1` — above `package main`

```text
// cmd_bridge_engine_roots_test.go — ADR-0103 unit 10 (design §6 tests 49-50):
// every non-test bridge.NewEngine call in the module is enumerated with its
// Signal Center wiring, so the roots where the unit's BRIDGE_EXIT_* and step
// codes fall into the nil Null Object are on record rather than implicit; and
// the --simulate root renders a launch death on the console and in the
// cycle-workspace signals.ndjson.
```

### `go/cmd/evolve/cmd_campaign.go:168` — above `if !*simulate {`

```text
// Cross-session ownership lease (ADR-0059): a real run takes the exclusive
// goal-hash lease in the git common dir (shared by every worktree) so a
// second autonomous session on the SAME plan refuses-or-attaches instead of
// clobbering the incumbent. --simulate is a dry plumbing check, not an owned
// run, so it does not take ownership. The flock frees on our exit (defer) or
// death, so a dead owner never blocks the next run.
```

### `go/cmd/evolve/cmd_campaign.go:264` — above `func campaignLeaseDir(projectRoot string) string {`

```text
// campaignLeaseDir resolves the directory that holds the cross-session ownership
// lease (ADR-0059). It uses the git COMMON dir — shared by every linked worktree
// of a repo — so two sessions running the same plan from different worktrees
// contend on the SAME lease file. Off a git repo (tests, non-repo roots) it
// falls back to the worktree-local .evolve so each isolated root self-contains.
```

### `go/cmd/evolve/cmd_campaign.go:397` — above `Bridge:  bridge.NewDefault(projectRoot, nil),`

```text
// Center-less registry default (ADR-0101 S3)
```

### `go/cmd/evolve/cmd_campaign_lease_test.go:82` — above `func TestCampaignRun_SimulateSkipsOwnershipLease(t *testing.T) {`

```text
// TestCampaignRun_SimulateSkipsOwnershipLease proves --simulate (a dry plumbing
// check, not an owned run) does NOT take the lease: it runs to completion even
// when an incumbent holds the goal-hash lease. Guards the ADR-0059 decision
// against a refactor that moves the `if !*simulate` guard.
```

### `go/cmd/evolve/cmd_campaign_resume_test.go:17` — above `func TestCampaignRun_AutoResumeSkipsCompletedWaves(t *testing.T) {`

```text
// TestCampaignRun_AutoResumeSkipsCompletedWaves drives the full cmd wiring
// (goal-hash, .evolve progress path, PlanSHA binding, RunWaves call) through the
// campaignLaunchFactory DI seam: with wave 0 pre-recorded complete, only wave 1
// should launch, and the status command should report both waves done afterward.
```

### `go/cmd/evolve/cmd_campaign_resume_test.go:34` — above `plan, err := loadVerifiedCampaignPlan(planPath)`

```text
// Pre-seed progress: wave 0 (cycle a) already shipped, bound to THIS plan.
```

### `go/cmd/evolve/cmd_carryover.go:1` — above `package main`

```text
// `evolve carryover apply-decisions` applies a reviewed keep/drop/cluster
// decisions file (authored by the cycle-997 carryover-consolidation-sweep pass)
// to state.json:carryoverTodos through the SANCTIONED locked read-modify-write
// path (flock.WithPathLock on the `<statePath>.lock` sidecar) — the same
// single-writer contract cmd_loop.go's auto-prune block and reset.go already
// honour. It is the missing link the inbox item names: the TTL prune machinery
// (failurelog.PruneExpiredCarryoverTodos) can only remove entries whose
// expiresAt is already past; it cannot act on a semantic keep/drop/cluster
// judgment. This command does.
//
// Semantics:
//   - `drop`    ids are removed from carryoverTodos (stale failure echoes /
//     landed duplicate shadows).
//   - `cluster` ids are ALSO removed — they have been re-filed as amortised
//     sweep-group inbox items (Task 3), so leaving them in carryoverTodos would
//     double-count them.
//   - `keep`    ids stay resident (genuinely-live small items).
//
// Guards:
//   - Every decision row MUST carry a non-empty reason. A single empty-reason
//     row aborts the whole apply BEFORE any lock is taken or byte is written —
//     the anti-hand-edit / anti-unjustified-drop contract (state.json is left
//     exactly as-is).
//   - The write is atomic (temp + rename) and serialized by the sidecar lock,
//     so concurrent callers never corrupt the array.
```

### `go/cmd/evolve/cmd_carryover.go:214` — above `var res carryoverApplyResult`

```text
// The locked RMW goes through statemap (cycle-999/1001 fixes): the path is
// symlink-resolved so a worktree link writes THROUGH to canonical and
// survives; the lock is taken on the resolved path (one lock per data
// file, cross-tree); stateRevision auto-bumps and a stale write is refused.
```

### `go/cmd/evolve/cmd_carryover_amplify_test.go:11` — above `func TestCarryoverApplyDecisions_DryRunDoesNotMutateState(t *testing.T) {`

```text
// Test-amplification pass for cycle-998 (carryover-decisions-authoring /
// carryover-sweep-group-filer). Written black-box against the CLI contract
// documented in tdd-report.md / build-report.md — the `-decisions`/`-state`/
// `-apply` flag surface (`evolve carryover apply-decisions --help`) and the
// exported test helpers already established in cmd_carryover_test.go — WITHOUT
// reading cmd_carryover.go's implementation. Targets adversarial edges the
// existing suite (registration, happy-path-to-ceiling, missing-reason,
// locked-RMW) does not: dry-run mutation safety, malformed input shape,
// duplicate/unknown ids, empty input, and large-scale volume.
```

### `go/cmd/evolve/cmd_carryover_test.go:227` — above `func TestCarryoverApplyDecisions_PreservesStateSymlink(t *testing.T) {`

```text
// TestCarryoverApplyDecisions_PreservesStateSymlink is the cycle-999
// regression pin: applying decisions through a WORKTREE-style symlinked
// state.json must write THROUGH to the canonical target and leave the link
// intact — the pre-fix hand-rolled temp+rename replaced the link with a
// detached regular file, stranding the 135->14 convergence in a dead copy.
```

### `go/cmd/evolve/cmd_compose_test.go:172` — above `func TestCompose_ExportsComposeSignal(t *testing.T) {`

```text
// TestCompose_ExportsComposeSignal — PhaseRequest.ComposePhases is true
// when the factory is called from evolve compose (cycle-10: replaced the
// retired EVOLVE_COMPOSE_PHASES env signal with a DI bool).
```

### `go/cmd/evolve/cmd_composition_apicover_gap_test.go:15` — above `func TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport(t *testing.T) {`

```text
// TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport reproduces
// percycle-audit-apicover-newexport-parity: the recurring "apicover RED on
// main" incidents (3 live recurrences 2026-07-20, fixed after the fact by
// c37dc324/46ff77f6 "close apicover gap on 3 orphaned internal/core exports",
// 9eacd83f "name+exercise fleet-rebase classify surface — fixes repo-wide
// apicover RED on main (recovered-commit export gap)", and aaeb4d4d
// "LandPrefixes naming test (apicover un-RED)").
//
// The RUNG0/RUNG2 fleet-rebase carry-forward fast path
// (internal/core/composition_carryforward.go: compositionCarryForward /
// scopedMergeCarryForward) reships straight to main — skipping a full
// re-audit — whenever runComposedGates reports every gate in
// ciparity.RequiredComposedGates (which includes "apicover") as "pass".
// composedGateTargets maps that "apicover" gate to the go/Makefile `apicover`
// target. But that target is Phase-0 WARNING-ONLY (go/Makefile:127 comment:
// "warning-only; ... -enforce in Phase 5"; its recipe at line 132 never
// passes -enforce), unlike CI's separate Phase-5 "api-coverage enforce" step
// (.github/workflows/go.yml:99-116) which does. apicover.Run only returns a
// non-zero exit when cfg.Enforce is true (internal/apicover/run.go) — so the
// Makefile target's bare `bin/apicover -cover ...` invocation ALWAYS exits 0,
// regardless of how many exported symbols are uncovered.
//
// Net effect: when a fleet-rebase folds in a peer lane's already-landed
// commit that introduced a brand-new, still-unnamed exported symbol (exactly
// what FleetRebaseVerdict/ClassifyFleetRebaseCandidate and LandPrefixes were
// when they landed), THIS lane's own apicoverEnforceChangedDefault never
// looks at it (it wasn't part of this lane's own changed-package diff), and
// the composed-gate re-check that's supposed to be the last line of defense
// reports "apicover: pass" unconditionally — so the carry-forward reships the
// gap to main, where only the separate repo-wide CI enforce step (not
// reproduced anywhere in the composed-gate set) eventually catches it.
//
// This test proves the gap directly: it adds a throwaway package containing
// one exported, zero-coverage, never-named function, then runs the EXACT
// Makefile target composedGateTargets["apicover"] names (scoped to just the
// fixture package via APICOVER_PKGS, so the run stays fast) and shows it
// exits 0 — while the real enforcing check (apicover.Run with Enforce:true)
// correctly flags the same package as having an uncovered export. A fix that
// closes the gap (e.g. pointing composedGateTargets["apicover"] at a real
// enforcing recipe) will make the Makefile-target run in this test also
// fail, at which point this test's core assertion flips to green.
```

### `go/cmd/evolve/cmd_composition_apicover_gap_test.go:57` — above `t.Skip("reproduction PERMANENTLY disabled: it mutates the live repo tree and poisons the CI coverage profile. The gap it…`

```text
// DISABLED until percycle-audit-apicover-newexport-parity (0.94) redesigns
// it: this reproduction MUTATES THE LIVE REPO TREE — it creates
// internal/apicoverreprofixture998 in-tree, shells out to `make -C go
// apicover` (which regenerates coverage.txt, poisoning the CI profile with
// a package the cleanup then deletes), and broke the `go` workflow's
// cover -func step on BOTH platforms (2026-07-21, commit 79ead521: "cover:
// cannot run go list: fork/exec ...: invalid argument"). The skip must sit
// BEFORE any side effect. The fix cycle must rebuild this against a
// throwaway COPY of the module (temp dir), never the live tree, and flip
// the core assertion to t.Fatalf as its regression pin.
```

### `go/cmd/evolve/cmd_composition_apicover_gap_test.go:120` — above `t.Skipf(`

```text
// KNOWN BUG, queued as percycle-audit-apicover-newexport-parity (0.94).
// This reproduction shipped ahead of its fix (cycle-998) and held main
// RED — a red-first proof belongs in the FIX's cycle, so until that
// lands this branch is a loud SKIP tripwire, not a failure. THE FIX
// CYCLE MUST flip this t.Skipf back to t.Fatalf as its regression pin.
```

### `go/cmd/evolve/cmd_composition_apicover_wiring_test.go:3` — above `import (`

```text
// The composed-gate "apicover" entry must name an ENFORCING Makefile recipe.
// Six recurrences of warnship-apicover-ci-gap trace to this one map entry
// pointing at the Phase-0 warning-only target: apicover.Run exits non-zero
// ONLY under -enforce, so the composed-gate re-check reported "apicover: pass"
// unconditionally and the fleet-rebase carry-forward reshipped uncovered
// exports to main, where repo-wide CI (the delayed detector) went RED.
//
// The pin is on the RECIPE TEXT, not a live run: it proves the wiring without
// mutating the tree (the cycle-998 reproduction poisoned CI's coverage profile
// doing that) and fails if the map is ever repointed at a non-enforcing
// target, or the target's -enforce flag is "simplified" away.
```

### `go/cmd/evolve/cmd_composition_runscope_test.go:3` — above `import (`

```text
// cmd_composition_runscope_test.go — pins the run-scoping of the composition
// snapshot's ledger reader (the third "latest auditor entry" consumer, found
// by the cycle-1571 H3 review sweep). The ledger is host-global across fleet
// worktrees and, since the H3 producer fix, contains auditor entries for FAIL
// verdicts too — so an unscoped "latest" can hand the RUNG 0 carry-forward a
// sibling lane's (or a FAILed) audit as "the audited snapshot". Same contract
// as ship.findLatestAudit: runID set ⇒ exact match or error; runID=="" keeps
// latest-any.
```

### `go/cmd/evolve/cmd_composition_verdict_guard_test.go:11` — above `func writeArtifact(t *testing.T, verdict string) string {`

```text
// cmd_composition_verdict_guard_test.go — cycle-1571 H2.
//
// PR #503 run-scoped latestAuditEntry and its own comment named the full
// hazard: an unscoped lookup "can be a sibling lane's — OR A FAILED — audit".
// The filter it added closed only the sibling half (Kind/Role/GitHEAD/RunID);
// no verdict was consulted. So a FAILed audit from THIS run was still taken as
// "the audited snapshot", and RUNG 0 carry-forward would run the entire
// composed-tree gate set and write a composition-verdict record certifying the
// carry-forward of a REJECTION. Ship blocks the result downstream, so the cost
// is a wasted gate pass plus a dishonest entry in a hash-chained ledger.
//
// The verdict lives in the bound artifact, not the ledger: exit_code is 1 for
// WARN and FAIL alike, and phase_bindings.go states the artifact is where the
// severity lives. So the guard reads the artifact the entry already points at.
```

### `go/cmd/evolve/cmd_composition_wiring.go:149` — above `func latestAuditEntry(ledgerPath, runID string) (auditLedgerEntry, error) {`

```text
// latestAuditEntry walks ledger.jsonl backwards for the most recent bound
// auditor entry OF THIS RUN. Run-scoped since 2026-08-26, same hardening as
// ship.findLatestAudit (whose old cross-run fallback was cycle-1571's H3
// fail-open hole): the ledger is host-global across fleet worktrees, and the
// producer now records auditor entries for FAIL verdicts too, so an unscoped
// "latest" can be a sibling lane's — or a FAILed — audit. runID=="" (no run
// context) keeps latest-any. findCompositionVerdict's LaneAuditRef equality
// against this run's own bound artifact remains the downstream safety net
// either way. Alien/unparseable lines are skipped; a miss is an error the
// snapshot surfaces so compositionCarryForward fails closed to full re-audit.
```

### `go/cmd/evolve/cmd_consensus_dispatch.go:27` — above `projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")`

```text
// Resolve script-relative defaults from the legacy/scripts/ tree. envOrCwd
// absolutizes a relative $EVOLVE_PROJECT_ROOT (cycle-119 class) + falls back
// to cwd.
```

### `go/cmd/evolve/cmd_continuation.go:3` — above `import (`

```text
// cmd_continuation.go — `evolve continuation list` / `evolve continuation
// release <scope-id>`: the operator surface for the scope-keyed continuation
// registry.
//
// The registry is written under a flock sidecar by the runtime, and until now
// the only way for console to inspect or drop a stale binding was to hand-edit
// .evolve/continuation-registry.json — outside that lock, with no preservation
// of the salvage pointer it destroyed. Both subcommands reach the SAME paths
// the runtime uses (continuation.ListRegistryEntries for the read,
// inboxmover.ReleaseContinuationBinding for the preserve-then-delete
// transaction) rather than re-implementing them here; a second copy of the
// release order is exactly the drift that produced audit cycle-1507's H2.
```

### `go/cmd/evolve/cmd_cycle.go:111` — above `projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {`

```text
// Absolutize the root so SealCycle's "workspace inside projectRoot" check
// compares absolute-to-absolute. A relative root here refused to seal
// cycle-120 (whose workspace_path was absolute, written by the fixed loop).
```

### `go/cmd/evolve/cmd_cycle.go:121` — above `res, err := core.SealCycle(context.Background(), ledger.New(evolveDir), core.SealOptions{`

```text
// The liveness fence lives in SealCycle — it reads the per-run .lease
// heartbeat (the SSOT for "is the owner alive?"). The OLD `.evolve/.lock`
// pre-check here was a false negative that caused the cycle-395 race: the
// dispatcher's lock is per-CYCLE (released between cycles), so a sibling
// reset acquired it in the gap and sealed a RUNNING loop. We pass Force
// through and let the heartbeat-backed fence decide.
```

### `go/cmd/evolve/cmd_cycle.go:133` — above `PidAlive: pidAlive,`

```text
// PID-aware liveness (cycle-554): a crashed owner whose heartbeat has not
// yet aged past the TTL (the 2-6min post-crash window) is no longer "live",
// so a plain `evolve cycle reset` seals it WITHOUT --force. A genuinely
// running owner (alive pid) still refuses. Same probe boot recovery uses.
```

### `go/cmd/evolve/cmd_cycle.go:201` — above `projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {`

```text
// Absolutize before any path is derived from projectRoot — a relative root
// (default ".") makes worktree-phase artifact paths diverge across the
// agent's worktree cwd and the in-process bridge's main cwd (cycle-119).
```

### `go/cmd/evolve/cmd_cycle.go:218` — above `var d orchDeps`

```text
// Both roots share the deps shape and the signal topology
// (newRootSignalCenter); --simulate differs in its runners (stubs), its
// worktree provisioner (the root, in place) and its dossier-commit knob
// (files only), and has no bridge. Unit 01 (ADR-0103): a Center-less simulate root had silenced the
// recorder's warnings, so the Null-Object root is gone.
```

### `go/cmd/evolve/cmd_cycle.go:231` — above `defer d.Signals.Flush()`

```text
// ADR-0101 S2a: no queued signal is lost at command exit (Center.Flush).
```

### `go/cmd/evolve/cmd_cycle.go:243` — above `var clf *core.ErrCycleLevelFailure`

```text
// A cycle-level FAIL must reach the inbox lifecycle from HERE too: every
// fleet lane runs this entrypoint as a subprocess, and fleet.Result
// carries neither the lane's cycle number nor its workspace — so the
// parent wave loop structurally cannot apply a lane's verdict. Applying
// it in-process (where cycle + workspace are local) mirrors the PASS half
// in phases/ship/postship.go and is what makes the ADR-0072 S5 retry
// ceiling reachable for fleet-dispatched work at all.
```

### `go/cmd/evolve/cmd_cycle.go:259` — above `if sf := result.SystemFailure; sf != nil && sf.Halt {`

```text
// ADR-0072 (adr0072-fleet-halt-unwired): every fleet lane runs THIS
// entrypoint as a subprocess, and its exit code is the ONLY channel back to
// the parent wave loop. A halting SystemFailure must therefore surface as a
// distinct exit code (not the ambiguous rc=2 FAIL) so the parent can halt the
// batch — and it writes the escalation dossier + P0 inbox item HERE, via the
// same shared helper the sequential path uses, so the breadcrumb exists even
// when the halt originated inside a lane. Ordinary outcomes keep the historical
// rc=2/0 mapping (cycleRunExitCode).
```

### `go/cmd/evolve/cmd_cycle.go:279` — above `func applyCycleFailureOutcome(projectRoot, evolveDir string, cycle int, stderr io.Writer, lifecycle inboxmover.LedgerApp…`

```text
// applyCycleFailureOutcome walks the failed cycle's triage-committed ids
// through the inbox failure lifecycle (bump failure_count, quarantine at the S5
// ceiling) via the shared seam — the ONE call every root makes (the cycle-run
// root and both sequential loop paths). The walk appends its lifecycle lines
// through the root's ledger so the Signal Center observes them like every
// other entry (ADR-0101 S4a); a nil ledger (the --simulate root) lets the
// mover fall back to its own, unobserved file ledger. The walk's own faults
// (ADR-0103 unit 06: a park that could not deliver, a double-move, a stamp
// that could not land) reach the root's Signal Center through signals, so
// they land in the cycle workspace's signals.ndjson beside the ledger events
// instead of on stderr alone. The error returns for the caller to WARN in
// its own voice: a lifecycle hiccup never changes a cycle's exit code (the
// lane's only channel to its parent) or a batch's flow.
```

### `go/cmd/evolve/cmd_cycle.go:325` — above `Signals *signalcenter.Center`

```text
// Signals is the ADR-0101 Signal Center: constructed here, before the
// bridge, always (a nil Center is a test affordance only). No production
// reader yet, by design: the bridge receives it at construction in S3 and
// cmd_loop reads Orchestrator.SignalSummary() for the batch report in S4.
```

### `go/cmd/evolve/cmd_cycle.go:330` — above `Bridge *bridge.Adapter`

```text
// Bridge is the production Adapter injected into every phase runner; it
// carries Signals into each engine it builds (ADR-0101 S3).
```

### `go/cmd/evolve/cmd_cycle.go:333` — above `Runners map[core.Phase]core.PhaseRunner`

```text
// Runners is the phase-runner map the orchestrator was built over — a root
// field for the per-runner wiring proof (ADR-0103 unit 11: every
// BaseRunner-backed runner's verdict engine reaches Signals through the
// Bridge), in the orchDeps.Bridge precedent; no production reader.
```

### `go/cmd/evolve/cmd_cycle.go:349` — above `func newRootSignalCenter(projectRoot, evolveDir string, console io.Writer) *signalcenter.Center {`

```text
// newRootSignalCenter is the ONE sink topology every root builds — and the
// loop tests' stub root, so a test that asserts a rendered line proves
// production's topology (ADR-0101 S4a). Listeners: the durable
// signals.ndjson — per cycle workspace for cycle-scoped signals, and
// <evolveDir>/signals.ndjson for batch-level (cycle-less) ones: the loop's
// own halts and wave summaries, a bridge warning before any cycle — and the
// console at WARN and above (signalcenter.ConsoleSink, the one home of that
// threshold; the severity contract's "log only" INFO tier stays in the files).
```

### `go/cmd/evolve/cmd_cycle.go:382` — above `signals := newRootSignalCenter(projectRoot, evolveDir, console)`

```text
// ADR-0101 S1: the Signal Center is built FIRST (the bridge receives it at
// construction in S3 — Deps normalize inside NewEngine) and unconditionally:
// TestNilSignalCenterRootsArePinned lists the only roots allowed to skip it.
// The orchestrator subscribes via WithSignalCenter.
```

### `go/cmd/evolve/cmd_cycle.go:387` — above `st := storage.New(evolveDir)`

```text
// Ledger and storage come next, so the bridge adapter can wire its
// stop-review callback to append kind=stop_review entries (ADR-0026 Stage 1 #5);
// the ledger is observed at its append chokepoint so every appended entry
// is also a ledger.appended signal (ADR-0101 S4a — the file ledger still
// chains and locks; WithSignals is a construction option, not a wrapper).
```

### `go/cmd/evolve/cmd_cycle.go:408` — above `registryPath := config.RegistryPath(projectRoot)`

```text
// Composition root: the SOLE reader of routing env+config. The unit-08
// Loader (cmd_cycle_config.go) maps the central registry + contained env
// overrides into one RoutingConfig; router.Select picks the brain once. With
// dynamic_routing=0 (Stage:Off, the escape hatch; advisory is the
// default since 2026-06-06) NewOrchestrator behaves exactly as before. A nil proposer means DynamicLLM degrades to the deterministic
// StaticPreset (the bridge-backed Proposer is a tracked follow-on).
// Loaded BEFORE the runners map so cfg.PhaseIO can thread into the
// build/scout/triage reconcile rung (ADR-0050 §3.10 Slice 1).
// Every warning the Loader resolves rides the Center as config.warning
// (the root StderrSink renders WARN; the cycle-less durable sink files it);
// the discarded slice is the same data — nothing to print twice. The path
// stays a local: phasespec.Load below reads the same file.
```

### `go/cmd/evolve/cmd_cycle.go:502` — above `diagf := func(format string, args ...any) { fmt.Fprintf(os.Stderr, format, args...) }`

```text
// bridgechain: the CLI/tier fallback chain is a property of the bridge HANDLE
// (Decorator, wrapped once here). Every consumer below launches through
// `walked`: the retro, the debugger, the spec runners, the registrar, the
// failure advisor and the swarm launcher get the walk by construction; the
// runner and the advisor walk their own chains and mark each attempt, so
// they pass straight through. Lanes 1676/1677 (2026-09-14): the retro's
// direct launch had no chain and one codex timeout sealed the cycle.
```

### `go/cmd/evolve/cmd_cycle.go:523` — above `var contractVerifier runner.ContractVerifier = deliverable.PlainVerifier{PhaseIO: cfg.PhaseIO}`

```text
// The contract gate's Reviewer is built after the runners (it needs the
// merged phase catalog); every BaseRunner's verdict engine reaches it
// through this accessor so gate and engine share ONE verifier — the bytes
// the engine classifies are the bytes the gate approves (research F22:
// cycle 1685 classified the unrepaired bytes the gate then salvaged).
// Gate OFF keeps the Null-Object PlainVerifier (no salvage) so the choice
// is explicit; the gate's Reviewer replaces it below when the gate is on.
```

### `go/cmd/evolve/cmd_cycle.go:535` — above `core.PhaseScout:        swarmrunner.New(scout.New(scout.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierO…`

```text
// Scout + Build are swarm-eligible (ADR-0032): wrapped in the swarmRunner
// Decorator so stage=advisory|enforce (policy.json "swarm.stage") dispatches
// them across N parallel workers (reader fan-out / writer merge-train).
// Default (stage absent/shadow) = byte-identical delegate — zero behavior change.
// PhaseIO threads cfg.PhaseIO into the reconcile rung (3.10 Slice 1); StageOff
// (the shipping default) keeps these byte-identical.
```

### `go/cmd/evolve/cmd_cycle.go:547` — above `core.PhaseShip:  ship.New(ship.Config{Runner: sysexec.DefaultRunner, PhaseIO: cfg.PhaseIO, ManifestGate: gatesCfg.Manife…`

```text
// ManifestGate is threaded from policy.json `gates.manifest_gate` (default
// "shadow") so the ship-bind manifest gate is operator-activatable — it was
// unreachable short of a code edit before cycle-1064.
```

### `go/cmd/evolve/cmd_cycle.go:576` — above `br.SetContractResolver(phasecontract.NewCatalogResolver(catalog.Get))`

```text
// Make the bridge catalog-aware so user/minted phases get their spec-derived
// Deliverable Contract block + exact-path footer injected (WS-A, ADR-0034).
```

### `go/cmd/evolve/cmd_cycle.go:579` — above `wireBridgeStages(br, cfg)`

```text
// …and keep it aware: `catalog.Get` above is a method value bound to THIS
// catalog VALUE, whose byName map is the pre-mint one. A phase minted
// mid-cycle is spliced into a NEW catalog value (Catalog.Merge allocates a
// fresh map), so without the publisher below the resolver misses the minted
// phase for the rest of the cycle and dispatch falls back to the
// unresolved-agent path — the cycle-1424 600s artifact-timeout halt. The
// orchestrator option is appended beside core.WithRegistrar (see below).
// ADR-0050 §3.8b: at EVOLVE_PHASE_IO>=advisory the injected contract block
// instructs build/scout/triage to self-report failure via a structured
// sentinel; default (off) leaves the dispatched prompt byte-identical.
```

### `go/cmd/evolve/cmd_cycle.go:593` — above `for _, s := range catalog.UserPhases() {`

```text
// Register a spec-driven runner for each valid user phase. This MUST use the
// same catalog-aware validator as ApplyUserRouting above (line ~399): the
// bare ValidateUserSpec re-imposes the two-tier single-word naming floor that
// ValidateUserSpecWithCatalog exempts for optional built-ins (e.g. "memo"),
// so routing would nominate the phase while dispatch silently dropped it —
// the cycle-563 memo-dispatch bug. Keep these two call sites in lockstep.
```

### `go/cmd/evolve/cmd_cycle.go:645` — above `opts := []core.Option{`

```text
// The same advisor also produces the upfront whole-cycle plan the integrity
// floor clamps (ADR-0024 §2). Wire it unconditionally: the orchestrator is the
// single gate — it consults the planner only at Stage>=Advisory AND
// Mode==DynamicLLM, so in static mode or below Advisory it is never called
// (no LLM cost), and the kernel falls back to the configurable spine.
```

### `go/cmd/evolve/cmd_cycle.go:654` — above `core.WithFailureAdviser(core.NewFailureAdvisor(walked, failureAdvisorOpts(projectRoot)...)),`

```text
// ADR-0044 Slice-6 post-soak step (R8.1): the LLM failure-advisor
// tail. Wired unconditionally — the hook is enforce-gated
// (cfg.PhaseRecovery) and best-effort, so below enforce it never
// dispatches; at enforce it turns one unclassified fatal pane into a
// validated promotion (each promotion saves ~20 min of maxExtends
// burn on every future occurrence).
```

### `go/cmd/evolve/cmd_cycle.go:665` — above `core.WithCatalogPublisher(catalogPublisher(br)),`

```text
// Mint advisor-proposed phases (Steps 11/12): persist their dispatch
// profile + spec under .evolve so the unchanged runner resolves them
// from disk, then dispatch by name like a built-in. Only active when the
// advisor drives (Stage>=Advisory) and a plan carries MintPhases.
// RegistryPath anchors to projectRoot (not evolveDir) because the
// tree-diff guard reads mintregistry.Path(ProjectRoot) — the guard only
// sees writes under the project tree, so write and read must agree
// there (cycle-967 Variant A2).
// Re-bind the bridge's deliverable-contract resolver on every mid-cycle
// mint, so a phase minted at Step 11/12 resolves its spec-derived contract
// in the SAME cycle (cycle-1429; the miss #429 only made safe).
```

### `go/cmd/evolve/cmd_cycle.go:685` — above `observerCfg := pol.ObserverConfig()`

```text
// Cycle-122 Fix 3 / ADR-0030: auto-spawn the per-phase observer
// goroutine unless explicitly disabled in policy.json.
// Restores the pre-v12 bash-dispatcher behavior the Go port silently
// dropped. The default StallS=600s matches the bridge's coarse
// artifact-timeout.
```

### `go/cmd/evolve/cmd_cycle.go:694` — above `ca.Signals = func() *signalcenter.Center { return signals }`

```text
// ADR-0103 unit 12: the adapter's own faults are observer.warning signals
```

### `go/cmd/evolve/cmd_cycle.go:697` — above `var reviewers []core.DeliverableReviewer`

```text
// Structural eval gates (internal/evalgate): Gate A (scout eval-file
// materialization) + Gate B (tdd predicate-quality), mounted at the
// per-phase DeliverableReviewer seam. Default enforce (config.defaults);
// policy.gates.eval_gate=off keeps the noopReviewer default.
// The gates fail open on any ambiguity, so enforce never false-blocks.
// Compose the structural eval gates with the deliverable-contract gate
// (internal/deliverable, ADR-0034) behind ONE reviewer via ChainReviewers —
// WithReviewer sets a single reviewer, so both gates must be chained at the
// same seam. Each is gated independently (default enforce); both fail open on
// ambiguity. With both off, no reviewer is wired (noopReviewer; byte-identical).
```

### `go/cmd/evolve/cmd_cycle.go:708` — above `if pol.WorkflowConfig().BuildFloorEnforced {`

```text
// Build handoff floor (2026-07-21 shift-left): deterministic self-check
// REJECTS a red build deliverable so the E2 correction ladder fixes it
// in-phase; judgment phases still follow as the verdict layer. Chained
// FIRST because its rejection carries the exact defect list the ladder
// needs. It IS the expensive reviewer (real go-test per changed package) —
// the advisory post-build selfcheck skips its duplicate run when this
// floor is enforced, so each build pays the go-test cost exactly once.
```

### `go/cmd/evolve/cmd_cycle.go:717` — above `if spec, ok := cfg.DocumentSpec(); ok {`

```text
// ADR-0099 slice 2: a document cycle's solutions/<slug>/ is judged by the
// same deterministic floor seam (internal/solutioncheck over the
// registry's deliverable_kinds.document contract); silent for code cycles.
```

### `go/cmd/evolve/cmd_cycle.go:729` — above `rev := deliverable.NewReviewerWithCatalogStageReportSize(`

```text
// Catalog-aware so user/minted phases get spec-derived contracts (WS-A):
// the host gate enforces the SAME well-formedness the agent's
// `evolve phase verify` self-check derives from the phase.json. The
// report-size gate (cycle-565 S1) rides the same reviewer as its own
// dial: default shadow (observe-only) so it is byte-identical until an
// operator promotes gates.report_size_gate to enforce.
```

### `go/cmd/evolve/cmd_cycle.go:750` — above `reviewers = append(reviewers, topngate.NewReviewer(cfg.TopNGate))`

```text
// build->audit task-binding clamp (internal/topngate): a build report
// whose ## Task: slug falls outside triage ## top_n is a CERTAIN
// wrong-task build; enforce aborts it before audit/ship spend (inbox
// builder-task-binding-topn-gate, 8th recurrence). Chained after the
// contract gate: well-formedness first, task-identity binding second.
// Fails open on ambiguity (missing report, empty top_n).
// The same reviewer also carries the triage->TDD scope clamp (inbox
// tdd-topn-binding-gate, cycle-660): a TDD deliverable that authors
// test files under an empty or non-overlapping ## top_n is aborted one
// phase earlier, before the orphan scaffolds reach build.
```

### `go/cmd/evolve/cmd_cycle.go:766` — above `opts = append(opts, core.WithContractVerifier(deliverable.NewVerifierWithCatalogStage(catalog, cfg.PhaseIO)))`

```text
// ADR-0045 I2: the breaker-neutral re-check the salvage rung verifies
// relocations with — same catalog-aware resolution as the gate, so the
// rung and the gate can never disagree about "well-formed". Wired at
// every contract-gate stage (shadow needs it for would-salvage soak
// telemetry); execution stays gated on EVOLVE_PHASE_RECOVERY=enforce.
```

### `go/cmd/evolve/cmd_cycle.go:780` — above `opts = append(opts, core.WithModelCatalogLookup(resolveModelTier))`

```text
// Catalog-resolvability gate for advisor model routing (cycle-440 MR4a).
// router.ClampPlanModelRouting clears a proposed {cli,tier} that cannot
// resolve to a model; without this injection o.modelCatalogLookup is nil
// and that gate silently does nothing.
```

### `go/cmd/evolve/cmd_cycle.go:803` — above `opts = append(opts, core.WithFailureCountReader(func(id string) int {`

```text
// ADR-0076 D: the retry-tier-escalation failure-count read seam, backed by
// the durable per-item counter the S5 chain maintains.
```

### `go/cmd/evolve/cmd_cycle.go:809` — above `opts = append(opts, core.WithScopePathResolver(scopePathResolver))`

```text
// ADR-0076 C: continuation-on-fail resolve seam — the orchestrator adopts a
// prior FAILed attempt's salvage snapshot when this cycle's scope carries
// one (validated in-orchestrator against live git state). Claimed scopes
// resolve from the processing claims; a lane whose scope came from the wave
// planner instead resolves from its pinned lane-scope todo ids (G2).
```

### `go/cmd/evolve/cmd_cycle.go:816` — above `return inboxmover.ResolveContinuationForScope(inboxmover.Options{ProjectRoot: root, Stderr: os.Stderr}, cycle, scopeIDs)`

```text
// Stderr is wired (was the io.Discard default): the live-scope guard's
// refusal line — "this scope has no live pending item, binding released"
// — is the operator's only signal that a ghost binding was caught, and a
// discarded warning is how cycles 1487/1497 stayed invisible for three
// waves.
```

### `go/cmd/evolve/cmd_cycle.go:825` — above `if fp, err := pol.FailurePolicyConfig(); err == nil {`

```text
// ADR-0072 system-failure decision policy. A malformed failure_policy block
// falls back to the orchestrator's compiled DefaultSystemFailurePolicy (the
// Go-enforced floor is preserved regardless) — same safe semantics as an
// absent block; only an operator's non-floor tuning would be dropped.
```

### `go/cmd/evolve/cmd_cycle.go:836` — above `opts = append(opts, compositionOptions()...)`

```text
// RUNG 0 trivial-rebase composition-verdict fast path (cycle-786/801 built
// the pieces, cycle-804 wires them): bind the snapshot / gate-runner /
// verdict-writer closures so recoverFromShipError's clean fleet-rebase
// branch carries the audit verdict forward instead of always re-auditing.
// All fail-closed — see cmd_composition_wiring.go.
```

### `go/cmd/evolve/cmd_cycle.go:895` — above `type routerDecisionType int`

```text
// resolveRouterDispatch resolves the routing advisor's {cli, model} the same way
// a phase resolves its capability: profile (.evolve/profiles/router.json) defaults,
// overridden by the per-agent env (EVOLVE_ROUTER_CLI / EVOLVE_ROUTER_MODEL). This
// makes the brain configurable to any LLM CLI (e.g. codex-tmux + deep→the family manifest's deep model).
// Fallback is opus on claude-tmux (deep reasoning for composition/minting).
// routerDecisionType selects which advisor decision a dispatch is resolved for
// (ADR-0052 WS6-S1). The confidence-critical whole-cycle decisions (plan,
// re-plan) want the DEEP tier; the lightweight off-critical-path ones (the
// reactive propose, the route-quality judge) can use the FAST tier (D2).
```

### `go/cmd/evolve/cmd_cycle.go:913` — above `func resolveRouterDispatchFor(evolveDir string, dt routerDecisionType, rc policy.RouterPolicy) (cli, model string) {`

```text
// resolveRouterDispatchFor resolves the (cli, model) for a SPECIFIC advisor
// decision type (ADR-0052 WS6-S1, optional multi-model). It starts from the
// single base dispatch (resolveRouterDispatch) and applies a per-type model
// override from RouterPolicy. With no override set it returns the base value for
// every type. The CLI is unchanged across types; only the model tier differs.
```

### `go/cmd/evolve/cmd_cycle.go:933` — above `func resolveRouterDispatchHealthy(evolveDir string, dt routerDecisionType, benched map[string]bool, rc policy.RouterPoli…`

```text
// resolveRouterDispatchHealthy resolves the per-decision dispatch and, if the
// chosen CLI's family is currently benched (the cli-health circuit breaker —
// repeated failures bench a family), falls back to the universal claude family
// (the no-agy-fallback rule: claude is the universal fallback). If the claude
// fallback is ALSO benched it returns ok=false and the caller degrades to the
// static spine — the advisor's existing fail-safe, so no separate breaker is
// minted (clihealth IS the breaker; ADR-0052 WS6-S2). benched is the set of
// benched family names (clihealth.Store.Active values' Family).
```

### `go/cmd/evolve/cmd_cycle.go:1048` — above `func failureAdvisorOpts(projectRoot string) []core.FailureAdvisorOption {`

```text
// failureAdvisorOpts resolves the failure advisor's dispatch identity from its
// tracked profile (.evolve/profiles/failure-advisor.json), mirroring the
// router advisor's WithProposerCLI wiring above. Review of the 2026-08-26
// deep-tier arrangement found the advisor hardcoding claude-tmux/opus and
// never reading its profile — dormant today (advise hook gates on
// PhaseRecovery=enforce) but wrong the moment that stage flips. Absent or
// unreadable profile keeps the compiled default (fail-open).
```

### `go/cmd/evolve/cmd_cycle.go:1066` — above `func documentSpecPtr(cfg config.RoutingConfig) *config.DeliverableKindSpec {`

```text
// documentSpecPtr is the registry's document deliverable contract as the
// nil-able pointer the audit phase takes (ADR-0099 slice 2): the ONE
// resolution the composition root hands to both the build floor and the audit
// gate, so the two surfaces judge the same shape.
```

### `go/cmd/evolve/cmd_cycle_advisor_signals_test.go:1` — above `package main`

```text
// cmd_cycle_advisor_signals_test.go — ADR-0103 unit 04: the phase advisor's
// WARN reaches the --simulate root's console sink and the durable stream, and
// the production composition root hands its Signal Center to the advisor.
```

### `go/cmd/evolve/cmd_cycle_bridgechain_test.go:10` — above `func TestWireOrchestratorDeps_EveryConsumerGetsTheChainWalkingBridge(t *testing.T) {`

```text
// TestWireOrchestratorDeps_EveryConsumerGetsTheChainWalkingBridge is the
// source-scan guard for the one construction site: wireOrchestratorDeps wraps
// the raw bridge in bridgechain.New exactly once and hands the WRAPPED handle
// to every consumer (phase configs, the swarm decorator, the retro, the
// debugger, the spec runners, the registrar, the advisor, the failure advisor,
// the catalog publisher). The raw `br` may only be constructed, configured
// through its Set* methods, wrapped, handed to the catalog publisher as its
// contract-resolver sink, and exposed on orchDeps (*bridge.Adapter) for the
// host's own configuration — never launched. A new `Bridge: br` is how the retro lost its
// fallback for two waves (2026-09-14); this test makes that a compile-time
// habit rather than a forensic finding.
```

### `go/cmd/evolve/cmd_cycle_bridgechain_test.go:42` — above `regexp.MustCompile('wireBridgeStages\(br, '),`

```text
// F27: takes a bridgeStageSink (three setters, no Launch) — a configurer by type
```

### `go/cmd/evolve/cmd_cycle_catalog_publisher_test.go:1` — above `package main`

```text
// cmd_cycle_catalog_publisher_test.go — cycle-1429 TDD contract, the WIRING
// half of task `mint-catalog-live-refresh`.
//
// A core seam whose only caller is a test is dead code. internal/core's
// mint_catalog_publisher_test.go proves the orchestrator PUBLISHES its catalog
// after a mid-cycle mint; this file proves the PRODUCTION composition root
// (wireOrchestratorDeps, cmd_cycle.go) actually subscribes that publication to
// the bridge's contract resolver. Without this half, cycle-1424 reproduces
// verbatim with a green core test.
//
// Pattern copied verbatim from TestWireOrchestrator_CompositionFastPathWired
// (cmd_cycle_composition_test.go, cycle-804): drive the real composition root
// against a real temp-dir project root, assert an exported orchestrator
// predicate — no fakes, no injected doubles.
//
// THE CONTRACT (what Builder must implement in cmd/evolve):
//
//  1. `contractResolverSink` — the narrow bridge-side seam:
//     `interface{ SetContractResolver(phasecontract.Resolver) }`.
//     *bridge.Adapter already satisfies it (bridge.go:174); the compile-time
//     assertion below is the reachability proof that the PRODUCTION type, not a
//     test double, is what the sink accepts.
//  2. `catalogPublisher(sink contractResolverSink) func(phasespec.Catalog)` —
//     returns the closure that re-binds a fresh
//     `phasecontract.NewCatalogResolver(c.Get)` onto the sink for each published
//     catalog. Named + extracted (not an inline literal) so it is testable.
//  3. wireOrchestratorDeps appends `core.WithCatalogPublisher(catalogPublisher(br))`
//     to opts, beside the existing core.WithRegistrar(...) mint wiring.
//
// RED today: none of the three exist, so this file does not compile.
//
// Reachability probe (cycle-644 obligation): package main already imports
// adapters/bridge, phasecontract, phasespec and core (cmd_cycle.go:21-56), so
// every pin below rides an existing import edge — no new edge, no cycle.
```

### `go/cmd/evolve/cmd_cycle_catalog_publisher_test.go:64` — above `func TestWireOrchestrator_CatalogPublisherWired(t *testing.T) {`

```text
// TestWireOrchestrator_CatalogPublisherWired is the cycle-1429 composition-root
// assertion: the real wireOrchestratorDeps binds a catalog publisher, so a
// mid-cycle mint reaches the bridge's contract resolver.
```

### `go/cmd/evolve/cmd_cycle_composition_test.go:1` — above `package main`

```text
// cmd_cycle_composition_test.go — cycle-804 TDD contract (inbox weight
// 0.98, wire-rung0-composition-writer-into-fleet-rebase).
//
// Mirrors cmd_cycle_failureadviser_test.go's pattern exactly: the RUNG 0
// composition-verdict writer (cycle-786) and the core seam
// (composition_carryforward.go, cycle-801) are fully built, but
// grepping go/cmd/evolve/*.go for WithCompositionSnapshot/
// WithCompositionGateRunner/WithCompositionVerdictWriter finds zero call
// sites — the composition root never binds them, so
// compositionCarryForward's nil-guard always trips and every clean fleet
// rebase falls through to a full re-audit, the exact behavior cycle-786+801
// were built to eliminate (scout-report.md cycle 804).
//
// This is a REAL (non-fake) test: it drives the actual production
// composition root (wireOrchestratorDeps, cmd_cycle.go) with a real
// temp-dir project root, not an injected fake — the same pattern
// TestWireOrchestrator_FailureAdviserWired already uses to pin the failure-
// advisor tail's wiring.
//
// RED today: wireOrchestratorDeps binds none of the three composition
// closures, so d.Orchestrator.CompositionFastPathWired() returns false.
```

### `go/cmd/evolve/cmd_cycle_config.go:3` — above `import (`

```text
// cmd_cycle_config.go — the ADR-0103 unit-08 seam: the routing-config
// Loader's ONE wired construction (the root's Center is built 31 lines before
// the load, so the Loader takes it at construction — no lazy accessor pair),
// the ONE projection of policy's four accessors onto the leaf's Parameter
// Object, and the two silent stage forwarders the report-size gate
// (cmd_cycle.go) and the nested-sandbox fallback (cmd_loop_preflight.go)
// keep — their dials are not RoutingConfig fields yet (unit doc F2) — and
// (F27) the ONE forwarding of the resolved bridge dials into the adapter
// (wireBridgeStages), and (F37) the ONE composition of the build handoff
// floor (productionBuildFloorChecks) — on the protected manifest, so no cycle
// can drop the protected-surface check from its own floor.
```

### `go/cmd/evolve/cmd_cycle_config.go:54` — above `func wireBridgeStages(br bridgeStageSink, cfg config.RoutingConfig) {`

```text
// wireBridgeStages is the ONE forwarding of the resolved rollout dials into
// the bridge adapter (TestWireBridgeStages_IsTheRootsOnlyStageForwarding):
// each dial rides its OWN setter — the fatal-pane fast-fail never borrows
// PhaseRecovery's (F27; TestWireBridgeStages_ForwardsEachDialOnItsOwnSetter).
```

### `go/cmd/evolve/cmd_cycle_config.go:64` — above `func productionBuildFloorChecks(ctx context.Context, in core.ReviewInput) []string {`

```text
// productionBuildFloorChecks is the ONE composition of the code-cycle build
// handoff floor both roots run — the cycle reviewer (cmd_cycle.go) and the
// in-session pre-flight (`evolve selfcheck build`): the protected-surface floor
// with the control-plane membership predicate injected (F37 — core cannot
// import guards), judged FIRST, before go test can leave untracked files
// behind, then the deterministic engine. A named function, so the wiring pin's
// pointer identifies it (every ChainBuildFloorChecks closure shares one code
// pointer).
```

### `go/cmd/evolve/cmd_cycle_config_test.go:3` — above `import (`

```text
// cmd_cycle_config_test.go — ADR-0103 unit 08: the routing-config Loader's
// one wired construction, the policy-stages projection, the Center-less
// facade sites pinned by name, and the render proofs at both roots.
```

### `go/cmd/evolve/cmd_cycle_config_test.go:258` — above `func TestWireBridgeStages_ForwardsEachDialOnItsOwnSetter(t *testing.T) {`

```text
// TestWireBridgeStages_ForwardsEachDialOnItsOwnSetter (F27 wiring proof): the
// root forwards the RESOLVED rollout dials into the bridge adapter, the
// fatal-pane dial on its OWN setter — crossed values prove no dial borrows
// another's (the fatal-pane stage never rides PhaseRecovery's).
```

### `go/cmd/evolve/cmd_cycle_config_test.go:273` — above `func TestWireBridgeStages_IsTheRootsOnlyStageForwarding(t *testing.T) {`

```text
// TestWireBridgeStages_IsTheRootsOnlyStageForwarding pins the seam: the bridge
// stage setters are called from cmd_cycle_config.go alone, so no root can
// forward one rollout dial and forget its twin (the F27 defect class: a dial
// resolved by the Loader that never reaches its consumer).
```

### `go/cmd/evolve/cmd_cycle_contract_verifier_test.go:12` — above `func TestWireOrchestratorDeps_ContractVerifierReachesEveryPhaseRunner(t *testing.T) {`

```text
// Research F22 (cycle 1685): the verdict engine and the deliverables gate
// must be ONE verifier, so the bytes a phase's classification judges are the
// bytes the gate approves (a sole recoverable bad_verdict salvaged, persisted
// and reported before classification). The composition root builds the
// gate's Reviewer after the runners and hands every BaseRunner an accessor
// to it — this is the wiring proof, in the SignalCenterReachesEveryPhaseRunner
// shape.
```

### `go/cmd/evolve/cmd_cycle_deliverables_gate_test.go:1` — above `package main`

```text
// cmd_cycle_deliverables_gate_test.go — ADR-0100 wiring proof: the production
// composition root installs a reviewer that verifies every agent-owed
// declared output. The gate lives inside the contract gate, which cmd_cycle
// mounts only when cfg.ContractGate != off — the compiled default is enforce,
// so a default-policy project must have it. Without this proof a future
// reordering of the reviewer chain (or a policy default flip) could drop the
// gate silently while every unit test stayed green.
```

### `go/cmd/evolve/cmd_cycle_failureadviser_test.go:1` — above `package main`

```text
// cmd_cycle_failureadviser_test.go — R8.1 (concurrency-factory plan): the
// ADR-0044 Slice-6 "post-soak step" — wiring the LLM failure-advisor tail
// at the production composition root. The hook itself shipped enforce-gated
// and best-effort (core/failure_hook.go), so wiring is safe at any dial:
// below enforce it never dispatches. Without this wiring, flipping
// EVOLVE_PHASE_RECOVERY=enforce would silently skip the advise→promote path
// the flip exists to activate (cycle-270 forensics: the named post-soak
// step from the ADR-0044 implementation record).
```

### `go/cmd/evolve/cmd_cycle_failureadvisor_test.go:3` — above `import (`

```text
// cmd_cycle_failureadvisor_test.go — the composition-root half of the
// failure-advisor identity wiring (2026-08-26 deep-tier review HIGH: the
// advisor hardcoded claude-tmux/opus and never read its profile; dormant while
// PhaseRecovery=shadow but wrong the moment it flips to enforce). The option's
// effect on the dispatched BridgeRequest is pinned in core
// (apicover_misc_test.go); THIS pins that the root actually resolves the
// tracked profile into that option.
```

### `go/cmd/evolve/cmd_cycle_gate_signals_test.go:3` — above `import (`

```text
// cmd_cycle_gate_signals_test.go — ADR-0101 S2b wiring proofs: the production
// root hands its Center to the contract gate (the chain capability proof, the
// DeclaredDeliverablesGateWired precedent), and the batch report's per-cycle
// signal line reports the gate's verdict counts so the operator sees the
// "checked → advanced" record per cycle.
```

### `go/cmd/evolve/cmd_cycle_health_dossier_commitment_test.go:12` — above `const dossierCommitmentSignal = "dossier_commitment"`

```text
// cmd_cycle_health_dossier_commitment_test.go — cycle 1652 RED contract, AC3 of
// triage-empty-commitment-still-dispatches-spine: "a cycle 1623-shaped dossier
// (empty top_n + full PhasesRun) … cyclehealth classifies such a historical
// dossier as an anomaly if it appears."
//
// Driven through the PRODUCTION caller (`evolve cycle-health <N> <workspace>`
// → runCycleHealth → cyclehealth.Check) rather than cyclehealth.Options
// directly, so the Builder is free to shape how the dossier reaches the check
// (an Options field threaded from projectRoot, or a derivation) while the
// contract stays: the CLI, given EVOLVE_PROJECT_ROOT, reads
// <root>/knowledge-base/cycles/cycle-N.json and reports the anomaly.
//
// Vocabulary pinned (the Builder implements it; the ACS predicates bind it):
//
//	signal   "dossier_commitment"
//	message  names at least one implementation phase the dossier recorded
//	          (tdd|build|audit|ship) so the operator sees WHAT ran against nothing
//
// The historical record is the DOSSIER, not the run dir — run dirs are
// gitignored and pruned, the dossier is committed — so the fixtures carry ONLY
// a dossier plus an empty workspace directory. Other signals (missing
// scout-report.md, …) will fire on that empty workspace; they are irrelevant
// here and deliberately not asserted on.
```

### `go/cmd/evolve/cmd_cycle_ledger_roots_test.go:3` — above `import (`

```text
// cmd_cycle_ledger_roots_test.go — ADR-0101 S4a: the code-homed inventory of
// ledger constructions the Signal Center does NOT observe. Every production
// `ledger.New(` passes `ledger.WithSignals(` except the pinned roots below,
// each with its reason and its count — the ledger twin of
// TestNilSignalCenterRootsArePinned, so the next self-constructed ledger
// cannot land silently and the design's prose (§15.4) points here instead of
// restating the list. The ledger package's own guard pins the writers inside
// the package (Rebaseline, WriteCompositionVerdict).
```

### `go/cmd/evolve/cmd_cycle_memo_runner_test.go:1` — above `package main`

```text
// cmd_cycle_memo_runner_test.go — cycle-563 fix-memo-phase-dispatch, criterion 1
// (the regression test that would have caught the silent drop). Root cause
// (fault-localization-report.md, confidence 0.97): the runner-registration
// loop in wireOrchestratorDeps (cmd_cycle.go:406) validates each discovered
// user-overlay PhaseSpec with the non-catalog-aware phasespec.ValidateUserSpec,
// which re-imposes the two-tier single-word naming floor that
// phasespec.ApplyUserRouting (three lines above, cmd_cycle.go:399) already
// exempted for "memo" via ValidateUserSpecWithCatalog (memo is a reserved
// single-word name, but the built-in registry marks it optional:true, which is
// exactly the exemption ValidateUserSpecWithCatalog grants). So the ROUTER
// legitimately plans "memo" after "ship" (cycle-561 routing-decision-12.json),
// but no PhaseRunner is ever registered for it — cyclerun_dispatch.go's
// missing-runner escape hatch then WARNs and silently advances past memo
// without ever running it or recording it in completed_phases.
//
// This test wires the ACTUAL composition root (wireOrchestratorDeps) against
// the real repo's real built-in registry (docs/architecture/phase-registry.json,
// which declares memo optional:true) and the real user overlay
// (.evolve/phases/memo/phase.json, a bare single-word name) and asserts a
// PhaseRunner was registered for "memo" — i.e. the dispatcher would actually
// attempt to launch it, not just that Route() nominates its name.
```

### `go/cmd/evolve/cmd_cycle_memo_runner_test.go:55` — above `func TestWireOrchestrator_MemoRunnerRegistered(t *testing.T) {`

```text
// TestWireOrchestrator_MemoRunnerRegistered is the RED regression test for the
// silent drop: the composition root must register a PhaseRunner for "memo"
// whenever the built-in registry marks it optional AND a (validly-shaped,
// single-word) user overlay activates it — mirroring cycle-561's live state.
// Uses a fresh temp evolveDir (storage/ledger) so this never touches real
// .evolve state; the projectRoot stays the real repo so the real registry +
// overlay + policy pins are exercised, not synthetic fixtures.
```

### `go/cmd/evolve/cmd_cycle_observer_signals_test.go:3` — above `import (`

```text
// cmd_cycle_observer_signals_test.go — ADR-0103 unit 12 §6 tests 41-42: every
// production construction of the live observer adapter hands it the root's
// Center (the ONE declared line inside wireOrchestratorDeps, unit 05's
// subject — a unit-05 cut must keep it), and the adapter's WARN renders at
// the --simulate root and lands in the cycle workspace's signals.ndjson.
```

### `go/cmd/evolve/cmd_cycle_outputs.go:93` — above `func catalogAwareResolver(projectRoot string, warn func(string)) phasecontract.Resolver {`

```text
// catalogAwareResolver assembles the same builtin+user-spec catalog cmd_cycle
// builds, so the survey resolves report names through the SAME vocabulary the
// contract gate and bridge use (a builtin-only lookup produced cycle-1452's
// false memo-report.md gap). Degrades loudly to builtin-only when the registry
// cannot load.
```

### `go/cmd/evolve/cmd_cycle_projroot_test.go:11` — above `func TestRunCycleReset_RelativeProjectRootAbsolutized(t *testing.T) {`

```text
// TestRunCycleReset_RelativeProjectRootAbsolutized encodes the cycle-120 reset
// refusal that motivated Workstream A's shared helper.
//
// The fixed loop writes an ABSOLUTE workspace_path into cycle-state.json. But
// `evolve cycle reset` still defaulted --project-root to "." and never
// absolutized it, so SealCycle's containment guard compared a relative root
// (".") against the absolute workspace_path — pathWithin() returned false for
// both roots and reset REFUSED with "outside evolveDir/projectRoot". The
// operator had to pass an explicit absolute --project-root as a workaround.
//
// This test reproduces that exact shape (relative --project-root, absolute
// workspace_path) and asserts the seal now SUCCEEDS — proving the root is
// absolutized before the containment check, so the comparison is
// absolute-to-absolute. It runs --force to skip the dispatcher-lock check and
// must NOT be parallel (it os.Chdir's to make "." resolve to the temp project).
```

### `go/cmd/evolve/cmd_cycle_projroot_test.go:40` — above `csJSON := '{"cycle_id":5,"phase":"build","workspace_path":' + strconv.Quote(workspace) + '}'`

```text
// cycle-state.json with an absolute workspace_path — the cycle-120 signature.
```

### `go/cmd/evolve/cmd_cycle_projroot_test.go:50` — above `prevWD, err := os.Getwd()`

```text
// Make "." resolve to the temp project root, then run reset with the
// RELATIVE default --project-root that broke cycle-120.
```

### `go/cmd/evolve/cmd_cycle_reset_test.go:45` — above `livePID := os.Getpid()`

```text
// A genuinely-alive owner pid: cycle-554's PID-aware fence (runlease.OwnerLive,
// wired via SealOptions.PidAlive) now demands BOTH a fresh heartbeat AND a
// live process to treat a cycle as owned. This test process's own pid is a
// guaranteed-live owner; an arbitrary pid (84055) is almost certainly dead
// and would now correctly seal — see the dead-owner subtest below.
```

### `go/cmd/evolve/cmd_cycle_reset_test.go:75` — above `root := t.TempDir()`

```text
// The cycle-554 fix's payoff at the CLI level: a crashed owner whose
// heartbeat is still fresh (the 2-6min post-crash window) used to force
// `evolve cycle reset --force` at every batch boundary; now a plain reset
// seals it because the owning pid is dead. Exercises the cmd_cycle.go
// PidAlive wiring end to end.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:1` — above `package main`

```text
// cmd_cycle_signal_center_test.go — ADR-0101 S1 wiring proofs. The production
// composition root constructs ONE Signal Center, attaches the durable NDJSON
// sink and the WARN-filtered stderr sink, and registers the orchestrator as a
// listener; the only roots allowed to build an orchestrator WITHOUT a Center
// are pinned here, so a forgotten wiring can never be silent again (the way
// bridge.Deps.LivenessCenter stayed unset in production).
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:63` — above `func TestNilSignalCenterRootsArePinned(t *testing.T) {`

```text
// Every production call site of core.NewOrchestrator must pass
// core.WithSignalCenter, except the one pinned nil root: the routing-test
// engine, test machinery that takes a *testing.T. The --simulate root builds
// the production topology since unit 01 (ADR-0103);
// TestWireSimulateOrchestrator_SignalCenterWired proves it.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:160` — above `for file, needle := range map[string]string{"cmd_cycle.go": "Signals.Flush()", "cmd_loop.go": "Signals.Flush()", "cmd_lo…`

```text
// The chain root (ADR-0103 unit 13) builds its own batch-level Center
// and flushes it at exit too.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:173` — above `func TestWireOrchestratorDeps_SignalCenterReachesTheBridge(t *testing.T) {`

```text
// ADR-0101 S3: the production root hands the Center to the bridge Adapter it
// injects into every phase runner, so bridge.warning / bridge.tripwire /
// pane.liveness from any dispatch reach the orchestrator's Center.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:232` — above `func TestWireOrchestratorDeps_LedgerIsObservedByTheSignalCenter(t *testing.T) {`

```text
// ADR-0101 S4a: the production root decorates the ledger so every appended
// entry is also a ledger.appended signal (Decorator over the file ledger).
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:247` — above `func TestWireSimulateOrchestrator_SignalCenterWired(t *testing.T) {`

```text
// Unit 01 (ADR-0103), architecture review HIGH-1: the --simulate root builds
// the production signal topology — Center, observed ledger, console sink at
// WARN, durable cycle-less sink — so the recorder's warnings render there as
// the deleted stderr lines did.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:277` — above `func TestWireSimulateOrchestrator_FailureDiagWarningRenders(t *testing.T) {`

```text
// Unit 02 (ADR-0103): the failurediag module tag renders at the --simulate
// root too — a writer built on the root's Center, writing into a file used as
// a workspace, reaches the console sink and the durable cycle-less sink.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:302` — above `func TestWireSimulateOrchestrator_CarryoverWarningRenders(t *testing.T) {`

```text
// Unit 03 (ADR-0103): the carryover module tag renders at the --simulate root.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:326` — above `func TestWireSimulateOrchestrator_FailureLearningWarningRenders(t *testing.T) {`

```text
// ADR-0103 unit 03b: the failure-learning engine's WARN reaches the --simulate
// root's console sink and the durable stream.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:346` — above `func TestConsoleSinkThresholdHasOneHome(t *testing.T) {`

```text
// ADR-0103 unit 12 (architecture review fold): the console threshold — "the
// operator console renders WARN and above" — has ONE home,
// signalcenter.ConsoleSink. Both composition roots (newRootSignalCenter and
// the `evolve phase-observer` subprocess) consume it; no production source
// outside the sink's own file re-spells Filter(StderrSink(…), SeverityWarn),
// so a console-policy change at one root can never leave the other behind.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:397` — above `func TestWireOrchestratorDeps_SignalCenterReachesEveryPhaseRunner(t *testing.T) {`

```text
// ADR-0103 unit 11 (test 44): the production root's Center reaches every
// BaseRunner-backed phase runner — through the bridge Adapter it injects
// (Signals() is the carrier's read seam) and, for scout/build, through the
// swarmrunner Decorator's forward. Ship and retro are not BaseRunner-backed.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:434` — above `func TestWireSimulateOrchestrator_AuditLedgerWarningRenders(t *testing.T) {`

```text
// ADR-0103 unit 09: the defect ledger's WARN reaches the --simulate root's
// console sink under the audit tag and the cycle workspace's durable stream —
// a directory at <ws>/defect-ledger.json on a continuation is an unreadable
// own ledger the grade blocks on.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:470` — above `func TestAuditRoot_PassesTheSignalCenter(t *testing.T) {`

```text
// ADR-0103 unit 09: the loop root hands the audit phase the Signal Center —
// the one production site where AUDIT_LEDGER_* codes can reach a sink.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:516` — above `func TestWireSimulateOrchestrator_RunnerWarningRenders(t *testing.T) {`

```text
// ADR-0103 unit 11 (test 45): the verdict engine's WARN reaches the --simulate
// root's console sink and the cycle-stamped durable stream.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:553` — above `func TestWireSimulateOrchestrator_LoopWaveAndChainWarningsRender(t *testing.T) {`

```text
// ADR-0103 unit 13: the wave engine's and the chain engine's WARNs reach the
// --simulate root's console sink and the durable batch-level stream.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:593` — above `func TestAuditRootPassesTheCenter(t *testing.T) {`

```text
// ADR-0103 unit 14: the loop root passes its Center to the audit phase's
// CI-parity gates through audit.WithSignals on the ONE production
// construction chain (D6) — a source pin, the TestNilSignalCenterRootsArePinned
// idiom; the behavioural chain option → gates → Center → sinks is proven by
// audit's TestNewDefaultWithStageCompactSpec_WithSignalsReachesTheGates_* and
// TestWireSimulateOrchestrator_CIParityWarningRenders below.
```

### `go/cmd/evolve/cmd_cycle_signal_center_test.go:610` — above `func TestWireSimulateOrchestrator_CIParityWarningRenders(t *testing.T) {`

```text
// ADR-0103 unit 14: a CI-parity gate WARN reaches the --simulate root's
// console sink and the cycle workspace's durable stream — the ≤ 3-line triage
// path: signals.ndjson and integration-tier.log sit in the SAME directory.
```

### `go/cmd/evolve/cmd_cycle_simulate.go:43` — above `if req.Workspace != "" {`

```text
// The contracted report stub: a no-LLM walk still hands the floors a
// deliverable to grade (docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md).
```

### `go/cmd/evolve/cmd_cycle_simulate.go:118` — above `return orchDeps{`

```text
// A --simulate walk must never mutate the operator's repository: no cycle
// worktree or branch (the phases never write, so the root is read in place)
// and no `dossier: cycle-N closeout` commit (the record is still written).
// docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md.
```

### `go/cmd/evolve/cmd_cycle_simulate_git_test.go:3` — above `import (`

```text
// cmd_cycle_simulate_git_test.go — the --simulate root is a no-LLM plumbing
// walk and must never mutate the operator's git repository: no
// `dossier: cycle-N closeout` commit, no `cycle-*` branch, no git worktree.
// It still writes its records (runs/, the dossier file) so the plumbing it
// exists to check is exercised. Incident: a whole-module floor that ran
// acs/cycle8's `evolve campaign run --simulate` from the checkout committed
// dossier closeouts onto the dev branch and left worktrees behind
// (docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md).
```

### `go/cmd/evolve/cmd_cycle_simulator.go:35` — above `projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")`

```text
// envOrCwd absolutizes a relative $EVOLVE_PROJECT_ROOT (cycle-119 class).
```

### `go/cmd/evolve/cmd_cycle_test.go:203` — above `func TestRunCycleReset_HeldLockAloneDoesNotBlock(t *testing.T) {`

```text
// TestRunCycleReset_HeldLockAloneDoesNotBlock pins the cycle-395 fix direction:
// holding the coarse .evolve/.lock WITHOUT a fresh run lease must NOT block a
// reset. The old lock pre-check was a false negative — the dispatcher's lock is
// per-CYCLE (released between cycles), so a sibling acquired it in the gap and
// sealed a running loop. Liveness is now the per-run lease (the FRESH-lease
// refusal is covered by TestRunCycleReset_LeaseFencing); a held lock with no
// proof-of-life owner does not gate the seal.
```

### `go/cmd/evolve/cmd_cycle_test.go:272` — above `func TestParseRouterStage_LadderAndUnknown(t *testing.T) {`

```text
// TestParseRouterStage_LadderAndUnknown is the missing twin of TestParseGateStage
// (ADR-0103 unit 08): the router ladder accepts advisory, trims, and maps an
// unknown word to off silently — the mapping cmd_cycle.go:700 and
// cmd_loop_preflight.go:32 keep through the seam's forwarders.
```

### `go/cmd/evolve/cmd_cycle_timing.go:100` — above `func latestCycleDir(runsDir string) string {`

```text
// latestCycleDir scans runsDir for cycle-N directories that contain a timing
// log and returns the directory name with the highest numeric cycle. A
// reset-suffixed dir (cycle-382.reset-…) parses on its leading integer, so a
// completed re-run still wins over an older sealed attempt of a lower number.
```

### `go/cmd/evolve/cmd_cycle_triagecap_test.go:1` — above `package main`

```text
// cmd_cycle_triagecap_test.go — R9.1 (concurrency-factory plan): the
// production composition root must wire the triage-throughput recorder so
// shipped coverage cycles feed the rolling window the R9.2 capacity clamp
// reads. Without this wiring the clamp would run forever on the cycle-281
// seed instead of the observed throughput.
```

### `go/cmd/evolve/cmd_dashboard.go:3` — above `import (`

```text
// cmd_dashboard.go — `evolve dashboard [--project-root P] [--addr A] [--snapshot]`
// serves the read-only live pipeline dashboard (internal/dashboard, ADR-0095):
// loop status, inbox, per-cycle progress, what went wrong, ship-rate trend.
// Pure reader: no state, ledger, inbox, or registry mutation; never takes the
// loop's flock sidecars — safe to run beside a live loop. --snapshot prints the
// JSON snapshot once to stdout and exits (scripting / smoke checks).
```

### `go/cmd/evolve/cmd_dashboard.go:62` — above `func dashboardProjectRoot(flagRoot string, stderr io.Writer) string {`

```text
// dashboardProjectRoot resolves flag → EVOLVE_PROJECT_ROOT → cwd through the
// shared cmdutil rule, then makes the flag value absolute (paths.AbsoluteRoot:
// a relative root is the cycle-119 defect class — the server and the artifacts
// it names must agree on one location).
```

### `go/cmd/evolve/cmd_dashboard_test.go:16` — above `func writeDashboardDossier(t *testing.T, root string, cycle int) {`

```text
// `evolve dashboard` is a READ-ONLY render of the loop's on-disk state
// (ADR-0095). --snapshot is the scriptable form; the served form goes through
// the dashboardServe seam so the wiring is provable without binding a port.
```

### `go/cmd/evolve/cmd_dossier.go:3` — above `import (`

```text
// cmd_dossier.go — ADR-0055 D3: `evolve dossier verify` reads every
// knowledge-base/cycles/cycle-N.json, parses it, and calls dossier.Validate().
// Pure reader — no state/ledger mutation — safe to run mid-batch.
```

### `go/cmd/evolve/cmd_dossier.go:89` — above `pol, perr := policy.Load(filepath.Join(root, ".evolve", "policy.json"))`

```text
// ADR-0055 D3: when the policy floor enrolls "dossier-closeout", an absent or
// empty knowledge-base/cycles/ is a FAILURE — the gate exists precisely to
// catch the no-dossier-written case. Without enrollment, absence stays a
// no-op success (safe to run mid-batch). A malformed policy fails loudly.
```

### `go/cmd/evolve/cmd_dossier_retro_mislabel_test.go:3` — above `import (`

```text
// cmd_dossier_retro_mislabel_test.go — `evolve dossier retro-mislabel`, the
// derived (never estimated) count of dossier-corpus-carries-retro-mislabel
// (cycle 1666): how many committed dossiers claim retro was SKIPPED while an
// execution receipt proves it RAN. It is the production caller of the
// corpus-reading seam (dossier.ReadCommitted + dossier.PhaseSkipEvidence) —
// the audit is a fold of that seam over the corpus, so this test is a
// reachability proof for the consumer-side safeguard, not a unit test of it.
//
// Output contract (`--json`): an object with at least
//
//	candidates     integer — legacy (unversioned) records whose skipped_phases names retro
//	mislabeled     [int]   — …of those, a receipt proves retro ran (THE derived count)
//	uncorroborated [int]   — …no receipt survives: unknown, never counted as mislabeled
//
// candidates == len(mislabeled) + len(uncorroborated); both lists ascending.
// The command is READ-ONLY: it must never rewrite a record (the inbox record
// forbids doing the backfill alongside the discriminator).
```

### `go/cmd/evolve/cmd_fanout_dispatch.go:63` — above `return fanoutdispatch.Run(cfg, stderr)`

```text
// CycleStateHelperBin intentionally left unset (zero value disables
// worker-status tracking). The legacy bash probe that pointed at
// legacy/scripts/lifecycle/cycle-state.sh was removed in v12 (ADR-0062/T1.7);
// the generic Config.CycleStateHelperBin seam remains for callers that inject
// their own helper.
```

### `go/cmd/evolve/cmd_flags.go:1` — above `package main`

```text
// cmd_flags.go — `evolve flags generate|check` (L2.2, concurrency-factory
// plan): projects the internal/flagregistry SSOT into a marker-delimited
// region of docs/architecture/control-flags.md, exactly like `evolve skills
// generate|check` projects phase facts (ADR-0040). Hand-written cluster
// prose outside the markers is preserved byte-for-byte; the generated region
// is the complete flat flag index. `check` exits 2 on drift so CI can gate
// undocumented flags.
```

### `go/cmd/evolve/cmd_flags.go:30` — above `project := sourceRoot()`

```text
// control-flags.md is a generated SOURCE doc, part of a cycle's committed
// deliverable — resolve it from the worktree under the ACS suite, not main's
// stale working copy (cycle-355 fix; see sourceRoot).
```

### `go/cmd/evolve/cmd_flags_test.go:18` — above `func anyRenderedFlagToken(t *testing.T) string {`

```text
// anyRenderedFlagToken returns the backticked name of the first registered flag,
// for assertions that "some registry content was rendered" without hardcoding a
// specific flag (the flag-reduction campaign deletes flags, so any pinned name
// eventually 404s — the cycle-12 lesson). Skips the test when the registry is
// empty (the campaign's terminal state).
```

### `go/cmd/evolve/cmd_flags_test.go:129` — above `func TestFlagsCheck_ResolvesWorktreeRootOverProjectRoot(t *testing.T) {`

```text
// TestFlagsCheck_ResolvesWorktreeRootOverProjectRoot is the cycle-355
// regression guard. Under the ACS suite a predicate runs with
// EVOLVE_PROJECT_ROOT pinned to the MAIN checkout so it can read `.evolve/`
// runtime STATE (issue #12). But a generated SOURCE doc like control-flags.md
// is part of the cycle's committed deliverable and lives in the WORKTREE — so
// `flags check` must validate the WORKTREE doc, not main's stale working copy.
// acssuite exports EVOLVE_WORKTREE_ROOT=<worktree>; flags resolution must
// prefer it over EVOLVE_PROJECT_ROOT. Before the fix, `flags check` read the
// stale main root and red-failed correct work (the cycle-355 audit FAIL).
```

### `go/cmd/evolve/cmd_fleet.go:1` — above `package main`

```text
// cmd_fleet.go — `evolve fleet` concurrent-cycle supervisor (ADR-0049 S6 / CE.2).
// Launches N cycles at the same time, each `evolve cycle run` in its OWN process
// with EVOLVE_FLEET=1, so the orchestrator skips the whole-cycle global lock
// (root-cause R1) and the per-resource flocks (S2–S5) serialize the shared
// writes. The missing producer for the EVOLVE_FLEET flag.
```

### `go/cmd/evolve/cmd_fleet.go:25` — above `func loadPlanSpecs(planJSON []byte, goalHash string, count int) ([]fleet.CycleSpec, []fleet.Todo, error) {`

```text
// loadPlanSpecs parses an `evolve fleet --plan` backlog ([{"id","files"}]) and
// partitions it into at most `count` disjoint-scoped cycle specs (ADR-0049 E:
// the advisor assigns independent todos to independent cycles), each stamped
// with goalHash. Returns the launch specs and the deferred todos (run in a later
// wave). The partition guarantees every file is owned by one cycle, so the
// launched cycles never collide on the shared tree.
```

### `go/cmd/evolve/cmd_fleet.go:90` — above `planJSON, rerr := os.ReadFile(planPath)`

```text
// Advisor-partitioned: each cycle gets a DISJOINT todo subset so the
// concurrent cycles never edit the same file (ADR-0049 E).
```

### `go/cmd/evolve/cmd_fleet.go:143` — above `if outputContract != "" {`

```text
// The plan's per-cycle output contract is the cycle's BINDING goal: threaded
// as --goal it reaches the scout (Context["goal"]) so the cycle executes the
// PLANNED removal instead of free-choosing a task (campaign-6 cycle-1 shipped
// a classify-only cycle and never lowered FlagCeiling). Empty keeps the
// generic goal (goal-hash only) — back-compat for plans without contracts.
// The contract is the goal text by design: the scout still reads the campaign
// docs in-repo for the "why" framing, and the adversarial auditor enforces the
// anti-gaming gate regardless of the goal prose — so a terse contract is the
// task, not a weakened constraint.
```

### `go/cmd/evolve/cmd_fleet_soak.go:1` — above `package main`

```text
// cmd_fleet_soak.go — evolve fleet soak: concurrency invariant harness (Slice 5, ADR-0049).
// Proves that Slices 1-4 (runscope, codex-pretrust, sessionreaper, cliadmit) compose
// correctly under concurrent load by running N in-process fake "cycles" and asserting
// the four structural invariants from the concurrency architecture spec.
//
// soakreport note: the internal/soakreport package aggregates ADR-0044 C2/C4/I2/I3/I4
// phase-recovery component evidence across historical cycles. This soak harness covers a
// distinct concern (concurrent-run isolation invariants) and renders its own 4-row verdict
// table; soakreport.Collect is not appropriate for per-soak-run structural checks.
```

### `go/cmd/evolve/cmd_fleet_soak.go:134` — above `ctx, cancel := context.WithTimeout(context.Background(), sessionreaper.DefaultReapTimeout)`

```text
// Same wedged-tmux bound as the loop-boot sweep (cycle-769).
```

### `go/cmd/evolve/cmd_fleet_soak_test.go:122` — above `tombstones := 0`

```text
// Invariant 2 (tombstone contract — cycle-806 reconcile): runFleetSoak's
// in-harness reap (soakCheckReap → ReapOrphans) has ALREADY reaped all N
// runs, renaming each registry to its `.reaped` tombstone
// (sessionreaper.go:92-95, the cycle-769 bounded-reap contract). So the new
// correct assertions are (a) exactly N tombstones exist on disk, and (b) a
// SECOND ReapOrphans is an idempotent 0-orphan no-op — a tombstoned registry
// is skipped, never re-swept. The pre-806 test wrongly re-expected N orphans
// from the second reap, which the tombstone rename made impossible.
```

### `go/cmd/evolve/cmd_fleet_soak_test.go:163` — above `killMu.Lock()`

```text
// Invariant 3 (tombstone contract — cycle-806 reconcile): every kill (all N
// recorded during the in-harness reap) must still resolve to a runDir whose
// registry contains the killed session. After the reap the live registry is
// tombstoned, so attribution is discovered through sessionrecord's
// tombstone-aware ReadAllResolving (task sweep-tombstone-attribution) rather
// than the live-path ReadAll — which now finds nothing. The UNKNOWN /
// cross-run-reap failure branch is KEPT (not relaxed): a session that
// resolves to no owning run is still a hard failure.
```

### `go/cmd/evolve/cmd_gc_workspace_test.go:3` — above `import (`

```text
// cmd_gc_workspace_test.go — RED tests for cycle-1172, inbox item
// `workspace-hygiene-s5-wiring-shadow-default` (scout task
// evolve-gc-workspace-sweep-dry-run).
//
// THE GAP: `evolve gc` (cmd_gc.go) reaps only orphan tmux sessions and dead
// per-run sockets. The S4/S5 worktree+branch sweep (gc.PlanWorktrees /
// gc.ApplyWorktrees) has NO operator surface at all: today it is observable
// only as a JSON manifest written mid-batch by runGCHook. An operator who
// wants to know what the sweep would do — or to drain the backlog on demand
// after a crash — has no command to run.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//
//  1. `evolve gc` gains a `--project-root` parameter (defaulting to the process
//     cwd) so the sweep is aimed explicitly. Never infer the repo implicitly
//     for a MUTATING run — same refusal posture runWorktreeGC already takes on
//     an empty ProjectRoot.
//  2. `evolve gc --dry-run` additionally plans the worktree/branch sweep via
//     gc.PlanWorktrees and prints every planned item, each line naming the
//     branch and prefixed `WOULD-` (parity with the existing orphan-session
//     dry-run vocabulary). It mutates NOTHING.
//  3. A bare `evolve gc` (no --dry-run) APPLIES the worktree sweep. This is the
//     documented asymmetry: an explicit operator run is enforce, while the
//     in-loop hook's default stays shadow (policy.json owns the loop's mode;
//     the operator owns their own invocation). Document it in the command help.
//  4. NEGATIVE — the apply must never touch UNMERGED work: gc.PlanWorktrees
//     flags an unmerged cycle-* branch instead of planning a delete, and the
//     CLI must not upgrade that to a deletion. Unlanded cycle work is exactly
//     what this backlog is protecting.
//
// The fixture helpers (gcGit / gcWorktreeEnv / gcOrphanBranch) are the ones
// already used by cmd_loop_gc_worktree_test.go in this package.
```

### `go/cmd/evolve/cmd_inbox.go:69` — above `dispatchable, console, reasons := inboxbatch.PartitionConsole(items, guards.IsProtectedScope)`

```text
// ADR-0074 I1: the operator's own worklist uses the SAME partition triage
// (internal/phases/triage/triage.go:236) and the claim floor
// (inboxmover.Claim) already use — console-routed work is operator-owned
// and is never a batch a lane may draw. Classifying the whole backlog
// presented it as selectable, with no reason and no separation.
```

### `go/cmd/evolve/cmd_inbox_ackfingerprint.go:1` — above `package main`

```text
// cmd_inbox_ackfingerprint.go — `evolve inbox ack-fingerprint <item-path>`,
// the transactional-consumption counterpart to cycle-1332's manual
// `evolve loop --reset --fingerprint <fp>`. Reads a consumed pipeline-defect
// inbox item's consumed_by/notes fields, extracts the failure fingerprint,
// and acks it into .evolve/resolved-fingerprints.json via
// core.ConsumePipelineDefectFingerprint — closing the gap the inbox item
// 2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json named: the
// operator otherwise has to hand-retype the fingerprint into --reset
// --fingerprint after every consumption.
```

### `go/cmd/evolve/cmd_inbox_ackfingerprint_test.go:3` — above `import (`

```text
// cmd_inbox_ackfingerprint_test.go — caller proof for cycle-1334's
// `evolve inbox ack-fingerprint <item-path>` subcommand, the transactional-
// consumption counterpart to cycle-1332's manual `evolve loop --reset
// --fingerprint <fp>` (cmd_loop_fingerprint_ack_test.go). Drives the REAL
// production entrypoint (runInbox) rather than calling
// core.ConsumePipelineDefectFingerprint directly — a predicate that only
// calls the core helper proves nothing about the operator/automation-facing
// CLI surface actually being wired (house rule: a wiring proof is a
// reachability test, not a unit test).
```

### `go/cmd/evolve/cmd_inbox_consume.go:1` — above `package main`

```text
// cmd_inbox_consume.go — the automated half of the blocker-breaker ack ledger
// (inbox item 2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json).
//
// `evolve inbox ack-fingerprint` already extracted a consumed item's failure
// fingerprint correctly — but it is operator-invoked, and the P0 of the
// cycle-1335 incident reached .evolve/inbox/consumed/ by a bare `mv` with
// nobody left to run it. The ledger therefore did not exist at all while the
// item naming the resolved fingerprint sat consumed on disk, and the breaker
// re-halted on three relaunches.
//
// Two writers close that, both routed through ONE extraction path
// (ackItemFingerprint → core.ConsumePipelineDefectFingerprint, the ledger's
// single writer):
//
//	reconcileConsumedFingerprints — makes the ledger a PROJECTION of the
//	  consumed corpus (single-source-with-projection, ADR-0047). Swept by
//	  blockerBreakerHalt before it loads the ledger, so items consumed by ANY
//	  route — including the bare `mv` that produced this incident — self-heal
//	  the current live tree.
//	runInboxConsume — the ergonomic seam: `evolve inbox consume <item-path>`
//	  makes the move and the ack one transaction, so the manual step cannot be
//	  forgotten in the first place.
//
// The gate is PARSE-SUCCESS, never an item `kind`: enumerating the live inbox,
// kind:"pipeline-defect" matches zero items (the incident's own P0 and the
// driving item are both kind:"pipeline-repair"), so a kind-gated
// implementation would pass every fixture and never fire in production.
// Routine items carry no fingerprint and no-op naturally.
```

### `go/cmd/evolve/cmd_inbox_consume.go:152` — above `func releaseConsumedItemBinding(projectRoot, itemPath string, stderr io.Writer) bool {`

```text
// releaseConsumedItemBinding releases one just-consumed item's continuation
// binding through the ONE shared transaction (inboxmover.
// ReleaseContinuationBinding: preserve-then-delete, loud on a failed
// preserve, cycle-guarded delete — cycle-1507's H2 is exactly the drift a
// hand-rolled copy here reintroduced once, caught in review). The direct
// consume is an explicit operator statement that the work is closed, so no
// recency guard applies here; the SWEEP (inboxmover.ReconcileConsumedBindings)
// carries the cycle-1507 guards for everything consumed by other routes.
```

### `go/cmd/evolve/cmd_inbox_consume_binding_test.go:3` — above `import (`

```text
// cmd_inbox_consume_binding_test.go — the immortal-binding class through the
// MANUAL consume path (cycles 1487/1497 → recurred 2026-08-25 as cycle-1558:
// an operator `evolve inbox consume` moved the premise-challenge item to
// consumed/ but left its scope-keyed continuation binding in the registry,
// and the next wave minted a lane straight off it — a full lane burned
// re-proving finished work AGAIN). The ship-path release landed with the
// lane-scope-union fix; this pins the operator command and the consumed-corpus
// reconciler to the same contract: consumption releases the binding, and a
// binding whose item already lives in consumed/ is definitionally dead.
```

### `go/cmd/evolve/cmd_inbox_consume_binding_test.go:124` — above `func TestReconcileConsumedBindings_NewerBindingSurvivesStaleConsumedCopy(t *testing.T) {`

```text
// The cycle-1507 RECENCY guard, inherited by the sweep: a consumed copy OLDER
// than the binding is stale evidence — the id was re-filed and rebound after
// that retirement, and releasing would destroy live preserved work (measured
// 7/91 real bindings pre-guard). The sweep must skip it, loudly.
```

### `go/cmd/evolve/cmd_inbox_consume_test.go:3` — above `import (`

```text
// cmd_inbox_consume_test.go — RED contract for the operator-facing half of
// Defect A (fault-localization-report.md E4/E5/E8).
//
// The incident's P0 reached .evolve/inbox/consumed/ by an operator `mv`, and
// the ack never rode along because it is a SEPARATE manual command
// (`evolve inbox ack-fingerprint`) that nobody remembered to run. `evolve
// inbox consume <item-path>` makes the move and the ack one transaction, so
// the toil-and-tamper surface the sanctioned flows exist to avoid disappears.
//
// This is the ergonomic seam. The load-bearing self-heal is the reconciler
// in cmd_loop_blockerbreaker_reconcile_test.go, which covers items consumed
// by any route (including a bare `mv`) and repairs the CURRENT live tree.
// Both must share ONE extraction path — do not duplicate the
// unmarshal+parse+append sequence per call site.
//
// Every predicate here drives the REAL production entrypoint (runInbox),
// never a helper in isolation: a wiring proof is a reachability test.
```

### `go/cmd/evolve/cmd_inbox_consume_test.go:51` — above `func TestRunInbox_Consume_MovesItemAndAcksFingerprint(t *testing.T) {`

```text
// TestRunInbox_Consume_MovesItemAndAcksFingerprint is the transaction: one
// invocation must both land the item in consumed/ AND write the ack ledger,
// with no separate `ack-fingerprint` call.
//
// The fixture's kind is "pipeline-repair" — the value the incident's own P0
// and driving item carry. kind:"pipeline-defect" matches ZERO live items, so
// an implementation gated on it would pass a synthetic fixture and never
// fire in production.
```

### `go/cmd/evolve/cmd_inbox_mover.go:12` — above `func runInboxMover(args []string, _ io.Reader, stdout, stderr io.Writer) int {`

```text
// runInboxMover is `evolve inbox-mover <subcmd> [args]`. Mirrors
// legacy/scripts/utility/inbox-mover.sh exit codes:
//
//	0  — success (or promote no-op for ship.sh compat)
//	1  — not-found / bad args (claim only)
//	2  — mv failed (claim only)
//	3  — console-routed refusal (claim only; ADR-0074 — the item is
//	     operator-owned and must not be drawn by a lane)
```

### `go/cmd/evolve/cmd_inbox_mover_exitcodes_test.go:3` — above `import (`

```text
// cmd_inbox_mover_exitcodes_test.go — ADR-0103 unit 06 step 0 (test 14): the
// exit map's 2 (mv failed) and 3 (console-routed) arms, which the existing
// cmd tests never reached, pinned before the mover moves.
```

### `go/cmd/evolve/cmd_inbox_quarantine.go:1` — above `package main`

```text
// cmd_inbox_quarantine.go — operator surface for ADR-0072 S5 task quarantine.
//
//	evolve inbox quarantine list [--json]     show quarantined poison todos
//	evolve inbox quarantine release <id>      un-quarantine an item (reset its
//	                                          failure count, return to inbox root)
//
// This is the escape hatch for the automatic S5 quarantine wired into the loop's
// cycle-failure drain: an operator inspects why a todo was quarantined (the item
// JSON carries failure_count + last_failure_reason) and, once the root cause is
// fixed, releases it back into triage.
```

### `go/cmd/evolve/cmd_inbox_quarantine_test.go:3` — above `import (`

```text
// cmd_inbox_quarantine_test.go — the coverage-gate CRITICAL remediation for
// cycle-1019 (its report prescribed exactly these three groups): the
// `evolve inbox quarantine` operator surface, the isTaskLevelFailure
// classification the S5 quarantine decision hinges on, and the runInbox
// dispatch wiring. Salvaged console-first from the preserved worktree.
```

### `go/cmd/evolve/cmd_inbox_signals_test.go:3` — above `import (`

```text
// cmd_inbox_signals_test.go — ADR-0103 unit 06 step 4 (tests 54-55): the
// inbox module tag renders at the --simulate root and lands in the run
// workspace's signals.ndjson through the FAIL closeout; the Center-less
// `inboxmover.Options{` literals are an allow-list with reasons (the 06-F1
// debt), the three applyCycleFailureOutcome( call sites pass a non-nil
// signals token, and no production package constructs a second Center.
```

### `go/cmd/evolve/cmd_inbox_signals_test.go:165` — above `centerRoots := map[string]bool{"cmd/evolve/cmd_cycle.go": true, "internal/cli/phasecmd/phase_observer.go": true}`

```text
// Two process roots build a Center: the cycle/loop root (cmd_cycle.go) and
// the manual `evolve phase observer` subcommand (its own process; ADR-0103
// unit 12). Every other Center reaches a component through an accessor.
```

### `go/cmd/evolve/cmd_ledger.go:76` — above `func verifiedFrom(s ledger.VerifiedScope) string {`

```text
// verifiedFrom states WHICH history the verification just accepted. A success
// over a full-strict chain and a success that deliberately trusted an
// adjudicated prefix (ADR-0048's epoch anchor) are different claims, and
// printing one string for both is what let the ledger-1740 damage stay
// invisible. The anchor is named by its own identity — read from the ledger,
// never a literal — so two ledgers sealed at different lines read differently
// and an operator can carry the pair straight to `evolve ledger anchor`.
```

### `go/cmd/evolve/cmd_ledger.go:116` — above `func runLedgerAnchor(args []string, stderr io.Writer) int {`

```text
// runLedgerAnchor records a non-destructive epoch-anchor (ADR-0048; the
// ledger-1740 disposition). An OPERATOR action: it declares the pre-anchor
// prefix trusted-as-preserved and validates the chain strictly forward from the
// anchored line. Always re-verifies after, so an anchor that does not green the
// chain is surfaced immediately (rc 2), not at the next audit.
```

### `go/cmd/evolve/cmd_ledger_anchor_wiring_test.go:1` — above `package main`

```text
// cmd_ledger_anchor_wiring_test.go — cycle-1433 durable WIRING proof for the two
// ledger-fleet-concurrency-chain repairs.
//
// The ledger package owns the behavior; this file owns the seam. Both surfaces
// are operator-facing CLI, so a correct AnchorLine/Rebaseline that no subcommand
// or flag reaches is dead code — and the cycle-1433 ACS predicates that drive the
// compiled binary are cycle-scoped and retire. These tests call runLedger, the
// production dispatcher, so a dropped `--line-sha` flag or `rebaseline` case
// stays caught after cycle-1433's predicates are gone.
```

### `go/cmd/evolve/cmd_ledger_composition_test.go:1` — above `package main`

```text
// cmd_ledger_composition_test.go — cycle-786 TDD contract, AC3:
// "TestCompositionVerdict_KernelRecomputesPatchId (tampered entry rejected);
// ledger verify covers new entry kind" (inbox
// merge-rung0-trivial-rebase-carryforward).
//
// Contract: `evolve ledger verify` must, for every kind="composition-verdict"
// entry, kernel-recompute the patch-id of BOTH persisted diff artifacts
// (audited_diff_path, composed_diff_path — via `git patch-id --stable`) and
// require each to equal the entry's recorded patch_id. A forged match (the
// entry claims a patch_id its own recorded diffs do not hash to) is tampering
// and must break verify with exit 2, exactly like a chain break. Deterministic,
// zero LLM tokens — re-derivable by anyone from the entry alone.
//
// RED status at authoring (cycle 786): the tampered subtests FAIL — today
// Verify only walks the hash chain and ignores unknown entry kinds, so a
// tampered composition-verdict entry exits 0. The valid-entry test is a
// pre-existing GREEN guard pinning that the checker never over-rejects an
// honest entry.
```

### `go/cmd/evolve/cmd_lessons_recurrence_c662_test.go:3` — above `import (`

```text
// cmd_lessons_recurrence_c662_test.go — cycle-662 RED contract for
// chronicle-s1-recurrence-index AC5: `evolve lessons recurrence` must surface the
// backfilled NON-generic top patterns, not the operator-reset/loop-fatal noise
// floor that dominates raw counts. This pins the render seam: Generic entries are
// excluded from the report so a de-noised, actionable top-N is what an operator
// sees.
//
// Builder contract: renderRecurrenceReport must skip entries whose Generic flag
// is set (they are classification noise), while keeping specific-defect patterns.
//
// RED today: recurrence.Entry has no Generic field, so package main fails to
// compile. GREEN once Builder adds Entry.Generic and filters the render.
```

### `go/cmd/evolve/cmd_loop.go:1` — above `package main`

```text
// `evolve loop` drives the cycle dispatcher loop with batch budget
// enforcement. Sequential by design — each cycle blocks the next until
// it completes or trips the batch cap (matches v8.34.0+ bash
// dispatcher behavior).
//
// v11.5.0 M1–M6: CLI surface mirrors the now-removed bash dispatcher —
// positional args ([CYCLES] [STRATEGY] [GOAL...]), --goal-text (computes
// hash via goalhash.Compute), --strategy, --resume, --dry-run, --reset,
// --consensus-audit. Existing --goal-hash callers continue to work
// unchanged.
```

### `go/cmd/evolve/cmd_loop.go:63` — above `Fingerprint       string 'json:"fingerprint,omitempty"'`

```text
// Fingerprint is the operator-driven --fingerprint <fp> arg: with --reset,
// acknowledges this exact failure fingerprint in
// .evolve/resolved-fingerprints.json so the blocker-breaker's Rule B
// excludes it going forward (ADR-0072 extension operator unblock — the
// cycle-1329 recurrence fix). Empty = no-op, byte-identical pre-existing
// --reset behavior.
```

### `go/cmd/evolve/cmd_loop.go:103` — above `func signalStop(stdout, stderr io.Writer, lr *loopResult, where string) {`

```text
// signalStop is the ONE interrupt disposition (stop reason "signal", the
// resume hint, the result emit); callers supply only where the interrupt
// landed. prepareIteration's interruptReturn shares it (F20 review).
```

### `go/cmd/evolve/cmd_loop.go:146` — above `if pi, perr := plane.Classify(cfg.ProjectRoot); perr == nil {`

```text
// ADR-0080 S2: report the worktree plane at boot — a loop launched into
// the PRIMARY checkout shares its tree with the operator console, the
// exact overlap that killed lanes 1149-1152. Classification failure is
// non-fatal (an exotic checkout still runs; the tripwire just stays dark).
```

### `go/cmd/evolve/cmd_loop.go:177` — above `defer deps.Signals.Flush()`

```text
// ADR-0101 S2a: no queued signal is lost at loop exit (Center.Flush).
```

### `go/cmd/evolve/cmd_loop.go:194` — above `cycleEnv := buildCycleEnv(cfg, os.Environ())`

```text
// Build per-cycle env map by propagating EVOLVE_* OS env vars then
// applying dispatcher-derived overrides. Pre-this-fix, only an
// allowlist of 4 keys made it through, which silently dropped
// EVOLVE_REQUIRE_INTENT, EVOLVE_SANDBOX_FALLBACK_ON_EPERM, and every
// other operator-facing flag the CLAUDE.md env-var table documents
// (source incident: cycle-108 meta-loop ran with intent_required=false
// despite EVOLVE_REQUIRE_INTENT=1 set in the operator's shell).
```

### `go/cmd/evolve/cmd_loop.go:240` — above `func isTaskLevelFailure(c cycleclassify.Classification) bool {`

```text
// readCarryoverCount returns the number of carryoverTodos in state.json — the
// goal's remaining backlog the planning phases produced. ok is false when the
// file is absent/unreadable/malformed, so the caller skips the completion check
// rather than ever treating an unreadable state as "goal complete".
// isTaskLevelFailure reports whether a cycle classification is a task-level
// failure eligible for ADR-0072 S5 quarantine. Thin forwarder: the rule itself
// lives in internal/cycleoutcome beside the failure seam that acts on it, so
// the loop's quarantine surface and the closeout can never fork on "whose fault
// was this" (transient infra and system breaches take the S3 halt path and
// never quarantine a todo — AC4).
```

### `go/cmd/evolve/cmd_loop_args.go:92` — above `absOrWarn := func(label, p string) string {`

```text
// Enforce the flag's "absolute path" contract for the project root AND the
// evolve dir. Downstream, WorkspacePath (= <root>/.evolve/runs/cycle-N) and
// every per-phase artifact path are derived by joining these; worktree phases
// run the agent with cwd=worktree, so a RELATIVE base makes the agent resolve
// the artifact path into the worktree subtree while the in-process bridge
// polls it against the main cwd — that divergence caused cycle-119's
// ExitArtifactTimeout (81). Resolving once here (the composition root) keeps
// every derived path cwd-independent. filepath.Abs only errors when os.Getwd
// fails (cwd deleted/unmounted), in which case continuing with a relative
// base would silently reproduce the very timeout this guards against — so we
// WARN loudly rather than swallow it (the loop may still serve non-worktree
// phases, so we degrade rather than abort).
```

### `go/cmd/evolve/cmd_loop_args.go:114` — above `if posCycles > 0 && cyclesFlag == 0 && maxCyclesFlag == 0 {`

```text
// Legacy positional-integer deprecation WARN — operators relying on bare
// `/evo:loop 3 ...` get nudged toward `--cycles 3`.
```

### `go/cmd/evolve/cmd_loop_args.go:231` — above `func buildCycleContext(cfg loopConfig) map[string]string {`

```text
// buildCycleContext returns the Context map handed to every cycle.
// Phase agents read it via PhaseRequest.Context: Scout for strategy,
// Intent for the canonical goal text (used to structure intent.md
// before Scout sees it).
//
// Pre-this-fix, only "strategy" was passed — `cfg.GoalText` was
// converted to a hash at parse time and the text discarded. Intent
// persona had no way to see the operator's goal, so intent.md was
// being structured around whatever leftover Scout artifacts happened
// to be in the workspace. Source incident: cycle-108 meta-loop where
// the user's "non-stop autonomy + /goal comparison" goal-text was
// dropped and intent.md got structured around the prior cycle's
// untested-package backlog work instead.
```

### `go/cmd/evolve/cmd_loop_batch.go:27` — above `waveEngine *loopwave.Engine`

```text
// waveEngine is the unit-13 wave engine (ADR-0103), lazily built by wave().
```

### `go/cmd/evolve/cmd_loop_batch.go:69` — above `fleetCfg := loadFleetConfig(cfg.EvolveDir)`

```text
// FLEET-AS-POLICY S2: the batch-start snapshot — wave 0's baseline. Every
// iteration re-resolves the committed fleet block via
// reloadFleetConfigAtWaveBoundary below (cycle 739), so operator
// count/min_lanes directives committed mid-batch take effect at the next
// wave without a lane-killing bounce. Count==1 (absent block, the default)
// keeps iterations on the existing sequential path below — shouldRunWave
// gates the wave branch off entirely, so no Supervisor is ever constructed.
```

### `go/cmd/evolve/cmd_loop_batch.go:96` — above `var goalStall, nonprogress goalStallTracker`

```text
// Stall escalation (cmd_loop_goalstall.go): TWO counters over the running
// goal, both held outside the loop so a streak spans cycles, both
// escalate-and-continue. goalStall counts consecutive empty/blocked cycles
// (cycles 640-644). nonprogress counts the UNION — every cycle that landed
// nothing, FAIL included — because the empty-only counter and the
// consecutive-FAIL breaker each reset on the other's outcome, so an
// interleaved FAIL,EMPTY,FAIL,EMPTY goal escaped both. Thresholds/weight
// sourced from policy.json (never a Go literal).
//
// SEQUENTIAL PATH ONLY (review MEDIUM — do not read this as closing the
// fleet gap): the wave and pool branches `continue` above, and each lane is
// a separate `evolve cycle` process holding no cross-cycle streak, so no
// fleet lane advances either counter. Fleet breaker parity is a standing
// separate gap.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker.go:3` — above `import (`

```text
// cmd_loop_blockerbreaker.go — loop wiring of the mid-batch pipeline-blocker
// breaker (core.EvaluateBlockerBreaker; ADR-0072 extension, operator directive
// 2026-07-22). Checked at the TOP of every batch iteration, before another
// cycle is dispatched: a pipeline blocker must be fixed directly, never passed
// to the following cycles. On a trip it reuses the ADR-0072 halt machinery
// verbatim — escalation dossier + P0 pipeline-repair inbox item + the
// system-failure exit code — so operators have ONE halt vocabulary.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker.go:37` — above `reconcileConsumedFingerprints(evolveDir, stderr)`

```text
// The ack ledger is a PROJECTION of the consumed inbox corpus: sweep
// .evolve/inbox/consumed/ into it before loading, so a P0 that was consumed
// by ANY route (including a bare `mv`, which is how the cycle-1335 P0 got
// there) stops re-halting the breaker without an operator remembering the
// manual `evolve inbox ack-fingerprint` step.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker.go:72` — above `rule := loopHaltRule{code: CodeLoopPipelineBlockerHalt, fields: map[string]string{"rule": v.Rule, "fingerprint": v.Finge…`

```text
// ADR-0101 S4a: the breaker's rule names the ONE loop.halt INCIDENT the
// shared halt action emits (its code + the rule's fields); nothing is
// signalled twice.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker_reconcile_test.go:3` — above `import (`

```text
// cmd_loop_blockerbreaker_reconcile_test.go — RED contract for Defect A of
// the cycle-1335 incident (fault-localization-report.md E6, Defect A).
//
// The defect, verified on live state: .evolve/resolved-fingerprints.json
// DOES NOT EXIST, while the P0 that named the halting fingerprint sits
// consumed at .evolve/inbox/consumed/ with a consumed_by narrative that
// core.ParseConsumptionFingerprint parses correctly TODAY. The extraction
// logic exists and works; nothing non-interactive ever calls it. The only
// writer is the operator-invoked `evolve inbox ack-fingerprint`.
//
// The fix makes the ledger a PROJECTION of the consumed corpus (the repo's
// single-source-with-projection convention, cited by name at
// postship.go:110-113 / ADR-0047): blockerBreakerHalt reconciles
// .evolve/inbox/consumed/ into the ledger before loading it. This is the
// only variant that self-heals the CURRENT live state, where the P0 is
// already consumed and the ledger does not exist.
//
// Two load-bearing constraints, both verified against live data:
//
//  1. Gate on PARSE-SUCCESS, never on `kind`. Enumerating every live inbox
//     item, kind:"pipeline-defect" matches ZERO items; the incident's own P0
//     and the driving item are both kind:"pipeline-repair". A kind-gated
//     implementation passes every fixture test and never fires in production
//     — the unit-green/live-green trap that produced this incident.
//  2. A reconciler defect must not become a new boot blocker: per-item
//     errors WARN and the sweep continues.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker_reconcile_test.go:38` — above `const realConsumedByNarrative = "console-2026-08-05: fingerprint ship|unknown|76d0f4fca190 = root cause fixed"`

```text
// realConsumedByNarrative is the consumed_by string carried verbatim by the
// live incident item
// (.evolve/inbox/consumed/2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json).
// Fixtures use the live shape, never a synthetic one.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker_reconcile_test.go:44` — above `const incidentFingerprint = "ship|unknown|76d0f4fca190"`

```text
// incidentFingerprint is the live fingerprint from the cycle-1335 incident,
// carried verbatim by the digests of cycles 1326/1328/1329 on the real tree.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker_reconcile_test.go:62` — above `func TestBlockerBreakerHalt_ReconcilesAlreadyConsumedItem(t *testing.T) {`

```text
// TestBlockerBreakerHalt_ReconcilesAlreadyConsumedItem replays the live
// state exactly: the three incident digests are on disk, the P0 naming the
// fingerprint is ALREADY sitting in consumed/, and the ledger does not
// exist. The breaker must reconcile and not halt — and must materialize the
// ledger, so the projection is durable rather than recomputed silently.
//
// The fixture's kind is "pipeline-repair" (the real live value), NOT the
// "pipeline-defect" that matches zero production items: a kind-gated
// implementation must fail this predicate.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker_test.go:3` — above `import (`

```text
// cmd_loop_blockerbreaker_test.go — wiring pins for the mid-batch pipeline-
// blocker breaker (unit-green != live-green): the helper must read REAL digest
// artifacts, honor batch scoping, and on a trip leave the ADR-0072 breadcrumbs
// (escalation dossier + P0 pipeline-repair inbox item) exactly like the forged-
// verdict halt.
```

### `go/cmd/evolve/cmd_loop_blockerbreaker_test.go:74` — above `func TestBlockerBreakerHalt_AckedFingerprintDoesNotReHalt(t *testing.T) {`

```text
// TestBlockerBreakerHalt_AckedFingerprintDoesNotReHalt is the P1 item's
// explicit wiring-proof fixture: the actual loop-boot call site
// (blockerBreakerHalt) replaying the cycle-1329 incident (3x identical-
// fingerprint digests on disk) must NOT halt once the ack ledger carries a
// matching record, and must still halt (unchanged ADR-0072 behavior) when
// the ledger is absent.
```

### `go/cmd/evolve/cmd_loop_boot_recovery.go:21` — above `type bootRecoveryResult struct {`

```text
// cmd_loop_boot_recovery.go — WIRING of the boot-time recovery primitives into
// runLoop's boot path (cycle 507, task wire-boot-recovery-functions). This is
// the layer cycle 506 was missing: it built the core primitives (fully unit
// tested) but never called them from runLoop (audit F1, CRITICAL — the project's
// own warnship_apicover_ci_gap "green unit test, absent integration" trap).
//
// Mirrors the established runLoopPreflightFn / wireOrchestratorDepsFn package-var
// seam idiom so tests can spy the invocation. Best-effort / fail-open: a recovery
// error WARNs but never halts the batch.
```

### `go/cmd/evolve/cmd_loop_boot_recovery.go:202` — above `func attemptBootRepin(cfg loopConfig, stderr io.Writer) bool {`

```text
// attemptBootRepin re-pins expected_ship_sha to the on-disk ship binary via the
// shared, provenance-gated phaseintegrity.RepinIfDrifted — the SAME primitive the
// post-build repin (core.repinShipSHAAfterBuild) uses, so boot and post-build can
// never diverge (cycle 636, "never duplicate, centralize"). operatorAuthorized is
// always false here: an unattended boot must never let a tampered binary bypass
// the anti-tamper gate, so the re-pin fires only on verified provenance. Returns
// true iff the re-pin fired. Fail-open: a refusal/error WARNs and returns false,
// leaving the mismatch flagged.
```

### `go/cmd/evolve/cmd_loop_boot_recovery_repin_test.go:3` — above `import (`

```text
// cmd_loop_boot_recovery_repin_test.go — RED tests (cycle 514, task
// boot-recovery-auto-repin-shipsha). Cycle 507 wired *detection* of a ship-binary
// SHA mismatch into runLoop's boot path (detectShipSHAMismatch → res.SHAMismatch),
// but it only WARNs — it never invokes the existing, provenance-gated repin
// primitive phaseintegrity.RepinShipSHA. Result: the SELF_SHA_TAMPERED ship
// cascade recurred on cycles 508-513 (nine of the last ~20 cycles). This task
// closes the wiring gap: on a detected mismatch, boot recovery AUTO-REPINS
// expected_ship_sha to the on-disk binary WHEN (and only when) the running
// binary's build-commit is provenance-verified (git ancestor of HEAD) — the
// unattended-boot successor to `evolve reset-sha`. An unverifiable mismatch
// (possible tampering) must still be refused, so the anti-tamper guarantee holds.
//
// Contract the Builder implements (TDD-defined seam; mirrors the established
// bootRecoverFn / runLoopPreflightFn package-var seam idiom):
//
//	type bootRecoveryResult struct { Quarantined, Sealed, SHAMismatch, Healed bool }
//	// Healed == an auto-repin fired (the cascade was self-healed at boot).
//
//	// shipRepinProvenanceFn resolves the build-commit + provenance check used to
//	// authorize a boot-time auto-repin. A seam so boot recovery stays git-free
//	// (deterministic) in tests. Production: version.Commit() + a
//	// `git merge-base --is-ancestor <commit> HEAD` closure over cfg.ProjectRoot,
//	// exactly what runResetSHA (cmd_resetsha.go) uses.
//	var shipRepinProvenanceFn = defaultShipRepinProvenance
//	func defaultShipRepinProvenance(projectRoot string) (commit string, prov phaseintegrity.ProvenanceVerified)
//
//	// defaultBootRecovery: on a detected SHA mismatch, attempt an auto-repin via
//	// phaseintegrity.RepinShipSHA(statePath, actualSHA, commit, "", prov, false)
//	// — NEVER operatorAuthorized=true from an unattended boot. On repin success,
//	// set res.Healed=true. On provenance failure, keep today's warn-only behavior
//	// (res.SHAMismatch stays true; the pin is untouched; the ship gate still blocks).
//
// RED now (Healed field + shipRepinProvenanceFn undefined → package main test
// build fails). Do NOT modify this file — implement the seam.
```

### `go/cmd/evolve/cmd_loop_boot_recovery_test.go:3` — above `import (`

```text
// cmd_loop_boot_recovery_test.go — RED tests (cycle 507, task
// wire-boot-recovery-functions) for the WIRING of boot-time recovery into
// runLoop's boot path. THIS is the layer cycle 506 was missing: it built
// QuarantineDirtyTree / ShipSHAMismatch / AutosealStaleMarker (core, fully unit
// tested) but never called them from runLoop — audit F1, CRITICAL, "green unit
// test, absent integration" (the project's own warnship_apicover_ci_gap trap).
// The cycle was reset, so the functions no longer exist; this cycle reinstates
// them (core function contract in
// internal/core/boot_preflight_test.go + stale_marker_autoseal_test.go) AND
// wires them here.
//
// Contract the Builder implements (TDD-defined seam; mirrors the established
// runLoopPreflightFn / wireOrchestratorDepsFn package-var seam idiom):
//
//	type bootRecoveryResult struct { Quarantined, Sealed, SHAMismatch bool }
//	func defaultBootRecovery(ctx context.Context, cfg loopConfig, ledger core.Ledger, stderr io.Writer) bootRecoveryResult
//	var bootRecoverFn = defaultBootRecovery
//	// runLoop calls bootRecoverFn(ctx, cfg, deps.Ledger, stderr) BEFORE the
//	// readiness gate (loopPreflightHalts), so a dirty/stranded tree self-heals
//	// before the first cycle's tree-diff guard runs. Best-effort / fail-open:
//	// a recovery error WARNs but never halts the batch.
//
// RED now (undefined symbols → package main test build fails). Do NOT modify
// this file — implement the seam.
```

### `go/cmd/evolve/cmd_loop_boot_recovery_test.go:209` — above `func TestRunLoop_InvokesBootRecoveryBeforeGate(t *testing.T) {`

```text
// AC (the wiring cycle 506 lacked): runLoop must INVOKE bootRecoverFn during
// its boot path, BEFORE the readiness gate halts. A spy seam records the call;
// forcing the preflight gate to halt isolates the boot path (no cycle runs).
// This is the anti-dead-code assertion — a function defined but never called by
// runLoop fails HERE, exactly the trap 506 fell into.
```

### `go/cmd/evolve/cmd_loop_boot_recovery_timestamp_test.go:3` — above `import (`

```text
// cmd_loop_boot_recovery_timestamp_test.go — RED test (cycle 519, committed
// ## top_n slice of loop-cannot-selfheal-dirty-main-tree).
//
// TRIAGE COMMITTED ONE slice this cycle (triage-decision.json top_n): "implement
// ONLY the boot pre-flight slice — detect uncommitted tracked-source changes in
// the main tree at loop boot (git status --porcelain, excluding .evolve/ and
// knowledge-base/) and auto-quarantine via a TIMESTAMPED `git stash`".
//
// Detection, the .evolve/ + knowledge-base/ exclusion, and the non-destructive
// stash all shipped in cycles 507/514 (pre-existing GREEN — pinned as regression
// predicates in acs/cycle519). The one behaviour the committed slice ADDS is the
// TIMESTAMP: cmd_loop_boot_recovery.go:104 currently quarantines under the FIXED
// constant label "boot-quarantine", so every boot quarantine across every batch
// collapses under one ambiguous name — an operator cannot tell which stash came
// from which boot, and `git stash pop` on the wrong one silently restores the
// wrong leak. A timestamped label makes each boot quarantine individually
// identifiable and recoverable.
//
// RED now: the label is the bare constant, so labelOnly(...) == "boot-quarantine"
// and the timestamp assertion fails. Builder makes it GREEN by threading a
// timestamped label into the QuarantineDirtyTree call. Do NOT modify this file —
// implement the production seam.
```

### `go/cmd/evolve/cmd_loop_boot_refresh.go:3` — above `import (`

```text
// cmd_loop_boot_refresh.go — boot-time binary staleness self-heal (the
// binary-lag class, 2026-08-05 retro: docs/chronicle/2026-08-binary-lag.md).
//
// The loop ships fixes to main but keeps executing the binary it booted with;
// until an operator manually rebuilds, every landed pipeline fix is inert.
// Measured cost in one night: the sentinel tail-anchor fix landed at
// cycle-1301 while cycles 1302–1309 ran the old parser — three wasted lane
// cycles and the cycle-1309 identical-fingerprint batch HALT on a defect the
// repo had already fixed. This is deterministic operator toil, which belongs
// in code.
//
// At boot, BEFORE recovery and the readiness gate: if the running binary's
// embedded build commit is behind the plane HEAD by a delta that touches go/,
// rebuild via the canonical make target and re-exec the fresh binary in
// place. The re-exec'd process's existing boot machinery (auto-repin,
// recovery, preflight) then runs on the new binary. EVERY step is fail-open:
// any failure WARNs and boots the old binary — a stale batch is yesterday's
// status quo, a bricked loop is worse. A consume-once marker FILE
// (.evolve/boot-refresh-marker, value = healed-to HEAD) caps the self-heal
// at one attempt per target so a rebuild that does not change the stamp can
// never re-exec forever.
//
// DOCUMENTED INTENT (adversarial review 2026-08-05, findings 3/4/6):
//   - --resume never refreshes (the resume branch returns before this call):
//     resume is the minimal-perturbation single-cycle protocol — swapping the
//     executor mid-cycle is a bigger risk than one more stale cycle. The heal
//     lands at the next fresh boot.
//   - The refresh runs after runLoop's early boot side effects (plane
//     classification, socket GC, carryover auto-prune), so a healed boot
//     repeats them once in the child. The only non-idempotent one is the
//     carryover cycles_unpicked bump (×2 on heal boots) — accepted: heal
//     boots are rare by construction and the counter self-corrects at the
//     next pick; moving the call earlier would put it before the resume
//     branch and violate the resume exclusion above.
//   - Concurrent double-launch can race two rebuilds of go/bin/evolve
//     (non-atomic tool copy). No longer an accepted risk resting on an
//     operational assumption: bootRefreshFleetLaneFn ENFORCES it. The fleet
//     runs N>=1 concurrent lanes, so "simultaneous launches are excluded
//     operationally" was stale relative to the real topology; the standing
//     rule is "NEVER rebuild the plane binary mid-batch". A fresh per-run
//     .lease anywhere under <EvolveDir>/runs stops the refresh, and an
//     unverifiable check stops it too (fail-open like every other step).
//   - A repin-success + exec-failure boot runs with intra-batch skew: the old
//     loop image continues while subprocesses exec'ing go/bin/evolve get the
//     new code. Traced safe at ship time (verifySelfSHA hashes the file, not
//     the image); bounded to rare exec-failure boots.
```

### `go/cmd/evolve/cmd_loop_boot_refresh.go:102` — above `bootRefreshFleetLaneFn = defaultBootRefreshFleetLane`

```text
// bootRefreshFleetLaneFn reports whether ANOTHER fleet lane is
// concurrently active. The plane binary is shared by every lane, so a
// mid-batch rebuild+re-exec swaps the executable out from under running
// lanes (the stale-binary false-FAIL class, 2026-08-05). The heal is a
// convenience; a live batch is not. Both "yes" and "cannot tell" skip.
```

### `go/cmd/evolve/cmd_loop_boot_refresh.go:218` — above `if pol, _ := policy.Load(filepath.Join(cfg.EvolveDir, "policy.json")); pol.BootBinaryRefresh() == "off" {`

```text
// Operator dial (.evolve/policy.json boot.binary_refresh): "off" pins the
// current binary deliberately (incident bisects). A load error resolves to
// the compiled default ("auto") — the self-heal is integrity posture.
```

### `go/cmd/evolve/cmd_loop_boot_refresh_test.go:3` — above `import (`

```text
// cmd_loop_boot_refresh_test.go — the binary-lag class (2026-08-05 retro):
// fixes land on main but the loop keeps executing a frozen binary until an
// operator manually rebuilds. Measured cost overnight: the sentinel tail-anchor
// fix landed at cycle-1301 while cycles 1302–1309 ran the OLD parser — three
// wasted lane-cycles and the cycle-1309 identical-fingerprint batch HALT, on a
// defect already fixed in the repo. These tests pin the boot-time self-heal:
// detect staleness → rebuild → re-exec, every step fail-open (a refresh
// failure WARNs and boots the old binary — never bricks the loop).
```

### `go/cmd/evolve/cmd_loop_boot_selfsha_gate_test.go:3` — above `import (`

```text
// cmd_loop_boot_selfsha_gate_test.go — RED tests (cycle 639, task
// self-sha-fail-early-boot-gate; inbox 2026-07-08T03-05-30Z, weight 0.95).
//
// Problem this cycle closes: a WITHIN-VERSION ship-binary SHA mismatch
// (state.json:expected_ship_version == the current plugin version, but
// expected_ship_sha != the on-disk go/bin/evolve) is a BOOT-time-knowable,
// cycle-FATAL condition — it is exactly what the terminal ship gate calls
// SELF_SHA_TAMPERED (internal/phases/ship/verify.go:119-127, the "same version,
// different SHA" INTEGRITY-FAIL branch). Yet boot only WARNs and proceeds: 8
// consecutive cycles (625-634) each burned a full ~32-40 min scout→…→ship lane
// before dying at that terminal gate on a ship structurally doomed from boot.
//
// The fix classifies the mismatch at boot the SAME way verifySelfSHA does:
//
//   - within-version (expected_ship_version present AND == pluginVersion(root))
//     + SHA differs  → HALT pre-scout with the operator-unblock recipe
//     (`make -C go build` → `evolve reset-sha -operator` → relaunch). Do NOT
//     start scout. The auto-repin is NOT attempted — a within-version SHA change
//     is tampering/corruption by the ship-gate's own definition, not a legitimate
//     rebuild (a legit rebuild is version-bumped, or healed by the post-build
//     repin of cycle 636).
//   - across-version / legacy-unversioned mismatch → the EXISTING boot auto-repin
//     path (cycle 514, phaseintegrity.RepinIfDrifted), byte-for-byte unchanged.
//   - matching SHA → no mismatch, no halt; boot proceeds into scout.
//
// Contract the Builder implements (TDD-defined seam; extends the established
// bootRecoverFn / shipRepinProvenanceFn package-var seam idiom):
//
//	type bootRecoveryResult struct {
//	    Quarantined, Sealed, SHAMismatch, Healed bool
//	    HaltSelfSHA bool // NEW: a within-version SHA mismatch — boot must HALT pre-scout
//	}
//	// defaultBootRecovery: on a detected SHA mismatch, read expected_ship_version
//	// and compare to pluginVersion(cfg.ProjectRoot). If both non-empty AND equal
//	// (within-version), set res.HaltSelfSHA=true, print the operator-unblock recipe
//	// to stderr, and return WITHOUT attempting the auto-repin. Otherwise
//	// (across-version / legacy) keep today's detect→auto-repin behavior verbatim.
//	//
//	// runLoop, immediately after bootRecoverFn returns (BEFORE the unfinished-cycle
//	// guard and readiness gate — hence pre-scout), checks res.HaltSelfSHA and, if
//	// set, sets lr.StopReason="self_sha_boot_halt", emits, and returns 2. No scout
//	// phase, no readiness gate, no LLM budget spent.
//
// RED now (HaltSelfSHA field undefined → package main test build fails). Do NOT
// modify this file — implement the seam.
```

### `go/cmd/evolve/cmd_loop_boundaryrefresh_summary_test.go:3` — above `import (`

```text
// cmd_loop_boundaryrefresh_summary_test.go — RED tests (cycle 1330, inbox
// item auto-refresh-binary-at-boundary, chronicle
// docs/chronicle/2026-08-binary-lag.md "Remaining scope" item 2: "surfacing
// refresh events in the dossier/loop summary").
//
// PRIOR STATE (verified live in this worktree, not assumed from the scout
// report — the scout's own knowledge of "already shipped" traced main, not
// this worktree, and this worktree's chain-boundary mechanism is a SEPARATE,
// already-complete lineage from cycles 1314/1320/1323/1325):
//   - maybeRefreshChainBoundary (cmd_loop_chain.go) is fully implemented,
//     wired into BOTH runLoopChain (cmd_loop_chain.go:538) and runLoopBatch's
//     wave/fleet loop (cmd_loop.go:552), and covered by 18 existing GREEN
//     tests (cmd_loop_chain_boundaryrefresh_test.go,
//     _hardening_test.go, cmd_loop_wave_boundaryrefresh_wiring_test.go).
//     Remaining-scope item 1 (the chain-boundary hook itself) is therefore
//     PRE-EXISTING GREEN — this file adds no coverage for it.
//   - Every successful boundary refresh already appends a durable audit
//     record to .evolve/boundary-refresh-log.jsonl
//     (appendChainBoundaryRefreshLog, cmd_loop_chain.go) BEFORE the re-exec —
//     the "from stamp / to stamp" data chronicle item 2 asks to surface
//     already exists on disk. What is missing is the LAST MILE: that record
//     is never read back into the chain/loop summary JSON an operator (or a
//     dossier consumer) actually looks at. `chainResult` sets StopReason =
//     "chain_boundary_refresh_reexec" / loopResult sets StopReason =
//     "loop_boundary_refresh_reexec", but neither says WHICH commits were
//     involved without a separate grep of the JSONL file.
//
// THE GAP THIS CYCLE CLOSES (undefined until the Builder adds it — every
// reference below to a not-yet-existing symbol IS this file's RED evidence;
// no new detection/rebuild logic is invented, per
// never_duplicate_centralize_via_design_patterns — this is pure plumbing
// over the log file that already exists):
//
//	// lastChainBoundaryRefreshLogEntry reads
//	// <evolveDir>/boundary-refresh-log.jsonl and returns the LAST
//	// (most-recently-appended) entry. Best-effort, mirroring
//	// spineFailOpenRollup's nil-when-clean shape: a missing file, an empty
//	// file, or a read error all resolve to (nil, nil) — the field is simply
//	// absent from the summary, never a hard failure (a summary emission must
//	// not itself become a new failure mode for an already-successful boundary
//	// refresh).
//	func lastChainBoundaryRefreshLogEntry(evolveDir string) (*chainBoundaryRefreshLogEntry, error)
//
//	// chainResult gains:
//	BoundaryRefresh *chainBoundaryRefreshLogEntry `json:"boundary_refresh,omitempty"`
//	// populated at the SAME call site that already sets
//	// res.StopReason = "chain_boundary_refresh_reexec" (cmd_loop_chain.go).
//
//	// loopResult (cmd_loop_outcome.go) gains the identical field, populated
//	// at the wave-boundary call site that already sets
//	// lr.StopReason = "loop_boundary_refresh_reexec" (cmd_loop.go).
//
// Predicate strategy (cycle-85 degenerate-predicate ban: every predicate
// below exercises the real system under test, never a source-grep alone):
//
//	positive   — the helper returns the LAST of several appended entries
//	             with its fields intact (T1).
//	edge       — a missing log file, and an empty-but-present log file, both
//	             degrade to (nil, nil) rather than an error (T2, T3).
//	negative   — a nil BoundaryRefresh is OMITTED from marshaled JSON, not
//	             merely null (exact-omission assertion, T4/T5) — the
//	             cycle-131-class "the field is technically there but useless"
//	             failure mode.
//	wiring     — runLoopChain / runLoopBatch actually populate the new field
//	             at their respective re-exec-stop call sites, waived per
//	             acsassert's config-check convention because the field's own
//	             correctness (does it hold the right OldSHA/NewSHA) is proven
//	             by T1 above, and WHERE it must be called from is a structural
//	             fact a behavioral end-to-end drive would just duplicate at
//	             much higher fixture cost (T6/T7, mirrors
//	             TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary).
//
// acs-predicate: config-check — T6/T7 are caller-existence checks; the value
// they surface is already pinned behaviorally by T1-T5.
```

### `go/cmd/evolve/cmd_loop_boundaryrefresh_summary_test.go:160` — above `func TestChainResult_MarshalOmitsBoundaryRefreshWhenNil(t *testing.T) {`

```text
// T4 (negative — the cycle-131-class failure mode): chainResult with a nil
// BoundaryRefresh must OMIT the "boundary_refresh" key entirely from the
// marshaled JSON, not emit `"boundary_refresh": null`. A consumer that
// checks key-presence (not merely non-null) to decide "did a refresh
// happen this run" must see a clean, absent key on every ordinary run.
```

### `go/cmd/evolve/cmd_loop_boundaryrefresh_summary_test.go:198` — above `func TestRunLoopChain_SetsBoundaryRefreshOnReExecStop(t *testing.T) {`

```text
// T6: the chain must populate res.BoundaryRefresh at the same call site
// that already sets res.StopReason = "chain_boundary_refresh_reexec" —
// otherwise the durable audit record the mechanism already writes to
// boundary-refresh-log.jsonl stays permanently invisible to the JSON
// summary an operator/dossier consumer actually reads. Since ADR-0103 unit 13
// that call site is loopchain.(*Driver).boundary (the Result runLoopChain
// prints IS the summary schema), so the proof reads the leaf.
//
// acs-predicate: config-check — lastChainBoundaryRefreshLogEntry's own
// correctness is proven by T1-T3; this only proves the chain calls it.
```

### `go/cmd/evolve/cmd_loop_boundaryrefresh_summary_test.go:218` — above `func TestRunLoopBatch_SetsBoundaryRefreshOnWaveBoundaryReExecStop(t *testing.T) {`

```text
// T7 mirrors T6 for runLoopBatch's wave/fleet boundary stage
// (cmd_loop_window.go), the
// non-chain caller maybeRefreshChainBoundary also fires from (cycle 1325
// wiring). Both callers of the SAME refresh mechanism must surface the
// SAME summary field — surfacing only the chain-mode caller would silently
// leave the plain `evolve loop --max-cycles N` / fleet path's refresh
// events unobservable, exactly the asymmetry cycle-1325 closed for the
// trigger wiring itself.
//
// acs-predicate: config-check — see T6.
```

### `go/cmd/evolve/cmd_loop_carryover_prune_test.go:3` — above `import (`

```text
// cmd_loop_carryover_prune_test.go — RED test (cycle 507, task
// prune-stale-carryover-todos) for the WIRING of the carryoverTodos TTL prune
// into runLoop's startup. Behavioral, not a source grep: it seeds an EXPIRED
// carryover todo in state.json, runs runLoop far enough to execute the
// AutoPrune block (which sits before the readiness gate), and asserts the
// expired entry is actually gone from state.json on disk.
//
// This closes the "prune wired into cmd_loop.go startup, not dead code" AC the
// same way Task 1 closes its wiring AC — by observing the runtime side effect,
// so a prune function that runLoop never calls fails HERE (the cycle-506
// dead-code trap). The prune must run under the existing wc.AutoPrune flag,
// beside the failedApproaches PruneExpired call.
//
// RED now: runLoop does not yet prune carryoverTodos, so the expired entry
// survives. Do NOT modify this file — wire PruneExpiredCarryoverTodos into the
// AutoPrune block.
```

### `go/cmd/evolve/cmd_loop_chain.go:1` — above `package main`

```text
// cmd_loop_chain.go — the unit-13 chain seam (ADR-0103). The outer
// batch-chaining loop (cycle 1075; the standing operator directive of
// 2026-07-11 that lanes keep running until the inbox is empty) and the
// boundary binary refresh (cycle 1314) live in internal/loopchain as the
// Driver and the Refresher. This file keeps the eleven package-var test
// seams (read INSIDE the engines' closures at every construction, so a swap
// between calls is always seen), the two process adapters the leaf takes as
// required deps (`make -C go build`, syscall.Exec), the chain's stdout
// envelope, the chain root's Signal Center and the facades the by-name tests
// keep. The chain owns no cycle-level logic: the quota wall is the batch's
// rc=5 contract, which the Driver refuses to relaunch into.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_fleetlane_test.go:3` — above `import (`

```text
// cmd_loop_chain_boundaryrefresh_fleetlane_test.go — RED tests (cycle 1364,
// inbox item auto-refresh-binary-at-boundary).
//
// Gap found by this cycle's scout (fleet_scope pinned to this one item):
// maybeRefreshChainBoundary (cmd_loop_chain.go, landed cycles 1314/1320/1323/
// 1325) already implements the ahead-check -> rebuild -> repin -> ledger ->
// re-exec sequence and is wired into BOTH the chain loop (cmd_loop_chain.go)
// and the plain sequential loop (cmd_loop.go ~L552) — the "sequential loop
// AND fleet mode" caller-proof obligation this very phase's own house rules
// require is already satisfied for those two paths. What is still MISSING is
// the one safety check a stranded, never-landed salvage commit
// (cycle-42824668-1360, e057d1b3, ".../cmd_loop_boot_refresh.go") added for
// the OLDER boot-time-only healer and this newer boundary healer never
// inherited: refusing the rebuild while a SIBLING fleet lane holds a live run.
// `grep -n "FleetLane\|fleet.*lane" cmd_loop_chain_boundaryrefresh*.go` has
// zero hits in this worktree today — chainRebuildFn currently runs
// unconditionally once chainBoundaryAheadFn reports stale, even if another
// lane is mid-batch on the SAME shared go/bin/evolve binary this rebuild
// overwrites. That is the exact scenario the standing rule "NEVER rebuild
// plane binary mid-batch (SELF_SHA)" (project memory
// stale_binary_false_fail) exists to prevent, and it is the AC4 the
// stranded salvage commit's design already solved for the boot-time path.
//
// This file does NOT re-implement the salvage commit's bespoke
// EvolveDir/runs/* scanner (bootRefreshFleetLaneFn / defaultBootRefreshFleetLane
// in the stranded commit) — that would duplicate logic gc.Discover
// (internal/gc/discover.go) already owns and adversarially hardens (L3.2):
// the SAME lease-aware run-dir scan the retention engine uses to decide a run
// is untouchable. Reusing it here means one fewer independent .lease reader
// to keep in sync (never_duplicate_centralize_via_design_patterns).
//
// Contract the Builder implements (TDD-defined seam):
//
//	// chainBoundaryFleetLaneFn is the test seam for "is a sibling fleet lane
//	// active" — checked AFTER chainBoundaryAheadFn confirms staleness and
//	// BEFORE chainBoundaryRefreshAlreadyAttempted/chainRebuildFn are ever
//	// reached, so an active sibling lane refuses the boundary heal before
//	// either rebuild or exec (mirrors the boot-time healer's own guard order
//	// in the stranded salvage design). An error from the check is UNVERIFIABLE
//	// safety state and must be treated exactly like laneActive=true — "cannot
//	// prove the plane is idle" refuses the rebuild — while still letting the
//	// chain continue on the current binary (fail-open for the CHAIN, fail-safe
//	// for the REBUILD; these are not the same axis).
//	var chainBoundaryFleetLaneFn = defaultChainBoundaryFleetLaneActive
//	func defaultChainBoundaryFleetLaneActive(cfg loopConfig) (active bool, err error)
//
// defaultChainBoundaryFleetLaneActive wraps gc.Discover(cfg.EvolveDir,
// gc.DiscoverOptions{}) and reports true iff any returned RunDir has
// Live==true. At the instant maybeRefreshChainBoundary runs (strictly
// BETWEEN this lane's own batches — the prior batch's run dir is already
// terminal, the next one has not been created yet), this lane's own history
// never surfaces as Live, so no self-exclusion by path is needed; a Live
// entry unambiguously means a DIFFERENT lane.
//
// maybeRefreshChainBoundary MUST call chainBoundaryFleetLaneFn(cfg)
// immediately after a positive chainBoundaryAheadFn result and log a
// stderr line containing "fleet lane" before returning refreshed=false,
// exactly like every other guard in this function (rebuild failure,
// ahead-check error) — auditable, never a silent no-op.
//
// RED now: chainBoundaryFleetLaneFn / defaultChainBoundaryFleetLaneActive are
// undefined -> this package's test build fails. Do NOT modify this file —
// implement the seam and wire the call site in cmd_loop_chain.go.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_hardening_test.go:3` — above `import (`

```text
// cmd_loop_chain_boundaryrefresh_hardening_test.go — RED tests (cycle 1323,
// continuation of cycle 1320, inbox item auto-refresh-binary-at-boundary).
//
// Cycle 1320 landed the boundary-refresh sequence (ahead-check -> rebuild ->
// repin -> ledger -> re-exec) but its audit FAILed with 9 defects, 2 of which
// (D1 short-sha ahead-check, D2 short-sha test gap) are already fixed in this
// tree. The remaining OPEN defects are what THIS file encodes:
//
//	dd8a8d64 / dcaf44e4 (CRIT, same defect) — maybeRefreshChainBoundary passes
//	  trivialProvenance = func(string) bool { return true } into
//	  phaseintegrity.RepinShipSHA, stubbing out the ProvenanceVerified
//	  anti-tamper control and stamping Authorized="provenance" without ever
//	  verifying anything. ADR-0072 forged-verdict class.
//	d7542cf6 (MAJOR) — the repin hashes the RUNNING executable via
//	  selfsha.Running() instead of the rebuilt go/bin/evolve the rebuild just
//	  produced, so it pins the STALE hash under the forged label.
//	ddb8f717 (CRIT) — rebuild writes go/bin/evolve but the re-exec targets
//	  exec.LookPath(os.Args[0]); nothing guarantees the process that comes back
//	  is the binary that was just built.
//	de8b9e49 / df20cf48 (CRIT) — no re-exec loop breaker: any ahead-check false
//	  positive (or a rebuild that does not move the binary's build commit)
//	  becomes an unbounded rebuild -> repin -> re-exec livelock in which zero
//	  batches ever run, because the refresh check precedes chainStartDecision.
//	d9d245d4 — dead fallback: repinCommit = "boundary-refresh" on an empty
//	  running commit is unreachable AND would launder an unverifiable commit
//	  past a real provenance gate if it ever were reached.
//
// Contract the Builder implements. Three NEW package-var seams, mirroring the
// established postBuildRepinProvenanceFn / chainRebuildFn idiom, plus a
// re-shaped repin that reuses the SHARED primitive post_build_repin.go already
// uses instead of hand-rolling a second repin path:
//
//	// chainRunningCommitFn resolves the running binary's build commit.
//	// Production = version.Commit. A seam so the loop breaker's
//	// same-commit-twice behaviour is deterministic under test.
//	var chainRunningCommitFn = version.Commit
//
//	// chainBoundaryRepinProvenanceFn mirrors core.defaultPostBuildRepinProvenance:
//	// it returns the running binary's build commit AND a REAL
//	// phaseintegrity.ProvenanceVerified closure asserting that commit is an
//	// ancestor of HEAD (`git merge-base --is-ancestor <c> HEAD`). An empty
//	// commit is unverifiable and MUST return false. There is no sentinel
//	// substitute for an empty commit — an unstamped binary simply does not get
//	// to self-authorize a re-pin.
//	var chainBoundaryRepinProvenanceFn = defaultChainBoundaryRepinProvenance
//	func defaultChainBoundaryRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified)
//
//	// chainReExecTargetFn resolves the executable the boundary refresh re-execs
//	// into: the REBUILT <projectRoot>/go/bin/evolve, never os.Args[0]. An
//	// absent/non-executable target is an error and degrades to no refresh.
//	var chainReExecTargetFn = defaultChainReExecTarget
//	func defaultChainReExecTarget(projectRoot string) (string, error)
//
//	// chainBoundaryRefreshAttemptFile is the on-disk loop breaker. A refresh
//	// records the running commit that triggered it; a LATER refresh attempt
//	// carrying that SAME running commit means the previous re-exec came back on
//	// a binary that had not moved — it is refused (WARN, refreshed=false) so the
//	// chain degrades to running batches on the current binary instead of
//	// livelocking. On-disk because a re-exec destroys any in-process counter.
//	var chainBoundaryRefreshAttemptFile = "boundary-refresh-attempt.json"
//
// The repin itself becomes phaseintegrity.RepinIfDrifted(statePath,
// <projectRoot>/go/bin/evolve, commit, "", prov) — the SAME shared
// detect-drift + provenance-gate + repin path core.repinShipSHAAfterBuild uses,
// which fixes the forged-provenance and wrong-hash defects together and deletes
// the second hand-rolled repin path (never_duplicate_centralize).
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_hardening_test.go:124` — above `func TestMaybeRefreshChainBoundary_UnverifiedProvenanceRefusesRepinAndReExec(t *testing.T) {`

```text
// --- AC1 (dd8a8d64 / dcaf44e4): the provenance gate is REAL, not stubbed ---
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_hardening_test.go:201` — above `func TestMaybeRefreshChainBoundary_NeverSubstitutesSentinelForEmptyCommit(t *testing.T) {`

```text
// AC1 edge (d9d245d4) — the dead sentinel fallback must be gone: an unstamped
// binary (empty build commit) must never have the literal "boundary-refresh"
// laundered through the provenance gate in its place.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_hardening_test.go:239` — above `func TestMaybeRefreshChainBoundary_PinsShaOfRebuiltBinaryNotRunningExecutable(t *testing.T) {`

```text
// --- AC2 (d7542cf6): the pin is the sha of the REBUILT binary ---
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_hardening_test.go:275` — above `func TestMaybeRefreshChainBoundary_ReExecTargetsRebuiltBinaryNotArgv0(t *testing.T) {`

```text
// --- AC3 (ddb8f717): re-exec targets the rebuilt binary ---
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_hardening_test.go:358` — above `func TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker(t *testing.T) {`

```text
// --- AC4 (de8b9e49 / df20cf48): the re-exec loop breaker ---
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_hardening_test.go:452` — above `func TestRunLoopChain_LoopBreakerLetsBatchesRunAfterAFruitlessReExec(t *testing.T) {`

```text
// AC4 reachability (production caller) — drives runLoopChain, not the helper.
// With a permanently-true ahead-check, the FIRST chain run refreshes and stops
// for the re-exec, and the process that comes back (a second runLoopChain over
// the SAME .evolve dir, still on the same build commit) must run real batches
// instead of refreshing again. Today, with no breaker, the second run refreshes
// too and zero batches ever execute — the bricked chain of df20cf48.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_repinbranch_test.go:3` — above `import (`

```text
// cmd_loop_chain_boundaryrefresh_repinbranch_test.go — RED test (cycle 1356,
// inbox item auto-refresh-binary-at-boundary, task
// pin-boundary-repin-branch-residual).
//
// Residual embedded in the inbox item's own live-fire note (2026-08-05
// 17:24): a boot-time binary refresh healed via the RE-EXEC'D CHILD's
// boot-recovery auto-repin rather than the PARENT's pre-exec reconcile ("the
// pin heal fired in the CHILD's auto-repin... verify which parent-repin
// branch no-op'd and tighten its test").
//
// Read-first (rule 8): that boot-time mechanism (a `cmd_loop_boot_refresh.go`
// file, `bootRefreshRepinFn` seam) does not exist anywhere in this worktree's
// checked-out source —
//
//	grep -rn 'bootRefreshRepinFn|BootBinaryRefresh' go/   -> zero hits
//	find go -name 'cmd_loop_boot_refresh*.go'             -> no file
//
// This worktree's merge-base with origin/main (4dadf62a923640c) is 71 commits
// behind current main; the boot-time rebuild+re-exec self-heal is a main-line
// feature this worktree's snapshot predates. Re-litigating the EXACT
// parent/child split the note describes would mean writing a predicate
// against code that is not present here — inventing an API, which rule 8
// forbids.
//
// What IS present, and carries the identical shape (a repin that can fire on
// either side of a re-exec boundary), is THIS SAME inbox item's other half:
// maybeRefreshChainBoundary (cmd_loop_chain.go) re-pins expected_ship_sha
// BEFORE it re-execs. The re-exec'd child's own boot path
// (defaultBootRecovery -> detectShipSHAMismatch -> attemptBootRepin,
// cmd_loop_boot_recovery.go) is the other candidate healer. Nothing pins
// which of the two actually performs the heal for a boundary refresh, or
// proves the other is a documented no-op rather than an accidental race —
// exactly the ambiguity class the live-fire note flagged, applied to the
// mechanism this worktree actually has.
//
// This predicate proves: (1) maybeRefreshChainBoundary's pre-exec repin is
// the branch that performs the heal (state.json's pin moves BEFORE the
// re-exec seam is invoked), and (2) with that pin already moved, the child
// boot path's detectShipSHAMismatch reports NO mismatch — attemptBootRepin is
// therefore a documented no-op on the boundary-refresh path, never reached.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_selflease_test.go:3` — above `import (`

```text
// cmd_loop_chain_boundaryrefresh_selflease_test.go — RED-then-GREEN
// regression guard for cycle-1364 D1/D2 (defect-ledger ids
// d0dfe5b123f4142f7765c19a4e03b3f4d / d73c31a6ef1bb00dac049419a1207f939,
// inherited through cycle-1368 into this continuation).
//
// D1: defaultChainBoundaryFleetLaneActive (cmd_loop_chain.go) had no
// self-exclusion, so the CALLING lane's own just-heartbeated run dir (a
// fresh .lease, TTL runlease.DefaultTTL=10m, written by the same process
// that is now asking "is anyone ELSE live?") read back as a live sibling.
// maybeRefreshChainBoundary therefore refused the rebuild on every single
// boundary, silently disabling the auto-refresh-binary-at-boundary feature
// this fleet_scope item exists to provide.
//
// D2: cmd_loop_chain_boundaryrefresh_fleetlane_test.go's own regression
// guard (TestMaybeRefreshChainBoundary_NoFleetLaneActiveStillRefreshes)
// deliberately leaves runs/ empty and plants no lease at all — structurally
// incapable of exercising the "my OWN lease is still fresh" path D1 lives
// in. That file's header says "Do NOT modify this file", so the missing
// case is covered here instead, in a new file, per this cycle's build-report
// disposition of D2.
//
// This file plants the CALLING process's own PID (os.Getpid()) into a fresh
// lease under a run dir and asserts the boundary heal still proceeds — the
// exact scenario D1's evidence quoted ("active=true, live=3 including this
// lane's own .evolve/runs/cycle-1364").
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_selflease_test.go:88` — above `sibling := exec.Command("sleep", "30")`

```text
// A different pid — never our own os.Getpid() — so it must still count.
// A sibling is a different LIVE process (since 2026-09-15 liveness is the
// lease owner's, not the heartbeat's): hold a real child for the duration.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_test.go:3` — above `import (`

```text
// cmd_loop_chain_boundaryrefresh_test.go — RED tests (cycle 1314, inbox item
// auto-refresh-binary-at-boundary, task boundary-binary-refresh).
//
// Defect: runLoopChain (cmd_loop_chain.go) relaunches runLoopBatchFn at every
// boundary but never checks whether the RUNNING binary (version.Commit()) has
// fallen behind HEAD — fixes that land on main mid-chain (e.g. the sentinel
// tail-anchor fix at cycle-1301 64f8620e) sit inert until an operator manually
// rebuilds + `evolve reset-sha -operator` + relaunches. Cycles 1302-1309 kept
// running the old parser on a stale binary.
//
// Contract the Builder implements (TDD-defined seams; mirrors the established
// bootRecoverFn / shipRepinProvenanceFn package-var seam idiom from
// cmd_loop_boot_recovery.go):
//
//	// chainBoundaryAheadFn reports whether HEAD carries commits beyond
//	// runningCommit — the "is my binary stale" check. Reuses the EXACT
//	// ancestor-check idiom runResetSHA already uses (`git merge-base
//	// --is-ancestor <commit> HEAD`), just inverted: ahead=true means
//	// runningCommit is a STRICT ancestor of HEAD (HEAD has moved past it).
//	// An empty runningCommit (unstamped dev binary — nothing to compare) is a
//	// no-op: ahead=false, err=nil, no git subprocess. Any git/network/repo
//	// failure returns err!=nil — the caller MUST treat that as "skip this
//	// boundary's refresh", never halt the chain (AC4).
//	var chainBoundaryAheadFn = defaultChainBoundaryAhead
//	func defaultChainBoundaryAhead(projectRoot, runningCommit string) (ahead bool, err error)
//
//	// chainRebuildFn runs the sanctioned rebuild recipe (`make -C go build`,
//	// runtime-reference.md:170) so the on-disk binary catches up to HEAD.
//	var chainRebuildFn = defaultChainRebuild
//	func defaultChainRebuild(projectRoot string) error
//
//	// chainReExecArgvFn resolves the argv to re-exec — deliberately just
//	// os.Args, so runLoopChain's SIGNATURE (and every prior call site / frozen
//	// test) stays byte-identical; only a new package var is added.
//	var chainReExecArgvFn = func() []string { return os.Args }
//
//	// chainReExecFn is the seam over syscall.Exec so tests can assert the
//	// re-exec was invoked with the right argv without replacing the test
//	// process. Returns an error only if the exec syscall itself fails to
//	// launch (e.g. argv0 not executable) — in production a SUCCESSFUL
//	// syscall.Exec never returns at all.
//	var chainReExecFn = defaultChainReExec
//	func defaultChainReExec(argv0 string, argv, envv []string) error
//
//	// maybeRefreshChainBoundary runs the ahead-check -> rebuild -> repin ->
//	// audit-log -> re-exec sequence for boundary `batch`. It is called ONLY
//	// from runLoopChain between chainStartDecision deciding to continue and
//	// runLoopBatchFn — the loop body is single-threaded per boundary, so this
//	// call-site placement is what satisfies "refuse entirely while a batch is
//	// mid-flight" (AC2); no separate lock is needed. Every failure at every
//	// stage (ahead-check error, rebuild error, repin error, re-exec error)
//	// degrades to refreshed=false, logged to stderr, and the CURRENT binary
//	// keeps running the chain — never halts (AC4). The repin reuses
//	// phaseintegrity.RepinShipSHA UNCHANGED (protected surface,
//	// go/internal/phaseintegrity/ — this cycle does not touch it): the
//	// provenance closure simply re-asserts the freshly-rebuilt commit is HEAD,
//	// which is trivially true, so RepinShipSHA stamps its own
//	// Authorized="provenance" as today. The DISTINGUISHABLE "boundary-refresh"
//	// authorization class the inbox item asks for is a SEPARATE, additive audit
//	// record — chainBoundaryRefreshLogFile (a JSONL file under evolveDir,
//	// .evolve/boundary-refresh-log.jsonl) — so a ledger read can tell an
//	// automatic boundary repin apart from a manual `evolve reset-sha`
//	// (RepinShipSHA's own two-value Authorized enum is unchanged; the THIRD
//	// class lives one layer up, in the chain-owned log).
//	var chainBoundaryRefreshLogFile = "boundary-refresh-log.jsonl"
//	func maybeRefreshChainBoundary(cfg loopConfig, batch int, stderr io.Writer) (refreshed bool)
//
// Wiring: runLoopChain calls maybeRefreshChainBoundary(cfg, n+1, stderr)
// immediately after chainStartDecision's continue branch and before the
// existing `width := loadFleetConfig(...)` line; refreshed==true breaks the
// loop with res.StopReason = "chain_boundary_refresh_reexec", exit 0 (the new
// process, once re-exec'd, resumes the chain from scratch under the same
// args). runLoopChain's own SIGNATURE is UNCHANGED — every existing frozen
// call site (cmd_loop.go:143, cmd_loop_chain_test.go and siblings) keeps
// compiling untouched.
//
// RED now: chainBoundaryAheadFn / chainRebuildFn / chainReExecArgvFn /
// chainReExecFn / maybeRefreshChainBoundary are all undefined -> this
// package's test build fails. Do NOT modify this file — implement the seams
// in cmd_loop_chain.go.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_test.go:248` — above `root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")`

```text
// Cycle-1323 fixture update (contract unchanged, setup completed): the
// re-pin now hashes the REBUILT <root>/go/bin/evolve and the provenance gate
// is real, so the fixture must supply both. Every assertion below is the
// cycle-1320 assertion, verbatim.
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_test.go:403` — above `inboxDir := filepath.Join(evolveDir, "inbox")`

```text
// Seed >=3 pending inbox items so chain_inbox_empty (an n>0-gated CONTINUE
// condition, cmd_loop_chain.go chainStartDecision) never fires before the
// 3rd boundary's [ahead-check, batch] pair is recorded — an empty inbox
// would stop the chain after batch 1, making the calls>=3 branch in the
// runLoopBatchFn stub below unreachable (cycle-1314/1315 fixture bug).
```

### `go/cmd/evolve/cmd_loop_chain_boundaryrefresh_test.go:455` — above `root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")`

```text
// Cycle-1323 fixture update (see the sibling test): a rebuilt binary to
// re-exec into and a real provenance seam. Assertions are unchanged.
```

### `go/cmd/evolve/cmd_loop_chain_inboxvalidity_test.go:13` — above `func writeInboxFile(t *testing.T, evolveDir, name, body string) {`

```text
// cmd_loop_chain_inboxvalidity_test.go — cycle-1098 RED contract tests for
// chain-inbox-pending-validity: inboxPendingCount counts every root-level
// `*.json` with no shape validation, so one malformed or non-item file pins
// pending>0 permanently and the chain burns batches to max_batches consuming
// nothing — reaching the runaway cap via a FALSE signal. Skips are silent
// today, which would also hide a real item lost to a typo.
//
// Builder makes these pass by changing production code only. Helpers
// (chainTestEnv/stubBatches/runChain) come from cmd_loop_chain_test.go.
```

### `go/cmd/evolve/cmd_loop_chain_minbatch_test.go:11` — above `func TestChainStartDecision_MinOneBatchOnDrainedInbox(t *testing.T) {`

```text
// cmd_loop_chain_minbatch_test.go — cycle-1098 RED contract tests for
// chain-min-one-batch: `--until-inbox-empty` against an ALREADY-drained inbox
// currently returns rc=0 having run ZERO cycles, silently weaker than the
// pre-chain contract where `evolve loop` always ran one batch. Chaining was
// specified as "keep going past the boundary", never as "may run nothing at
// all". The drained-inbox check is a CONTINUE condition mis-sited as a START
// condition.
//
// These tests are the TDD contract: Builder makes them pass by changing
// production code only. The helpers (chainTestEnv/stubBatches/runChain) are
// reused from cmd_loop_chain_test.go — same package, no duplicate fixtures.
```

### `go/cmd/evolve/cmd_loop_chain_test.go:17` — above `func chainTestEnv(t *testing.T, items int, policyJSON string) loopConfig {`

```text
// cmd_loop_chain_test.go — the fake-runner harness for the outer batch-chaining
// loop (cycle 1075). The batch dispatcher is replaced via runLoopBatchFn so the
// BOUNDARY decisions (start another batch? stop, and why?) are exercised
// end-to-end without spawning cycles: every test below asserts on the chain's
// own summary + the number of batches the fake actually saw.
```

### `go/cmd/evolve/cmd_loop_chain_test.go:98` — above `func TestRunLoopChain_InboxDrainStartsNextBatchThenCleanExit(t *testing.T) {`

```text
// TestRunLoopChain_InboxDrainStartsNextBatchThenCleanExit — AC1. Two pending
// todos, one consumed per batch: the chain must start batch 2 with no external
// invocation and then exit CLEAN (rc=0) once the inbox is empty.
```

### `go/cmd/evolve/cmd_loop_chain_test.go:124` — above `func TestRunLoopChain_QuotaExhaustionDefersInsteadOfRelaunching(t *testing.T) {`

```text
// The cycle-1075 test TestRunLoopChain_EmptyInboxExitsWithoutRunningABatch
// asserted the OPPOSITE of today's contract: that a chain launched against an
// already-drained inbox runs ZERO batches. Cycle 1098 (`chain-min-one-batch`)
// judged that a defect — opting into chaining was silently weaker than the
// pre-chain contract, where `evolve loop` always ran one batch — so the
// behaviour it pinned is deliberately reversed, not merely relaxed. Its
// coverage is not lost: the drained-inbox launch is now pinned by
// TestRunLoopChain_DrainedInboxRunsExactlyOneBatch (exactly one batch, rc=0,
// chain_inbox_empty) in cmd_loop_chain_minbatch_test.go, and the zero-batch
// outcome it guarded survives for the case that still means it —
// TestRunLoopChain_PreEngagedBrakeRunsZeroBatchesOnDrainedInbox.
```

### `go/cmd/evolve/cmd_loop_chain_test.go:181` — above `func TestRunLoopChain_LoopStopFileBrakeHalts(t *testing.T) {`

```text
// TestRunLoopChain_LoopStopFileBrakeHalts — AC4. `.evolve/loop-stop` dropped
// while batch 1 runs must halt the chain at the next boundary even though the
// inbox still has work and the cap is far away.
```

### `go/cmd/evolve/cmd_loop_chain_test.go:233` — above `func TestRunLoopChain_BatchErrorStopsChain(t *testing.T) {`

```text
// TestRunLoopChain_BatchErrorStopsChain — a fatal batch outcome (preflight
// failure, unfinished cycle, ADR-0072 halt) must stop the chain and propagate
// the code, while an rc=3 batch (completed with absorbed failures) must NOT
// halt the queue.
```

### `go/cmd/evolve/cmd_loop_chain_test.go:322` — above `func TestInboxPendingCount(t *testing.T) {`

```text
// TestInboxPendingCount pins that only unclaimed top-level todos count, and
// that a missing inbox is zero rather than an error. Cycle 1098 added the third
// return value (the skip list); well-formed fixtures must produce NO skips —
// shape validation must not start rejecting real items.
```

### `go/cmd/evolve/cmd_loop_committed_ids_test.go:3` — above `import (`

```text
// cmd_loop_committed_ids_test.go — cycle-1176, task
// `wave-lane-claim-into-processing` (inbox item wave-lane-task-quarantine-dead).
//
// failedCycleCommittedIDs (cmd_loop.go:1060) is the reader that feeds
// CycleOutcome.CommittedIDs at the production FAIL site (cmd_loop.go:728). It
// is the hinge of the whole quarantine-dead fix: return the wrong set and the
// drain either bumps nothing (ceiling stays unreachable — the original defect)
// or bumps the entire menu (healthy backlog quarantined after N failures of an
// unrelated task). It had no direct coverage; these tests pin its contract.
//
// The `_ =` on the fixture writes is deliberate: t.Fatal on error would be
// noise for a t.TempDir write that cannot realistically fail, and every
// assertion below reads the value back, so a silent write failure still fails
// the test loudly at the assertion.
```

### `go/cmd/evolve/cmd_loop_committed_ids_test.go:66` — above `func TestFailedCycleCommittedIDs_ExcludesDeferredAndDropped(t *testing.T) {`

```text
// TestFailedCycleCommittedIDs_ExcludesDeferredAndDropped — NEGATIVE, menu
// semantics (PR #366). An id triage explicitly did NOT commit to must never
// enter the committed set: it accrues no failure_count and cannot be walked
// toward the S5 ceiling by a failure it had no part in.
```

### `go/cmd/evolve/cmd_loop_control.go:27` — above `var disableWorkspaceGuardForTest bool`

```text
// disableWorkspaceGuardForTest is a test seam: package-level test harnesses
// that pre-seed cycle workspaces (M4/M5 dispatch validators, etc.) set this
// to true so the orchestrator does not archive the pre-seeded files before
// phases run. Production code always leaves this false. Replaces the retired
// EVOLVE_DISABLE_WORKSPACE_GUARD env signal (cycle-10 flag-reduction).
```

### `go/cmd/evolve/cmd_loop_control.go:45` — above `const orphanGCTimeout = sessionreaper.DefaultReapTimeout`

```text
// orphanGCTimeout bounds the crash-recovery session sweep so a wedged tmux
// socket (corrupted server, not the common "no server" case) can never hang the
// loop — the GC must stay robust even when the surrounding pipeline is broken.
// Hoisted to sessionreaper.DefaultReapTimeout so the boot preflight and soak
// checks share the same bound (cycle-769).
```

### `go/cmd/evolve/cmd_loop_control.go:145` — above `SignalSummary() signalcenter.Summary`

```text
// SignalSummary is the runner's per-cycle view of the Signal Center, read
// by the batch report (ADR-0101 S4a) — through this seam, so a scripted
// runner is reported exactly like the real orchestrator.
```

### `go/cmd/evolve/cmd_loop_control.go:180` — above `func readBatchWindowFloor(ctx context.Context, st core.Storage) (int, error) {`

```text
// readBatchWindowFloor returns the cycle number the blocker breaker's batch
// window starts above: max(LastCycleNumber, LastAllocatedCycleNumber).
//
// Two counters with two meanings. LastCycleNumber tracks cycles COMPLETED —
// an aborted cycle exits through abnormalEpilogue, which writes its failure
// digest but returns before finalizeCycle, so the counter never advances past
// it. LastAllocatedCycleNumber tracks cycles DISPATCHED: it advances at mint
// time (alloc.go — "a crashed run BURNS its number"). A time boundary belongs
// on the dispatch counter, otherwise the digests of aborted cycles stay inside
// `> floor` on every relaunch and Rule B re-trips before any cycle runs (the
// cycle-1335 triple re-halt: lastCycleNumber=1325 while cycles 1326/1328/1329
// had aborted with one shared fingerprint).
//
// max, not a bare swap: allocateCycle falls back to LastCycleNumber+1 when
// storage is not a StateUpdater, so a legacy state can carry a zero lease —
// anchoring on it alone would zero the window and re-collect all of runs/.
```

### `go/cmd/evolve/cmd_loop_control.go:466` — above `func (lr *loopResult) emitQuotaPause(cfg loopConfig, cycle int, stdout, stderr io.Writer) {`

```text
// parseLoopArgs parses `evolve loop` arguments per the v11.5.0 M1 CLI
// surface. Returns the resolved config + rc (0 = success, 10 = bad
// args, exits printed to stderr).
//
// Argument precedence:
//
//	--goal-hash takes priority over --goal-text (--goal-text computes hash)
//	--goal-text takes priority over positional [GOAL...]
//	--cycles / --max-cycles take priority over positional [CYCLES]
//	--strategy takes priority over positional [STRATEGY]
//
// Positional parsing matches the bash dispatcher heuristic at
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:325-349:
//
//	first numeric token (if any) → CYCLES
//	next token if matching strategy whitelist → STRATEGY
//	remaining tokens (joined by space) → GOAL
```

### `go/cmd/evolve/cmd_loop_cyclelevel_test.go:1` — above `package main`

```text
// cmd_loop_cyclelevel_test.go — cycle-234 task `cycle-level-bridge-failure` (RED).
//
// Loop-side half of Invariant 3: when RunCycle returns a CYCLE-level failure
// (core.ErrCycleLevelFailure — bridge exhaustion, contract dead-end, …), the
// batch must log it and CONTINUE to the next cycle. Today cmd_loop.go breaks
// on ANY error with StopReason="error" rc=2 — the exact batch-fatal inversion
// that killed batches c225/c230/c231.
//
// Uses the wireOrchestratorDepsFn seam + the m4 stub fakes (noopRunner,
// newFakeLedger, fixtures.FakeStorage) defined in cmd_loop_m4_test.go.
```

### `go/cmd/evolve/cmd_loop_env_test.go:7` — above `func TestBuildCycleEnv_PropagatesRequireIntent(t *testing.T) {`

```text
// TestBuildCycleEnv_PropagatesRequireIntent is the regression test for
// the cycle-108 silent-skip bug: EVOLVE_REQUIRE_INTENT=1 in the
// operator shell MUST land in CycleRequest.Env so the orchestrator's
// intent gate at orchestrator.go:126 evaluates true.
```

### `go/cmd/evolve/cmd_loop_env_test.go:66` — above `func TestBuildCycleEnv_DispatcherFlagsPropagate(t *testing.T) {`

```text
// TestBuildCycleEnv_DispatcherFlagsPropagate covers the explicitly-set
// dispatcher IPC flags (Resume) — present iff bool set.
// ConsensusAudit is no longer written to the cycle env; it is configured
// via policy.json workflow.consensus_audit_enabled instead.
// EVOLVE_RESET is no longer written: cfg.Reset is consumed at cmd_loop.go
// before buildCycleEnv is called (dead env write removed, cycle-44).
```

### `go/cmd/evolve/cmd_loop_env_test.go:107` — above `func TestBuildCycleContext_PropagatesGoalText(t *testing.T) {`

```text
// TestBuildCycleContext_PropagatesGoalText is the regression test for
// the cycle-108 silent-drop bug #3: --goal-text "..." flag value must
// land in CycleRequest.Context["goal"] so the Intent persona can
// structure intent.md around the operator's actual goal rather than
// inferring from leftover workspace artifacts.
```

### `go/cmd/evolve/cmd_loop_escalate.go:59` — above `emitLoopEscalation(signals, cycle, "applyEscalationBoundary",`

```text
// ADR-0101 S4a: the boundary's outcome is a loop.escalation WARN.
```

### `go/cmd/evolve/cmd_loop_escalate_test.go:69` — above `if !strings.Contains(stderr.String(), "LOOP_ESCALATION_BOUNDARY") || strings.Contains(stderr.String(), "escalation bound…`

```text
// ADR-0101 S4a: the boundary's outcome is a loop.escalation WARN the stub
// root's console renders into this stderr.
```

### `go/cmd/evolve/cmd_loop_escalation_test.go:14` — above `func TestWritePipelineEscalation_WritesDossierAndInboxItem(t *testing.T) {`

```text
// ADR-0072 S6: the halt writes a diagnostic dossier AND auto-files a P0
// pipeline-repair inbox item — so the QUEUE is injected (never_stop honored)
// even though the loop halts. On resume the pipeline fix is worked first.
```

### `go/cmd/evolve/cmd_loop_escalation_test.go:60` — above `if item["injected_by"] != escalationInjectedBy {`

```text
// Research F25: the halt record carries its producer, so the ADR-0074
// classifier's route:"lane" clamp keeps it console-owned however it is
// later annotated.
```

### `go/cmd/evolve/cmd_loop_escalation_test.go:204` — above `func TestWritePipelineEscalation_IdentityIncludesCycleNumber(t *testing.T) {`

```text
// TestWritePipelineEscalation_IdentityIncludesCycleNumber pins the fix for the
// todo-halt-autofiler-mints-unique-ids carryover (state.json): the auto-filed
// inbox item's id must be minted as pipeline-defect-<category>-cycle<N>, never
// pipeline-defect-<category> alone. A category-only id is not a record
// identity — cycle 1550 dispatched into scope committed to a scope snapshot of
// the bare category id, but the on-disk record it once pointed at had already
// been overwritten by a LATER halt sharing that category, so scout's live scan
// found no matching content and the lane ran 8 phases for an empty diff (the
// same inst-L1543c empty-scope class that FAILed audit at cycle 1548, defect
// H1). Stamping the cycle into the id makes every halt's record identity
// unique, so a scope snapshot can never resolve to a different halt's content.
```

### `go/cmd/evolve/cmd_loop_escalation_test.go:244` — above `func TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk(t *testing.T) {`

```text
// TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk is the negative
// case for the same defect: two ADR-0072 halts sharing a category (the common
// case — "pipeline-blocker" alone has 17 on-disk records today per
// state.json) must NOT collapse onto one inbox file. Under the current
// category-only filename, the second halt's atomicwrite.JSON silently
// destroys the first halt's evidence and a future lane scoped to the id
// resolves to whichever halt happened to write last — never the one it was
// actually scoped to. Both cycles' records must survive side by side.
```

### `go/cmd/evolve/cmd_loop_failbreaker_test.go:19` — above `func TestConsecutiveFailBreaker(t *testing.T) {`

```text
// cmd_loop_failbreaker_test.go — EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS: the
// circuit-broken continue-on-verdict-FAIL. Soaks #3/#3b/#3c/#3d (2026-06-13)
// each ended on the FIRST cycle whose FinalVerdict was FAIL, even when the
// failure was a localized work-quality miss in an otherwise healthy batch —
// turning every miss into an operator relaunch and preventing any
// 3-consecutive-PASS streak from forming. The flag lets a batch absorb up to
// max-1 consecutive FAILs (a streak of PASS/SHIPPED resets the count); the
// default of 1 reproduces the pre-flag stop-on-first-FAIL contract exactly.
```

### `go/cmd/evolve/cmd_loop_fingerprint_ack_test.go:3` — above `import (`

```text
// cmd_loop_fingerprint_ack_test.go — caller proof for the cycle-1332
// blocker-breaker fingerprint-ack CLI flag: `evolve loop --reset
// --fingerprint <fp>`, driven from the REAL production entrypoint (runLoop),
// must append the ack ledger record. A predicate that calls
// core.AppendResolvedFingerprint directly proves nothing about the
// operator-facing flag actually being wired end-to-end.
```

### `go/cmd/evolve/cmd_loop_gc_amplify_test.go:10` — above `func TestAmplifyGCNoPolicyFileDefaultsToShadow(t *testing.T) {`

```text
// TestAmplifyGCNoPolicyFileDefaultsToShadow: no policy.json on disk at all →
// gcPol.Mode="" → workspace-hygiene S5 resolves it to "shadow", so the run-dir
// manifest IS published (shadow never mutates). Amended in cycle 1159: this
// test previously asserted the pre-S5 ""→off default, the exact behavior the
// slice inverts (same amendment as TestGCOff). The assertion is not weakened —
// explicit gc.mode=off remains pinned by TestGCOff and
// TestRunGCHook_ExplicitOffSkipsWorktreeSweep.
```

### `go/cmd/evolve/cmd_loop_gc_amplify_test.go:33` — above `func TestAmplifyGCShadowContinuesAfterPolicyLoadError(t *testing.T) {`

```text
// TestAmplifyGCShadowContinuesAfterPolicyLoadError verifies that when policy.json
// is unreadable, runGCHook logs a WARN and CONTINUES on the zero-value policy.
// Amended in cycle 1159 (workspace-hygiene S5): an unreadable policy yields
// mode="" which now resolves to "shadow", not "off". That is still the safe
// default — shadow only plans and publishes, it never mutates the tree, which
// the run-dir assertions below pin directly.
```

### `go/cmd/evolve/cmd_loop_gc_batchend_test.go:3` — above `import (`

```text
// cmd_loop_gc_batchend_test.go — RED tests for cycle-1172, inbox item
// `workspace-hygiene-s5-wiring-shadow-default` (scout task
// workspace-hygiene-s5-batch-end-gc-hook).
//
// WHAT IS ALREADY DONE (cycle-1159): runGCHook exists, defaults an absent
// gc.mode to "shadow", and drives BOTH gc.Plan (run dirs) and
// gc.PlanWorktrees/ApplyWorktrees (worktree+branch backlog).
//
// THE REMAINING GAP: the hook has exactly ONE call site — cmd_loop.go:408,
// which fires at batch START, before the preflight/cycle loop begins. The S5
// plan (docs/plans/workspace-hygiene-2026-07.md) specifies a batch-END
// invocation with "finalize FIRST then hook": the sweep must observe the
// just-finished batch's state (a finalized, marker-cleared final cycle), not
// the state left over from the PREVIOUS batch. A start-only hook can never
// reap the worktrees the batch it just ran produced — the backlog it is meant
// to drain always lags one batch behind.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//
//  1. Introduce the package-var seam `var gcHookFn = runGCHook` (the
//     bootRecoverFn / runLoopPreflightFn / runLoopBatchFn idiom already used in
//     this package) and route EVERY gc-hook call site through it.
//  2. On a clean batch exit (max-cycles budget reached / normal completion),
//     the loop must invoke gcHookFn AFTER the batch's final
//     finalizeCompletedCycle — proven here by the spy observing that
//     cycle-state.json is already GONE at hook time — and after the batch's
//     cycles have run.
//  3. Keeping or dropping the existing batch-START call is the implementer's
//     call (scout AC1); these tests assert only about the LAST invocation, so
//     either choice passes. Document whichever you pick.
//  4. NEGATIVE: a signal-interrupted exit (rc=130) must NOT fire a batch-end
//     sweep. That run is resumable (`evolve loop --resume`), its cycle-state
//     marker is deliberately preserved, and reaping its worktrees/branches
//     would destroy the very state the resume needs.
```

### `go/cmd/evolve/cmd_loop_gc_batchend_test.go:100` — above `func TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd(t *testing.T) {`

```text
// TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd is the cycle-1172 crux:
// the sweep must run at batch END, after the final cycle and after finalize.
// The fixture writes a REAL cycle-state.json marker that is present at batch
// start, so a start-only hook (today's single call site) records only
// markerPresent==true/cycleHadRun==false invocations and fails here.
```

### `go/cmd/evolve/cmd_loop_gc_test.go:3` — above `import (`

```text
// cmd_loop_gc_test.go — RED white-box tests for the L3.4 GC loop hook
// (task gc-shadow-wiring, cycle 298). These call the unexported runGCHook
// directly so they exercise the real Discover→Plan→(Apply) wiring against a
// synthetic .evolve tree and assert on observable side effects (manifest file
// written / not written, run dirs mutated / preserved, live runs excluded).
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - Add `func runGCHook(cfg loopConfig, workspace string, stderr io.Writer)`
//     to cmd_loop.go. It reads os.Getenv("EVOLVE_GC") (default "off"):
//       off            → return immediately, no manifest.
//       <invalid>      → log a warning to stderr, return; no manifest, no crash.
//       shadow         → policy.Load + gc.Discover + gc.Plan; write
//                        <workspace>/gc-shadow-manifest.json (valid JSON,
//                        decodes to gc.Manifest); NO filesystem mutations.
//       enforce        → shadow + gc.Apply (run dirs actually archived/deleted).
//   - The hook is fail-open: a missing runs/ dir yields an empty manifest, no
//     error, no crash.
//
// These tests are currently RED: runGCHook does not exist, so package main
// fails to compile. They turn GREEN when Builder adds the hook.
```

### `go/cmd/evolve/cmd_loop_gc_test.go:138` — above `func TestGCOff(t *testing.T) {`

```text
// TestGCOff: an EXPLICIT off mode writes no manifest and touches nothing.
// (Cycle 1159 / workspace-hygiene S5: an ABSENT gc.mode no longer means off —
// it now resolves to shadow, pinned by TestRunGCHook_DefaultModeIsShadow. Only
// an operator's explicit "off" disables the hook, so this test sets it.)
```

### `go/cmd/evolve/cmd_loop_gc_worktree_test.go:3` — above `import (`

```text
// cmd_loop_gc_worktree_test.go — RED tests for workspace-hygiene S5
// (cycle 1159, task workspace-hygiene-s5-wire-gc-hook). S4 built
// gc.PlanWorktrees/gc.ApplyWorktrees fully tested but with ZERO non-test
// callers, and runGCHook still defaults an absent gc.mode to "off" — so the
// worktree+branch sweep can never fire in production. These tests call the
// unexported runGCHook against a REAL temp git repo and assert on observable
// side effects (manifest published / branch actually deleted / tree untouched).
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - runGCHook: an ABSENT gc.mode resolves to "shadow", not "off"
//     (docs/plans/workspace-hygiene-2026-07.md §S5). Explicit "off" still
//     returns immediately.
//   - In shadow and enforce, runGCHook additionally runs gc.PlanWorktrees over
//     cfg.ProjectRoot and publishes the manifest to
//     <workspace>/workspace-gc-manifest.json (tmp+rename, valid
//     gc.WorktreeManifest JSON).
//   - In enforce ONLY, it then calls gc.ApplyWorktrees — a merged, worktree-less
//     cycle-* branch is really deleted. Shadow must mutate nothing.
//   - Fail-open throughout: a non-git ProjectRoot warns and skips the worktree
//     sweep without breaking the run-dir GC that already works.
```

### `go/cmd/evolve/cmd_loop_goalstall.go:17` — above `const goalStallWeightFloor = 0.9`

```text
// cmd_loop_goalstall.go — goal-stall escalation for the sequential loop.
//
// When a goal keeps producing EMPTY (nothing shipped, no signal —
// CycleOutcomeSkippedUnknown) or BLOCKED (audit would-have-blocked —
// CycleOutcomeSkippedAuditAdvisory) cycles, the scheduler re-dispatched the
// identical goal forever: goal_hash 805f6ced burned 4 full pipelines in one
// batch (cycles 640/642/643/644) landing nothing, because nothing COUNTED the
// non-progress at the goal layer (the consecutive-FAIL breaker misses it — an
// empty/blocked cycle is not a FAIL). This adds that counter: after N
// consecutive non-shipping cycles on one goal, the loop self-files a weighted
// inbox todo naming the stalled goal + the block reasons and emits an
// abnormal-event, instead of blindly re-running it. The counter resets on any
// shipping cycle.
//
// TWO breakers share this machinery, differing only in the outcome class they
// count (see stallKind) and their policy threshold:
//
//   - goalStallKind — EMPTY/BLOCKED only, the consecutive-FAIL breaker's blind spot.
//   - nonprogressKind — the UNION: every cycle that landed nothing, FAIL included.
//     Added because the two class-keyed counters each RESET on the other's
//     outcome, so a goal alternating FAIL,EMPTY,FAIL,EMPTY crossed NEITHER
//     ceiling and burned pipelines forever while landing nothing (inbox
//     nonprogress-breaker-interleaved-fail-empty). The union escalates and
//     CONTINUES — the hard-halt decision for a pure-FAIL streak stays with
//     consecutiveFailBreaker.
//
// Held in-memory for the loop invocation, mirroring consecutiveFailBreaker
// (cmd_loop_control.go) and fleet.StarvationTracker — a stall burns WITHIN one
// batch, so per-invocation tracking catches the live case; cross-process
// durability would need a dossier schema migration (goal_hash + raw outcome
// preservation) for marginal benefit and is deliberately out of scope.
```

### `go/cmd/evolve/cmd_loop_iteration_state_coherence_test.go:3` — above `import (`

```text
// cmd_loop_iteration_state_coherence_test.go — frozen RED contract for the
// iteration-state-coherence-sentinel lane (cycle-1693). DO NOT MODIFY: the
// Builder makes these pass with production code in cmd_loop_window.go /
// cmd_loop_blockerbreaker.go / cmd_loop_control.go only.
//
// THE GAP. unfinishedCycle (cmd_loop_control.go) is the only guard that reads
// canonical cycle-state as possibly stale, and it runs once per batch in
// prepareFreshBatch. prepareIteration (cmd_loop_window.go) — the chokepoint
// cmd_loop_batch.go runs before EVERY dispatch, sequential or fleet — never
// reads it. A fleet lane SIGKILLed by cmd_fleet.go's WaitDelay escalation dies
// before its abnormalEpilogue defer (cyclerun_epilogue.go) can apply the state
// floor (Phase="aborted", ActiveAgent=""), so the canonical record keeps
// claiming a live phase for a dead cycle. Once the batch has moved past that
// cycle (CycleID <= lastCycleNumber) unfinishedCycle cannot see it at all.
//
// THE CONTRACT these tests pin (inbox acceptance, verbatim scope):
//
//	stale in-flight record at the iteration top → WARN + Phase reconciled to
//	"aborted"; a genuinely resumable unfinished cycle (unfinishedCycle shape)
//	is NOT touched; wired at loopBatchCoordinator.prepareIteration for every
//	iteration, fleet waves included; go test -race.
//
// "Stale in-flight" is read as the conjunction the repo already has words for:
//   - the recorded phase is live: not fresh (CycleID 0), not a terminal phase
//     ("aborted", "end"), and not a phase the cycle already completed and
//     closed out (a cleanly finished lane's record is history, not residue);
//   - the owner is dead: no run lease, or a lease whose owner fails
//     runlease.OwnerLive (the SIGKILL shape is a FRESH heartbeat with a DEAD
//     pid — the lane was killed seconds before this iteration top);
//   - unfinishedCycle(cs, last) is false (the boot guard owns that shape).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : StaleCanonicalState (3 shapes, incl. the "phase=retro
//     residue" the epilogue's own comment names) + idempotence.
//   - Negative : ResumableCycleUntouched, OwnInFlightCycleUntouched,
//     CompletedCycleRecordUntouched, TerminalOrFreshRecordUntouched — every one
//     diffs the persisted record and the write log, never just "no error".
//   - Edge/OOD : CoherenceReadErrorSurfacesWithoutWrite,
//     ReconcileWriteErrorSurfaces (fail loudly, never clobber what was unread).
//   - Wiring   : WiredBeforeEveryIteration drives the production entrypoint
//     runLoop (sequential) and prepareIteration under wave AND pool fleet
//     configs (cmd_loop_batch.go:137 is the one call site for all three).
```

### `go/cmd/evolve/cmd_loop_live_e2e_test.go:1` — above `package main`

```text
// Live LLM e2e for the v11.5.0 M4 pipeline.
//
// Gated on EVOLVE_E2E_LIVE_LLM=1 — without that env var, the test
// skips so unit-test runs stay fast and free. When enabled, the test
// spawns ONE claude-p subagent (Haiku, ~$0.05) via the production
// bridge + subagent runner, then asserts that the M4 verification
// pipeline (ledgerverify, cycleclassify, dispatchevents) reads the
// resulting on-disk state correctly.
//
// What this proves:
//
//  1. The bridge can launch a real subagent invocation in this
//     environment (probe of binaries, profile JSON, prompt, artifact).
//  2. The subagent runner appends a ledger entry with the
//     shape ledgerverify expects (kind=agent_subprocess, role=scout,
//     exit_code=0).
//  3. ledgerverify.VerifyCycle reads bridge-produced ledger entries
//     correctly (the integration shape between the writer and the
//     counter has no drift).
//  4. cycleclassify.Classify reads a realistic orchestrator-report.md
//     and returns the expected classification.
//  5. dispatchevents.Writer can append to abnormal-events.jsonl in
//     the cycle workspace without colliding with any other writes.
//
// Cost: one Haiku subagent (~$0.05).
```

### `go/cmd/evolve/cmd_loop_live_e2e_test.go:98` — above `br := bridge.New()`

```text
// Step 1: drive bridge.Launch directly with a clean prompt (no
// leading `--`). Avoids the known claude-CLI bug where a prompt
// value starting with `--` confuses the flag parser. The Go
// subagent.Runner currently composes prompts that start with
// "## INVOCATION CONTEXT ##" and v11.5.2+ uses that non-dash form so
// separately as a composePrompt fix (out of M4 scope).
```

### `go/cmd/evolve/cmd_loop_live_e2e_test.go:170` — above `vr, err := ledgerverify.VerifyCycle(`

```text
// Step 3: verify the ledger now contains a scout agent_subprocess
// entry with exit_code=0 for cycle 1.
```

### `go/cmd/evolve/cmd_loop_m4_test.go:22` — above `type fakeLedgerNoAppend struct {`

```text
// fakeLedgerNoAppend wraps fixtures.FakeLedger but makes Append a deliberate
// NO-OP, so verify reads ONLY the test-controlled Entries slice — NOT whatever
// the stub orchestrator incidentally writes during a no-op run.
//
// (cycle-137: an accumulating Append plus a verify that counts kind:"phase"
// — the vocabulary the Go orchestrator actually writes — would make every
// no-op run look like a complete cycle, silently defeating the failure-path
// tests. fixtures.FakeLedger.Append accumulates by design, so the cmd/evolve
// loop tests keep this thin no-op-Append wrapper rather than the canonical
// accumulating fake.)
```

### `go/cmd/evolve/cmd_loop_m4_test.go:115` — above `Signals:      newRootSignalCenter(projectRoot, evolveDir, console),`

```text
// ADR-0101 S4a: the stub root builds the production sink topology with
// the same constructor, so its rendered lines prove production's.
```

### `go/cmd/evolve/cmd_loop_m4_test.go:595` — above `if err := os.MkdirAll(cycleWorkspace(projectRoot, 1), 0o755); err != nil {`

```text
// Pre-create workspace cycle-1 so EmitCircuitBreakerTripped can
// write its abnormal event.
```

### `go/cmd/evolve/cmd_loop_m5_test.go:26` — above `func writeStdoutLog(t *testing.T, workspace, phase string, costUSD float64) {`

```text
// writeStdoutLog seeds a cycle's cost source: <phase>-events.ndjson with a
// kind==result envelope (what cyclecost now reads, ADR-0020), plus the legacy
// <phase>-stdout.log for any path that still inspects raw output.
```

### `go/cmd/evolve/cmd_loop_m5_test.go:184` — above `workspace := cycleWorkspace(projectRoot, 1)`

```text
// Pre-seed the cycle-1 workspace with a real-shaped stdout log so
// cyclecost finds cost data.
```

### `go/cmd/evolve/cmd_loop_nonprogress_test.go:74` — above `func TestNonprogressTracker_InterleavedFailEmptyEscalates(t *testing.T) {`

```text
// TestNonprogressTracker_InterleavedFailEmptyEscalates — the union counter must
// fire on the threshold-th consecutive NON-SHIPPING cycle regardless of class,
// so the interleaved stream that escapes both siblings escalates on cycle 5.
```

### `go/cmd/evolve/cmd_loop_outcome.go:53` — above `BoundaryRefresh *chainBoundaryRefreshLogEntry 'json:"boundary_refresh,omitempty"'`

```text
// BoundaryRefresh mirrors chainResult.BoundaryRefresh for the non-chain
// wave/fleet boundary path (auto-refresh-binary-at-boundary, cycle 1325's
// caller): nil-when-clean, populated with the last boundary-refresh-log.jsonl
// entry only when this batch's stop is "loop_boundary_refresh_reexec".
```

### `go/cmd/evolve/cmd_loop_outcome.go:327` — above `func sweepRulePromotions(stderr io.Writer, projectRoot string, cycles []core.CycleResult) {`

```text
// sweepRulePromotions is the I4 measured auto-enforce sweep (R8.2): scan the
// batch's interaction ledgers for shadow-rule would-fire evidence and flip
// rules that cleared the bar — ≥minShadowFires fires, ZERO non-would_fire
// outcomes for that rule (conservative: any anomaly disqualifies), and a
// fresh healthy-corpus re-validation inside the flip itself
// (bridge.EnforceMeasuredRule). Until this sweep existed, "measured
// auto-enforce never fires" was true by construction (ADR-0045 I4 record).
// Best-effort throughout: a missing dir/ledger is just absent evidence.
```

### `go/cmd/evolve/cmd_loop_outcome.go:386` — above `func fileUnexplainedOutcomeDefect(projectRoot string, cycle int, detail string) {`

```text
// fileUnexplainedOutcomeDefect self-files an inbox item for the alarm
// bucket (R6.3): FAILED_UNEXPLAINED means the "every terminal path records
// its outcome" invariant (ADR-0044 C1) has a hole — exactly what the inbox
// exists to capture. Idempotent per cycle (fixed filename); best-effort.
```

### `go/cmd/evolve/cmd_loop_outcome.go:466` — above `func batchEndGCCycle(lr loopResult, startNext int) int {`

```text
// batchEndGCCycle picks the cycle workspace the batch-end sweep publishes its
// manifests into: the batch's own last cycle. A batch that exited before any
// cycle ran has no run dir of its own, so it falls back to the same workspace
// the batch-start sweep used (startNext = lastCycleNumber+1) — never cycle-0,
// which is not a real run dir.
```

### `go/cmd/evolve/cmd_loop_pool.go:1` — above `package main`

```text
// cmd_loop_pool.go — FLEET-AS-POLICY rolling-pool dispatch (cycle-553
// supervisor-continuous-lane-keeping). The pool analogue of cmd_loop_wave.go's
// wave-barrier dispatch: where dispatchIteration launches a wave and blocks on
// EVERY sibling before re-planning, dispatchPoolIteration drives the backlog
// through fleet.RunPool, which BACKFILLS a replacement lane the instant any lane
// exits — removing the min-over-time width collapse the wave barrier suffers on
// skewed lane durations (batch bh1rt946t). fleet.RunPool shipped cycle 550 with
// its own exhaustive pool_test.go but had ZERO call sites outside its package,
// and policy.fleet.scheduling=="pool" parsed into a knob wired to nothing; this
// file is that wiring. Gated behind Scheduling=="pool" (shouldRunPool), mutually
// exclusive with the wave path.
```

### `go/cmd/evolve/cmd_loop_pool.go:87` — above `todos, _, err := fleet.TodosFromTriage(decisionJSON, cardPackages, consoleRoutedResolver(cfg.ProjectRoot, stderr))`

```text
// ADR-0074 plan-time gate, pool scheduler: same resolver, fresh per
// plan call; refusals WARN inside the resolver wrapper.
```

### `go/cmd/evolve/cmd_loop_pool_amplify_test.go:3` — above `import (`

```text
// cmd_loop_pool_amplify_test.go — test-amplification for cycle 553's
// supervisor-continuous-lane-keeping contract (test-report.md AC1-AC5 +
// build-report's "New Surface" signatures). Black-box against the spec only:
// written without reading cmd_loop_pool.go/cmd_loop.go/cmd_loop_wave.go's
// implementations, targeting gaps the TDD engineer's RED suite
// (cmd_loop_pool_test.go) left uncovered — degenerate/boundary counts, exact
// string-match requirements on Scheduling/PlanSource, the planFn-error path
// (mirroring the sibling dispatchIteration's documented wrapped-error
// contract per build-report S3), the Count/PlanSource axes of the gate
// crossed directly against dispatchPoolIteration (not just shouldRunPool),
// and large-scale/short-backlog limits on the rolling pool itself.
```

### `go/cmd/evolve/cmd_loop_pool_systemfailure_halt_test.go:3` — above `import (`

```text
// cmd_loop_pool_systemfailure_halt_test.go — RED contract for cycle-959's fleet
// lane task adr0072-fleet-pool-halt-unwired (scout-report.md sole selected task;
// inbox adr0072-fleet-halt-unwired, weight 0.97).
//
// PROBLEM (scout Key Findings, verified against the CURRENT tree): the WAVE
// dispatch branch of runLoop (cmd_loop.go:533) already calls
// anyLaneHaltedForSystemFailure(results) after dispatch and STOPS the batch on a
// forged verdict (ADR-0072). The POOL dispatch branch (cmd_loop.go:490-498, the
// opt-in policy.fleet.scheduling=="pool" path) gets back the IDENTICAL
// []fleet.Result shape (same ExitCode field, same systemFailureHaltExitCode
// contract) but NEVER consults it — it only logs the ok/failed lane count and
// `continue`s. A lane that forges a verdict under pool scheduling still files the
// escalation dossier + P0 (subprocess side), but the BATCH DOES NOT STOP — the
// exact churn-on-forged-verdict failure mode ADR-0072 exists to prevent, now
// confined to the pool code path.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to COMPILE today; that compile failure IS the
// RED evidence, mirroring the cycle-465/507/547/550/951 precedent):
//
//	// dispatchHaltDecision inspects a completed dispatch iteration's lane
//	// results and returns the ADR-0072 halt outcome BOTH the wave and pool
//	// branches apply, so the two structurally-similar branches cannot drift on
//	// the halt floor (per [[never_duplicate_centralize_via_design_patterns]]):
//	// when any lane exited with the system-failure halt code (a forged verdict)
//	// it returns halt=true with rc=systemFailureHaltExitCode and
//	// stopReason="system_failure_halt" so the caller STOPS the batch; otherwise
//	// halt=false (rc=0, stopReason="") so the caller keeps the never-stop retry
//	// semantics ordinary lane FAILs are entitled to. It MUST single-source the
//	// detection through the existing anyLaneHaltedForSystemFailure helper (AC4 —
//	// no branch re-implements the ExitCode==systemFailureHaltExitCode scan).
//	func dispatchHaltDecision(results []fleet.Result) (rc int, stopReason string, halt bool)
//
// The pool branch (cmd_loop.go `case ran:` at ~490) must then, after logging the
// lane count, apply this decision: `if rc, sr, halt := dispatchHaltDecision(results);
// halt { lr.StopReason = sr; lr.emitFatal(...); return rc }` BEFORE its `continue`
// — that inline branch glue is verified by the Auditor diff-scope checklist
// (test-report.md), exactly as the wave branch's own inline glue is; these unit
// tests pin the shared DECISION the branch consumes.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : TestDispatchHaltDecision_HaltsOnSystemFailureLane
//     (a lane with the halt code STOPS the batch — the core wiring proof).
//   - Negative  : TestDispatchHaltDecision_OrdinaryFailuresContinue
//     (strongest anti-no-op: ordinary FAIL/launch-error lanes must NOT halt —
//     a decision that halts on any non-zero exit fails here and would freeze the
//     never-stop retry loop ADR-0072 deliberately preserves).
//   - Edge/OOD  : TestDispatchHaltDecision_EmptyResultsContinue
//     (nil / empty results never halt).
```

### `go/cmd/evolve/cmd_loop_pool_systemfailure_halt_test.go:59` — above `func TestDispatchHaltDecision_HaltsOnSystemFailureLane(t *testing.T) {`

```text
// TestDispatchHaltDecision_HaltsOnSystemFailureLane (AC1, positive): a pool
// iteration whose results include ANY lane that exited with the ADR-0072
// system-failure halt code must resolve to halt=true, rc=systemFailureHaltExitCode
// and stopReason="system_failure_halt" — the batch stops instead of dispatching
// the next pool iteration.
```

### `go/cmd/evolve/cmd_loop_pool_systemfailure_halt_test.go:82` — above `func TestDispatchHaltDecision_OrdinaryFailuresContinue(t *testing.T) {`

```text
// TestDispatchHaltDecision_OrdinaryFailuresContinue (AC2, NEGATIVE / regression):
// a pool iteration with only ordinary lane failures (rc=2 FAIL, rc=1, and a
// launch error rc=-1) must resolve to halt=false, rc=0, stopReason="" — ordinary
// task-level failures keep the never-stop retry semantics ADR-0072 draws the line
// at. A decision that halts on any non-zero exit code fails here.
```

### `go/cmd/evolve/cmd_loop_pool_test.go:3` — above `import (`

```text
// cmd_loop_pool_test.go — RED contract for cycle-553's
// supervisor-continuous-lane-keeping task (the DISPATCHER-WIRING half of the
// L5 "ceiling-keeper"; the fleet-layer primitive fleet.RunPool already shipped
// cycle 550 with its own exhaustive pool_test.go).
//
// PROBLEM (triage-report.md sole `## top_n` item, inbox weight 0.95): cycle 550
// shipped fleet.RunPool (rolling lane pool that BACKFILLS a replacement lane the
// instant any lane exits) AND a policy knob that parses fleet.scheduling=="pool"
// into policy.FleetConfig.Scheduling (policy.go:1032) — but the loop DISPATCHER
// (cmd_loop.go's batch loop) still only ever consults shouldRunWave; it never
// reads fleetCfg.Scheduling, so a "pool"-scheduled operator STILL gets the wave
// barrier. RunPool has ZERO call sites outside its own package/test. The knob is
// wired to nothing.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS the
// RED evidence, mirroring the cycle-465/507/547/550 precedent):
//
//	// poolPlanFn produces the pool's backlog of file-disjoint todos to roll
//	// through (the pool analogue of wavePlanFn's decisionJSON+cardPackages).
//	type poolPlanFn func(ctx context.Context, waveIndex int) ([]fleet.Todo, error)
//
//	// shouldRunPool gates the rolling-pool dispatch path. It requires the SAME
//	// fleet preconditions as shouldRunWave (Count>1 && PlanSource=="triage")
//	// PLUS the resolved Scheduling strategy being "pool". Mutually exclusive
//	// with shouldRunWave: a default/"wave" fleet never enters the pool, and a
//	// "pool" fleet never enters the wave barrier (no double-dispatch).
//	func shouldRunPool(fc policy.FleetConfig) bool
//
//	// dispatchPoolIteration runs one iteration's pool path when shouldRunPool
//	// gates it on, and reports ran=false (no side effects) otherwise so the
//	// caller falls through unchanged. On the pool path, in order: runs preflight
//	// (the SAME S3 dirty-control-plane guard dispatchIteration uses — a refusal
//	// surfaces wrapped/errors.Is-matchable with ran=false and NEITHER planFn NOR
//	// launch invoked); obtains the backlog via planFn (ctx threaded); and drives
//	// it through fleet.RunPool with the injected launch (the SAME isolated launch
//	// seam the wave path's Supervisor uses — per L4 no dispatch ever takes the
//	// unisolated in-process sequential path), sizing PoolConfig{Target:fc.Count,
//	// Concurrency:fc.Concurrency}. An empty backlog reports ran=false, err=nil so
//	// the caller falls back instead of consuming a --max-cycles iteration doing
//	// no work (the pool analogue of dispatchIteration's D1 empty-plan guard).
//	// Returns the backlog and the per-lane RunPool results.
//	func dispatchPoolIteration(ctx context.Context, fc policy.FleetConfig,
//		preflight func() error, planFn poolPlanFn, launch fleet.LaunchFn,
//		waveIndex int) (ran bool, backlog []fleet.Todo, results []fleet.Result, err error)
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning
//     (the core wiring proof — a dispatcher that wired the WAVE BARRIER instead
//     of RunPool, or that no-ops, fails this)
//   - Negative  : TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch
//     (strongest anti-no-op: a naive "always ran=true" dispatcher fails here)
//   - Safety    : TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches
//   - Regression: TestDispatchPoolIteration_WaveConfigInertNoLaunch pins that the
//     new seam is INERT for a default/"wave" fleet — no regression to the wave
//     path; TestShouldRunWaveAndPool_MutuallyExclusive pins the two gates never
//     both fire (no double dispatch).
```

### `go/cmd/evolve/cmd_loop_projroot_test.go:9` — above `func TestParseLoopArgs_ProjectRootResolvedAbsolute(t *testing.T) {`

```text
// TestParseLoopArgs_ProjectRootResolvedAbsolute encodes the cycle-119
// ExitArtifactTimeout (exit 81) root cause.
//
// --project-root defaults to "." and was never absolutized, so WorkspacePath
// (= <root>/.evolve/runs/cycle-N) and the artifact path derived from it were
// RELATIVE. Worktree phases (tdd/build) run the agent with cwd=worktree, so the
// agent resolved a relative artifact path INTO the worktree subtree while the
// in-process bridge polled the same relative path against the main-repo cwd.
// The two cwds diverged, the artifact "never appeared" where the bridge looked,
// and the driver returned ExitArtifactTimeout — aborting the cycle at tdd.
//
// The flag's own help text promises "absolute path to project root"; this test
// asserts that documented invariant is actually enforced for every input shape,
// so the workspace/artifact path is cwd-independent for worktree-phase agents.
```

### `go/cmd/evolve/cmd_loop_reset_guard_test.go:98` — above `func TestUnfinishedCycleGuard_DeadOwnerFreshLease_NotReportedAsLive(t *testing.T) {`

```text
// TestUnfinishedCycleGuard_DeadOwnerFreshLease_NotReportedAsLive — cycle-554
// workspace-hygiene-s1 sibling: the loop's F1-sibling guard (cmd_loop.go:317)
// reads a lease as "owned by a LIVE run" using freshness alone. A crashed
// owner (dead pid) with a still-fresh heartbeat must fall through to the
// normal unfinished_cycle (resume|reset) guidance instead — steering an
// operator at `owned_by_live_run` never to reset would wedge them forever
// against a run that will never come back. PID 999999 is a real, guaranteed-
// dead pid (same convention as TestDefaultBootRecovery_AutosealsDeadOwnerMarker)
// so the production pidAlive probe drives the decision, no injection needed.
// bootRecoverFn is stubbed to a no-op (the established spy-seam idiom, see
// TestRunLoop_InvokesBootRecoveryBeforeGate) so this test isolates the guard's
// OWN liveness check as defense-in-depth, independent of whether boot-time
// AutosealStaleMarker also would have healed the same marker first.
```

### `go/cmd/evolve/cmd_loop_resume_quota_test.go:59` — above `ledger := newFakeLedger()`

```text
// satisfies rootLedger (ADR-0101 S4a)
```

### `go/cmd/evolve/cmd_loop_sequential_observe.go:30` — above `fmt.Fprint(b.stderr, formatSignalReport(cycle.cycle, b.orch.SignalSummary()))`

```text
// ADR-0101 S4a: the batch report shows the driven runner's per-cycle view
// of the Signal Center — read through the loop's own orchestrator seam, so
// a scripted runner is reported exactly like the real one. It reports,
// never gates.
```

### `go/cmd/evolve/cmd_loop_sequential_outcome.go:58` — above `func (b *loopBatchCoordinator) applyCycleFailureOutcome(cycle int) {`

```text
// applyCycleFailureOutcome is the loop's voice for the shared failed-cycle
// inbox walk (applyCycleFailureOutcome in cmd_cycle.go — the ONE call every
// root makes): best-effort, a lifecycle hiccup WARNs but never changes the
// batch's flow. The walk appends its lifecycle lines through the root's
// ledger (deps.Ledger) so the Signal Center observes them like every other
// entry (ADR-0101 S4a), and reports its own faults through the root's Center
// (deps.Signals; ADR-0103 unit 06).
```

### `go/cmd/evolve/cmd_loop_signal_test.go:15` — above `type cancelOnRunOrch struct {`

```text
// cmd_loop_signal_test.go — F4: a SIGINT/SIGTERM must stop the loop GRACEFULLY
// (checkpoint + resumable, rc=130), not on the OS default disposition (the
// silent kill that lost cycles 394/395). The loopSignalContext seam lets the
// test cancel the loop's context exactly as a signal would, without delivering
// a real process signal to the test runner.
```

### `go/cmd/evolve/cmd_loop_socket_test.go:11` — above `func TestRunSocketTeardown(t *testing.T) {`

```text
// TestRunSocketTeardown — 2026-09-09 token-waste root cause #1 (second half):
// after the loop parent and its cycle children exited, their per-run tmux
// server still held provider sessions; the operator had to kill it by hand.
// Batch termination reaps exactly the socket this PROCESS derives for itself
// (every session on it is this run's by construction). Ownership is the
// socket's identity, not env presence — a chain re-entering the batch in the
// same process still owns its socket, while an operator override or an
// enclosing run's socket (another pid) is never killed.
```

### `go/cmd/evolve/cmd_loop_spine_failopen_test.go:3` — above `import (`

```text
// cmd_loop_spine_failopen_test.go — RED contract for the BATCH half of
// spine-failopen-telemetry (inbox weight 0.85).
//
// Cycle-1166 landed the per-cycle half: the spine gate records every fail-open
// into CycleResult.SpineFailOpens and finalizeCycle projects it into the
// committed dossier. It also landed dossier.RollupSpineFailOpens — but NOTHING
// in production called it, so the batch-level number the item actually asks for
// ("76 occurrences in one width-3 batch") was still nobody's output. A rollup no
// caller invokes is indistinguishable from the counter that never existed.
//
// The item names this test verbatim: TestLoopSummary_RollsUpSpineFailOpensPerBatch.
// Here it is asserted where the loop summary is actually produced — loopResult.emit,
// the single output chokepoint every exit path funnels through.
//
// RED today: loopResult has no SpineFailOpens field and spineFailOpenRollup does
// not exist — this file does not compile.
```

### `go/cmd/evolve/cmd_loop_systemfailure_halt.go:12` — above `const systemFailureHaltExitCode = 4`

```text
// systemFailureHaltExitCode is the process exit code a cycle run returns when an
// ADR-0072 SYSTEM-level failure (a forged/incoherent verdict, not a task-code
// failure) mandates a loop halt. It is distinct from the ordinary FAIL mapping
// (rc=2) and the soft batch-complete rc=3, so the parent wave/fleet loop can
// tell a forged-verdict halt apart from an ordinary task-level lane failure and
// stop the batch instead of continuing to the next wave.
```

### `go/cmd/evolve/cmd_loop_systemfailure_halt.go:38` — above `func haltOnSystemFailure(evolveDir, projectRoot string, cycle int, workspace string, sf *cyclestate.SystemFailureSignal,…`

```text
// haltOnSystemFailure is the ONE shared halt+escalate action (ADR-0072 AC2)
// invoked by the sequential single-cycle path (cmd_loop.go), each fleet lane
// subprocess (runCycleRun) and the pipeline-blocker breaker — so the
// escalation dossier, the P0 inbox item, the halt exit code and the ONE
// loop.halt INCIDENT are produced identically on every code path instead of
// the logic being duplicated inline per call site. The caller's rule names
// the INCIDENT's code and adds the rule's own fields; the chokepoint adds the
// floor (category, level) and what it wrote — fields.next is the dossier's
// next_action (its one home), fields.escalation and fields.inbox_item the
// paths. Returns systemFailureHaltExitCode so the caller propagates the halt
// via its exit code.
```

### `go/cmd/evolve/cmd_loop_systemfailure_halt.go:59` — above `func anyLaneHaltedForSystemFailure(results []fleet.Result) bool {`

```text
// anyLaneHaltedForSystemFailure reports whether any fleet lane result exited
// with the ADR-0072 system-failure halt code. The wave/fleet dispatch loop
// consults it to stop dispatching further waves: a forged verdict makes the
// pipeline untrustworthy fleet-wide, so one halting lane stops the whole batch.
// An ordinary lane FAIL (rc=2) or launch error (rc=-1/1) is deliberately NOT
// conflated with a halt — those keep the never-stop retry semantics ADR-0072
// draws the line at.
```

### `go/cmd/evolve/cmd_loop_systemfailure_halt.go:75` — above `func dispatchHaltDecision(results []fleet.Result) (rc int, stopReason string, halt bool) {`

```text
// dispatchHaltDecision is the ONE ADR-0072 halt outcome BOTH the wave and pool
// dispatch branches apply after an iteration completes, so the two
// structurally-similar branches cannot drift on the halt floor (per
// [[never_duplicate_centralize_via_design_patterns]]): the pool branch shipped
// without any halt check while the wave branch had one, exactly the drift this
// single-sources away. Detection is delegated to anyLaneHaltedForSystemFailure
// (no branch re-implements the ExitCode==systemFailureHaltExitCode scan): when
// any lane forged a verdict it returns halt=true with rc=systemFailureHaltExitCode
// and stopReason="system_failure_halt" so the caller STOPS the batch; otherwise
// halt=false (rc=0, stopReason="") so the caller keeps the never-stop retry
// semantics an ordinary lane FAIL is entitled to.
```

### `go/cmd/evolve/cmd_loop_systemfailure_halt_test.go:13` — above `func TestCycleRunExitCode_HaltsOnSystemFailureRegardlessOfVerdict(t *testing.T) {`

```text
// ADR-0072 fleet-halt-unwired (inbox adr0072-fleet-halt-unwired, cycle 956).
//
// Today `result.SystemFailure` is read ONLY by the sequential single-cycle
// path in cmd_loop.go (line ~650). `runCycleRun` (cmd_cycle.go, the exact
// entrypoint every fleet lane subprocess runs) maps FinalVerdict to an exit
// code and never looks at SystemFailure, so a system-failure halt inside a
// fleet lane is indistinguishable from an ordinary FAIL (rc=2) to the parent
// wave loop — which in turn (cmd_loop_wave.go dispatchIteration / the
// cmd_loop.go wave/pool branches) only counts `ExitCode != 0` as "failed lane"
// and always continues to the next wave.
//
// These tests encode the exit-code contract as pure, unit-testable functions
// so both the single-cycle path and the fleet subprocess boundary can share
// ONE halt decision (AC2: no duplicated logic) instead of the sequential path
// re-implementing the check inline.
```

### `go/cmd/evolve/cmd_loop_systemfailure_halt_test.go:130` — above `results := []fleet.Result{`

```text
// Negative case: an ordinary FAIL (rc=2) or process error (rc=-1/1) must
// NOT be conflated with a system-failure halt — only the batch loop
// should stop; ordinary task-level failures keep the never-stop retry
// semantics (ADR-0072 draws this line deliberately).
```

### `go/cmd/evolve/cmd_loop_triage_cardfiles_test.go:3` — above `import (`

```text
// cmd_loop_triage_cardfiles_test.go — the WRITER→CONSUMER proof for
// triage-cards-carry-files. The two halves are individually tested
// (internal/triagecap/cardfiles_test.go for the declaration, PR #366's menu tests
// for the partitioning); what failed live on batch-14 was the JOIN: the
// orchestrator's projection dropped the footprint, so by the time the next wave
// planned, the card's file knowledge existed only in prose and the planner saw an
// id island.
//
// This drives the REAL production chain end to end:
//
//	triage-report.md  →  triagecap.ProjectDecisionJSON   (ship/postship writes it)
//	                  →  widenNarrowDecision             (next wave's primary path)
//	                  →  fleet.PlanFromTriage            (lane partitioning)
//
// RED before the projection carried files=: "mate" cannot join the committed
// card's lane, because nothing downstream knows they touch the same file.
```

### `go/cmd/evolve/cmd_loop_unfinished_test.go:31` — above `livePID := os.Getpid()`

```text
// Fresh lease AND a genuinely-alive owner pid ⇒ a live loop owns cycle 395.
// Use this test process's own pid so the production pid-aware liveness fence
// (runlease.OwnerLive, cycle-554) sees a real live owner — a fresh heartbeat
// alone is no longer sufficient (a crashed owner's stale-but-fresh lease now
// correctly falls through to the resume|reset guidance instead).
```

### `go/cmd/evolve/cmd_loop_unit13_golden_test.go:3` — above `import (`

```text
// cmd_loop_unit13_golden_test.go — ADR-0103 unit 13 fold 0: the characterization
// goldens captured on 8e8f080f BEFORE the wave engine (cmd_loop_wave.go) and
// the chain engine (cmd_loop_chain.go) moved into internal/loopwave and
// internal/loopchain. Every test here drives the package-main spellings the
// unit keeps as facades, so the same fixtures replay through the leaves. The
// goldens live beside the leaves (internal/<leaf>/testdata) so the leaf tests
// read the identical bytes; temp paths are templated {ROOT} / {EVOLVE_DIR},
// RFC3339 stamps {TS}.
```

### `go/cmd/evolve/cmd_loop_unit13_golden_test.go:142` — above `func u13PruneFixture(t *testing.T, decision string) (plan func(stderr io.Writer) []byte, root string) {`

```text
// u13PruneFixture seeds the prior cycle's decision (cycle 7) with one pending
// and one consumed id and drives the production plan source: the prune runs
// before the widen, and with alpha the only pending item the widen has
// nothing to add, so the plan's bytes ARE the prune's.
```

### `go/cmd/evolve/cmd_loop_unit13_golden_test.go:326` — above `func TestWaveStderr_EveryLineIsByteIdentical(t *testing.T) {`

```text
// The `.golden.txt` files are the base capture (8e8f080f) — the leaf tests
// derive every replaced line's reason from them. The `.rendered.golden.txt`
// files are the fold-4 console: byte-identical for every KEPT line, and the
// declared replacements (D-1 the three min-width lines, D-2 the dispatch
// failure, D-4 the all-stale gate) rendered by the root sink as
// `[loop] loop.wave WARN <CODE> … — <the old sentence> k=v`. The all-stale
// row disappears from the test-only launcher facade's console (a Null
// Center); the leaf asserts the signal.
```

### `go/cmd/evolve/cmd_loop_unit13_golden_test.go:595` — above `if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err == nil && strings.TrimSpace(string(data)) !…`

```text
// The chain's Signal Center records nothing on a clean run: the durable
// batch-level stream is absent (8e8f080f: no chain Center exists) or empty.
```

### `go/cmd/evolve/cmd_loop_unit13_seam_test.go:3` — above `import (`

```text
// cmd_loop_unit13_seam_test.go — ADR-0103 unit 13: the host-side pins. The
// origin guard (fold 0, red on 8e8f080f: "runWaveIteration" names no
// function) and the seam tests of fold 4.
```

### `go/cmd/evolve/cmd_loop_wave.go:1` — above `package main`

```text
// cmd_loop_wave.go — the unit-13 wave seam (ADR-0103). The wave engine —
// the sequential-vs-wave gate, the ONE dispatch body the fan-out and the
// min-width repair share, the fleet-config loaders, the freshness-gated
// launcher, the quota/budget sizing and the plan source — lives in
// internal/loopwave. This file keeps the engine's ONE wired construction
// (newWaveEngine), the coordinator's accessor pair (wave / wiredWave) and the
// Strangler Fig facades the pool scheduler, the budget wrapper and the
// by-name tests keep, so no production call site learned the unit exists.
```

### `go/cmd/evolve/cmd_loop_wave.go:41` — above `Protected: guards.IsProtectedScope,`

```text
// routing roots judge declared surfaces in SCOPE (F29)
```

### `go/cmd/evolve/cmd_loop_wave_amplify_test.go:3` — above `import (`

```text
// cmd_loop_wave_amplify_test.go — test-amplification (salvaged from cycle
// 465) for the loop-wave-dispatch seam. Black-box against the spec only:
// shouldRunWave is the pure Count>1 && PlanSource=="triage" gate (closed
// vocab, no normalization at this layer), and dispatchIteration is the
// single per-iteration wave-or-sequential decision point whose injected
// planFn / launcher seams these tests drive with hostile inputs. Reuses the
// scaffold's fakeWaveLauncher / waveScopeIDs helpers (same package).
//
// TestDispatchIteration_EmptyPlanNeverClaimsAWave is the cycle-466 D1
// regression: cycle 465's audit (audit-report.md, confidence 0.95)
// independently reproduced this exact defect via its predecessor
// TestLoopWave_EmptyTriagePlanNeverClaimsAWave — dispatchIteration
// (cmd_loop_wave.go:56-70 in the 465 worktree) did not guard
// len(specs)==0, so an empty adapted plan invoked launcher.Run with a
// zero-lane spec list and returned ran=true, silently consuming a
// --max-cycles iteration doing zero work (the livelock class named in the
// cycle-466 goal). Renamed to the TestDispatchIteration_ prefix so this
// cycle's eval AC1 grading command (`go test -run 'TestDispatchIteration'`)
// exercises it directly; assertions preserved verbatim from the proven
// cycle-465 audit evidence, plus an explicit launcher.calls-count check.
```

### `go/cmd/evolve/cmd_loop_wave_boundaryrefresh_wiring_test.go:3` — above `import (`

```text
// cmd_loop_wave_boundaryrefresh_wiring_test.go — cycle 1325, task
// auto-refresh-binary-at-boundary (fleet_scope for this lane).
//
// PRIOR STATE (verified live in this worktree, not assumed from the inbox
// item): cycle-1314 built the full boundary binary-refresh mechanism —
// maybeRefreshChainBoundary (cmd_loop_chain.go) — with the ahead-check,
// rebuild, provenance-gated re-pin, re-exec, and the re-exec loop breaker,
// all independently unit-tested and hardened across cycle-1320
// (cmd_loop_chain_boundaryrefresh_test.go,
// cmd_loop_chain_boundaryrefresh_hardening_test.go — both GREEN in this
// worktree: TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch
// and TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch PASS).
// That mechanism is wired into ONE of the two batch loops: runLoopChain (the
// `evolve loop --chain` path, cmd_loop_chain.go:538).
//
// THE GAP THIS CYCLE CLOSES: the inbox item's own incident (cycles 1302-1309
// running a stale binary) motivated cycle-1314's fix, but runLoopBatch's OWN
// per-wave/fleet batch loop (cmd_loop_batch.go, the `for i := 0; i < effectiveMax;
// i++` loop used by plain `evolve loop --max-cycles N` / fleet mode WITHOUT
// --chain — runLoop itself is a thin dispatcher that hands off to either
// runLoopChain or runLoopBatch) never calls maybeRefreshChainBoundary at all
// — confirmed by acsassert.CountInGoFunc(cmd_loop_window.go, "prepareIteration",
// "maybeRefreshChainBoundary") == 0 in this worktree today. A chained loop
// self-heals at every boundary; a non-chained multi-wave/fleet loop does
// not — the exact class of bug cycle-1314 fixed for one caller and left
// open for the other.
//
// FIX CONTRACT (undefined until the Builder adds it — this file's caller-
// proof predicate fails to find the call today, which IS the RED evidence;
// no new logic is invented, maybeRefreshChainBoundary is REUSED verbatim per
// never_duplicate_centralize_via_design_patterns):
//
//	runLoop's wave/fleet batch loop must call
//	  maybeRefreshChainBoundary(cfg, i+1, stderr)
//	at the same boundary point reloadFleetConfigAtWaveBoundary already
//	occupies (before that iteration's wave/lane dispatch — i.e. never
//	mid-lane, structurally guaranteed by the loop's own sequential shape:
//	dispatch only ever starts AFTER this check returns). A true result means
//	a re-exec is imminent/terminal (mirrors runLoopChain's own handling) —
//	the iteration must stop cleanly without starting that wave's dispatch.
//
// Why a structural (AST) caller-proof here, not a full runLoop() behavioral
// drive: maybeRefreshChainBoundary's OWN behavior (fire path, refuse-while-
// already-attempted path, every failure-mode fallback) is ALREADY proven
// GREEN by cycle-1314/1320's tests above — re-driving that behavior through
// a second, much heavier harness (a real runLoop() invocation needs a live
// git repo, storage, launcher, and CLI-health fakes) would duplicate
// coverage, not add it. What is genuinely unverified is only the ONE-LINE
// wiring gap, which this codebase's own precedent (cycle-968
// TestClassifyFleetRebaseCandidate_WiredIntoRecoverFromShipError,
// cmd_loop_wave_minwidth_wiring_test.go) tests exactly this way: an AST
// caller-proof over the real production function's source, waived per
// acsassert's config-check convention because the *behavior* it gates is
// pinned elsewhere.
//
// acs-predicate: config-check — caller-existence is an inherent source-
// structure check; maybeRefreshChainBoundary's behavior is already pinned by
// cmd_loop_chain_boundaryrefresh_test.go / _hardening_test.go (both GREEN).
```

### `go/cmd/evolve/cmd_loop_wave_boundaryrefresh_wiring_test.go:67` — above `func TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary(t *testing.T) {`

```text
// TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary is the wiring
// proof: runLoop's own per-wave/fleet batch loop must call
// maybeRefreshChainBoundary, not just runLoopChain's. A regression here
// (the call site never added, or later deleted in an unrelated refactor)
// silently reopens the cycles-1302-1309 stale-binary class for every
// non-chained multi-wave/fleet run.
```

### `go/cmd/evolve/cmd_loop_wave_menu_test.go:1` — above `package main`

```text
// cmd_loop_wave_menu_test.go — the seed and widen paths hand PlanFromTriage a
// MENU pool, not pre-flattened single reps (fleet-lane-batch-menu). Two
// defects pinned here, both proven live on batch-14 wave-1:
//
//  1. seedWavePlanFromInbox dropped candidate FILES from the synthesized
//     top_n cards, so fleet.Partition saw every card as an independent island
//     — it could never cluster same-file items into one lane, and (worse) it
//     could SPREAD two same-file items across two concurrent lanes.
//  2. Both paths carried exactly one id per lane, so a lane could never
//     amortize its worktree/build/audit across the cluster the batching layer
//     deliberately groups.
```

### `go/cmd/evolve/cmd_loop_wave_minwidth_test.go:3` — above `import (`

```text
// cmd_loop_wave_minwidth_test.go — RED contract for cycle-547's
// fleet-min-width-lane-fallback task.
//
// PROBLEM (scout Key Finding 2): cmd_loop.go's batch loop dispatches a wave
// via dispatchIteration; when that reports ran=false with a nil error (0
// lanes planned — either an empty triage plan, D1, or the wave's Count was
// quota/budget-shrunk to <=1 so shouldRunWave's Count>1 gate rejects it
// before planning), the loop unconditionally WARNs and falls through to the
// legacy sequential orch.RunCycle path — cmd_loop_wave.go's own doc calls
// this "the ONLY path that can leak into the main tree" (unisolated, runs in
// the process cwd instead of a dedicated worktree). A fleet.count=2 operator
// whose wave shrank to 1 lane via a quota bench gets width ZERO (sequential),
// not width 1 — defeating fleet.count in the worst way.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence, mirroring the cycle-465/507 precedent):
//
//	forceOneLaneDispatch(ctx, preflight, planFn, launcher, waveIndex) —
//	drives up to ONE disjoint candidate through the SAME isolated-worktree
//	path dispatchIteration uses (preflight -> planFn -> fleet.PlanFromTriage
//	capped at count=1 -> launcher.Run), WITHOUT the shouldRunWave(Count>1)
//	gate (the caller already knows the original fleet config wanted >1 lanes
//	and only reached here because the wave-sized Count shrank to <=1 — this
//	is the shrink-repair path, not the general multi-lane entry point).
//	Mirrors dispatchIteration's other safety contracts exactly: a preflight
//	refusal surfaces an error with planFn/launcher never invoked, and a
//	genuinely empty candidate backlog (PlanFromTriage adapts to zero specs)
//	reports ran=false, err=nil so the caller correctly falls back to
//	sequential — true sequential fallback stays reserved for that case.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestForceOneLaneDispatch_DispatchesIsolatedWaveWhenCandidateExists
//   - Negative : TestForceOneLaneDispatch_EmptyBacklogStaysFalseNoLauncherInvoked
//     (the strongest anti-no-op: a naive "always dispatch" impl fails here)
//   - Safety   : TestForceOneLaneDispatch_PreflightRefusalNeverPlansNorLaunches
//     (the S3 dirty-control-plane guard must still gate the repair path)
//   - Regression (guards against the WRONG fix): TestShouldRunWave_CountOneOrZeroStillFalse
//     pins that shouldRunWave itself is NOT loosened to Count>=1 — that would
//     also route an operator's genuinely-static fleet.count=1 config through
//     the wave path, violating "fleet.count=1 legacy path untouched".
```

### `go/cmd/evolve/cmd_loop_wave_minwidth_wiring_test.go:3` — above `import (`

```text
// cmd_loop_wave_minwidth_wiring_test.go — cycle 552, task
// eliminate-sequential-fallback-min-width-lane (triage-report.md top_n, the
// single fleet-assigned id for this lane).
//
// GAP (triage-report.md Rationale): dispatchIteration and
// forceOneLaneDispatch (cmd_loop_wave.go, cycle-547) are both independently
// unit-tested as pure functions (cmd_loop_wave_test.go,
// cmd_loop_wave_minwidth_test.go), but nothing exercises the RunLoop
// call-site (cmd_loop.go's batch for-loop, ~lines 486-514) that wires them
// together: the `fleetCfg.Count > 1 && waveCfg.Count <= 1` guard that
// decides whether the min-width repair even applies, the one-lane launcher
// construction, and the WARN-vs-dispatch stderr branching that decides
// whether the batch iteration `continue`s (repaired) or falls through to
// the legacy sequential path. That wiring could silently regress — an
// inverted guard condition, or the whole call site deleted during an
// unrelated refactor — without any existing test catching it, because the
// call site itself was never extracted into a testable unit.
//
// FIX CONTRACT (this cycle's new surface — undefined until the Builder adds
// it, so this package's test build fails to compile today; that compile
// failure IS the RED evidence, mirroring the cycle-465/507/547 precedent):
//
//	minWidthRepair(ctx, fleetCfg, waveCfg, preflight, planFn, launcher,
//	waveIndex, stderr) (handled bool) — extracted from RunLoop's inline
//	switch (byte-identical stderr messages + control flow) so the guard
//	condition and WARN-vs-dispatch branching are independently testable
//	without a real fleet-lane subprocess:
//	  - guard not met (fleetCfg.Count<=1 — the operator wanted one lane): WARNs
//	    "planned zero lanes (empty triage plan)", returns handled=false, NEVER
//	    calls preflight/planFn/launcher. (cycle-557: the empty-plan-at-full-
//	    capacity shape, waveCfg.Count>1 with zero planned lanes, is now IN-guard.)
//	  - guard met, forceOneLaneDispatch dispatches a candidate: WARNs/logs
//	    "min-width repair dispatched N/M isolated lane (fleet.count=X shrank
//	    to Y)", returns handled=true (caller must `continue`).
//	  - guard met, forceOneLaneDispatch reports a genuinely empty backlog
//	    (ran=false, err=nil): WARNs "planned zero lanes (empty backlog)",
//	    returns handled=false (true sequential fallback — the case it stays
//	    reserved for).
//	  - guard met, forceOneLaneDispatch errors (preflight refusal or plan
//	    adapt failure): WARNs "min-width repair failed: <err>", returns
//	    handled=false — the error is surfaced, never silently swallowed.
//	RunLoop's call site becomes: on dispatchIteration's default (ran=false,
//	err=nil) case, construct the one-lane launcher and call minWidthRepair;
//	`continue` the batch iteration when handled is true.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Negative (the critical anti-gaming case): TestMinWidthRepair_GuardNotMetNeverInvokesLauncher
//     — a fleetCfg.Count<=1 config must leave the launcher UNTOUCHED; an
//     inverted/loosened guard is the exact wiring regression this task exists
//     to catch. (cycle-557 widened eligibility: waveCfg.Count>1 no longer
//     excludes the repair — see cmd_loop_wave_starvation_test.go.)
//   - Positive: TestMinWidthRepair_GuardMetDispatchesOneIsolatedLaneAndSignalsContinue
//   - Edge (empty backlog): TestMinWidthRepair_EligibleButEmptyBacklogFallsBackToSequential
//   - Edge (error surfaced, never swallowed): TestMinWidthRepair_ForceDispatchErrorSurfacesAndFallsBack
```

### `go/cmd/evolve/cmd_loop_wave_prune_test.go:1` — above `package main`

```text
// cmd_loop_wave_prune_test.go — cycle-1182 RED contract for
// wave-planner-pass-scope-prune.
//
// The defect: widenNarrowDecision is the PRIMARY per-wave planning path (it runs
// whenever a prior cycle's triage-decision.json exists — the common case, not
// just the first wave). It builds `committed` straight from decision.top_n and
// then either short-circuits (`len(committed) >= count`) or hands the list to
// WidenTopNToFleetWidth, which copies the committed prefix through VERBATIM.
// Neither branch consults the inbox lifecycle, so an id a previous cycle already
// CONSUMED survives in the prior decision file and gets re-pinned into the next
// wave's plan / lane-scope.json (cycle-1116 re-pinned tdd-topn-binding-gate after
// cycle-1113 consumed it). The sibling fresh-seed path
// (triagecap.SelectWaveSeedMenus) already prunes; only this seam does not.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - Call the exported triagecap.PruneConsumed on `committed` immediately after
//     it is built from decision.TopN and BEFORE the `len(committed) >= count`
//     early-return, so a fleet-width-but-stale list is still cleaned and then
//     re-widened from the backlog.
//   - A prune that drops ids must never fall back to returning the ORIGINAL
//     bytes: the stale id would ride through the `len(topN) <= len(committed)`
//     guard untouched.
//   - The committed_floors byte-identical passthrough is unchanged (pinned by
//     the existing TestWidenNarrowDecision_CommittedFloorsShortCircuit).
```

### `go/cmd/evolve/cmd_loop_wave_reload_test.go:3` — above `import (`

```text
// cmd_loop_wave_reload_test.go — TDD contract for cycle 739's sole committed
// top_n task fleet-config-hot-reload-wave-boundary (inbox item
// 2026-07-12T12-25-00Z, weight 0.92).
//
// Live incident 2026-07-12 (~20:17): the operator committed fleet.min_lanes
// 3->10 mid-batch (sanctioned ship a33ffd6a), but wave dispatch still shrank
// "wave count 10 -> 9 (min 3)" — RunLoop resolves the fleet policy block ONCE
// per batch (cmd_loop.go: "resolved once per batch") and every subsequent wave
// consumes the stale snapshot. The control-plane preflight already re-validates
// policy.json CLEANLINESS per wave; these tests pin the missing half: dispatch
// must consume the committed VALUES per wave too.
//
// Contract under test (the seam the batch loop must call at every wave
// boundary, before the quota/budget sizing):
//
//	reloadFleetConfigAtWaveBoundary(evolveDir string, prev policy.FleetConfig, warn io.Writer) policy.FleetConfig
//
//   - re-resolves the fleet block from the committed .evolve/policy.json via
//     the SAME loader semantics as batch start (loadFleetConfig);
//   - logs "[loop] fleet config reloaded: count=N min_lanes=M" to warn ONLY
//     when the resolved count/min_lanes changed vs prev;
//   - on an unreadable/malformed policy.json it HOLDS prev (the operator's
//     width commitment) and WARNs, instead of silently collapsing to the
//     Count=1 defaults the batch-start loader degrades to.
//
// DO NOT MODIFY THESE TESTS (builder contract) — implement the seam and its
// batch-loop wiring to make them pass.
```

### `go/cmd/evolve/cmd_loop_wave_reload_test.go:54` — above `func TestFleetDispatch_ReloadsMinLanesAtWaveBoundary(t *testing.T) {`

```text
// TestFleetDispatch_ReloadsMinLanesAtWaveBoundary is the incident twin: the
// operator commits min_lanes 3->10 between waves; the next wave boundary must
// resolve the NEW floor, and that floor must actually hold width under a
// quota bench (the exact "wave count 10 -> 9 (min 3)" shape that motivated
// the fix — with min_lanes=10 the bench is absorbed and width holds at 10).
```

### `go/cmd/evolve/cmd_loop_wave_reload_test.go:80` — above `if eff := fleet.QuotaAwareCount(got.Count, map[string]string{"codex": "rate_limit"}, got.MinLanes, io.Discard); eff != 1…`

```text
// Behavioral closure of the incident: under one benched family the NEW
// floor absorbs the shrink — width holds at 10 instead of dropping to 9.
```

### `go/cmd/evolve/cmd_loop_wave_s3_test.go:3` — above `import (`

```text
// cmd_loop_wave_s3_test.go — fleet-s3-guards AC1/AC2/AC5 (cycle 467):
// RED-first contract for the S3 wave guards at the dispatch seam. This file
// pins the POST-S3 signatures, so the cmd/evolve package fails to COMPILE
// until Builder lands them — that compile failure IS the RED evidence:
//
//	type wavePlanFn func(ctx context.Context, waveIndex int) ([]byte, []string, error)
//	dispatchIteration(ctx, fc, preflight func() error, planFn, launcher, nil, waveIndex)
//
// AC5 (reviewer note on PR #298): the loop's cancellable ctx must thread
// through the PLAN path too — productionWavePlanFn currently minted
// context.Background() at cmd_loop_wave.go:116, so cancellation never
// reached readLastCycleNumber. wavePlanFn gains the ctx parameter and the
// context.Background() mint dies (the paired ACS predicate asserts its
// absence from cmd_loop_wave.go).
//
// AC1/AC2 (wiring): the dirty-control-plane preflight gates the WAVE PATH
// ONLY, through an injected func() error — same injected-fn seam style as
// wavePlanFn/waveLauncher (production wiring: a closure over
// fleet.PreflightControlPlane(cfg.ProjectRoot)). A preflight refusal is
// surfaced as a wrapped error with ran=false so cmd_loop.go's existing WARN
// branch falls back to sequential; the launcher AND planFn are never invoked.
//
// Builder note: the pre-existing tests in cmd_loop_wave_test.go /
// cmd_loop_wave_amplify_test.go use the OLD signatures — update their call
// sites MECHANICALLY (add the ctx param to planFn literals, pass a nil-error
// preflight), preserving their names and assertions. Do NOT weaken or rename
// any test in THIS file.
```

### `go/cmd/evolve/cmd_loop_wave_scopeprune_test.go:3` — above `import (`

```text
// cmd_loop_wave_scopeprune_test.go — RED tests for cycle-1172, inbox item
// `wave-planner-pass-scope-prune` (scout task wave-planner-plan-time-scope-prune).
//
// THE GAP: productionWavePlanFn's PRIMARY path reads the prior cycle's
// triage-decision.json and widens it (widenNarrowDecision). Nothing on that
// path re-resolves each top_n id against the inbox lifecycle, so an id that was
// consumed — moved to inbox/processed|rejected|retry/ — during an earlier wave
// is still planned into the NEXT wave's lane-scope.json (one confirmed
// instance: cycle-1116). The dispatch-time freshness gate
// (productionFreshnessProbe / freshnessGatedLauncher) then skips it, so the
// lane does not actually re-execute the dead work — but the PLAN is still
// wrong: lane-scope.json advertises work that no longer exists, which is what
// every downstream reader (operator, dossier, retro) sees.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//
//  1. Before the plan is returned, productionWavePlanFn drops every top_n id
//     whose inboxmover.ResolveDispatchState is a CONSUMED state (processed,
//     rejected, retry). Reuse ResolveDispatchState — the same resolver the
//     dispatch-time probe already uses. No second bookkeeping file (the inbox
//     item's own fix note).
//  2. Prune BEFORE widening, so the freed lane slots are refilled from the live
//     backlog instead of being lost.
//  3. FAIL OPEN: `pending` ids and ids with NO lifecycle evidence (`unknown` —
//     not every planned id is inbox-backed) are retained untouched. Over-
//     pruning would starve the wave, which is strictly worse than the stale
//     entry this fixes.
//  4. The dispatch-time freshness gate STAYS as defense in depth — this is a
//     plan-hygiene addition, not a replacement.
```

### `go/cmd/evolve/cmd_loop_wave_scopeprune_test.go:125` — above `func TestProductionWavePlanFn_PrunesConsumedScopeFromPriorDecision(t *testing.T) {`

```text
// TestProductionWavePlanFn_PrunesConsumedScopeFromPriorDecision is the crux
// (the cycle-1116 shape): `gamma` was consumed during the prior wave and now
// lives in inbox/processed/, yet the prior decision still lists it. The next
// wave's plan must not re-pick it. RED today: the primary path never consults
// the lifecycle, so gamma survives into lane-scope.json.
```

### `go/cmd/evolve/cmd_loop_wave_starvation_test.go:3` — above `import (`

```text
// cmd_loop_wave_starvation_test.go — cycle-557, task
// fix-wave-plan-source-starvation (scout-report.md ## Selected Tasks, Task 1).
//
// Two composed regressions this cycle removes, both of which force the wave
// planner off its isolated-lane path and onto the leak-prone in-supervisor
// sequential fallback (the standing rule `fleet_width_always_respected` calls
// this "the leak path"):
//
//  1. widenNarrowDecision returned a present-but-EMPTY prior triage decision
//     unchanged (the observed cycle-554 shape: top_n:[]). An empty top_n must
//     instead widen fully from the inbox backlog, exactly as an absent decision
//     would — otherwise the wave plans zero lanes from a non-empty backlog.
//  2. minWidthRepair's guard excluded the empty-plan-at-full-capacity shape
//     (fleetCfg.Count>1, waveCfg.Count>1, zero planned lanes) so it fell
//     through to sequential instead of repairing to one isolated lane.
```

### `go/cmd/evolve/cmd_loop_wave_test.go:3` — above `import (`

```text
// cmd_loop_wave_test.go — RED-first contract for the loop-wave-dispatch seam
// (FLEET-AS-POLICY S2, salvaged from cycle 465's preserved worktree per
// cycle-466's operator T1: fix D1 empty-plan livelock). Pins the
// per-iteration seam cmd_loop.go's batch for-loop must call, factored into
// cmd_loop_wave.go: shouldRunWave (the Count>1 && PlanSource=="triage" gate,
// mirroring the consecutiveFailBreaker pure-decision-function precedent in
// cmd_loop_failbreaker_test.go) and dispatchIteration (obtains one wave's
// triage plan via the injected wavePlanFn, adapts it through
// fleet.PlanFromTriage, and launches through the injected waveLauncher —
// production wiring is *fleet.Supervisor + execCycleLaunch). None of these
// symbols exist yet; every test below fails to COMPILE until Builder adds
// cmd_loop_wave.go — that compile failure IS the RED evidence (mirrors
// cycle-465's precedent). Functions directly exercising dispatchIteration
// are named TestDispatchIteration_* (renamed from cycle 465's TestLoopWave_*
// prefix) so `go test -run 'TestDispatchIteration'` — the eval's AC1 grading
// command — exercises the full contract, including the D1 empty-plan guard
// added in cmd_loop_wave_amplify_test.go. See
// .evolve/evals/s2-wave-salvage-fix-d1.md for the acceptance criteria.
```

### `go/cmd/evolve/cmd_loop_wave_test.go:85` — above `func TestDispatchIteration_TwoWavesDisjointLaneScopes(t *testing.T) {`

```text
// TestDispatchIteration_TwoWavesDisjointLaneScopes (AC1, positive): fleet{count:2,
// plan_source:triage} driven for 2 iterations (the --max-cycles 2 contract:
// each iteration IS a wave) must run the wave path both times, launch through
// the injected launcher exactly twice, and never repeat a scoped todo id
// across any lane in either wave. Gaming fake it kills: a wave loop that
// launches Count unscoped identical cycles.
```

### `go/cmd/evolve/cmd_loop_wavesync.go:3` — above `import (`

```text
// cmd_loop_wavesync.go — ADR-0080 S3: refresh the runtime plane from origin
// at the wave boundary, fast-forward ONLY. Post-cutover, origin/main is the
// single integration channel (console PRs merge there; lane ships push
// there), so a wave that plans against a stale local main bases its lanes on
// work origin has already superseded. FF-only covers HISTORY safety; the one
// review-HIGH hazard it does not cover is handled explicitly below: a merge
// refusal distinguishes "local tracked changes block FF" (expected: binary
// rebuild churn) from real history divergence — the wrong diagnosis
// prescribed the stowaway-adopting remedy.
//
// Self-SHA note (review round 2, investigated to ground truth): an FF CANNOT
// drift the ship self-SHA pin — verifySelfSHA hashes os.Executable()
// (gitignored bin/evolve, an untracked build output no git operation
// rewrites). The hazard is confined to operator rebuilds, already covered by
// the boot and post-build re-pins. An earlier draft shipped a repin seam
// here; it was provably inert and was DELETED rather than left as a false
// protection claim (ADR-0080 implementation notes).
```

### `go/cmd/evolve/cmd_loop_wavesync.go:84` — above `func syncMainFromOriginAtWaveBoundary(ctx context.Context, projectRoot string, warn io.Writer) (synced bool, halt error)…`

```text
// syncMainFromOriginAtWaveBoundary fetches origin and fast-forwards a
// checked-out `main` onto origin/main. synced is true only when the tree
// actually moved. Every skip path is deliberate: not-on-main (sequential /
// console launches), no origin remote (offline dev), fetch failure
// (transient network — WARN), already current (quiet), local main AHEAD
// (WARN — the local integration HEAD is the lane base; nothing moved),
// blocked by local tracked changes (WARN, names the cause). DIVERGED history
// returns halt: the ONE main-relation resolver (gitexec.RelationToRemote)
// that laneStartRef also reads would refuse every lane, so the batch stops
// here, before any lane spends a phase, with the same sentence — the two
// consumers can no longer disagree (2026-09-09 token-waste root cause #3).
```

### `go/cmd/evolve/cmd_loop_wavesync_test.go:3` — above `import (`

```text
// cmd_loop_wavesync_test.go — ADR-0080 S3: the runtime plane refreshes `main`
// from origin at wave boundaries, fast-forward ONLY. Real git fixtures: a
// bare origin plus two clones (the runtime and a "console" that lands work
// via origin), because the failure modes under test are git's own (non-FF
// divergence, missing remote, detached branch).
```

### `go/cmd/evolve/cmd_loop_wavesync_test.go:248` — above `func TestSyncMainAtWaveBoundary_LocalAheadOnlyIsNotReportedAsFastForward(t *testing.T) {`

```text
// TestSyncMainAtWaveBoundary_LocalAheadOnlyIsNotReportedAsFastForward pins the
// 2026-09-09 token-waste root cause #3: `git merge --ff-only origin/main`
// SUCCEEDS without moving HEAD when the local main is strictly AHEAD (unpushed
// dossier closeouts), and the boundary reported "fast-forwarded main" while
// the fresh lanes were about to base on a different commit than the landing
// branch. The honest report names the ahead state; nothing "fast-forwarded".
```

### `go/cmd/evolve/cmd_loop_window.go:50` — above `if err := runPreWaveProbes(b.ctx, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.cycleEnv, b.stderr); err != nil {`

```text
// An interrupt that landed during the probes must not dispatch a wave
// that is cancelled at spawn (2026-09-15: "wave 2: 0/2 lanes ok" printed
// after the boundary SIGINT).
```

### `go/cmd/evolve/cmd_loop_window_test.go:3` — above `import (`

```text
// cmd_loop_window_test.go — RED contract for Defect B of the cycle-1335
// incident (fault-localization-report.md, suspects 1/2/6; premise-challenge
// Attack 1 corrected the framing).
//
// The defect, verified on live state: aborted cycles exit through
// abnormalEpilogue (cyclerun_epilogue.go:41-107), which writes the failure
// digest but NEVER advances state.LastCycleNumber — every loopAbort path
// returns from RunCycle (orchestrator.go:865-916) before the finalizeCycle
// call at orchestrator.go:969. Meanwhile the breaker's batch window is
// derived from that same COMPLETION counter (cmd_loop.go:522-525), so the
// digests of cycles 1326/1328/1329 (all phase:"aborted", all carrying
// fingerprint ship|unknown|76d0f4fca190) stayed inside `> batchStartCycle`
// on every relaunch and Rule B tripped at i==0, before any cycle ran. Three
// re-halts; the operator's repair was a manual lastCycleNumber advance.
//
// The fix anchors the window on the monotone ALLOCATION lease instead:
// state.LastAllocatedCycleNumber advances at MINT time (alloc.go:14-17 —
// "a crashed run BURNS its number"), so it tracks cycles DISPATCHED while
// LastCycleNumber tracks cycles COMPLETED. A time boundary belongs on the
// dispatch counter. Live state carries exactly this asymmetry:
// lastCycleNumber=1334 alongside lastAllocatedCycleNumber=1335.
//
// NOT reset.go: nothing in the tree rewinds either counter — reset.go:322
// advances under the comment "number never reused" (fault-localization V11).
```

### `go/cmd/evolve/cmd_loop_window_test.go:44` — above `const haltMarker = "LOOP_PIPELINE_BLOCKER_HALT"`

```text
// the code the rendered loop.halt line carries (ADR-0101 S4a)
```

### `go/cmd/evolve/cmd_loop_window_test.go:46` — above `func TestReadBatchWindowFloor_PrefersAllocationLease(t *testing.T) {`

```text
// TestReadBatchWindowFloor_PrefersAllocationLease is the unit-level core of
// the fix: the breaker window floor must be the MAX of the completion and
// allocation counters, so digests written by cycles that were minted but
// never completed fall OUTSIDE a fresh batch's window.
//
// Shape taken from live state at the time of the incident: three aborted
// cycles at 1326/1328/1329 left lastCycleNumber pinned at 1325 while the
// allocation lease had advanced to 1330.
```

### `go/cmd/evolve/cmd_loop_window_test.go:106` — above `func TestRunLoop_AbortedCycleDigestsFallOutsideBatchWindow(t *testing.T) {`

```text
// TestRunLoop_AbortedCycleDigestsFallOutsideBatchWindow is the caller proof:
// the incident replayed through the REAL production entrypoint (runLoop),
// not through the derivation helper in isolation. A predicate that only
// calls readBatchWindowFloor proves nothing about cmd_loop.go:525 actually
// consuming it.
```

### `go/cmd/evolve/cmd_models_live.go:275` — above `func salvageProbeDiagnostics(scratch, evolveDir, tag string, now func() time.Time, log io.Writer) {`

```text
// salvageProbeDiagnostics copies the diagnostic side-effects the bridge wrote
// under the scratch probe workspace into evolveDir/models-probe before the
// scratch dir is deleted (adversarial-review HIGH: a quota wall mid-probe
// writes escalation-report.json — pane tail + repair instructions — and the
// teardown used to delete the only copy while the refresh reported success):
//
//   - escalation-report.json → escalation-report-<UTC stamp>.json (each event
//     kept, never clobbered) + a WARN naming the salvaged path
//   - *-launch-error.txt → copied verbatim + WARN
//   - llm-calls.ndjson → APPENDED to the durable ledger, so the
//     token-telemetry trail keeps every classifier call the probe made
//
// Deliberately allowlist-shaped: artifacts/prompts/pane logs are probe
// plumbing and stay disposable. Best-effort throughout — salvage must never
// fail the refresh — and fully quiet when a clean probe left nothing behind.
// tag disambiguates destinations when multiple probes salvage CONCURRENTLY
// into the shared durable home (`evolve setup latest` runs one salvage per
// family in parallel): the second-granularity stamp alone collided in exactly
// this shape before (tmux resolveSession, ADR-0049 N15), and the un-stamped
// launch-error name collides outright since every family's capturer is
// Agent "models". Empty tag keeps the historical names (single-probe callers).
```

### `go/cmd/evolve/cmd_models_salvage_test.go:108` — above `func TestSalvageProbeDiagnostics_TagsKeepConcurrentFamiliesDistinct(t *testing.T) {`

```text
// The tag path IS the collision fix (parallel per-family salvage into the
// shared durable home): two same-second probes with different tags must land
// as two distinct durable files for BOTH the stamped escalation report and
// the un-stamped launch-error name. An implementation that ignores the tag
// passes every other test while last-write-wins destroys a sibling family's
// diagnostics — the exact class ADR-0049 N15 closed for tmux session names.
```

### `go/cmd/evolve/cmd_release_verify_binaries.go:1` — above `package main`

```text
// cmd_release_verify_binaries.go implements `evolve release-verify-binaries
// <tag>`: the release-flow gate that proves every prebuilt binary the release is
// supposed to publish is actually present as an asset on the GitHub release.
//
// Why this exists: `evolve release` proves only LOCAL binary consistency, and
// "all binaries published" lived only as prose in skills/publish/SKILL.md, which
// is non-deterministic — v21.1.0 reported success yet published zero assets. This
// command makes that closing check deterministic Go.
//
// Single source of truth: the expected asset set is DERIVED from .goreleaser.yml
// (the only place the build matrix must exist inline). Add a target there and
// this gate automatically requires its archive published — no second list.
//
// Determinism: no live LLM. The one effect — listing a release's assets — is
// injected via releaseAssetLister so the orchestration (coverage, no early
// return) is unit-testable with pure stubs; defaultReleaseAssetLister wires the
// real GitHub REST call used by the release workflow.
```

### `go/cmd/evolve/cmd_release_verify_clis.go:1` — above `package main`

```text
// cmd_release_verify_clis.go implements `evolve release-verify-clis`: the
// release-flow gate that proves the release artifact can be INSTALLED for every
// supported LLM CLI and that the release BINARY answers the core subcommands the
// installed skills shell out to.
//
// Why this exists: `evolve release` (releasepipeline) proves only LOCAL binary
// consistency — disk == committed blob == expected_ship_sha. "Every CLI installs
// and performs" lived only as prose in skills/publish/SKILL.md, which is
// non-deterministic (v21.1.0 shipped with the prose green yet published zero
// assets). This command makes that closing check deterministic Go.
//
// Determinism: no live LLM, no network. Each CLI is verified by exercising the
// real install/projection path into an isolated location, and the binary is
// smoke-checked with a side-effect-free `<bin> <sub> --help`. The effects are
// injected via matrixDeps so the orchestration (coverage, isolation, no early
// return) is unit-testable with pure stubs; defaultMatrixDeps() wires the real
// implementations used by the release flow.
```

### `go/cmd/evolve/cmd_resetsha.go:17` — above `func runResetSHA(args []string, _ io.Reader, stdout, stderr io.Writer) int {`

```text
// runResetSHA implements `evolve reset-sha` — the sanctioned successor to
// hand-editing state.json:expected_ship_sha (ADR-0065). It re-pins the ship
// gate's binary anti-tamper SHA to the RUNNING evolve binary, so a legitimate
// rebuild (e.g. after pulling a fix) can ship/resume without a false
// SELF_SHA_TAMPERED. It is provenance-gated: the re-pin is granted only when the
// running binary's embedded build-commit is an ancestor of HEAD, UNLESS
// --operator explicitly authorizes an unverifiable binary.
//
// Exit codes: 0 re-pinned; 1 refused/error (pin unchanged on refusal).
```

### `go/cmd/evolve/cmd_router_dispatch_test.go:10` — above `func TestAdvisorDispatch_FallsBackWhenFamilyBenched(t *testing.T) {`

```text
// TestAdvisorDispatch_FallsBackWhenFamilyBenched pins WS6-S2 (ADR-0052): when the
// router's resolved CLI family is benched (the cli-health circuit breaker), the
// dispatch falls back to the universal claude family — keeping the advisor alive
// on a healthy footing rather than failing outright.
```

### `go/cmd/evolve/cmd_router_dispatch_test.go:32` — above `func TestResolveRouterDispatch_PerDecisionType(t *testing.T) {`

```text
// TestResolveRouterDispatch_PerDecisionType pins WS6-S1 (ADR-0052, optional
// multi-model): resolveRouterDispatchFor returns the BASE dispatch for every
// decision type by default (strictly no-op, single-model), and applies per-type
// model overrides from RouterPolicy to deep and fast decisions without crosstalk.
```

### `go/cmd/evolve/cmd_routing.go:3` — above `import (`

```text
// cmd_routing.go — ADR-0052 WS3-S4: `evolve routing explain --cycle N` renders
// a recorded routing decision (the clamped plan, the integrity-floor clamps,
// and the OTel decision span) for debugging WHY a cycle ran the phases it did.
// WS3-S5 adds the `replay` subcommand. Pure reader: no state/ledger/registry
// mutation — safe to run mid-batch.
```

### `go/cmd/evolve/cmd_routing_test.go:14` — above `func writeJSON(t *testing.T, path string, v any) {`

```text
// WS3-S4 (ADR-0052): `evolve routing explain --cycle N` is a READ-ONLY render
// of a recorded routing decision — the clamped plan (run/skip + justification),
// the integrity-floor clamps that fired, and the OTel decision span — so an
// operator can debug WHY a cycle ran the phases it did. Missing artifacts are a
// clean message, not an error (a partially-recorded cycle still explains).
```

### `go/cmd/evolve/cmd_salvage.go:3` — above `import (`

```text
// cmd_salvage.go — `evolve salvage report`: the operator-facing surface for the
// recoverable-malformed `bad_verdict` rate.
//
// The salvage layer's extraction/coercion stage is gated on that rate
// (docs/research/deliverable-alignment-2026-08/README.md §7). Instrumentation
// has been appending .evolve/bad-verdict-baseline.jsonl since cycle-1389 with
// no reader, so the number could only be produced by hand-reading JSONL. This
// command is the reader.
//
// Pure reader — opens the sidecar, folds it, prints. No state, ledger, or
// sidecar mutation, so it is safe to run mid-batch.
```

### `go/cmd/evolve/cmd_salvage_test.go:3` — above `import (`

```text
// cmd_salvage_test.go — cycle-1442 audit H2: `evolve salvage report` shipped as
// a new operator-facing surface at 0.0% coverage (`runSalvage 0.0%`,
// `runSalvageReport 0.0%`, no test in this package referencing either symbol).
// The number it prints is the one an operator reads as "the gate coerced N
// verdicts", so an untested renderer is an untested claim about the gate.
//
// Driven through the real entry points with a real on-disk sidecar pair — no
// seams stubbed — because the defect class here is exactly a renderer that
// diverges from what the gate writes.
```

### `go/cmd/evolve/cmd_selfcheck.go:3` — above `import (`

```text
// cmd_selfcheck.go — ADR-0076 slice B (green-before-handoff): `evolve
// selfcheck build` runs the build handoff floor's EXACT deterministic checks
// (productionBuildFloorChecks: the protected-surface floor, then
// core.DefaultBuildFloorChecks) as an in-session pre-flight, so the builder
// fixes findings inside its own loop and budget instead of post-hoc
// correction windows. The floor itself is unchanged — it remains the
// trust-boundary backstop; this is the same check moved to where fixing is
// cheap. Exit codes: 0 green, 1 findings, 2 usage. This mirrors the CODE floor
// only; a document cycle's deliverable self-checks with `evolve solution check`
// (ADR-0099 slice 2), whose engine the cycle composition root chains after
// productionBuildFloorChecks. It diffs against HEAD (no --base yet), so a
// builder that already committed sees less than the floor (follow-up F38).
```

### `go/cmd/evolve/cmd_selfcheck_protected_integration_test.go:5` — above `import (`

```text
// cmd_selfcheck_protected_integration_test.go — F37 wiring proof: the ONE
// production floor composition both roots run refuses the cycle-1689 diff
// through the REAL control-plane manifest (guards.IsProtectedSurface), not a
// stub — go/internal/core/cyclerun.go is a manifest member.
```

### `go/cmd/evolve/cmd_selfcheck_test.go:3` — above `import (`

```text
// cmd_selfcheck_test.go — RED contract for ADR-0076 slice B: `evolve selfcheck
// build` is the builder's in-session pre-flight running the EXACT build-floor
// checks, so fixing happens inside the builder's loop and budget instead of
// post-hoc correction windows. DI seam (buildFloorChecksFn) so tests inject a
// stub; a wiring pin holds the seam to productionBuildFloorChecks (the ONE
// composition: the engine plus the protected-surface floor, F37).
```

### `go/cmd/evolve/cmd_selfcheck_test.go:68` — above `func TestBuildFloorRoots_ComposeOnlyThroughTheProductionFloor(t *testing.T) {`

```text
// TestBuildFloorRoots_ComposeOnlyThroughTheProductionFloor (F37): the only
// non-test reference to core.DefaultBuildFloorChecks in this root is the ONE
// production composition, so no root can wire the engine without the
// protected-surface floor beside it.
```

### `go/cmd/evolve/cmd_setup.go:64` — above `fmt.Fprintf(stderr, "[setup] WARN: could not determine cwd (%v); marker/state paths may be relative\n", err)`

```text
// os.Getwd failed (cwd deleted/unmounted) — surface it rather than
// fall through to a silent relative ".evolve" (the cycle-119 class).
```

### `go/cmd/evolve/cmd_setup.go:69` — above `project = paths.AbsoluteRoot("--project-root", project, func(m string) {`

```text
// Absolutize so `setup complete`'s marker lands in the SAME .evolve the loop
// nudge reads, regardless of a relative flag/env root (cycle-119 class).
```

### `go/cmd/evolve/cmd_ship.go:80` — above `projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {`

```text
// A relative --project-root or $EVOLVE_PROJECT_ROOT would make audit-binding
// and commit-gate paths diverge from the worktree-relative ones (cycle-119).
```

### `go/cmd/evolve/cmd_ship.go:88` — above `signals := shipRootSignals(projectRoot, stderr)`

```text
// ADR-0103 unit 07: the manual/release root wires the landing's warnings
// like every other root — the tracked-binary reset WARN that used to be a
// raw stderr line stays visible on the operator's console here.
```

### `go/cmd/evolve/cmd_ship_signal_center_test.go:1` — above `package main`

```text
// cmd_ship_signal_center_test.go — ADR-0103 unit 07 wiring proofs: the ship
// phase built by the orchestrator root carries the root's Signal Center, the
// landing's WARN renders at the --simulate root, and the standalone
// `evolve ship` root builds the one sink topology for its own Center.
```

### `go/cmd/evolve/cmd_signals.go:1` — above `package main`

```text
// cmd_signals.go — `evolve signals codes generate|check`: projects the Signal
// Center code registry (every module's RegisterCode, all linked into this
// binary) into the GENERATED region of docs/architecture/signal-codes.md,
// exactly like `evolve flags generate|check` projects the flag registry.
// `check` exits 2 on drift; a cmd/evolve test runs it, so CI gates an
// undocumented or re-documented code (ADR-0101 S2, design §5.4).
```

### `go/cmd/evolve/cmd_signals_test.go:3` — above `import (`

```text
// cmd_signals_test.go — `evolve signals codes generate|check` (ADR-0101 S2):
// the checked-in docs/architecture/signal-codes.md is a projection of the code
// registry every module links into this binary. The repo-doc test is the CI
// gate: a code registered without regenerating the doc (or documented
// differently from its registration) fails here.
```

### `go/cmd/evolve/cmd_skills.go:1` — above `package main`

```text
// cmd_skills.go implements `evolve skills` — the CLI front for the ADR-0040
// projection (generate|check|publish). The projection + drift logic lives in
// internal/skillcheck so the autonomous cycle's audit phase can run the SAME
// drift check in-process without importing package main; this file only routes
// the subcommands and keeps the thin skillsRun seam the producer-side drift
// tests (cmd_skills_drift_test.go) call.
```

### `go/cmd/evolve/cmd_skills.go:23` — above `return skillsRun(sourceRoot(), true, stdout, stderr)`

```text
// SKILL.md is a generated SOURCE doc (ADR-0040 projection), part of a
// cycle's committed deliverable — resolve it from the worktree under the
// ACS suite, not main's stale copy (cycle-355 fix; see sourceRoot).
```

### `go/cmd/evolve/cmd_skills_drift_test.go:1` — above `package main`

```text
// cmd_skills_drift_test.go is the producer-side alarm for ADR-0040's skill
// projection: it runs `evolve skills check` in-process against the live repo,
// so a hand edit inside a GENERATED:phase-facts region — or an SSOT change
// without a regenerate — fails CI instead of silently shipping drifted docs.
// Same pattern as phasecontract/contract_test.go (runtime.Caller locates the
// repo; the live tree is the fixture).
```

### `go/cmd/evolve/cmd_skills_publish.go:1` — above `package main`

```text
// cmd_skills_publish.go implements `evolve skills publish` — the cross-CLI
// half of the skill projection story (ADR-0041, extends ADR-0040). Canonical
// skills (skills/<name>/SKILL.md, enumerated from .claude-plugin/plugin.json)
// are projected into the surfaces of three foreign LLM CLIs:
//
//	codex   $CODEX_HOME/skills/evolve-<name>/SKILL.md — flat namespace, so the
//	        frontmatter name is rewritten with an evolve- prefix
//	agy     a native plugin staging dir (plugin.json + skills/<name>/) that
//	        `agy plugin validate|install` consumes; the plugin name supplies
//	        the namespace, so skill names stay unprefixed
//	ollama  Modelfiles embedding the skill body as a SYSTEM prompt (ollama has
//	        no skill system); read-only-tier subset only, mirroring
//	        driver_ollamatmux.go's write-phase rejection
//
// Safety invariant: bare `publish` is stage-only — it writes gitignored
// mirrors under .evolve/publish/<target>/ and runs the read-only
// `agy plugin validate`; it never mutates the user environment. `--install`
// performs the mutating steps. Every projected artifact carries the
// EVOLVE-PUBLISH:projection provenance marker; prune deletes only
// evolve-*-prefixed artifacts carrying that marker, never user-authored files.
```

### `go/cmd/evolve/cmd_skills_publish.go:101` — above `Name        string`

```text
// dir name == frontmatter name (ADR-0040 rule 3)
```

### `go/cmd/evolve/cmd_skills_publish.go:168` — above `cfg.OllamaBase = "llama3.1:8b"`

```text
// Default mirrors driver_ollamatmux.go's model default; no ollama
// profile exists in .evolve/profiles/ to read it from (ADR-0041).
```

### `go/cmd/evolve/cmd_skills_publish.go:324` — above `const agyStalePluginName = "evolve-loop"`

```text
// agyStalePluginName is the pre-rename (v21.0.0) plugin name installAgy prunes
// so an upgrading agy user does not keep both evo and evolve-loop installed with
// colliding skills (mirrors installOllama's stale-model prune).
```

### `go/cmd/evolve/cmd_skills_publish_test.go:1` — above `package main`

```text
// cmd_skills_publish_test.go covers `evolve skills publish` (ADR-0041): the
// cross-CLI projection of canonical skills/ into Codex, agy, and Ollama
// surfaces. All tests are hermetic — temp project trees, a temp CODEX_HOME,
// and seam-recorded exec calls (no real agy/ollama runs).
```

### `go/cmd/evolve/cmd_skills_publish_test.go:233` — above `for rel, content := range first {`

```text
// ADR-0041 analogue of the ADR-0040 invariant: frontmatter name == dir name.
```

### `go/cmd/evolve/cmd_solution.go:14` — above `func runSolution(args []string, _ io.Reader, stdout, stderr io.Writer) int {`

```text
// runSolution — `evolve solution check <solutions/slug> [--project-root DIR]
// [--registry PATH]` (ADR-0099 slice 2): the document deliverable's self-check
// and eval [code] grader. It runs the SAME engine as the build handoff floor
// and the audit gate (internal/solutioncheck) over the contract the registry
// declares, so the three surfaces can never disagree. Exit 0 = well-formed,
// 1 = violations (one per line), 2 = usage or contract not declared.
```

### `go/cmd/evolve/cmd_solution_test.go:11` — above `func TestSolutionCheck(t *testing.T) {`

```text
// TestSolutionCheck — ADR-0099 slice 2: `evolve solution check <dir>` is the
// eval [code] grader and the agent's self-check for a document deliverable;
// it runs the same engine as the build floor and the audit gate. Exit 0 =
// well-formed, 1 = violations (printed one per line), 2 = usage.
```

### `go/cmd/evolve/cmd_subagent.go:90` — above `func resolveWorkspaceArg(workspace, projectRoot string) (string, error) {`

```text
// resolveWorkspaceArg absolutizes a relative <workspace_path> CLI argument
// against projectRoot (EVOLVE_PROJECT_ROOT / layout.ProjectRoot), NOT the
// ambient process cwd. The workspace value is LLM-typed at the bridge boundary;
// a raw relative value would otherwise resolve against whatever cwd the process
// happens to have, scattering run artifacts (.bridge-inbox/, session records,
// prompt/stdout/stderr logs) into that cwd — the untracked-tree-drift the
// tree-diff cycle-killer reacts to (cycle-616 subagent-workspace-absolutize
// fix). An already-absolute value passes through unchanged; an empty value or
// empty projectRoot is left untouched (no base to anchor against). This is the
// single ingestion point every subagent subcommand routes its workspace arg
// through.
//
// Containment (cycle-619): a relative arg whose cleaned join escapes the
// project root (via ".." traversal) is REJECTED with an error rather than
// silently resolving into an arbitrary sibling tree — otherwise the same
// artifact-scatter the absolutization prevents for cwd just relocates outside
// the root. Absolute args keep their passthrough contract: the real loop hands
// absolute worktree/runs paths that legitimately live outside project root.
```

### `go/cmd/evolve/cmd_subagent.go:388` — above `signals := newRootSignalCenter(layout.ProjectRoot, layout.EvolveDir, stderr)`

```text
// The second Signal Center root (ADR-0103 unit 16): the dispatcher's
// BRIDGE_SUBAGENT_* warnings and the bridge engine's own producers land in
// <runs/cycle-N>/signals.ndjson beside the orchestrator's (the cycle-less
// file at cycle 0) and on stderr at WARN — the same topology cmd_cycle builds.
```

### `go/cmd/evolve/cmd_subagent.go:520` — above `func sourceRoot() string {`

```text
// sourceRoot resolves the root for reading SOURCE/DOC artifacts that are part
// of a cycle's committed deliverable — generated-from-source docs such as
// docs/architecture/control-flags.md (from the flagregistry) or skills/*/SKILL.md
// (from phase facts). These live in the WORKTREE the cycle commits to, not the
// main checkout.
//
// This is the source half of the dual-root pattern. EVOLVE_PROJECT_ROOT is the
// STATE root: the ACS suite pins it to MAIN so predicates resolve `.evolve/`
// runtime data there (issue #12). But a generated SOURCE doc must be validated
// against the worktree, so acssuite also exports EVOLVE_WORKTREE_ROOT=<worktree>.
// Precedence: EVOLVE_WORKTREE_ROOT (the active worktree, exported by the ACS
// suite) > EVOLVE_PROJECT_ROOT (explicit root in CI/dev) > cwd. Outside the
// suite EVOLVE_WORKTREE_ROOT is conventionally unset, making this byte-identical
// to envOrCwd("EVOLVE_PROJECT_ROOT"); if it is present in a developer's shell
// from a prior session it takes precedence (unset it if a command reads the
// wrong root). Root-cause fix for the cycle-355 trap, where `flags check` read
// main's stale control-flags.md and red-failed correct work.
```

### `go/cmd/evolve/cmd_subagent_run_test.go:3` — above `import (`

```text
// cmd_subagent_run_test.go — ADR-0103 unit 16: the `evolve subagent run` root's
// stderr lines and exit map, pinned before the execution path moved into
// internal/subagent/subagentrun (nothing pinned them on 8e8f080f).
```

### `go/cmd/evolve/cmd_subagent_workspace_test.go:11` — above `func TestRunSubagentRun_RelativeWorkspaceResolvesAgainstProjectRootNotCwd(t *testing.T) {`

```text
// cmd_subagent_workspace_test.go is the cycle-616 regression for the
// fable5_deep_scan finding "subagent-workspace-absolutize": cmd_subagent.go
// passes the LLM-typed <workspace_path> positional argument (runSubagentRun,
// runSubagentDispatchParallel, runSubagentCachePrefix's --workspace) straight
// through to subagent/run.go and bridge/engine.go, which only os.Stat/MkdirAll
// it — no absolutization, no validation. A relative arg resolves against
// whatever the process's ambient CWD happens to be at invocation time
// (confirmed in the repo tree by untracked go/tmux-sessions.jsonl and
// go/.bridge-inbox/ scattered by exactly this path), which is the class of
// untracked-tree-drift the tree-diff cycle-killer guard reacts to (see the
// boundary_only_main_tree_writes standing rule).
//
// This test targets `run`, the highest-traffic of the three call sites. It
// proves the CURRENT behavior black-box: point EVOLVE_PROJECT_ROOT at one temp
// dir, chdir the test process into a DIFFERENT (empty) temp dir, and invoke
// `evolve subagent run <agent> <cycle> <relative-workspace>` where
// <relative-workspace> exists under the project root but NOT under cwd.
//
//   - Today: workspace is used exactly as typed, so
//     os.Stat("<relative-workspace>") resolves against cwd (the empty temp
//     dir), fails, and subagent/run.go returns "workspace dir does not exist" —
//     even though the SAME relative path is a real, valid directory one level
//     up under the project root. This is the observable symptom of "no
//     absolutization at the ingestion point."
//   - After the fix: the ingestion point must resolve a relative workspace arg
//     against a known base (EVOLVE_PROJECT_ROOT / layout.ProjectRoot) — NOT
//     raw cwd — before handing it to subagent.Run, so the stat succeeds and
//     the run proceeds (failing later for an unrelated, expected reason: no
//     profile fixture is set up in this test). Either way, nothing may be
//     created under the invoking cwd.
```

### `go/cmd/evolve/cmd_subagent_workspace_test.go:114` — above `func TestRunSubagentCachePrefix_RelativeWorkspaceResolvesAgainstProjectRootNotCwd(t *testing.T) {`

```text
// --- Test-amplification additions (cycle 616 black-box adversarial pass) ---
//
// The AC for subagent-workspace-absolutize names THREE ingestion points
// (runSubagentRun, runSubagentDispatchParallel, runSubagentCachePrefix), but
// the TDD RED test above only exercises `run` — "the highest-traffic of the
// three call sites" per its own comment. The tests below extend the same
// black-box contract (relative workspace resolves against project root, not
// cwd; nothing is ever written into the invoking cwd) to the other two
// ingestion points, plus edge/limit inputs (empty, parent-traversal,
// deeply-nested) against the `run` call site the RED test already covers.
```

### `go/cmd/evolve/cmd_subagent_workspace_test.go:325` — above `func TestRunSubagentRun_WorkspaceOutsideProjectRootRejected(t *testing.T) {`

```text
// TestRunSubagentRun_WorkspaceOutsideProjectRootRejected is the cycle-619
// containment slice of subagent-workspace-absolutize. Cycle 616 landed
// absolutization (a relative arg resolves against project root, not cwd) but
// left the ".." escape unhandled — a relative workspace like "../sibling"
// still resolves to a real directory OUTSIDE the project root and then gets
// os.Stat'd + MkdirAll'd (workers/, logs), scattering run artifacts into an
// arbitrary sibling tree. That is the same untracked-tree-drift class the
// absolutization fixed for cwd, just relocated. The resolver must now REJECT a
// relative workspace whose cleaned join escapes the project root, loudly and
// with no MkdirAll side effect. (Absolute args keep their documented
// passthrough contract — the real loop hands absolute worktree/runs paths.)
```

### `go/cmd/evolve/cmd_swarm.go:1` — above `package main`

```text
// Command cmd_swarm.go — `evolve swarm status|reap`. Operator surface for the
// swarm harness (ADR-0032): inspect the per-cycle session manifest and reap
// orphaned worker sessions after a crash. Read-only `status`; teardown `reap`.
//
// `reap` is the crash-safe backstop: the in-process dispatcher reaps on normal
// exit, but a hard SIGKILL of the orchestrator leaves orphaned tmux sessions +
// process groups that only the on-disk manifest can recover. It NEVER does a
// broad `pkill` — it kills exactly the pgids/sessions the manifest recorded.
```

### `go/cmd/evolve/cmd_swarm_test.go:3` — above `import (`

```text
// cmd_swarm_test.go — RED contract for cycle-549's
// cli-command-layer-test-coverage task (see cmd_worktree_test.go's package
// doc comment for the full task/lane background). `evolve swarm
// status|reap|reap-orphans` (cmd_swarm.go) had ZERO direct test coverage
// (0.0% on every handler per `go tool cover -func`).
//
// SAFETY: runSwarmReap's production killer sends a REAL syscall.Kill(-pgid,
// SIGKILL) when a session's PGID>1, and a real tmux kill-session when
// TmuxSession != "" (see internal/swarm/kill.go's 2026-06-11 killer-B
// incident doc). Fixtures here use PGID=0 and an empty TmuxSession — both
// gates in ExecSessionKiller.Kill (h.PGID > 1 / h.TmuxSession != "") skip the
// dangerous calls entirely, so the reap path is exercised end to end (manifest
// load, registry rebuild, Reap dispatch, output) without ever touching a real
// process group or tmux server. reap-orphans tests additionally pass
// --dry-run, which the CLI itself stubs to a no-op Kill regardless of fixture
// content.
```

### `go/cmd/evolve/cmd_syncmain.go:18` — above `func runSyncMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {`

```text
// runSyncMain implements `evolve sync-main` — an operator/boundary command that
// reconciles a locally-diverged main with origin via a plain merge, so a
// recurring diverged-origin stall no longer needs manual `git` reconciliation
// (inbox ship-repair-merge-diverged-origin). It is the sanctioned successor to
// hand-running git after ship/repair refuses on a diverged origin (repair.go
// only ever preserves the local commit and points here — it never mutates the
// tree itself).
//
// Preconditions (ALL checked before any git mutation):
//   - no live run lease: an active cycle owns the tree; refuse (cycle-395 race).
//   - clean index: any uncommitted change blocks the sync (.evolve/** is
//     gitignored, so lease/cycle-state churn never counts as dirty).
//
// Behavior: fetch origin, then `git merge --no-edit origin/<branch>`. A clean
// divergence merges (real merge commit). A conflict aborts back to the exact
// pre-merge state (`git merge --abort`) and refuses. It NEVER rebases,
// force-pushes, or pushes — local history only ever moves forward via merge.
//
// Exit codes: 0 merged (or already up to date); 1 refused/error (tree unchanged
// on refusal).
```

### `go/cmd/evolve/cmd_syncmain.go:67` — above `if ws := liveLeaseWorkspace(absRoot); ws != "" {`

```text
// Precondition 1: no live run lease. An active cycle owns the working tree;
// merging under it is the cycle-395 clobber race. Mirrors cmd_loop.go's
// live-owner probe (runlease.Read + OwnerLive + pidAlive).
```

### `go/cmd/evolve/cmd_syncmain_test.go:3` — above `import (`

```text
// cmd_syncmain_test.go — RED tests (cycle 611, task sync-main-boundary-command)
// for the new `evolve sync-main` operator/boundary command (inbox
// ship-repair-merge-diverged-origin, weight 0.85): a recurring diverged-origin
// stall (3x on 2026-07-07) currently requires manual operator reconciliation.
//
// Contract the Builder implements (TDD-defined seam):
//
//	func runSyncMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int
//
// Registered in registry.go as the "sync-main" subcommand (mirrors
// runResetSHA's `--project-root` flag convention, cmd_resetsha.go).
//
// Preconditions (ALL must hold before any git mutation is attempted):
//   - no live run lease: read .evolve/cycle-state.json's workspace_path (if the
//     marker exists) and refuse if runlease.OwnerLive is true there
//   - clean index (git status --porcelain empty, ignoring .evolve/** per repo
//     .gitignore) — refuse on any uncommitted change
//   - cycle-state idle (see above)
//
// Behavior:
//   - fetch origin, then `git merge --no-edit origin/<branch>` on divergence
//   - a clean, non-conflicting divergence merges (a real merge commit, two
//     parents) — quiet tree, no operator involvement
//   - a conflicting divergence aborts cleanly: working tree and HEAD end up
//     EXACTLY as they started (no MERGE_HEAD, no conflict markers)
//   - NEVER rebases, force-pushes, or pushes — sync-main only ever moves local
//     history forward via merge; the bare origin ref must be byte-identical
//     before and after every scenario in this file
//
// RED now (undefined symbol runSyncMain → package main test build fails). Do
// NOT modify this file — implement the seam in a new cmd_syncmain.go.
```

### `go/cmd/evolve/cmd_tokens.go:33` — above `type TokensReport struct {`

```text
// TokensReport is the walked-window aggregate `evolve tokens report` emits.
// PhasesWithData/PhasesRun are the telemetry-coverage counters (cycle-779):
// how many walked phase runs carried ANY token data versus how many ran —
// so an unmeasured window reads as a coverage gap, not as "free".
```

### `go/cmd/evolve/cmd_tokens.go:45` — above `TripwireCount int             'json:"tripwire_count"'`

```text
// TripwireCount/Tripwires surface the engine's telemetry-coverage tripwire
// (cycle-1005): a non-claude launch that exits 0, runs past the 60s success
// threshold, and resolves to source=none burned real tokens the resolver
// never measured. The engine records `"tripwire":true` in llm-calls.ndjson;
// this reporter reads and surfaces it so the miss shows up in the report,
// not just engine stderr.
```

### `go/cmd/evolve/cmd_tokens.go:219` — above `func stripControlBytes(s string) string {`

```text
// stripControlBytes removes control bytes (< 0x20 and 0x7f) from an
// llm-calls.ndjson-sourced string before it reaches the TTY. A compromised
// non-claude driver could embed ANSI escapes in its own record's CLI/agent/
// phase fields to rewrite or hide the very tripwire line meant to expose it
// (cycle-1010 audit F1); the --json path is unaffected (already safe).
```

### `go/cmd/evolve/cmd_tokens.go:278` — above `func renderTripwires(w io.Writer, r TokensReport) {`

```text
// renderTripwires prints the telemetry-coverage tripwire section. It runs
// unconditionally above the empty-phases early return so a cycle with tripwire
// hits but no phase-timing data still surfaces the miss (cycle-1007 render-order
// regression). Silent when no tripwire fired. CLI/agent/phase fields are
// control-byte-stripped before hitting the TTY (F1).
```

### `go/cmd/evolve/cmd_tokens_coverage_test.go:3` — above `import (`

```text
// cmd_tokens_coverage_test.go — cycle-779 TDD contract (RED) for the
// token-telemetry-input-cache-fidelity task's AC3 report half: `evolve tokens
// report` gains a coverage line — phases WITH token data / phases RUN — so a
// telemetry gap (the 2026-07-13 all-zeros baseline) is visible in the report
// itself instead of masquerading as "this phase is free".
//
// The contract is deliberately loose about layout: the rendered report must
// carry a line starting with "Coverage:" containing the `<with-data>/<run>`
// ratio. Builder owns the exact wording and any TokensReport JSON field.
```

### `go/cmd/evolve/cmd_tokens_coverage_test.go:46` — above `func TestTokensReport_CoverageCountsOnlyPhasesWithData(t *testing.T) {`

```text
// TestTokensReport_CoverageCountsOnlyPhasesWithData (negative): a window where
// EVERY phase ran but NONE recorded tokens (the 2026-07-13 baseline shape)
// must report coverage 0/2 — zero-token phases are uncovered, never counted
// as covered-and-free.
```

### `go/cmd/evolve/cmd_tokens_coverage_test.go:70` — above `func TestTokensReport_TripwireFiresOnNonClaudeSuccess(t *testing.T) {`

```text
// --- cycle-1014: report-level telemetry-coverage tripwire regression lock ---
//
// tripwire-regression-lock (inbox telemetry-coverage-tripwire-nonclaude-success,
// weight 0.93). Production is already landed and unchanged — the engine records
// `"tripwire":true` in llm-calls.ndjson (recordTokenUsage) and cycle-1013 wired
// `evolve tokens report` to read and surface it (readCycleTripwires /
// renderTripwires). This cycle adds the single explicitly-AC-named consolidated
// positive+negative regression at the report layer so a future hostile edit to
// the render path (the exact cycle-1007 render-order defect) is caught by a test
// whose name states the AC1/AC2/AC3 contract. The ACS predicate
// (go/acs/cycle1014/predicates_test.go) requires these two names verbatim.
// Fixture helpers (writeTokensTimingFixture / writeTokensLLMCalls /
// lineContaining) are shared from cmd_tokens_test.go (same package main).
```

### `go/cmd/evolve/cmd_tokens_coverage_test.go:84` — above `func TestTokensReport_TripwireFiresOnNonClaudeSuccess(t *testing.T) {`

```text
// TestTokensReport_TripwireFiresOnNonClaudeSuccess — AC1+AC2. A single non-claude
// launch that exited 0, ran past the 60s success threshold, and resolved to
// source=none surfaces a TRIPWIRE line in the plain-text report, and that line
// names the offending CLI, agent, AND cycle together (not just one). Cycle 6 is
// discovered via its phase-timing.json; duration 90000 / exit 0 carry no stray
// "6" digit, so the cycle-number assertion is real.
```

### `go/cmd/evolve/cmd_tokens_test.go:106` — above `func writeTokensLLMCalls(t *testing.T, root, cycle string, records []map[string]any) {`

```text
// --- cycle-1013: surface engine tripwire records in `evolve tokens report` ---
//
// The engine (go/internal/bridge/engine.go recordTokenUsage, unchanged) already
// writes a `"tripwire":true` field into a cycle's llm-calls.ndjson whenever a
// non-claude launch exits 0, runs past the 60s success threshold, and resolves to
// source=none. These RED tests encode the report-side gap: `evolve tokens report`
// must READ those records and SURFACE the tripwire (plain-text + --json), because
// today buildTokensReport only walks phase-timing.json and never opens
// llm-calls.ndjson. Fixtures write the exact on-disk record shape; assertions run
// over the rendered output / generic JSON so they exercise real behavior without
// binding to a not-yet-authored struct field name.
```

### `go/cmd/evolve/cmd_tokens_test.go:232` — above `func TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty(t *testing.T) {`

```text
// TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty — the cycle-1007
// render-order regression. Cycle 7 is discovered (its phase-timing.json exists)
// but has ZERO phase entries, so r.Phases is empty — the exact shape that made
// renderTokensReport early-return before printing anything. The tripwire must
// still surface.
```

### `go/cmd/evolve/cmd_tokens_test.go:284` — above `func TestRunTokensReport_SanitizesTripwireControlBytes(t *testing.T) {`

```text
// TestRunTokensReport_SanitizesTripwireControlBytes — F1 EDGE (carried from the
// cycle-1010 audit). A compromised non-claude driver embeds ANSI escape bytes in
// its own record's CLI/agent/phase fields to rewrite or hide the tripwire line
// meant to expose it. The plain-text render must escape/strip control bytes: no
// raw ESC (0x1b) may reach the TTY, and the tripwire must still surface.
```

### `go/cmd/evolve/cmd_worktree.go:38` — above `func absWorktreeRoot(projectRoot string, stderr io.Writer) string {`

```text
// absWorktreeRoot absolutizes a worktree subcommand's --project-root (default
// ".") so the recorded worktree path and base dir are cwd-independent — the
// same canonicalization every entrypoint applies (cycle-119 path-divergence).
```

### `go/cmd/evolve/cmd_worktree_retry_test.go:3` — above `import (`

```text
// cmd_worktree_retry_test.go — RED contract for cycle-1268 task
// `worktree-provisioning-retry-consolidate`, adoption site #3:
// runWorktreeCreate (cmd_worktree.go:82).
//
// This site is doubly exposed. It has no retry, and it is the only one of the
// four that bypasses the gitexec seam entirely — a raw
// exec.Command("git", "-C", ...) whose failure handling prints err and returns
// 1 without ever seeing git's exit code. That is why it has no test coverage
// today: there is nothing to inject. Adopting the shared helper therefore also
// buys the rc/stderr parity the other three sites already have.
//
// The two pins below (worktreeGitRunner, worktreeAddRetry) are the minimum
// seam that makes the operator path testable at all; they mirror core's
// gitRunner and swarm's newGit/retry precedents rather than inventing a shape.
```

### `go/cmd/evolve/cmd_worktree_test.go:3` — above `import (`

```text
// cmd_worktree_test.go — RED contract for cycle-549's
// cli-command-layer-test-coverage task (triage-report.md top_n item, this
// lane's fleet_scope: cli-command-layer-test-coverage-worktree-swarm).
//
// PROBLEM: `evolve worktree create|list|cleanup` (cmd_worktree.go) had ZERO
// direct test coverage (0.0% per `go tool cover -func` on every handler) even
// though cycle-543 already lifted the sibling guardcmd/opscmd packages to the
// 80% bar — this file is the un-shipped remainder of the original inbox item
// (`cli-command-layer-test-coverage`), scoped by this lane's fleet_scope to
// cmd/evolve's worktree + swarm-reap surface.
//
// These tests drive the REAL `git worktree` subprocess against a throwaway
// repo in t.TempDir() — no fake Runner seam exists for this file today (the
// functions call exec.Command directly), so success+error coverage means
// actually creating/listing/removing a real worktree. Skips if `git` is
// unavailable (mirrors the existing e2e convention in this package, e.g.
// cmd_loop_coverage_test.go's TestRunLoop_ResumePhaseRunnerError).
```

### `go/cmd/evolve/cycle_failure_outcome_ledger_test.go:3` — above `import (`

```text
// cycle_failure_outcome_ledger_test.go — ADR-0101 S4a: the failed-cycle inbox
// walk (cycleoutcome.ApplyFailure) appends its lifecycle lines through the
// ROOT's ledger — the one the Signal Center observes — on every path that
// reaches it: the cycle-run root and both sequential loop paths. Before this
// the inbox mover built its own unobserved FileLedger over the same file
// (S4a architecture review HIGH-1).
```

### `go/cmd/evolve/demote_personaless_test.go:3` — above `import (`

```text
// demote_personaless_test.go — the registration-seam prevention layer for
// cycle-1551: a discovered spec whose persona doc does not load is demoted to
// catalog:"on-demand" IN MEMORY before the catalog merge, so the SELECT menu
// never offers an undispatchable phase — on any host, tracked or not.
```

### `go/cmd/evolve/demote_personaless_test.go:25` — above `{Name: "no-doc", Optional: true},`

```text
// cycle-1551 shape: demoted
```

### `go/cmd/evolve/dispatch_test.go:91` — above `var stdout, stderr bytes.Buffer`

```text
// loop with no args still reaches runLoop which then errors on
// missing goal (v11.5.0 M1: accepts --goal-hash, --goal-text,
// positional goal, or --resume; error wording reflects the menu).
```

### `go/cmd/evolve/docs_contract_test.go:1` — above `package main`

```text
// docs_contract_test.go — v12.1 test layer 7: docs-contract enforcement.
// Asserts that every EVOLVE_* env var referenced in the Go code (via
// envchain.PhaseEnvKey, os.Getenv, or req.Env lookups) appears in the
// "Current behavior" table — which lives in
// docs/operations/runtime-reference.md since 2026-06-05 (moved out of
// CLAUDE.md to keep it under the 40k-char context limit; both files are
// scanned). Fails when a developer adds a new env var without
// documenting it.
//
// Two intentional softnesses:
//   - We allow EVOLVE_<PHASE>_PERMISSION_MODE / _MODEL / _PLAN_INPUT /
//     _PLAN_OUTPUT / _INTERACTIVE_POLICY as documented patterns; only
//     the parent variable needs to be in the scanned docs.
//   - Test-only env vars (EVOLVE_TEST_*, EVOLVE_GO_BIN test override)
//     are exempted.
```

### `go/cmd/evolve/docs_contract_test.go:95` — above `var claudeBody string`

```text
// The env-var table moved to docs/operations/runtime-reference.md
// (2026-06-05); CLAUDE.md keeps a digest. Scan both so a row in
// either file satisfies the contract.
```

### `go/cmd/evolve/e2e_cli_degradation_test.go:3` — above `package main`

```text
// End-to-end coverage of the CLI fallback chain (ADR-0029 / Workstream G)
// driven through `evolve cycle run`: when the primary CLI fails with a
// trigger exit code, the runner must retry the phase on the next allowed CLI
// and the cycle must still complete.
//
// Scope note — what this asserts vs. what lives elsewhere:
//   - Fallback ORCHESTRATION (the runner looping candidates on a trigger code)
//     is exercised here, end-to-end, because nothing else covers it.
//   - The require-full → exit-99 GATE (BRIDGE_REQUIRE_FULL with a sub-full
//     tier) is already covered at the bridge level — see
//     internal/bridge/launch_modes_test.go:TestLaunchArgs_RequireFull_Unmet
//     and coverage_batch2_test.go:TestRequireFull_ManifestMissing — so it is
//     NOT re-tested here. Exit 99 appears below only as a NON-trigger code
//     that must NOT fall back.
//   - The bash adapters' graceful-degradation stub (missing binary → stub
//     artifact → exit 0) does NOT exist in the v11+ Go path; the Go path
//     returns a fallback-trigger exit code (e.g. 127 ExitMissingBinary) and
//     relies on the fallback chain instead. Asserting the bash stub here would
//     test behavior the Go path does not have. See docs/TEST_PLAN.md.
//
// Deterministic + host-independent: the fake's per-CLI exit injection
// (FAKE_CLI_CLAUDE_EXIT) makes the primary claude-p fail with a chosen code
// while codex (same fake binary, codex invocation style) succeeds.
```

### `go/cmd/evolve/e2e_cycle_cli_matrix_test.go:128` — above `"EVOLVE_NATIVE_SHIP=0",`

```text
// v11.3.0: pin the legacy shell-out path. EVOLVE_SHIP_SCRIPT only
// takes effect when the dispatcher routes to bash; the fake-cli
// e2e harness depends on the script substitution.
```

### `go/cmd/evolve/e2e_cycle_cli_matrix_test.go:260` — above `func setupTempProject(t *testing.T, repoRoot string) string {`

```text
// setupTempProject builds an isolated project root in t.TempDir() with
// everything the cycle path needs:
//   - git init + initial commit (so ship's git commit has a parent)
//   - .evolve/profiles/{intent,scout,triage,tdd,build,audit,retro}.json
//     (stubs — bridge profile loader only requires `name`)
//   - .evolve/state.json bootstrapped to cycle 0
//   - committed durable Go predicate input for native Audit
//
// The in-process Go bridge resolves paths from the request (no
// tools/agent-bridge tree is symlinked — that was the pre-cutover bash path).
```

### `go/cmd/evolve/e2e_cycle_cli_matrix_test.go:275` — above `if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {`

```text
// Stub profiles. Names match what the Go phase runner constructs:
// strings.TrimPrefix(AgentPromptName, "evolve-") → AGENT-named, not
// phase-named. This aligns with the production .evolve/profiles/
// layout (tdd-engineer.json, builder.json, auditor.json,
// retrospective.json) AND with CLAUDE.md's documented env-var
// convention (EVOLVE_TDD_ENGINEER_PERMISSION_MODE, etc.).
//
// Source: cycle 106 (2026-05-25) integration smoke caught the
// mismatch when the runner looked for `tdd.json` and prod only had
// `tdd-engineer.json`.
// .evolve/policy.json — disable the cycle-start LIVE model-catalog refresh.
// The compiled default is AutoRefresh=true (policy.go: "the cycle-start live
// refresh is on") and production turns it off in the checked-in
// .evolve/policy.json; this fixture seeded no policy at all, so it inherited
// the default and every `evolve cycle run` launched real CLI probes for EVERY
// family before its first phase — agy and ollama with no fake binary at all,
// plus a `/model` picker wait per family that can only time out against the
// fake REPL. That startup cost alone exceeded the 120s harness budget, so the
// cycle was killed before reaching ship: TestE2ECLIFallbackChain failed on all
// four trigger codes for a reason that had nothing to do with the fallback
// chain it exists to prove. Model discovery has its own tests; a fallback-chain
// fixture must not pay for it.
```

### `go/cmd/evolve/e2e_live_harness_test.go:86` — above `"usage limit", "upgrade to plus", "limit will reset", "plan limit",`

```text
// Subscription usage/quota exhaustion. The CLI booted and authenticated;
// the account is simply capped until a reset date — a provider limit, not a
// broken contract. Observed live 2026-05-30: codex "You've hit your usage
// limit. Upgrade to Plus … try again at Jun 4th". claude/agy use similar
// phrasings ("usage limit reached", "limit will reset"). These are
// quota-SPECIFIC substrings unlikely in model-generated output. We
// deliberately exclude the codex message's generic "try again at" tail:
// phaseStderrTail folds the model's own stdout/stderr into the classified
// string, and that ordinary English phrase could mask a real contract break
// (the quota-specific markers below already catch the codex/claude/agy caps).
```

### `go/cmd/evolve/e2e_live_harness_test.go:310` — above `out += "\n" + phaseStderrTail(projRoot)`

```text
// The classifier matches against Out, but the `evolve cycle run` subprocess
// only surfaces a structured "bridge: launch exit=1" line — the real provider
// message (e.g. codex "You've hit your usage limit") lands in the per-phase
// *-stderr.log artifact, not the subprocess stdout. Fold those artifacts into
// Out so a quota/provider failure is classified transient instead of being
// mis-graded a contract break. (Confirmed live 2026-05-30: without this the
// codex quota cap red-failed the suite.)
```

### `go/cmd/evolve/e2e_live_matrix_test.go:3` — above `package main`

```text
// Tier 2 — LIVE model-tier matrix. For each CLI, fire a single cheap bridge
// launch at EACH supported tier's concrete model and assert the call succeeds
// and writes an artifact. This catches model-deprecation / tier-resolution
// drift (e.g. a provider 400-rejecting a model the manifest still advertises —
// the cycle-142 codex ChatGPT-safe-model class of bug) that the cheapest-tier-
// only T0/T1 would miss. Gate: EVOLVE_E2E_LIVE_MATRIX=1.
```

### `go/cmd/evolve/e2e_live_matrix_test.go:25` — above `"codex": codexTierModels(),`

```text
// codex: a PROJECTION of the family manifest, never a copy (this table sat
// three model generations stale until 2026-09-09).
```

### `go/cmd/evolve/e2e_pipeline_paths_test.go:28` — above `const strictPolicyMarker = "TEST_WRITE_STRICT_POLICY=1"`

```text
// pipelineCycle runs one headless claude-p cycle with the given env overlay and
// returns the ledger entries. It never fails the test on a non-zero cycle exit —
// a blocked cycle (and, post Stage-5.1, a native ship that cannot ff-merge in
// the fixture) legitimately returns non-zero — so callers assert on the
// ledger-observable routing (which phases the pipeline reached) via ledgerHasRole.
// strictPolicyMarker is a TEST-ONLY sentinel (not a production flag, so it never
// reaches the cycle subprocess). When present in a pipelineCycle extraEnv list it
// makes the harness drop a .evolve/policy.json with workflow.strict_audit:true into
// the cycle's project root — the policy.json replacement for the retired
// EVOLVE_STRICT_AUDIT env dial (flag-reduction, ADR-0064).
```

### `go/cmd/evolve/e2e_pipeline_paths_test.go:77` — above `out, err := runWithTimeout(cmd, 300*time.Second)`

```text
// 300s, was 120s: the Build-explanation contract added real per-cycle work
// (contract activation, the build handoff floor's base-bound git diffs, the
// audit review gate, and up to two correction re-dispatches), and an audit
// FAIL now takes the bounded ADR-0093 repair loop (tdd+build+audit again)
// before retro. A probe cycle completes in ~200-250s; 120s was tuned for
// the shorter pre-contract pipeline and killed every cycle mid-flight,
// losing the subprocess output with it.
```

### `go/cmd/evolve/e2e_serve_phase_subprocess_test.go:3` — above `package main`

```text
// End-to-end proof that phaseproto.SubprocessRunner can drive a real
// `evolve serve-phase` subprocess. Closes the "cross-CLI parity
// hardening" gate from Phase 3 task #17 (progress doc, sub-bullet 1).
//
// Strategy: build the evolve binary with -tags evolve_test_phases so a
// test-only `echo` phase is registered. The wire path under test is:
//
//	test goroutine
//	  └── phaseproto.SubprocessRunner.Run
//	         └── exec.CommandContext("evolve", "serve-phase", "echo")
//	                └── dispatch -> runServePhase
//	                       └── phaseproto.ServeStdio
//	                              └── echoPhaseRunner.Run
//	         (response envelope flows back up)
//
// No real phase work, no Claude CLI required.
```

### `go/cmd/evolve/e2e_setup_validation_test.go:3` — above `package main`

```text
// Coverage of `evolve setup detect` — the kernel clamp behind the /setup skill
// that enforces the CLI×model integrity floor (ADR-0027). Step 9b removed the
// standalone `setup validate` subcommand (+ the llm_config.json it clamped); the
// same floor is now applied by policy.ValidatePin and surfaced inline by detect:
//   - pinned tier ∉ profile model_tier_envelope → phase `pin_violation` (+ dispatch hard-fail)
//   - pinned cli  ∉ profile allowed_clis         → phase `pin_violation` (+ dispatch hard-fail)
//
// Driven in-process via runSetup (this is package main), so these are fast and
// host-independent: the pin overlay reads only the fixture .evolve/policy.json +
// profiles. (detect additionally scans the host for CLI binaries, so the clis[]
// array is asserted on SHAPE only, not on host-specific values.)
```

### `go/cmd/evolve/echo_phase_testbuild.go:23` — above `registry.Register("echo", func(req core.PhaseRequest) core.PhaseRunner {`

```text
// Register into the phases registry that `evolve serve-phase`
// (phasecmd.RunServePhase → registry.For) resolves against. The former
// package-local `phaseFactories` map was removed when serve-phase moved to
// internal/cli/phasecmd, which silently bit-rotted this test build
// (undefined: phaseFactories) — see ADR-0062/T1.7 follow-up.
```

### `go/cmd/evolve/inboxmover_claim_binding_test.go:3` — above `import (`

```text
// inboxmover_claim_binding_test.go — ADR-0074 finding-1 pin: the triage agent
// doc and its permission profile must bind the claim step to the Go floor
// (`evolve inbox-mover claim`), not the deleted inbox-mover.sh script. Without
// this, ErrConsoleRouted (and every future claim-side control) is unreachable
// on the live path — the producer-without-consumer disease ADR-0074 exists to
// end. This is a WIRING test: it reads the real repo files.
```

### `go/cmd/evolve/main_test.go:11` — above `func TestMain(m *testing.M) {`

```text
// TestMain disables the workspace-pollution guard for every test in this
// package. Many cmd/evolve tests pre-seed workspace files in temp dirs to
// simulate phase state (M4/M5 dispatch validators, ledger writers,
// cycle-state tests, etc.). The orchestrator's workspace-guard (added in
// v12.2.2 for bug #4) would archive those pre-seeded files away, breaking
// the tests. Production keeps the guard on by default (disableWorkspaceGuardForTest=false);
// tests opt out via the package-level seam (cycle-10: replaced the retired
// EVOLVE_DISABLE_WORKSPACE_GUARD env signal with a DI bool).
//
// Individual tests that specifically EXERCISE the guard should set
// disableWorkspaceGuardForTest=false and restore it with t.Cleanup.
//
// It also installs a pass-through preflight seam so loop-mechanics tests
// (budget, circuit breaker, checkpoint, quota) can drive runLoop with a
// faked orchestrator in a temp project that has no real environment. The
// gate has its own coverage in cmd_loop_preflight_test.go, which overrides
// the seam per-test via the runLoopPreflightFn package var.
```

### `go/cmd/evolve/model_tier_resolver_test.go:3` — above `import (`

```text
// model_tier_resolver_test.go — the model-resolvability gate and its wiring.
//
// router.ClampPlanModelRouting clears an advisor-proposed {cli,tier} that
// cannot resolve to a model, fed by an injected lookup so router stays a leaf
// (ADR-0069's import-cycle lesson). core.WithModelCatalogLookup is that seam:
// defined, documented, unit-tested — and never called from the composition
// root, so o.modelCatalogLookup was nil in every real cycle and the gate was
// dead. TestWireOrchestrator_ModelCatalogLookupWired is the regression guard,
// asserted through the REAL composition root (mirroring
// TestWireOrchestrator_CompositionFastPathWired) rather than by source
// inspection: an AST scan cannot tell WithModelCatalogLookup(resolver) from
// WithModelCatalogLookup(nil), and nil is precisely the dead-gate state.
```

### `go/cmd/evolve/phaseroots.go:10` — above `func phaseRoots(projectRoot string) []string {`

```text
// phaseRoots delegates to phasespec.Roots — the single home for the
// EVOLVE_PHASE_ROOTS discovery-root policy (ADR-0038), shared with the
// merged-catalog loader so cmd and library consumers can never diverge.
// Kept as a local name for the cmd call sites.
```

### `go/cmd/evolve/phaseroots.go:18` — above `func discoverUserSpecsClamped(projectRoot string, prm *prompts.Loader) ([]phasespec.PhaseSpec, []string) {`

```text
// discoverUserSpecsClamped is the COMPOSED discovery path: discovery-root
// load + the registrar-parity clamp (ADR-0073 on-disk-spec trust gap; see
// phasespec/clamp.go) + the persona-resolvability demotion (cycle-1551; see
// demotePersonalessSpecs). Every catalog admission of a discovered spec goes
// through here, so writes_source eligibility can never bypass the
// sandboxed-profile verification and an undispatchable phase can never reach
// the SELECT menu — the wiring lives in the composed path, not just a unit.
```

### `go/cmd/evolve/phaseroots.go:33` — above `func demotePersonalessSpecs(specs []phasespec.PhaseSpec, prm *prompts.Loader) ([]phasespec.PhaseSpec, []string) {`

```text
// demotePersonalessSpecs returns a copy of specs with every phase whose
// persona doc cannot be loaded demoted to catalog:"on-demand" IN MEMORY, plus
// a warning per demotion. The SELECT menu must never offer a phase the runner
// cannot dispatch: cycle-1551 (soak-20260824a) died rc=4 when the advisor
// inserted defect-disposition-preflight, whose derived persona exists nowhere.
// This is the runtime seam counterpart of registerBuiltinSpecRunners' persona
// check for builtin spec phases — it covers TRACKED and UNTRACKED specs on any
// host, where the repo-catalog guard test only sees tracked ones. Demotion,
// not removal: the phase stays requestable on demand, and the dispatch-time
// fail-soft (core.ErrAgentDocMissing → optionalInfraSkip) bounds the blast
// radius if it is dispatched anyway. Control-role and non-llm phases are
// exempt (they do not dispatch through prompts.Loader.Agent).
```

### `go/cmd/evolve/phaseroots_clamp_test.go:3` — above `import (`

```text
// phaseroots_clamp_test.go — wiring proof for the load-time registrar-parity
// clamp (inbox loadtime-userspec-registrar-clamp; ADR-0073 Finding 1): the
// clamp must run in the COMPOSED cmd_cycle discovery path, not just as a
// phasespec unit. A smuggled .evolve/phases/*/phase.json claiming
// writes_source:true must come out of the composed path stripped unless its
// dispatch profile is registrar-minted with sandbox enabled.
```

### `go/cmd/evolve/routing_order_realtree_test.go:3` — above `import (`

```text
// Layer-N+1 wiring pin for the anchor-fixpoint splice (cycle-1550): the REAL
// production composition — config.Load(registry) order + discoverUserSpecsClamped
// (alphabetically sorted, clamped) + ApplyUserRouting — must place every
// anchored tracked phase AFTER its declared anchor. The unit fixpoint tests in
// internal/phasespec can pass while this path still mis-slots (different specs,
// different order source); this test runs the exact seam cmd_cycle.go runs.
```

### `go/cmd/evolve/scope_path_resolver_test.go:3` — above `import (`

```text
// scope_path_resolver_test.go — the composition-root half of the cycle-1548
// fix, tested against a REAL temp inbox in the live namesake shape.
```

### `go/cmd/evolve/signal_loop.go:1` — above `package main`

```text
// signal_loop.go — ADR-0101 S4a: the loop module's Signal Center producers.
// A batch halt is ONE loop.halt INCIDENT whose code names the rule (system
// failure, pipeline blocker, a fleet lane's halt code, a wave-boundary halt);
// a wave is loop.wave (INFO for the summary the report also prints — INFO
// never reaches the console — WARN with a code for a min-width repair); an
// escalation boundary is loop.escalation WARN. Each replaces a hand-written
// "[loop] …" line 1:1 (the wave summary stays: it is the operator's report,
// not a fact printed twice). The batch report reads the driven runner's
// per-cycle SignalSummary through formatSignalReport — it reports, never
// gates.
```

### `go/cmd/evolve/signal_loop.go:26` — above `CodeLoopMinWidthRepair signalcenter.Code = loopwave.CodeMinWidthRepair`

```text
// CodeLoopMinWidthRepair projects the wave engine's code (ADR-0103 unit
// 13): the leaf registers it, once.
```

### `go/cmd/evolve/signal_loop.go:59` — above `func emitLoopWave(signals *signalcenter.Center, wave int, origin string, code signalcenter.Code, reason string, fields m…`

```text
// emitLoopWave is the coordinator's spelling of the ONE loop.wave producer
// (loopwave.EmitWave, ADR-0103 unit 13): batch-level (no cycle), the wave
// number stamped on the producer's own copy of the caller's fields; INFO
// without a code, WARN with one.
```

### `go/cmd/evolve/signal_loop.go:88` — above `if p, r, c := s.ByKind[signalcenter.KindGatePassed], s.ByKind[signalcenter.KindGateRejected], s.ByKind[signalcenter.Kind…`

```text
// ADR-0101 S2b: the gate verdicts per cycle — the "checked → advanced"
// record the operator asked the orchestrator to keep.
```

### `go/cmd/evolve/signal_loop_test.go:3` — above `import (`

```text
// signal_loop_test.go — ADR-0101 S4a: the loop module's producers. A batch halt
// (system failure, pipeline blocker, a fleet lane's halt code, a wave-boundary
// halt) is ONE loop.halt INCIDENT whose code names the rule; a wave summary is
// loop.wave INFO (the report line stays — INFO never prints); a min-width
// repair is loop.wave WARN; an escalation boundary is loop.escalation WARN.
// The hand-written "[loop] … HALT" lines are gone: the root's WARN-filtered
// stderr sink renders them. The batch report reads the driven runner's
// per-cycle SignalSummary through the loop's own orchestrator seam.
```

### `go/cmd/evolve/skill_loop_goal_contract_test.go:1` — above `package main`

```text
// skill_loop_goal_contract_test.go — durable regression lock for the
// /evo:loop goal-required doc contract (cycle-1029, inbox item
// loop-skill-goal-mandatory-prompt-before-act).
//
// The Go binary REQUIRES a goal unless --resume/--dry-run
// (go/cmd/evolve/cmd_loop_args.go:151-156 → rc=10 "a goal is required …",
// locked by dispatch_test.go's TestDispatch_LoopRoutesToRunLoop). This test
// locks the OTHER half of that contract: skills/loop/SKILL.md must frame the
// goal as REQUIRED, so a future edit that re-introduces the optional `[goal]`
// bracket in the argument-hint or Usage line fails the suite here. It is the
// permanent counterpart to the per-cycle ACS predicates in
// go/acs/cycle1029/predicates_test.go, which are pruned after the cycle.
```
