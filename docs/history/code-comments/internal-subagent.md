# Comment history: `internal/subagent`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/subagent/bridgeadapter.go:18` — above `func driverExists(cli string) bool {`

```text
// bridgeadapter.go is the host's bridge touch (ADR-0103 unit 16): the
// gobridge-backed defaults of the run path — the driver-presence check the
// dispatcher's AdapterExists port is bound to and the production Adapter that
// dispatches through the in-process engine carrying the root's Signal Center
// — plus the Center-less projection ValidateProfile's func-shaped seam
// defaults to. engine.go/autorespond.go are untouched; the one field this
// file sets on gobridge.Deps is Signals.
```

### `go/internal/subagent/bridgeadapter.go:35` — above `func execAdapterDeps(env map[string]string) gobridge.Deps {`

```text
// defaultExecAdapter dispatches the subagent through the in-process Go bridge
// instead of shelling `bash <cli>.sh`. The bridge owns the same contract the
// bash adapter had: it materializes the prompt, dispatches the driver, and
// writes the artifact at ArtifactPath. A VALIDATE_ONLY=1 entry in env is
// honored by the bridge's launch path (it prints the resolved config and
// returns ExitOK without invoking an LLM), so ValidateProfile's dry-validate
// keeps working (same ExitOK contract; no LLM invoked).
//
// adapterPath is retained for the injectable ExecAdapter seam (tests stub the
// whole function), but the default no longer reads the .sh file — it reads
// RESOLVED_CLI from env and projects it onto a registered driver via
// bridge.DriverFor.
// execAdapterDeps builds the gobridge.Deps for the subagent composition
// root, wiring TokenResolver via tokenusage.DefaultResolver against the
// env's HOME — the same configRoot-resolution convention as
// internal/adapters/bridge's productionEngineDeps (env["HOME"] falling back
// to os.Getenv("HOME"), joined with ".claude"). The two production
// composition roots (adapters/bridge, this package) share the single
// tokenusage.DefaultResolver helper, each resolving configRoot identically —
// and, since F27, the single policy.BridgeRecoveryStages accessor for the two
// ADR-0044 recovery dials, so a dead pane fast-fails on this root exactly as
// on the cycle root (it built Deps with neither dial, pinning both to shadow).
```

### `go/internal/subagent/bridgeadapter.go:84` — above `func execAdapterDepsWith(env map[string]string, signals *signalcenter.Center) gobridge.Deps {`

```text
// execAdapterDepsWith is execAdapterDeps carrying the root's Signal Center —
// the ONE bridge touch of unit 16 (ADR-0103): the engine's own bridge.warning,
// bridge.tripwire and pane.liveness producers report into the same Center the
// dispatcher does instead of into nil. nil stays the Null Object.
```

### `go/internal/subagent/cacheprefix.go:36` — above `func WriteCachePrefix(req CachePrefixRequest, opts CachePrefixOptions) error {`

```text
// WriteCachePrefix renders a deterministic markdown cache-prefix file used by
// sibling fan-out workers in the same batch. Same cycle+workspace+agent must
// produce byte-identical bytes — no timestamps, no randomness — so the
// Anthropic prompt cache reuses the prefix across workers.
//
// Mirrors _write_cache_prefix in legacy/scripts/dispatch/subagent-run.sh
// (v8.23.0 Task C). Goal is extracted from orchestrator-prompt.md if present
// (line matching `^goal:\s*`); cycle-state condensed to a single-line summary
// of phase + active_agent + completed_phases.
```

### `go/internal/subagent/conformance_registry_test.go:96` — above `func TestAgentRoles_DerivedFromPhaseContractRegistry(t *testing.T) {`

```text
// TestAgentRoles_DerivedFromPhaseContractRegistry pins the cycle-1145
// required-roles-ssot refactor from both sides: the allow-list must COVER every
// dispatchable registry agent (the drift that let "router" fall out of it), must
// RETAIN the profile-backed roles the registry does not know, and must not
// over-reach onto NoArtifact phases like "ship" (which has no profile).
```

