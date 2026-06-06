# Incident Report & Remediation: Relative-Path ExitArtifactTimeout + Cross-CLI Trust Bypass — Cycle 119

**Date:** 2026-05-28 | **Severity:** CRITICAL (a `/evolve-loop` launched via the documented slash-command path could not clear *any* worktree phase → no cycle could build or ship) | **Status:** Issue 1 FIXED + shipped `80f4206` (main, CI green); the **Issue-1 fix validated on cycle 120** — the tdd worktree phase (where 119 died) cleared cleanly, no `exit=81` — though that cycle then hit a *separate* failure at build-planner (Issue 4). Issues 2–4 OPEN with fix designs below.

**Validation result (cycle 120, full-phase, fixed binary):** intent→scout→triage→**tdd ALL completed** (tdd = the worktree phase where 119 died; it wrote `test-report.md` to the now-absolute workspace, the bridge detected it at the matching absolute path, `bridge.Launch` returned cleanly). Per-phase: all `claude-tmux`; intent/tdd/build-planner=opus, scout/triage=sonnet. The cycle then FAILED at **build-planner** with a *new* `exit=81` (see Issue 4) — unrelated to the relative-path root cause.

---

## Part 1: What Happened

**Context.** An all-phases verification run (`/evolve-loop`, every optional phase enabled, goal = "optimize token usage") was launched as **cycle 119** to (a) exercise the full pipeline and (b) do real token-optimization work. Intent → Scout → Triage ran; the cycle then **aborted at the TDD phase**:

```
evolve loop: cycle 119: phase tdd: tdd: bridge: bridge: launch exit=81
stop_reason: "error"   PhasesRun: [intent, scout, triage]
```

`exit=81` = `bridge.ExitArtifactTimeout` ("artifact never appeared within the wait window"). Nothing shipped to `main` (no partial commit). Investigation surfaced **three** distinct issues plus a main-tree contamination.

**Timeline (UTC):**

| Time | Event |
|---|---|
| 23:13:36 | Cycle 119 starts (intent) |
| 23:14–23:19 | Scout runs on `agy-tmux` (Gemini) |
| **23:17** | **`runner.go` + `runner_test.go` (P-NEW-23 impl) appear in the MAIN tree** — during the read-only scout phase (Issue 2) |
| 23:20:26 | TDD phase starts (`claude-tmux`, cwd=worktree) |
| 23:26 | TDD writes valid artifacts (`test-report.md`, `tdd-reflection.yaml`) — **into the worktree subtree**, not where the bridge polls |
| ~23:30:26 | Bridge artifact-wait (300s ×2 via ADR-0026 extend) expires → `ExitArtifactTimeout` → cycle aborts (Issue 1) |

---

## Part 2: The Three Issues (root cause + fix)

### Issue 1 — Relative `--project-root` → ExitArtifactTimeout — **FIXED `80f4206`**

**Root cause.** `cmd_loop.go` declared `--project-root` with default `"."` and **never absolutized it**, despite the flag's own help text "absolute path to project root". Every derived path inherited the relativity:
- `orchestrator.go:252` `WorkspacePath = <projectRoot>/.evolve/runs/cycle-N` → `./.evolve/runs/cycle-119`
- `runner.go:239` `artifactPath = filepath.Join(req.Workspace, "test-report.md")` → relative

The bridge uses the **artifact contract** (`completion.go` `artifactDetector` → `artifactReady(cfg.Artifact)`): it polls `cfg.Artifact` for a non-empty file. The divergence:

| Actor | cwd | resolves `./.evolve/runs/cycle-119/test-report.md` to |
|---|---|---|
| TDD agent | **worktree** (`driver_tmux_repl.go:121` `cd <worktree>`) | `<worktree>/.evolve/runs/cycle-119/test-report.md` |
| in-process bridge poll | **main repo** | `<main>/.evolve/runs/cycle-119/test-report.md` |

The agent wrote the artifact; the bridge polled a different absolute path; the file "never appeared" → timeout. **Read-only phases (intent/scout/triage) passed** because `orchestrator.go:452` sets `phaseWorktree=""` for them → no `cd` → agent and bridge share the main cwd → the relative path resolved identically. **TDD is the first phase that runs in the worktree**, so it was the first to expose the latent relativity.

**Fix.** Resolve `projectRoot` AND `evolveDir` to absolute once at the composition root (`parseLoopArgs`, right after `fs.Parse`) via a small `absOrWarn` closure that **WARNs loudly** on `filepath.Abs` failure rather than swallowing it (a swallow would silently reproduce the very timeout). Restores the documented invariant; keeps every derived path cwd-independent. Behavioral table-driven test added (RED→GREEN), incl. an explicit relative `--evolve-dir` case.

```go
absOrWarn := func(label, p string) string {
    abs, err := filepath.Abs(p)
    if err != nil {
        fmt.Fprintf(stderr, "evolve loop: WARN: could not resolve %s %q to an absolute path (%v); ...\n", label, p, err)
        return p
    }
    return abs
}
projectRoot = absOrWarn("--project-root", projectRoot)
// ... and after evolve-dir resolution:
evolveDir = absOrWarn("--evolve-dir", evolveDir)
```

