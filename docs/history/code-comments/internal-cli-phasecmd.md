# Comment history: `internal/cli/phasecmd`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/cli/phasecmd/phase.go:47` — above `if strings.ToLower(args[0]) == "verify" {`

```text
// `evolve phase verify ...` is the deliverable-contract self-check (ADR-0034),
// not an in-process phase run — it reads no stdin JSON.
```

### `go/internal/cli/phasecmd/phase.go:52` — above `if strings.ToLower(args[0]) == "lint" {`

```text
// `evolve phase lint <name>` validates a phase descriptor against the unified
// schema (ADR-0035). Fail-open: warnings only, never blocks.
```

### `go/internal/cli/phasecmd/phase_inventory.go:15` — above `func RunPhaseInventory(args []string, _ io.Reader, stdout, stderr io.Writer) int {`

```text
// runPhaseInventory implements `evolve phase-inventory <subcommand>` — the
// phase counterpart of skill-inventory (ADR-0038). Exit codes:
//   - 0  success (cache hit or fresh build)
//   - 10 bad args / unknown subcommand
//   - 1  internal error
```

### `go/internal/cli/phasecmd/phase_lint.go:14` — above `func runPhaseLint(args []string, stdout, stderr io.Writer) int {`

```text
// runPhaseLint implements `evolve phase lint <name>` — a developer aid that
// checks an operator-authored phase descriptor against the unified phase
// descriptor (ADR-0035) and reports what the runtime WILL derive from it. It
// reuses the exact runtime path (DiscoverUserSpecsFromRoots → ValidateUserSpec
// → FromSpec) so the lint reflects production behavior, not a parallel schema.
//
// It is FAIL-OPEN by contract: every finding is a warning and the command
// always exits 0 (except a usage error: missing name → 10). Linting must never
// block a developer — the runtime gates (ValidateUserSpec floor, contract gate)
// are where enforcement lives.
```

### `go/internal/cli/phasecmd/phase_observer.go:19` — above `func RunPhaseObserver(args []string, _ io.Reader, stdout, stderr io.Writer) int {`

```text
// RunPhaseObserver is the `evolve phase-observer [--enforce] [--scope=...] <ws> <pgid> <cycle> <phase> <agent> [state]` subcommand.
// Ports the core stall-detection behavior of legacy/scripts/dispatch/phase-observer.sh.
// The composition root of ADR-0103 unit 12: it parses argv once, arms the
// SIGUSR1 shutdown, builds the subprocess's stderr-only Signal Center and
// hands the engine's host a Config whose accessor reaches it.
```

### `go/internal/cli/phasecmd/phase_observer.go:106` — above `cfg.StallPolicy = resolveStallPolicy(a.enforce)`

```text
// ADR-0044 C3: the chain-backed stall policy executes ONLY at enforce. For
// this standalone subcommand the operator's --enforce flag is the live signal
// (the IPC stage env key is an accepted fallback for an injecting parent).
// off/shadow/unset + no --enforce ⇒ nil policy ⇒ byte-identical legacy Enforce
// branch — shadow observability for stalls already exists via the INCIDENT
// events themselves.
```

### `go/internal/cli/phasecmd/phase_observer.go:113` — above `cfg.ProcessAlive = phaseobserver.DefaultProcessAlive`

```text
// R3.4: the process-liveness probe is wired unconditionally — it is
// deterministic ground truth (signal-0), not policy; nil in Run means
// probe-off (fixture Configs). The ACTION on a dead group stays
// policy/Enforce-gated; at shadow the INCIDENT is pure soak telemetry
// (pane echo ≠ liveness, cycles 274/277).
```

### `go/internal/cli/phasecmd/phase_observer.go:119` — above `cfg.Signals = func() *signalcenter.Center { return signals }`

```text
// ADR-0103 unit 12: the engine reports its own faults through the
// subprocess's Center (read late; nil = the Null Object).
```

### `go/internal/cli/phasecmd/phase_observer.go:159` — above `const envIPCPhaseRecoveryStage = "EVOLVE_" + "PHASE_RECOVERY_STAGE"`

```text
// envIPCPhaseRecoveryStage is the IPC key the parent orchestrator injects into
// the subprocess env to communicate the policy-resolved ADR-0044 stage.
// The split-const form keeps "EVOLVE_PHASE_RECOVERY" out of this file as a
// string literal (the retired key), which the flagreaders guard checks.
```

### `go/internal/cli/phasecmd/phase_observer.go:165` — above `func resolveStallPolicy(enforce bool) recovery.StallPolicy {`