### `go/internal/subagent/contract.go:9` — above `type VerifyInput = subagentrun.VerifyInput`

```text
// contract.go is the host's view of the one verification ladder: "is this
// dispatched-agent artifact valid?" lives in internal/subagent/subagentrun
// (ADR-0103 unit 16) and every dispatch path — the run path, the Runner
// twin's classify, the fan-out parent's per-worker verifier — reaches it
// through these aliases and facades, so the three copies that once drifted
// apart cannot come back. (The lighter `evolve subagent check-token` probe in
// checktoken.go is intentionally a separate exists+token contract.)
```

### `go/internal/subagent/dispatchparallel_test.go:457` — above `func TestDispatchParallel_SilentCLIIsAnError(t *testing.T) {`

```text
// TestDispatchParallel_SilentCLIIsAnError pins the cycle-1262 replacement for
// the old `cli = "claude"` default. dispatch-parallel is a passthrough — it
// never chooses the CLI its workers run — so a profile that declares none is
// unresolvable and must fail loudly here exactly as it does in Run and
// ValidateProfile, rather than being tiered against an invented default.
```

### `go/internal/subagent/helpers.go:169` — above `const ledgerZeroSeed = subagentrun.LedgerZeroSeed`

```text
// The chained-append primitives are the unit-16 leaf's (ADR-0103): the fan-out
// writer above keeps its own append skeleton beside the run path's — the
// duplicated belief "a chained ledger append", named, folded by the fan-out
// unit (follow-up 16-1).
```

### `go/internal/subagent/recovery_dials_test.go:11` — above `func TestExecAdapterDeps_CarriesThePolicyRecoveryDials(t *testing.T) {`

```text
// TestExecAdapterDeps_CarriesThePolicyRecoveryDials (F27 architecture review,
// HIGH): the `evolve subagent run` root builds its engine Deps directly, so it
// must carry both ADR-0044 recovery dials from the dispatched project's
// policy.json through the one accessor every setter-less root shares — before
// the fold it set neither, pinning the fatal-pane fast-fail to shadow here
// whatever policy said (a dead pane idled out the full backstop on this path).
```

### `go/internal/subagent/run.go:21` — above `type RunRequest struct {`

```text
// run.go is the unit-16 seam (ADR-0103): the `evolve subagent run` execution
// path lives in internal/subagent/subagentrun; this file keeps the exported
// RunRequest / RunOptions / RunResult bag the root and the by-name test
// binders spell, the ONE wired construction of the dispatcher from those
// options, the request/result projections, and the facades the fan-out
// dispatcher, the Runner twin, the validate pipeline and the host tests keep.
```

### `go/internal/subagent/run.go:79` — above `AdapterExists func(cli string) bool`

```text
// AdapterExists reports whether the resolved cli has a registered bridge
// driver. Since ADR-0103 unit 16 it receives the CLI, not the vestigial
// <AdaptersDir>/<cli>.sh path (the func type is unchanged; the production
// default is driverExists — nothing on the run path decodes a file name).
```

### `go/internal/subagent/run.go:96` — above `Signals *signalcenter.Center`

```text
// Signals is the root's Signal Center (ADR-0103 unit 16): the dispatcher's
// BRIDGE_SUBAGENT_* warnings and, through the exec seam, the bridge
// engine's own producers report into it. nil is the Null Object.
```

### `go/internal/subagent/run.go:136` — above `func buildAgentRoles() []string {`

```text
// buildAgentRoles derives the allow-list as the UNION of (a) every
// phasecontract-registered agent that actually produces an LLM deliverable and
// (b) nonRegistryRoles. Before cycle-1145 this was a second hand-typed slice
// beside the registry and had already drifted: "router" is registered (and ships
// .evolve/profiles/router.json) yet was not dispatchable.
//
// NoArtifact phases are excluded: "ship" is registered but is a native
// host-side phase with no profile, so accepting it would break the
// role↔profile conformance invariant (TestAgentRoles_EveryRoleHasProfile).
// Output is sorted so the derived regex — and every test that iterates the
// list — is deterministic despite Contracts()' unordered map iteration.
```