**Verification.** `gofmt -s` clean, `go vet` ok, `cmd/evolve` green with `-race`; code-simplifier (clean) + go-reviewer (APPROVE; its HIGH `--evolve-dir` + MEDIUM silent-swallow folded in). CI green (both `CI` + `go` workflows). End-to-end: cycle 120 (full-phase, fixed binary) records `workspace_path` + `active_worktree` as **absolute** and clears the TDD worktree phase.

### Issue 2 — Trust kernel is Claude-Code-specific; cross-CLI phases bypass it — **OPEN (interim-mitigated)**

**Root cause.** The evolve-loop trust kernel (`role-gate`, `phase-gate`, `ship-gate`) is implemented as **Claude Code PreToolUse hooks**. Non-Claude drivers (`agy-tmux`/Gemini, `codex-tmux`) do not execute Claude Code hooks. With `scout.cli=agy-tmux`, the **Gemini scout agent wrote source files (`runner.go` + `runner_test.go`) directly into the MAIN working tree** during the read-only scout phase — bypassing both the read-only-phase contract and worktree isolation, unguarded by role-gate. Evidence: the edits' mtime (23:17) falls inside the scout window; `git worktree list` shows no cycle-119 worktree; the work was loose in `main`, not committed on the `cycle-119` branch (0 commits ahead).

**Interim mitigation (applied).** Reverted `scout.json` → `claude-tmux` (restores role-gate enforcement on scout). The Gemini-injected P-NEW-23 work is preserved in `git stash@{0}` pending review (do not trust-merge an unguarded write blind).

**Durable fix (proposed, needs design sign-off).** Move invariant enforcement DOWN to the CLI-agnostic bridge/runner layer instead of per-CLI hooks:
- Confine every driver's agent process to the worktree cwd for source-writing phases (OS-level: `sandbox-exec`/`bwrap` write-scope = worktree only), independent of CLI.
- Have the bridge/runner verify post-phase that no out-of-scope main-tree source files changed (artifact-diff guard) for any driver.
- Until then: restrict source-writing phases (tdd/build) to Claude drivers, OR gate non-Claude phases behind an explicit "trusted CLI" allowlist.

### Issue 3 — `EVOLVE_PLAN_REVIEW=1` silent no-op under static routing — **OPEN**

**Root cause.** `plan-review` is a **router-only** phase: no core `Phase` constant (`phase.go`), no state-machine edge (`statemachine.go` goes `triage → tdd`). `EVOLVE_PLAN_REVIEW=1` only sets `cfg.PhaseEnable["plan-review"]=EnableOn`, which is consulted **solely** by the dynamic router (`router.go:enableOf`), and the router only drives at `Stage>=Advisory`. Under `EVOLVE_DYNAMIC_ROUTING=off` (the default at the time; advisory since 2026-06-06), the flag is read by nothing → silent no-op (no phase, no warning). Violates fail-loudly.

**Fix (proposed).** In `config.Load` (which already emits `[]Warning`), add an `inert-phase-enable` warning when a phase is set `EnableOn` but the routing stage will never run the advisor that inserts it. Do **not** add plan-review to the static state machine (that would break the byte-identical legacy spine).

### Issue 4 — Advisory-phase artifact-timeout hard-fails the cycle — **OPEN (found during cycle-120 validation)**

**Root cause.** `build-planner` is an **opt-in, advisory** phase (`buildplanner.go`: "skipped unless the flag enables it"; artifact `build-plan.md`). In cycle 120 its `claude-tmux` session produced **nothing** — only `build-planner-prompt.txt` (no `events.ndjson`/stdout/stderr/`build-plan.md`) — so the bridge's artifact-wait expired → `ExitArtifactTimeout (81)` → the orchestrator treated it as fatal (`FinalVerdict FAIL`, cycle aborted). Two problems:
1. **Empty session (likely trigger):** "session launched, produced zero output" after an opus-heavy, deep-research run (scout alone ~38k tokens/15 min; intent/tdd/build-planner on opus) matches the documented **subscription quota / rate-limit exhaustion** signature (empty output = "quota-likely"). The dispatcher did NOT classify it as `QUOTA-PAUSE` (RC=5) — it fell through to a generic artifact-timeout error.
2. **Design fault (trigger-independent):** an **advisory/optional** phase must NOT be able to abort the whole cycle. A missing advisory artifact should degrade to **WARN/skip**, not a fatal `ExitArtifactTimeout`.

