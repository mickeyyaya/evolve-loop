# Agent-Bridge Dispatch Contract

> Status: living doc for the agent-bridge hardening program (plan: `sleepy-wandering-salamander`).
> Covers the invariants that make every subagent/phase-agent dispatch go through the one bridge
> process, safely and at scale. B1–B3 landed; B4–B6 will extend this doc.

## Request

The agent-bridge (`evolve subagent run` / `evolve subagent dispatch-parallel`) must be the **only**
path that dispatches a phase agent or subagent — the in-process Agent/Task tool is banned. Three
structural weaknesses surfaced while fixing the sandbox-confinement bug:

1. The bridge-only invariant was not enforced — a `LEGACY_AGENT_DISPATCH` escape hatch fell back to
   the in-process tool.
2. Recursion (fan-out workers re-invoking the binary) was implicit and unbounded, and its
   sandbox-coherence under nesting was unproven.
3. The output contract / verification / concurrency guarantees were under-tested.

## B1 — Bridge-only dispatch invariant

**Problem.** `LEGACY_AGENT_DISPATCH=1` made `evolve subagent run` print a *"fall back to in-process
Agent tool"* instruction plus a `LEGACY_DISPATCH` stdout token. That token was a **contract signal**
(not a Go branch) telling the LLM orchestrator to dispatch in-process — the exact path that must be
banned.

**Alternatives considered.**
- *Silently ignore the flag (always bridge).* Rejected — a silent change (Rule 3); a caller setting
  the flag would get surprising behavior with no signal.
- *Loud, default-off deprecation with a removal date.* Reasonable, but leaves the banned path
  reachable during the deprecation window.
- *Hard error (chosen).* Setting the flag fails loudly at the single `Run()` chokepoint. No dispatch
  can reach the in-process tool, and the failure is self-describing.

**Design.** `ErrInProcessDispatchBanned` sentinel + one `enforceBridgeOnly(legacyRequested)` predicate,
called early in `Run()` (before profile/CLI resolution) — the one chokepoint all dispatch (single,
fan-out, recursive) funnels through. `RunResult.LegacyDispatch` and the `cmd_subagent` fallback
block/token were deleted. A source-guard test pins that no in-process fallback signal (or
"honored-env" advertisement of the retired flag) can regress.

The agent allow-list was collapsed to an `agentRoles[]` SSOT; `agentRolePattern` derives from it and
the all-roles invariant test iterates it, so a new role is added in exactly one place.

## B2 — Recursion through the same bridge

**Problem.** A fan-out worker recurses by running `evolve subagent run <agent>-worker-<subtask>` —
re-entering the same binary's `Run()` (good: never the in-process tool). But (a) the command was
built inline and untested, (b) the recursion was unbounded, and (c) a recursive agent is
*nested-Claude inside nested-Claude*, so the Part A sandbox-confinement SSOT must stay coherent at
every depth.

**Design.**
- **Tested recursion command.** Extracted pure `buildWorkerRecursionCommand()` — composes
  `PROMPT_FILE_OVERRIDE=… CLAUDECODE_TYPE= EVOLVE_DISPATCH_DEPTH=<n+1> <bin> subagent run
  <role>-worker-<subtask> <cycle> <ws>`. A conformance test proves it re-enters the bridge dispatch
  path for every worker.
- **Depth cap.** `EVOLVE_DISPATCH_DEPTH` threads recursion depth across each bridge re-entry
  (`ReadDispatchDepth` parses it; absent ⇒ 0). `enforceDispatchDepth` rejects past `maxDispatchDepth`
  (3) at both the `Run()` and `DispatchParallel()` entry points (one predicate). A fan-out loop can't
  recurse unboundedly.
- **Sandbox coherence by construction.** The worker command clears `CLAUDECODE_TYPE` — a recursive
  child is, by definition, **not** the top-level host. If a stale `CLAUDECODE_TYPE=host` leaked into a
  child, `sandbox.DetectNested` would return false → `ShouldWrap` would attempt the inner OS sandbox →
  on macOS `sandbox_apply()` EPERMs and hangs the REPL (the Part A failure mode). Since `claude`
  self-sets `CLAUDECODE` in every process, clearing the host marker keeps `DetectNested=true` at any
  depth → no inner wrap.

**Why not also strip/inject `CLAUDECODE` itself?** Unnecessary — `claude` sets it in every claude
process, so the nested signal is already present at every depth. Only the *host* marker can wrongly
suppress nesting, so only it is cleared.

## B3 — Output-contract verification SSOT + tunable progress-aware deadline

Two single-source-of-truth fixes: one for the *verdict* a dispatched artifact earns, one for the
*deadline* it is judged within.

### B3a — Artifact verification SSOT

**Problem.** The "is this artifact valid?" ladder (stat → freshness < 5 min → readable/non-empty →
token-bearing → exec status ⇒ `PASS` / `FAIL` / `INTEGRITY_FAIL`) existed in **three** Go copies that
could drift: `classifyArtifact` (`run.go`, single dispatch, bare verdict), `(*Runner).classify`
(`subagent.go`, same ladder + `[]core.Diagnostic`), and the bash `verify_artifact()` they were ported
from. A fourth, weaker copy — the fan-out aggregator's per-worker gate — is unified separately in B5
(it must skip freshness, see below).