### `go/internal/subagent/run.go:173` — above `func Run(ctx context.Context, req RunRequest, opts RunOptions) (RunResult, error) {`

```text
// Run is the `evolve subagent run` execution path: argument validation,
// worker-name parsing, profile load, cli/model resolution, the driver check,
// model tier resolution, artifact placement, the challenge token and git
// state, the prompt (PROMPT_FILE_OVERRIDE or stdin) assembled into the v2
// cache-prefix envelope with the adversarial auditor framing, the adapter
// exec with VALIDATE_ONLY=0 and the full env, artifact verification (exists,
// fresh <5min, token-bearing) and the kind="agent_subprocess" ledger entry.
// The path is internal/subagent/subagentrun (ADR-0103 unit 16); this facade
// fills the production defaults, builds the ONE wired dispatcher and projects
// the result.
```

### `go/internal/subagent/run.go:302` — above `func capabilityTier(m capability.Manifest) string {`

```text
// capabilityTier maps Manifest support flags to the v8.51.0 quality_tier
// label used by ledger entries (the fan-out dispatcher's spelling).
```

### `go/internal/subagent/run_project_root_env_test.go:3` — above `import (`

```text
// run_project_root_env_test.go — the subprocess sees the plane's root.
//
// core/phase.go documents PhaseRequest.ProjectRoot as "what a subprocess sees
// as EVOLVE_PROJECT_ROOT", and every phase runs with cwd = its cycle worktree
// (CB.1). Nothing exported the variable. Any `evolve` subcommand the agent
// runs — `inbox-mover claim` first among them — resolves its root through
// cmdutil.EnvOrCwd, so it fell back to the worktree, whose .evolve/inbox is a
// git-tracked SNAPSHOT of the plane's queue. Batch cycle 1631 (2026-09-12)
// printed `[inbox-mover] claimed:` against that copy while the plane's item
// never moved and its ledger recorded no claim; the cycle then FAILed with
// its lane's item still queued.
```

### `go/internal/subagent/run_seam_test.go:3` — above `import (`

```text
// run_seam_test.go — ADR-0103 unit 16 step 3: the seam between the host's
// RunOptions bag and the subagentrun dispatcher — one construction, one
// projection each way, the Center forwarded, every facade projecting the leaf.
```

### `go/internal/subagent/run_unit16_pins_test.go:3` — above `import (`

```text
// run_unit16_pins_test.go — ADR-0103 unit 16 step 1: the behavioural pins the
// `evolve subagent run` execution path had never carried, written GREEN on the
// pre-extraction code (8e8f080f) and each proven red against its named mutant
// before the path moved into internal/subagent/subagentrun. They drive the
// production entry Run, so after the move they are the strangler proof: the
// facade over the leaf is byte-identical on the prompt, the adapter env, the
// Warns channel, every error text, the verdict wiring and the ledger line.
```

### `go/internal/subagent/run_unit16_pins_test.go:27` — above `const unit16Goldens = "subagentrun/testdata"`

```text
// unit16Goldens is where the pre-extraction goldens live (captured by a
// throwaway writer on 8e8f080f, paths templated as {WS} / {ROOT} / {WORKTREE}).
```

### `go/internal/subagent/subagent.go:1` — above `package subagent`