**Fix (proposed).** (a) For phases marked optional/advisory, treat artifact-timeout as a soft skip (WARN + continue), not a fatal error — the `Skipper`/advisory metadata already exists. (b) Tighten quota detection so an empty-output session classifies as `QUOTA-PAUSE` (RC=5, auto-resume) rather than a generic error. (c) Consider cheaper model tiers / effort for advisory phases to reduce quota pressure (the cycle's own token-opt work — `--effort` wiring — is aligned).

---

## Part 3: The Meta-Pattern

**Both Issue 1 and Issue 2 are the same architectural smell: pipeline safety invariants enforced at a layer that does not hold uniformly across the heterogeneous-CLI, worktree-isolated design.**
- Issue 1: correctness depends on **cwd-relative paths** matching across two processes (cd'd agent vs in-process bridge) — they don't.
- Issue 2: integrity depends on **Claude-Code-specific hooks** — absent for Gemini/Codex.

The single `scout.cli→agy-tmux` config change exposed both at once. The durable direction for the whole class: **make invariants absolute and CLI-agnostic** — absolute paths everywhere a value crosses a process/cwd boundary; enforcement at the bridge/runner/OS layer, not in per-CLI prompt hooks.

---

## Part 4: Why the Tests Didn't Catch These

| Issue | Gap |
|---|---|
| 1 | The relative-path × worktree-cwd interaction is an **integration seam** (orchestrator → runner → bridge → cd'd agent) never exercised by a test. Unit tests passed an absolute workspace; no test asserted "artifact path is cwd-independent for worktree phases." The new `cmd_loop_projroot_test.go` closes the composition-root half. |
| 2 | No test runs a **non-Claude driver** through a read-only phase and asserts the main tree is unchanged. Trust-kernel tests assume Claude hooks fire. |
| 3 | No test asserts that an enabled phase actually **runs** (or warns if it can't) under the default routing stage. |

Recurring root: **tests written from the code, not the contract** — and integration joints (cross-process, cross-CLI, cross-cwd) deferred. Same anti-pattern as `cycle-109-116-go-meta-loop-bringup.md`.

---

## Part 5: Remediation + Recommendations

**Shipped (main):** `80f4206` — Issue 1 fix (absolute `projectRoot`/`evolveDir`) + behavioral test. CI green. Validated on cycle 120 (full-phase, fixed binary).

**Applied (working tree):** `scout.json` reverted to `claude-tmux` (Issue 2 interim mitigation). Gemini-injected work preserved in `git stash@{0}`.

**Open follow-ups (prime loop / next-session tasks):**
1. Issue 2 durable fix — CLI-agnostic write confinement (bridge/OS layer) + post-phase main-tree-diff guard. **Highest priority** (an unguarded cross-CLI write to main is an integrity hole).
2. Issue 3 — `inert-phase-enable` warning in `config.Load`.
3. Test-plan: add an integration test that a worktree-phase artifact path is absolute end-to-end; add a cross-CLI read-only-phase isolation test (assert main tree unchanged); add an "enabled phase runs-or-warns" test.
4. Review `git stash@{0}` (the P-NEW-23 budget-hint wiring is on-goal token optimization) and re-land it cleanly through the gated pipeline — or let a fixed-binary cycle re-produce it.

---

## Part 6: Lessons

1. **A flag whose help text promises a property must enforce it.** "absolute path to project root" defaulting to `.` with no `filepath.Abs` is a contract documented and then violated.
2. **Any path that crosses a process/cwd boundary must be absolute.** Worktree isolation + in-process polling means relative paths resolve differently per actor.
3. **Trust enforcement must live at the CLI-agnostic layer.** Per-CLI hooks silently vanish when you switch a phase's CLI — the safety property is only as strong as its weakest driver.
4. **The first worktree phase is the canary.** Read-only phases mask cwd/isolation bugs; failures concentrate at the first phase that `cd`s into the worktree.
5. **Investigate to root cause before fixing (systematic-debugging).** `exit=81` looked like an observer SIGTERM; gathering evidence at each component boundary (exit-code semantics, file mtimes, worktree status, cwd) revealed the true relative-path root cause and a second (cross-CLI) issue that a quick patch would have missed.
6. **Preserve, don't discard, contaminated-but-valuable work.** The Gemini-injected fix is on-goal; stashed for review rather than reverted away.

---

## References

- Fix commit: `80f4206` — `fix(cmd): resolve --project-root and --evolve-dir to absolute at composition root`
- Code: `go/cmd/evolve/cmd_loop.go` (`parseLoopArgs`), `go/cmd/evolve/cmd_loop_projroot_test.go`
- Mechanism: `go/internal/core/orchestrator.go:252,452`; `go/internal/phases/runner/runner.go:239`; `go/internal/bridge/{completion.go,driver_common.go,driver_tmux_repl.go,exitcodes.go}`
- Issue 3: `go/internal/core/{phase.go,statemachine.go}`, `go/internal/router/router.go`, `go/internal/config/config.go`
- Related: `docs/incidents/cycle-109-116-go-meta-loop-bringup.md` (same test-gap meta-pattern), ADR-0026 (self-healing review), ADR-0027 (commit-as-evidence)
- Validation: cycle 120 (full Scout→…→Ship cycle on the fixed binary)