```text
// resolveStallPolicy resolves the ADR-0044 chain-backed stall policy for the
// observer subprocess. It activates ONLY at enforce, from EITHER source:
//   - enforce: the operator's --enforce flag on the manual `evolve phase-observer`
//     command. This is the live signal for the standalone subcommand — the
//     orchestrator's auto-spawn path uses the in-process observer adapter
//     (adapters/observer.CoreAdapter, which reads its own RecoveryStage field),
//     not this subprocess, so nothing injects the IPC stage key here.
//   - envIPCPhaseRecoveryStage == "enforce": a parent that DOES inject the stage
//     (kept as an accepted IPC channel for forward-compat).
//
// Any other state (off, shadow, unset, typo, --enforce absent) ⇒ nil policy ⇒
// byte-identical legacy behavior. A typo never enables a kill-path.
```

### `go/internal/cli/phasecmd/phase_observer_root_test.go:3` — above `import (`

```text
// phase_observer_root_test.go — ADR-0103 unit 12 §6 tests 39-40: the manual
// subcommand's argument contract pinned verbatim (help text, flags, the three
// usage lines, the discarded Atoi errors — Q8) and the root's stderr-only
// Signal Center rendering the engine's codes on the SAME stderr the replaced
// [phase-observer] lines used.
```

### `go/internal/cli/phasecmd/phase_order.go:32` — above `projectRoot = paths.AbsoluteRoot("EVOLVE_PROJECT_ROOT", projectRoot, func(m string) {`

```text
// Absolutize a relative env root (cycle-119 class). The git/cwd
// fallbacks below already yield absolute paths.
```

### `go/internal/cli/phasecmd/phase_verify.go:22` — above `func runPhaseVerify(args []string, stdout, stderr io.Writer) int {`