```text
// Package subagent ports the orchestration loop from
// legacy/scripts/dispatch/subagent-run.sh into Go. Its job is to:
//
//  1. Load the agent profile JSON (via profiles.Loader)
//  2. Generate a 16-hex challenge token (provenance proof)
//  3. Capture git state (HEAD + tree-diff sha256) for audit-binding
//  4. Compose the prompt — caller supplies body, subagent prepends the
//     challenge-token + artifact-path context block
//  5. Call core.Bridge.Launch() (the bridge binary internally handles
//     sandbox-exec/bwrap wrapping per profile.sandbox; the Go runner
//     does not double-wrap)
//  6. Verify the artifact: exists, non-empty, age < 300s, contains
//     challenge token
//  7. Append a kind=agent_subprocess ledger entry
//
// Out of scope for v11.5.0 M2 (deferred to later milestones):
//   - parallel sibling fan-out (cmd_dispatch_parallel in bash)
//   - phase-observer spawn/reap
//   - cache-prefix v2 prompt rewriter
//   - dispatch-plan log emission
//   - fast-fail consecutive-failure counter
//
// These remain available via bash subagent-run.sh while the Go path
// expands.
```

### `go/internal/subagent/subagent.go:266` — above `RunID: core.RunIDFromWorkspace(req.Workspace),`

```text
// Cycle-1571 H1: ship's binding lookup is run-scoped, so an entry with
// no run identity can never be bound. Resolved from the run workspace
// because this runner may execute out of the orchestrator's process.
```

### `go/internal/subagent/subagent.go:336` — above `func composePrompt(body, token, artifactPath, agent string, cycle int) string {`

```text
// composePrompt prepends the CHALLENGE TOKEN context block to the user
// prompt. Order matches subagent-run.sh:803-818 — token first, artifact
// path second, then the task prompt body.
//
// v11.5.2 fix: section markers use `## ... ##` not `--- ... ---`.
// claude CLI 2.1.149's flag parser rejects any prompt value whose first
// argv-character is `-` ("unknown option" error), and the bridge
// driver passes the composed prompt as `-p "$prompt_content"` — so a
// leading `--` prefix tripped the parser. Switching to `## ... ##`
// keeps the visual section-marker convention while avoiding the
// flag-parser collision. The fix is structural: any future prompt
// content the runner prepends must not start with `--`.
```

### `go/internal/subagent/subagent_test.go:446` — above `func TestComposePrompt_NoLeadingDashes(t *testing.T) {`

```text
// TestComposePrompt_NoLeadingDashes is the v11.5.2 regression guard:
// the prompt must NEVER start with `--`. claude CLI 2.1.149's flag
// parser rejects any prompt value whose first argv-character is `-`,
// and the bridge driver passes the prompt as `-p "$content"`. A
// leading `--` would be reparsed as a flag.
```

### `go/internal/subagent/tokenresolver_wiring_test.go:3` — above `import (`

```text
// tokenresolver_wiring_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96).
//
// Confirmed bug: `grep -rn TokenResolver go/internal/subagent/validateprofile.go`
// returns zero non-test hits — defaultExecAdapter's `gobridge.NewEngine(
// gobridge.Deps{Env: env})` (validateprofile.go:347) never sets
// TokenResolver, so every real (VALIDATE_ONLY=0) subagent dispatch through
// this composition root also gets silent zero telemetry — the second half of
// the same bug fixed on the adapters/bridge side (see
// internal/adapters/bridge/tokenresolver_wiring_test.go).
//
// Fix contract (Builder implements): a new unexported function
//
//	func execAdapterDeps(env map[string]string) gobridge.Deps
//
// that sets TokenResolver: tokenusage.DefaultResolver(configRoot) — SAME
// configRoot-resolution convention as productionEngineDeps in
// internal/adapters/bridge (env["HOME"] falling back to os.Getenv("HOME"),
// joined with ".claude") — the ONE shared tokenusage.DefaultResolver helper,
// two composition roots, both resolving configRoot the same way.
// defaultExecAdapter must build its gobridge.Deps via this function in place
// of the current `gobridge.Deps{Env: env}` literal. execAdapterDeps is
// undefined today, so this package fails to compile — the intended RED
// signal. DO NOT modify this file; implement production code only.
```
