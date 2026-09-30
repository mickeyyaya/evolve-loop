# Comment history: `internal/looppreflight`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/looppreflight/apicover_named_test.go:3` — above `package looppreflight`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - const DefaultBootBudget (looppreflight.go) — the per-driver REPL boot
//     deadline; asserted as the default resolve() applies when BootBudget<=0.
//   - var DefaultSpinePhases (looppreflight.go) — the always-dispatched spine
//     phases; asserted via its role as the fallback resolve() uses for an empty
//     SpinePhases, plus its documented membership.
//   - method CheckResult.MarshalJSON (result_json.go) — MUST be invoked by name
//     (json.Marshal alone names it only implicitly); we call c.MarshalJSON().
//   - method Result.MarshalJSON (result_json.go) — likewise invoked as
//     r.MarshalJSON() directly.
//
// Each test asserts a real contract (Rule 9), not a no-op reference.
```

### `go/internal/looppreflight/basedivergence.go:1` — above `package looppreflight`

```text
// basedivergence.go — boot-time guard against cutting lanes from a stale base.
//
// A fleet lane's worktree is branched from whatever the project root's HEAD is
// at boot. When that local base has fallen behind `origin/<base>`, every lane in
// the batch is built on stale history and the ship at the end fails with
// GIT_PUSH_REJECTED — after the whole batch's work is already spent (cycle-969).
// The reconcile is a single operator command (`evolve sync-main`), so the cheap
// remedy is to fetch origin at boot and HALT loudly, naming that command, BEFORE
// any lane spawns.
//
// The check fetches origin ITSELF rather than reading a possibly-stale local
// `origin/<base>` ref: an operator-prepared ref is exactly the thing that is out
// of date in this failure mode. A fetch that cannot complete degrades to Warn —
// unverified is surfaced, never silently passed — and a base that is merely
// AHEAD of origin (normal unpushed work) is a pass, not a halt.
```

### `go/internal/looppreflight/basedivergence_test.go:22` — above `func TestCheckBaseDivergence_BehindHalts(t *testing.T) {`

```text
// TestCheckBaseDivergence_BehindHalts — the cycle-969 case: a base behind
// origin must HALT and the halt must name `evolve sync-main`, so the operator
// gets a stop WITH a next step.
```

### `go/internal/looppreflight/boot.go:13` — above `func checkBridgeBoot(o resolved) CheckResult {`

```text
// checkBridgeBoot (Halt) is the check that catches the cycle-258 failure: it
// REALLY boots each configured *-tmux driver's REPL (boot-only, no prompt) and
// halts if any fails to reach its prompt marker. When SkipBoot is set it warns
// instead (CI/offline). Boots run sequentially, each under its own BootBudget
// deadline. The sandbox boot path is exercised iff the profiles request it AND
// the host can actually sandbox.
```

### `go/internal/looppreflight/boot.go:117` — above `return fmt.Sprintf("boot failure (exit=%d)", rc)`

```text
// Carry the numeric code: a bare "boot failure" made rc=42
// indistinguishable from rc=99 in preflight output (cycle-270
// fault-localization Rank 2).
```

### `go/internal/looppreflight/boot_integration_test.go:12` — above `func TestIntegration_BridgeBoot_ClaudeTmux(t *testing.T) {`

```text
// TestIntegration_BridgeBoot_ClaudeTmux is the one test that reproduces the
// cycle-258 catch: it runs the readiness gate with a REAL claude-tmux boot
// (BootTester defaulted to bridge.BootSmokeTest) and asserts the bridge-boot
// check passes on a healthy host. On a host where the REPL cannot boot it would
// halt with the captured scrollback — exactly the signal the loop lacked.
//
// Opt-in only (`go test -tags integration ./internal/looppreflight/...`); skips
// when tmux or claude is absent so the default suite stays hermetic.
```

### `go/internal/looppreflight/bug_reproduction_test.go:9` — above `func TestBootRCName_DefaultBranch_IncludesExitCode(t *testing.T) {`

```text
// TestBootRCName_DefaultBranch_IncludesExitCode reproduces the diagnostic gap:
// bootRCName() returns "boot failure" for unrecognized exit codes, discarding the
// numeric value. Operators cannot distinguish rc=42 from rc=99 from any other
// unrecognized code in preflight output — the diagnostic is semantically empty.
//
// Reproducer for cycle-270 fault-localization Rank 2 (boot.go bootRCName):
// RED on the pre-fix tree (bare "boot failure", no numeric code); GREEN once
// the default branch carries the exit code ("boot failure (exit=%d)"). The
// assertion is format-agnostic on purpose — it requires the number, not the
// exact phrasing.
```

### `go/internal/looppreflight/checks.go:154` — above `reapCtx, cancel := context.WithTimeout(context.Background(), sessionreaper.DefaultReapTimeout)`

```text
// Deadline-bound the boot sweep: a wedged tmux must abandon the kill, not
// hang loop boot forever (cycle-769 incident; orphanGCTimeout discipline).
```

### `go/internal/looppreflight/checks.go:190` — above `func checkCLIVersionDrift(o resolved) CheckResult {`

```text
// checkCLIVersionDrift (Warn) detects silent CLI version changes between
// batches. It compares the current version inventory (via o.versionInventory)
// against the last-seen versions persisted at .evolve/cli-versions.json.
// A version change on any inventoried CLI is a WARN — the operator should
// validate the change was intentional (incident: claude 2.1.173→2.1.175 despite
// autoUpdates:false, invisible because no version was recorded). First-batch
// (no prior cache) is always PASS and establishes the baseline. The updated
// inventory is persisted at the end of each run so the NEXT batch can compare.
```

### `go/internal/looppreflight/clihealth.go:11` — above `func checkCLIHealth(o resolved) CheckResult {`

```text
// checkCLIHealth surfaces ACTIVE CLI-family benches (.evolve/cli-health.json,
// written when a dispatch died on a classified transient wall like
// rate_limit) so the operator sees AT BATCH START that chains will run
// fallback-first — instead of discovering it from per-phase fallback logs
// (cycle-283: codex was quota-walled all night and only the dispatch trail
// showed it). Always Warn, never Halt: the fallback chain exists precisely so
// a benched family doesn't block the batch, and expired benches are canaried
// per-cycle by the loop.
```

### `go/internal/looppreflight/freeze.go:3` — above `import (`

```text
// freeze.go — ADR-0044 C5: the CLI-version-freeze readiness check
// (Specification pattern).
//
// cycle-262 D6: codex self-upgraded its own binary mid-phase — its updater
// ran `brew upgrade` on the TUI launch, printed "Update ran successfully!
// Please restart Codex.", and exited the REPL to a bare shell, which the
// bridge then nudged for ~20 minutes. The host fix was `brew pin codex`: a
// CONVERGENT STEADY STATE (survives reboots and crashed batches), not a
// per-cycle pin/unpin toggle (an unpin-on-exit leaks on any SIGKILL/OOM —
// see the ADR's alternatives-considered). This check verifies the steady
// state at batch start: any *-tmux CLI with self-update evidence on the host
// must be pinned, or the batch Halts with the exact convergent action.
//
// Scope: interactive *-tmux drivers only — the incident vector is the TUI
// launch path; headless `codex exec` does not run the updater. Probes are
// read-only (stat an evidence file, list brew pins) so the check is
// idempotent by construction. Ambiguity (pin listing failed: brew absent,
// exec error) WARNs with manual guidance — only CONFIRMED risk halts, the
// same fail-open posture as the eval gate.
```

### `go/internal/looppreflight/freeze.go:41` — above `func defaultSelfUpdateEvidence(bin string) (bool, string, error) {`

```text
// defaultSelfUpdateEvidence reports whether bin is known to self-update on
// launch, based on host evidence. Registry-style: codex maintains
// ~/.codex/version.json (the file that recorded dismissed_version=0.137.0 <
// latest=0.138.0 right before the cycle-262 mid-phase upgrade), and claude
// maintains ~/.claude/settings.json. A CLI without evidence is not
// freeze-checked; new self-updaters are added here as incidents reveal them.
// Assumption: these CLIs keep updater state under their default home
// directories (no CODEX_HOME/CLAUDE_HOME-style override is documented today).
// A failed home-dir lookup is AMBIGUITY (error), not absence of evidence —
// the caller WARNs instead of silently passing (fail loudly).
```

### `go/internal/looppreflight/freeze_test.go:3` — above `import (`

```text
// freeze_test.go — ADR-0044 C5 (Slice 3) RED tests: the CLI-version-freeze
// readiness check (Specification pattern).
//
// cycle-262 D6: codex self-upgraded its own binary mid-phase (its updater ran
// `brew upgrade` on launch, printed "Update ran successfully! Please restart
// Codex.", and exited the REPL to a bare shell). The host fix was `brew pin
// codex` — a CONVERGENT STEADY STATE, not a per-cycle toggle (an unpin-on-exit
// would leak on any SIGKILL/OOM/reboot). This check makes the steady state a
// verified precondition: a *-tmux CLI with self-update evidence on the host
// must be pinned or the batch Halts with the exact pin guidance.
//
// Scope: interactive *-tmux drivers only — the incident vector is the TUI
// launch path (headless `codex exec` does not trigger the updater). Probes are
// read-only (stat an evidence file; list brew pins), so the check is
// idempotent by construction; the pin itself is the operator's one-time
// convergent action.
```

### `go/internal/looppreflight/freeze_test.go:108` — above `func TestRun_VersionFreeze_HeadlessOnly_NotChecked(t *testing.T) {`

```text
// Headless-only usage is out of scope: the incident vector is the interactive
// TUI launch (headless `codex exec` does not run the updater).
```

### `go/internal/looppreflight/freeze_test.go:160` — above `func TestDefaultSelfUpdateEvidence_Unregistered(t *testing.T) {`

```text
// TestDefaultSelfUpdateEvidence_Unregistered pins the registry default: a CLI
// with no self-update entry (here "agy") has no evidence and no error. (This
// test previously probed "claude"; cycle-297 adds claude to the registry, so
// the non-registered example moved to a binary that genuinely has no entry —
// otherwise it would now collide with the real claude evidence on a host that
// has ~/.claude/settings.json.)
```

### `go/internal/looppreflight/freeze_test.go:179` — above `func TestDefaultSelfUpdateEvidence_ClaudePresent(t *testing.T) {`

```text
// TestDefaultSelfUpdateEvidence_ClaudePresent is the load-bearing RED test for
// cycle-297 Task 2 (claude-cli-version-freeze inbox HIGH). claude 2.1.173
// self-updated mid-soak (removed the `esc to interrupt` affordance), breaking
// PaneBusy detection and causing exit=81 in cycles 286/288/289/291. The freeze
// registry must recognize claude as self-updating via its updater state file
// ~/.claude/settings.json, exactly as it recognizes codex via
// ~/.codex/version.json. HOME is redirected to a temp dir with the file present
// so the assertion is deterministic and host-independent. RED baseline:
// defaultSelfUpdateEvidence("claude") returns (false,"",nil) because the
// function only handles "codex" — this test fails until the claude case lands.
```

### `go/internal/looppreflight/looppreflight.go:1` — above `package looppreflight`

```text
// Package looppreflight is the pre-batch environment-readiness gate for
// `evolve loop`. It runs BEFORE the first cycle dispatches and verifies the
// pipeline can actually run: every spine phase has a factory + deliverable
// contract, the profiles load and name known drivers, the LLM CLIs are present,
// the host has the capabilities the bridge needs, and — the check that matters
// most — each configured *-tmux CLI's REPL really boots.
//
// Motivation (cycle-258): a 3-cycle batch churned ~30 min before anyone
// discovered the bridge could not boot the CLI (exit 80 = ExitREPLBootTimeout).
// This gate catches that at batch start and aborts with a clear diagnostic so a
// doomed run never costs a cycle.
//
// Design: a DETERMINISTIC host-side gate, NOT an LLM agent phase — an env-check
// agent would have to run THROUGH the very bridge it is meant to verify
// (chicken-and-egg), and environment verification is deterministic work. It
// mirrors the releasepreflight blueprint (Options → Run → Result, nil→default
// seams) but ACCUMULATES every check result before deciding, so the operator
// sees all problems at once. The overall verdict halts iff any check halts.
```

### `go/internal/looppreflight/looppreflight.go:150` — above `SelfUpdateEvidence func(bin string) (bool, string, error)`

```text
// CLI-version-freeze seams (ADR-0044 C5).
// SelfUpdateEvidence reports whether bin self-updates on launch, plus the
// host evidence found. A non-nil error means the evidence was
// UNVERIFIABLE (ambiguity → Warn), distinct from a clean absence.
// Default: the known-updater registry (codex → ~/.codex/version.json).
```

### `go/internal/looppreflight/looppreflight_test.go:50` — above `SelfUpdateEvidence: func(string) (bool, string, error) { return false, "", nil },`

```text
// Freeze seams (ADR-0044 C5): benign defaults so unrelated tests
// never stat the real ~/.codex or exec real brew.
```

### `go/internal/looppreflight/orphanreap_deadline_test.go:14` — above `func TestPreflight_OrphanReapIsDeadlineBounded(t *testing.T) {`

```text
// Cycle-769 boot-orphan-sweep-bounded-tombstone regression contract (preflight
// half). Incident: checks.go ran the boot orphan sweep with
// context.Background() — a wedged tmux server hangs loop boot silently and
// indefinitely, while the per-cycle sweep has been deadline-bounded
// (orphanGCTimeout, cmd_loop_control.go) since the same incident class.
//
// Contract: the preflight sweep's killer receives a context carrying a
// boot-scale deadline (≤30s from the call — the existing orphanGCTimeout is
// 15s; the exact home of the hoisted const is the implementer's choice), so a
// blocked tmux exec is abandoned instead of wedging boot. The killer is
// injectable via Options.OrphanKill (defaulting to swarm.ExecTmuxKill) —
// preflight is otherwise untestable without a real tmux server.
```

### `go/internal/looppreflight/phase_routing.go:10` — above `func checkPhaseRoutingWarnings(o resolved) CheckResult {`

```text
// checkPhaseRoutingWarnings surfaces user-phase specs that phasespec DROPPED
// during catalog merge/routing (a built-in-name hijack, a non-optional
// override, a malformed phase.json) into the SAME accumulated, gate-visible
// preflight Result every other readiness problem lands in. Before this check,
// those warnings were only fmt.Fprintf'd to stderr by the CLI/dispatch callers
// and discarded — an operator who typo'd a user phase.json got a silently
// missing phase and no batch-start signal (scout cycle-591 Beyond-the-Ask).
//
// Always Warn, never Halt: a dropped user phase is degraded-but-runnable — the
// built-in spine is untouched, so the batch must still start. Routing a
// legitimate memo-overlay typo straight to Halt would turn every working
// deployment into a batch-blocker the moment any unrelated phase.json breaks.
```

### `go/internal/looppreflight/phase_routing_warnings_test.go:3` — above `import (`

```text
// phase_routing_warnings_test.go — RED contract for cycle-591's
// phase-routing-warning-escalation task.
//
// SCOUT-REPORT PIVOT (Rule 3, documented in test-report.md): scout's Task 1
// ("Fix memo phase routing collision") re-described a defect that cycles
// 547/554/563 already fixed in full — ValidateUserSpecWithCatalog,
// ApplyUserRouting(3-arg), and Catalog.Merge all already exempt/adopt the
// optional-builtin-name overlay shape (see validate_builtin_exempt_test.go /
// merge_builtin_exempt_test.go in internal/phasespec), and `go test
// ./internal/phasespec/...` is GREEN today. Scout's Task 2 (untracked binary
// in go/acs/cycle536 + .gitignore + staging guard) is likewise already fully
// shipped: `git ls-files go/acs/cycle536/` has no binary, .gitignore already
// has `go/acs/**/evolve`, and internal/binaryguard exists with passing tests.
// Writing RED tests against either already-GREEN surface would violate
// "RED is success" (a test that fails to fail proves nothing).
//
// The one genuinely unimplemented piece from scout's own "Beyond-the-Ask
// Hypotheses" section survives: every caller of phasespec's warning-producing
// functions (DiscoverUserSpecsFromRoots, Catalog.Merge, ApplyUserRouting) —
// go/cmd/evolve/cmd_cycle.go:386-399 and
// go/internal/core/routing_dispatch.go:155-166 — only fmt.Fprintf them to
// stderr and discard them; nothing in the codebase escalates a dropped/invalid
// user-phase spec into a structured, gate-visible signal (confirmed: no
// HealthSignal/LivenessCenter integration anywhere consumes these warnings).
//
// FIX CONTRACT (this cycle's new surface — undefined until Builder adds it,
// so this package fails to compile today; that compile failure IS the RED
// evidence, mirroring the phasespec package's own cycle-547/554 precedent):
//
//   - Options gains a new seam field, PhaseRoutingWarnings func() []string.
//     nil (the production default) wires to phasespec.MergedCatalog(projectRoot)
//     with the error swallowed (fail-open, matching DiscoverUserSpecs'
//     existing "missing dir → no specs" posture — a preflight gate must never
//     itself become the reason a batch can't start).
//   - A new check, checkPhaseRoutingWarnings(o resolved) CheckResult, named
//     "phase-routing-warnings": LevelPass when the seam returns no warnings;
//     LevelWarn (never LevelHalt — a dropped user phase is degraded-but-
//     runnable, the built-in spine is untouched) when it returns any,
//     joining them into Detail so the operator sees every one, not just a
//     count.
//   - Run() adds checkPhaseRoutingWarnings(o) to its checks slice, so an
//     invalid/dropped user-phase spec surfaces in the SAME accumulated,
//     gate-visible Result that every other readiness problem does — reusing
//     looppreflight's existing CheckResult/CheckLevel machinery
//     (never_duplicate_centralize_via_design_patterns) instead of inventing a
//     second WARN-collection type.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestRun_PhaseRoutingWarnings_WarningsPresent_Warn — the core
//     ask itself: a warning that used to vanish into stderr now surfaces in
//     the structured Result.
//   - Negative : TestRun_PhaseRoutingWarnings_NoWarnings_Pass — an empty seam
//     must not fabricate a warning (no-op-detector: a stub that always warns
//     would fail this).
//   - Anti-gaming (the critical negative) :
//     TestRun_PhaseRoutingWarnings_DoesNotHalt — many warnings must still be
//     LevelWarn, never LevelHalt. The cheapest gaming fake for "escalate
//     WARN to a health signal" is to route it straight to LevelHalt (trivially
//     "escalated"); that would turn every legitimate, working
//     memo-overlay-style deployment into a batch-blocking failure the moment
//     ANY unrelated user phase.json has a typo — a regression this test
//     exists to prevent.
//   - E2E      : TestRun_PhaseRoutingWarnings_DefaultUsesRealMergedCatalog
//     drives the actual production default (no injected seam) against a real
//     on-disk registry + a hijack-shaped user overlay and asserts the warning
//     reaches Result — proving the wiring end to end, not just the check
//     function in isolation.
```

### `go/internal/looppreflight/versioninventory.go:3` — above `import (`

```text
// versioninventory.go — CLI version capture and drift-detection cache.
//
// Motivation (cycle-308 inbox item 2026-06-12T16-08-42Z): claude moved
// 2.1.173→2.1.175 between batches despite autoUpdates:false. loop-preflight.json
// recorded no version strings, so the silent change was invisible. This file
// provides:
//
//  1. execVersion — replaceable seam for `<bin> --version`
//  2. captureVersionInventory — parses the version token from --version output
//  3. loadVersionCache / saveVersionCache — persist last-seen to .evolve/cli-versions.json
```

### `go/internal/looppreflight/versioninventory_amplified_test.go:3` — above `import (`

```text
// versioninventory_amplified_test.go — cycle-308 adversarial amplification
// for captureVersionInventory and checkCLIVersionDrift
// (cli-version-lifecycle-preflight task).
//
// Targets gaps in the TDD contract: empty bins list, simultaneous multi-CLI
// drift, and corrupted cache fail-open.
```

### `go/internal/looppreflight/versioninventory_amplified_test.go:35` — above `func TestVersionDrift_MultipleCLIsDriftSimultaneously(t *testing.T) {`

```text
// TestVersionDrift_MultipleCLIsDriftSimultaneously: the incident-replay extends
// to multiple CLIs changing in the same batch. The WARN detail must name ALL
// drifting binaries so the operator can see the full scope of the transition.
```

### `go/internal/looppreflight/versioninventory_test.go:3` — above `import (`

```text
// versioninventory_test.go — RED tests for cycle-308 task
// `cli-version-lifecycle-preflight` (inbox item 2026-06-12T16-08-42Z).
//
// Root cause: claude moved 2.1.173 → 2.1.175 BETWEEN batches despite
// autoUpdates:false (a staged-update-on-boot path the freeze setting doesn't
// gate). loop-preflight.json recorded no version strings, so the silent version
// change was invisible. This task adds:
//
//	(1) version inventory — capture `<bin> --version` for each tmux CLI into
//	    loop-preflight.json (Result.CLIVersions, persisted as "cli_versions");
//	(2) drift detection — persist last-seen versions to .evolve/cli-versions.json
//	    and WARN when an inventoried CLI's version changed vs the previous batch.
//
// New API this file pins (Builder implements):
//
//	var execVersion = func(bin string) (string, error)            (versioninventory.go)
//	captureVersionInventory(bins []string) map[string]string      (versioninventory.go)
//	checkCLIVersionDrift(o resolved) CheckResult                  (checks.go) — name "cli-version-drift"
//	Options.VersionInventory func() map[string]string             (looppreflight.go) — seam, default captures tmux bins
//	Result.CLIVersions map[string]string                          (looppreflight.go) — JSON "cli_versions"
//
// Helpers goodPipelineOptions/findCheck/fixedNow live in looppreflight_test.go.
```

### `go/internal/looppreflight/versioninventory_test.go:82` — above `func TestCLIVersionInventory_LandsInPreflight(t *testing.T) {`

```text
// TestCLIVersionInventory_LandsInPreflight: the captured inventory is exposed on
// Result.CLIVersions AND serialized into the persisted loop-preflight.json under
// "cli_versions" (the observability gap the incident exposed).
```

### `go/internal/looppreflight/versioninventory_test.go:109` — above `func TestVersionDrift_Fires_On_Synthetic_Transition(t *testing.T) {`

```text
// TestVersionDrift_Fires_On_Synthetic_Transition: the incident replay. The prior
// batch recorded claude 2.1.173; this batch sees 2.1.175 → the drift check WARNs
// and names both versions so the operator sees exactly what moved.
```