```text
// runPhaseVerify implements `evolve phase verify <phase> --workspace DIR
// [--worktree DIR] [--evolve-dir DIR] [--json]`. It is the agent-callable
// self-check (the Deliverable Contract block tells each agent to run it before
// finishing) and shares its verifier with the host-side contract gate so the
// two run byte-identical logic. ADR-0034.
//
// Exit codes:
//
//	0  — deliverable well-formed
//	1  — confirmed contract violation (agent must fix)
//	10 — usage error (missing/unknown phase)
//	2  — ambiguity/infra (e.g. unreadable dir) — caller should fail OPEN
```

### `go/internal/cli/phasecmd/phase_verify.go:64` — above `resolver := phaseVerifyResolver()`

```text
// Resolve through the SAME merged catalog the host-side contract gate uses, so
// the agent's self-check and the gate agree on user/minted phases (no drift —
// ADR-0034). A catalog-load failure degrades to built-in-only resolution.
```

### `go/internal/cli/phasecmd/phase_verify.go:81` — above `needsSections, needsEffects := len(contract.ExplanationSections) > 0, len(contract.Effects) > 0`

```text
// Self-check ≡ gate (ADR-0034): the host gate judges the conditional
// explanation-documentation sections with the cycle's contract version and a
// declared effect under this cycle's processing/cycle-N/, so the self-check
// takes both from the persisted cycle state — or it would print OK on a
// report the gate blocks. Only contracts that declare either pay the read,
// and each consumer states its own consequence when the state is missing:
// the section check is skipped (0 = not active), while an effect cannot be
// judged at all and the verify aborts (fail open) rather than deciding blind.
```

### `go/internal/cli/phasecmd/phase_verify.go:125` — above `func verifyDeliverable(phase string, roots phasecontract.Roots, resolver phasecontract.Resolver) (deliverable.Result, er…`

```text
// verifyDeliverable runs the well-formedness checks, adding the ADR-0077
// documentation floor when — and only when — this invocation has a diff to
// judge: phase `build` with a `--worktree`. That is exactly the shape the
// host-side docs-floor reviewer sees, and the changed-path set comes from the
// same derivation it uses (core.ChangedWorktreePaths), so the agent's
// self-check and the gate cannot drift (ADR-0034).
//
// Fail-open everywhere else, byte-identical to before: no `--worktree` means no
// diff to classify, and the floor is build-scoped (ADR-0077) so no other
// phase's deliverable is taxed by it. A non-architecture-class diff never
// yields the violation, so ordinary cycles are unaffected.
```

### `go/internal/cli/phasecmd/phase_verify.go:153` — above `func withFrozenPinViolations(res deliverable.Result, worktree string) deliverable.Result {`

```text
// withFrozenPinViolations adds the cycle-644 reachability gate to a tdd
// verdict: every call site frozen by this deliverable (`doNotModifyTests:
// true`) that would require its pinning package to import a package already
// importing it back is a permanently unsatisfiable acceptance criterion, and
// cycle-644 proved that costs a whole cycle to discover from the build side.
// Catching it here — the self-check every tdd agent runs before handing off,
// sharing its verifier with the host-side contract gate (ADR-0034) — makes the
// check deterministic instead of a doc obligation the agent may forget
// (agents/evolve-tdd-engineer.md:132).
//
// Placement mirrors the ADR-0077 docs-floor precedent: phase-scoped and
// `--worktree`-scoped, because the worktree is the only place the pinned
// production files and their import graph exist. Fail-open on every infra
// ambiguity (unparseable handoff, no module, `go list` failure) — only a
// compiler-provable cycle turns a well-formed deliverable red.
```

### `go/internal/cli/phasecmd/phase_verify.go:187` — above `func phaseVerifyPhaseIO() config.Stage {`

```text
// phaseVerifyPhaseIO resolves the EVOLVE_PHASE_IO rollout stage the SAME way the
// host gate does (config.Load over the phase registry + env), so the agent's
// self-check and the gate apply identical PhaseIO-gated checks — the package's
// no-drift invariant (ADR-0050 §3.8). A registry that cannot be read degrades to
// env + code defaults, never a hard failure.
```

### `go/internal/cli/phasecmd/phase_verify_docsfloor_test.go:13` — above `const docsFloorBuildReport = "## Changes\n- go/internal/policy/policy.go\nVerdict: PASS\n"`

```text
// RED contract for cycle-1150 / wire-docsfloor-verify-cli.
//
// `evolve phase verify build` is the exact self-check every phase prompt's
// Deliverable Contract tells the agent to run before declaring done. It calls
// deliverable.VerifyWithStage, which never sees the build's diff — so the
// ADR-0077 blocking-grade classifier (deliverable.VerifyBuildWithChangedPaths,
// added cycle-1144) has zero production callers and an architecture-class build
// with no docs delta passes the agent's own self-check.
//
// These tests drive the REAL CLI entry point (runPhaseVerify) over a REAL git
// worktree, so they assert on exit codes and stderr the operator actually sees
// — not on an internal seam that is already green in isolation.
```

### `go/internal/cli/phasecmd/phase_verify_docsfloor_test.go:186` — above `func TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor(t *testing.T) {`

```text
// TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor — AC3, the scope guard.
// The docs floor is a BUILD-phase contract (ADR-0077). Threading the changed
// path set through verify must not leak the floor into other phases'
// deliverables, even when the same architecture-class worktree is supplied.
```

### `go/internal/cli/phasecmd/phase_verify_effects_test.go:3` — above `import (`

```text
// phase_verify_effects_test.go — ADR-0100 slice 2: the agent self-check
// judges a declared EFFECT exactly as the host gate does (self-check ≡ gate,
// ADR-0034). It takes the cycle from the persisted cycle state the way it
// already takes the explanation-documentation version, and the inbox from
// the project root the resolver already uses.
```

### `go/internal/cli/phasecmd/phase_verify_frozenpin_test.go:10` — above `func frozenPinWrite(t *testing.T, root, rel, body string) {`

```text
// Permanent (non-acs) regression guard for the cycle-644 reachability gate
// wired into `evolve phase verify tdd` (cycle-1238, inbox item
// tdd-structural-test-reachability-probe).
//
// The acs predicates for this cycle vanish with the cycle; without these the
// gate could silently rot in a later refactor and `go test ./...` would not
// notice. Precedent: phase_verify_docsfloor_test.go (cycle-1150). Like that
// one, these drive the REAL CLI entry point (runPhaseVerify) over a real Go
// module, so they assert on the exit code and stderr an operator actually sees
// — a gate reachable only from a unit test is dead code.
```

### `go/internal/cli/phasecmd/phase_verify_frozenpin_test.go:33` — above `func frozenPinWorktree(t *testing.T) string {`

```text
// frozenPinWorktree builds a throwaway worktree whose go/ subdirectory is a
// real, resolvable module carrying both shapes the gate must tell apart:
// storage imports core (so pinning storage.UpdateStateMap( inside a core file
// is the cycle-644 shape), and leafutil imports nothing (so pinning
// leafutil.Helper( inside a core file is perfectly buildable).
```

### `go/internal/cli/phasecmd/phase_verify_frozenpin_test.go:88` — above `func TestPhaseVerifyTDD_FrozenPinCycle_Exit1(t *testing.T) {`

```text
// TestPhaseVerifyTDD_FrozenPinCycle_Exit1 is the crux rejection contract: the
// cycle-644 shape is a CONFIRMED violation on the live CLI path, before the
// build phase ever starts, with the stable code and all three identifiers an
// agent needs to act on it.
```

### `go/internal/cli/phasecmd/phase_verify_test.go:15` — above `func runVerify(t *testing.T, args ...string) (int, string, string) {`

```text
// Layer 3 CLI: `evolve phase verify` — the agent-callable self-check. Same
// verifier the host gate uses (go/internal/deliverable). ADR-0034.
```

### `go/internal/cli/phasecmd/phase_verify_test.go:135` — above `func TestPhaseVerify_FailureContextPhaseIO_RespectsStage(t *testing.T) {`

```text
// TestPhaseVerify_FailureContextPhaseIO_RespectsStage — Phase 3.8 (ADR-0050):
// the self-check runs the SAME PhaseIO-gated logic the host gate does (the
// package's no-drift invariant). A build report that self-reports FAIL without a
// structured failure block is a confirmed violation (exit 1) at
// EVOLVE_PHASE_IO=enforce (now the default since the 3.10 cutover), and dormant
// (exit 0) only when explicitly rolled back to off.
```

### `go/internal/cli/phasecmd/phases_coherence_test.go:1` — above `package phasecmd`

```text
// cmd_phases_coherence_test.go — RED tests for migration step 4
// (projection-generation-and-meta-gates; cycle-239 retry of cycle-238, test
// contract salvaged verbatim from 878df21 per intent non-goal "salvage,
// don't rewrite"). Three CLI surfaces (architecture blueprint B3/B8/B9/B12):
//
//  1. `evolve phases validate [--strict-provenance]` — profiles missing
//     `generated_from` emit `WARN: profile <name> missing generated_from`
//     (eval profile-provenance-field C4 greps `missing.*generated_from`);
//     advisory exit 0 by default, --strict-provenance ⇒ exit 2. Profile dir
//     resolution: EVOLVE_PROFILE_DIR env → paths default
//     (<project>/.evolve/profiles).
//  2. `evolve phases check-coherence [--strict]` — persona tools: frontmatter
//     vs profile allowed_tools; WARN advisory exit 0, --strict ⇒ exit 2;
//     EVOLVE_PERSONA_OVERRIDE="<path>:<name>" substitutes one persona file
//     (eval persona-tools-coherence-gate C4).
//  3. `evolve phases check-artifact-coherence [--strict]` — persona
//     output-format: artifact basename vs profile output_artifact basename.
//
// Layout: agents at <project>/agents/evolve-<name>.md, profiles at
// <project>/.evolve/profiles/<name>.json, project = EVOLVE_PROJECT_ROOT.
```

### `go/internal/cli/phasecmd/phases_coherence_test.go:124` — above `root := newCoherenceProject(t)`

```text
// --profile-dir flag routes validate to the alternate profiles dir;
// the unstamped profile there must be detected even though the
// project-root profiles dir is clean.
// (Renamed from TestRunPhases_ProvenanceHonorsProfileDirEnv — cycle-16
// migrates EVOLVE_PROFILE_DIR to the --profile-dir CLI flag.)
```

### `go/internal/cli/phasecmd/phases_coherence_test.go:207` — above `root := newCoherenceProject(t)`

```text
// --persona-override <path>:<name> substitutes the named persona's file.
// On-disk pair is clean; the override adds a contradicting tool → WARN.
// (Renamed from TestRunPhases_CheckCoherencePersonaOverrideEnv — cycle-16
// migrates EVOLVE_PERSONA_OVERRIDE to the --persona-override CLI flag.)
```

### `go/internal/cli/phasecmd/phases_coherence_test.go:254` — above `writeCoherencePersona(t, root, "plan-review",`

```text
// I-3(d) incident replica.
```

### `go/internal/cli/phasecmd/phases_create.go:1` — above `package phasecmd`

```text
// cmd_phases_create.go implements `evolve phases create` — the registration
// path of the phase plugin system (ADR-0038). It is the SINGLE enforcement
// point for conversational phase creation: any LLM CLI (claude/codex/gemini)
// designs a spec, pipes it here, and self-corrects from the machine-parseable
// JSON envelope this command prints to stdout. The thin `phase-create` skill
// is documentation around this command, not a second implementation.
```

### `go/internal/cli/phasecmd/stallpolicy_test.go:5` — above `func TestResolveStallPolicy_EnforceFlagActivates(t *testing.T) {`

```text
// resolveStallPolicy activates the ADR-0044 chain-backed stall policy ONLY at
// enforce, from either the operator's --enforce flag (the live signal for the
// standalone `evolve phase-observer` subcommand) or an injected IPC stage key.
// These tests pin that contract — the behavioral signal the deterministic gates
// (build/vet/test, flagreaders) structurally cannot provide, which is why a
// prior cycle's rename to a never-injected env key silently disabled the manual
// --enforce path and shipped green.
//
// All cases force EVOLVE_PROJECT_ROOT to a temp dir so policy.Load fails over to
// a zero Policy (hermetic — no dependency on a real policy.json).
```