**Alternatives considered.**
- *Keep the seams-blob signature (`Verify(opts RunOptions, …)`).* Rejected — couples the verdict to
  the whole dispatch-options struct and to the filesystem, so the ladder can only be tested with temp
  files + `os.Chtimes` (no `t.Parallel`).
- *One pure verdict-only function, callers re-emit their own diagnostics.* Rejected — re-introduces the
  per-call-site duplication for the diagnostic wording (the exact drift we're removing).
- *Pure core + one I/O adapter (chosen).* A pure `Verify(VerifyInput) VerifyResult` holds the ladder
  and the diagnostics; one `VerifyArtifact(stat, read, now, …)` adapter does the filesystem gather and
  delegates. Both call sites collapse onto the adapter.

**Design (the projection pattern).**
- `Verify(in VerifyInput) VerifyResult` — pure: no field touches the filesystem or the wall clock
  (`MTime`/`Body`/`Now`/`MaxAge` are passed in). The verdict ladder and the diagnostic wording live
  here, once. Table-tested with literals → every case is `t.Parallel`.
- `VerifyArtifact(stat, read, now, path, token, exitCode, execErr)` — the single I/O-bearing entry
  point. Gathers the stat (and, only on stat success, the body — preserving the "missing artifact is
  judged before any read" short-circuit), then calls `Verify` against `ArtifactMaxAge`.
- `run.go` step 13 calls `VerifyArtifact(…).Verdict`; `(*Runner).classify` calls it and returns
  `(.Verdict, .Diagnostics)`. `classifyArtifact` is deleted. Verdict and diagnostic bytes are
  unchanged — the four legacy FS tests retarget onto `VerifyArtifact`, and the new contract table adds
  the branches they never covered (token mismatch, exit≠0 ⇒ `FAIL`, happy `PASS`, the leading
  bridge-error diagnostic that does **not** short-circuit integrity).

The `evolve subagent check-token` CLI probe (`checktoken.go`) is deliberately **not** folded in: it is
a lighter exists+token check for operators, not a dispatch verdict, and adding freshness would change
that contract.

### B3b — Tunable deadline: env-resolution SSOT

**Problem.** The artifact-wait deadline is already progress-aware — the `deterministicReviewer` extends
**unbounded** while the pane shows substantive new content (`Progressed`) and up to `maxExtends` while
it is busy-but-quiet (`Busy`), so a thorough agent is not killed mid-write (the cycle-311/312 and
254/255 fixes). What was broken is the **tunable** part: the `EVOLVE_ARTIFACT_TIMEOUT_S` override
silently did nothing. Root cause is a second SSOT split — the *subprocess* env is built by
`driverEnv = os.Environ() + deps.Env`, but the *in-process* int reads went through
`lookupEnv → deps.LookupEnv` **only**, never consulting `deps.Env`. An override carried on the launch
(in `deps.Env`, the same place all other request env lives) reached the inner CLI but was invisible to
the deadline. `envInt`'s doc-comment even *claimed* it read "the Deps env overlay" — it didn't.

**Alternatives considered.**
- *Fold the `LaunchArgs` `env` arg into `deps.Env`.* Broader blast radius — would export every
  request env var to the subprocess; not needed to fix the knob.
- *Make the env override outrank an explicit `cfg.ArtifactTimeoutS`.* Rejected — livesmoke and
  integration tests set `cfg.ArtifactTimeoutS` deliberately (1–120 s); a stray global `=600` must not
  make them hang.
- *Make `lookupEnv` consult `deps.Env` first (chosen).* The minimal SSOT fix: the in-process env view
  now matches the subprocess env view, the lying doc-comment becomes true, and the existing precedence
  (`cfg.ArtifactTimeoutS` > `EVOLVE_ARTIFACT_TIMEOUT_S` > 300 default) is preserved.

**Design.** `lookupEnv(deps, key)` resolves `deps.Env[key]` → `deps.LookupEnv` → `os.LookupEnv`. Every
`envInt` consumer (the deadline interval and `EVOLVE_ARTIFACT_MAX_EXTENDS`) now honors the launch
overlay. Tests pin: the overlay is consulted and wins over `LookupEnv`, falls through when absent, the
override reaches the recorded `StopEvent.IntervalS` end-to-end, and `cfg.ArtifactTimeoutS` still wins.

## Invariants (enforced by tests)

| # | Invariant | Enforced by |
|---|-----------|-------------|
| I1 | No dispatch reaches the in-process Agent tool | `enforceBridgeOnly` @ `Run()`; source-guard test |
| I2 | Agent allow-list is single-sourced | `agentRoles[]` → `agentRolePattern`; all-roles test |
| I3 | Fan-out workers recurse via `subagent run` | `buildWorkerRecursionCommand` + conformance test |
| I4 | Recursion depth is bounded | `EVOLVE_DISPATCH_DEPTH` + `enforceDispatchDepth` (cap 3) |
| I5 | Recursive children stay nested (no inner wrap) | `CLAUDECODE_TYPE` cleared per worker; sandbox test |
| I6 | Artifact verdict is single-sourced | `Verify`/`VerifyArtifact` (`contract.go`); contract table |
| I7 | In-process env view == subprocess env view | `lookupEnv` consults `deps.Env`; overlay + e2e tests |

## Out of scope (later phases)

B4 (conformance suite), B5 (concurrency `-race` stability + aggregator per-worker verification using
the B3 SSOT with freshness skipped), B6 (registry-driven allow-list + session reaping).
