# Incident Report & Remediation: Go Meta-Loop Bring-Up — Cycles 109–116

**Date:** 2026-05-27 | **Severity:** HIGH (the v13 Go meta-loop could not complete a single cycle) | **Status:** all 7 layers remediated + shipped to main; cycle 116 (the end-to-end validation) sealed unfinished via `evolve cycle reset` (dispatcher died mid-audit)

> A `/evo:loop --budget-usd 150 ultrathink` run against the repo itself surfaced **seven distinct pipeline-failure layers** in a row. Each was a cycle-runtime integration behavior the v11 bash→Go port dropped or changed, that the Phase-1 unit suite (fakes only) never exercised. Each fix advanced the loop exactly one phase deeper, revealing the next. By cycle 116 the loop ran intent → scout → triage → tdd → build-planner → **build** (writing the goal's own code via TDD) — the first Go-era cycle to write production code.

## Part 1: What Happened

**Context.** The meta-loop (evolve-loop improving itself) was launched on the v13.0.0 Go binary to research + implement long-running/self-healing/progress-tracking. It had, in fact, **never completed a full build cycle on the Go orchestrator** — every prior attempt died before writing code. This run isolated why.

**Timeline (each cycle sealed via `evolve cycle reset`, then relaunched on the fix):**

| Cycle | Died at | Exit | Layer | Root cause (one line) |
|---|---|---|---|---|
| 109 | scout | 81 | 1 | artifact wait was a hard 300s **wall-clock kill**, not inactivity/review |
| 110 | triage | 81 | 2 | triage doc hardcoded `<token from runner>` + bare filename → wrote to `workspace/` subdir |
| 111/112 | tdd | 81 | 3 | **no per-cycle worktree provisioned** → role-gate denied all source writes |
| 112 | tdd | 81 | 4 | guard hooks can't find the binary inside the **worktree** (`CLAUDE_PROJECT_DIR`) |
| 113 | scout | 85 | 5 | `rate_limit` auto-respond regex matched scout's **own `rate_limit` grep output** |
| 114 | tdd | 81 | 6 | tdd runner polled stale `team-context.md`; agent writes `test-report.md` |
| 115 | audit | 81 | 7 | codex **alt-screen** pane is blank to `capture-pane scrollback=0` |
| 116 | (build ✓ → audit) | — | — | end-to-end validation; sealed unfinished (dispatcher died in audit) |

Every `exit=81` is `ExitArtifactTimeout` (the bridge waited out a phase whose artifact never appeared at the path it polled); `exit=85` is `ExitUnknownPrompt` (auto-respond escalation).

## Part 2: The Seven Layers (root cause + fix)

### Layer 1 — Artifact wall-clock timeout → self-healing review (`350e01a`, ADR-0026)
- **Symptom:** cycle-109 scout streamed output for 5 min, killed at exactly 300s.
- **Cause:** `driver_tmux_repl.go`'s artifact wait was `for elapsed < 300 { … }` — a total wall-clock cap that could not distinguish "stuck" from "still thinking."
- **Fix:** review-before-kill. At each interval the loop reviews evidence (did the pane change?) → extend (working) or pause (stalled). `StopEvent`/`StopReviewer` seam. Operator-designed; see ADR-0026 and ADR-0027 (commit-as-evidence, the universal end-state).
- **Lesson:** liveness timeouts must be **inactivity/progress-based**, never total wall-clock.

### Layer 2 — Agent-doc prompt-substitution contract (`90c09e6`)
- **Symptom:** triage finished its work but wrote `triage-decision.md` to a `workspace/` subdir with a literal `<!-- challenge-token: <token from runner> -->` placeholder; bridge polled the canonical path → timeout.
- **Cause:** the `*-tmux` bridge feeds the agent doc verbatim and substitutes `$ARTIFACT_PATH` / `$CHALLENGE_TOKEN` (`preparePrompt`). The triage + tester docs used **neither** — they hardcoded the placeholder + a bare filename.
- **Fix:** triage doc writes to `$ARTIFACT_PATH` with `$CHALLENGE_TOKEN`; tester token fixed.
- **Lesson:** agent docs MUST use the bridge's substitution variables, never hardcoded example placeholders. (Scout/intent worked only because their hardcoded names happened to match.)

### Layer 3 — Worktree provisioning dropped in the port (`fc315b7`) — THE systematic blocker
- **Symptom:** tdd/build `exit=81`; role-gate denied `Write` ("may not write outside workspace").
- **Cause:** bash `run-cycle.sh` provisioned a per-cycle `git worktree`; the v11 Go orchestrator **never ported it**. `cs.ActiveWorktree` was always `""`, and the role-gate's only source-write allowance (`phase=="build" && ActiveWorktree!=""`) was therefore unsatisfiable. **No phase could write source code.**
- **Fix:** `core/worktree.go` `WorktreeProvisioner` (named-branch `git worktree add -B cycle-N`, not `--detach` — ship needs a branch to ff-merge); orchestrator provisions at cycle start + passes `Worktree` to tdd/build; role-gate allows `WorktreePhase = tdd||build`.
- **Lesson:** a bash→Go port can silently drop an **entire subsystem** with zero failing tests, because the unit tests mocked the seam it lived in.

### Layer 4 — Worktree-aware guard hooks (`28af2693`)
- **Symptom:** tdd in the worktree: `PreToolUse:Bash hook error … <worktree>/go/bin/evolve: No such file`.
- **Cause:** guard hooks run `$CLAUDE_PROJECT_DIR/go/bin/evolve …`; Claude Code re-derives `CLAUDE_PROJECT_DIR` from cwd (the worktree) and does **not** honor a pre-set value (confirmed via claude-code-guide); `go/bin/` is gitignored → absent in the worktree checkout.
- **Fix:** `linkGuardDeps` symlinks the live dispatcher binary (`os.Executable()`) + `.evolve/{cycle-state,state,ledger}` into each worktree.
- **Lesson:** running an agent with cwd=worktree breaks every `$CLAUDE_PROJECT_DIR`-relative tool/path; the worktree must be made self-sufficient.

### Layer 5 — Auto-respond false-positive on the agent's own output (`bd3a6bf`)
- **Symptom:** cycle-113 scout `exit=85` (escalate) while grepping the phaseobserver detection rules.
- **Cause:** the `rate_limit` regex `rate.?limit|too many requests|429` matched the literal token `rate_limit` in scout's grep output (`.?` matches the underscore). **Goal-deterministic** — the goal is *about* rate-limiting, so scout prints the token.
- **Fix:** require a real banner: `(usage|rate)[ -]limit (reached|exceeded|hit)|too many requests` (+ family tokens) across all 3 `-tmux` manifests; added a regression case (code-grep → `noop`).
- **Lesson:** auto-respond patterns scan the agent's own pane output — they need **adversarial/negative test cases**, not just happy-path banners.

### Layer 6 — Doc↔runner artifact-name mismatch (`e7918e2`)
- **Symptom:** cycle-114 was a breakthrough — tdd wrote `goalprogress_test.go` + acs predicates, ran them (7/7 RED for the right reason), wrote `test-report.md` — yet `exit=81`.
- **Cause:** the tdd phase runner's `ArtifactFilename()` returned the stale `team-context.md`; the agent ecosystem (5 docs) writes `test-report.md`. The bridge polled a file the agent never writes. **The unit test `tdd_test.go` *asserted* `team-context.md`** — it pinned the bug.
- **Fix:** `tdd.go ArtifactFilename → test-report.md`. Verified build/buildplanner/audit names already match their docs (tdd was the lone outlier). The source-of-truth for the polled name is `phases/<p>/ArtifactFilename()`, NOT `profile.output_artifact` (vestigial).
- **Lesson:** there is **no contract test** binding the runner's polled artifact name to the name the agent doc instructs — and a test written from the code (not the contract) locked the bug.

### Layer 7 — Alt-screen capture blindness (`6817063`, shipped)
- **Symptom:** cycle-115 audit (codex/GPT, first live codex run) produced empty scrollback, no logs, no `audit-report.md` → `exit=81`.
- **Cause:** codex (and agy) render in tmux **alt-screen**, where a bare `capture-pane` (scrollback=0) is blank. The boot loop compensates (`bootScrollback=200`), but the artifact-wait loop's auto-respond + review captured `scrollback=0` → blind to codex after boot. (agy/builder survived cycle-115 only because it needed no prompt and finished before the review interval.)
- **Fix:** thread `lp.bootScrollback` into the task-phase captures (autoResponder + review). claude (0) unchanged; codex/agy (200) now visible.
- **Lesson:** the fake-tmux test double does not model per-CLI rendering modes (alt-screen), and the real-CLI tests are flaky — so CLI-specific behaviors aren't gated.

## Part 3: The Meta-Pattern

All seven are the **same shape**: a cycle-runtime integration behavior (timeout semantics, prompt-substitution contract, worktree provisioning, worktree-relative tooling, auto-respond scanning, doc↔runner naming, per-CLI rendering) that the v11 bash→Go port **dropped or changed**, and that the Phase-1 unit suite (fakes for bridge/tmux/runner/storage) **never exercised**. Each fix advanced the loop one phase; the next phase then surfaced the next gap. The loop itself was never the problem — it correctly scoped and wrote the goal's code the moment the harness let it.

## Part 4: Why the Tests Didn't Catch These

`docs/TEST_PLAN.md` is explicitly **"Phase 1"**: Goal #1 is *"Lock the behavior of every ported subsystem **before Phase 2 wires it into the orchestrator**."* The integration layer — where all 7 bugs live — was **deferred to a "Phase 2+" marked TBD that was never built.**

| Failure | Test-plan gap |
|---|---|
| 3 (worktree provisioning) | "Gaps" defers `evolve loop`/`cycle run` **full-cycle test** to Phase 2; no bash→Go **parity test** for the dropped subsystem. |
| 4, 7 (worktree hooks, alt-screen) | Cross-CLI/sandbox → Phase 2/3; `TestRealTmux_*` real-CLI tests are **flaky (REPL-boot)** so not reliably gated; fake-tmux doesn't model alt-screen. |
| 2, 6 (prompt vars, artifact name) | `internal/phases/*` deferred to Phase 2 (fakes); **no doc↔runner contract test**; `tdd_test.go` *pinned* the wrong name. |
| 5 (rate_limit) | auto-respond decision matrix had only happy-path banners, **no adversarial inputs**. |
| role-gate (root of 3) | spec line codified the **buggy build-only allowance** — written from the code, not the requirement. |

**Two anti-patterns, one root:** (1) integration testing deferred indefinitely (units-with-fakes locked; the wiring never tested); (2) tests written **from the code, not the contract** — so a bug baked into both code and test (role-gate build-only; tdd `team-context.md`) is invisible. This violates AGENTS.md Rule 9 ("tests verify intent, not surface behavior").

## Part 5: Remediation + Recommendations

**Shipped (main):** `350e01a` (1) · `90c09e6` (2) · `fc315b7` (3) · `28af2693` (4) · `bd3a6bf` (5) · `e7918e2` (6) · `6817063` (7); plus ADR-0026 (self-healing review) + ADR-0027 (commit-as-evidence). All reviewed (go-reviewer / code-simplifier) + tested (`-race`); committed `--class manual` (golangci errcheck flags the codebase's intentional `fmt.Fprintf(Stderr)` idiom; CI doesn't run golangci).

**Test-plan additions (the real fix — items 1–2 are prime loop tasks):**
1. **Full-cycle e2e** with a realistic fake-agent that *actually writes* artifacts/source → exercises worktree-provision → role-gate → artifact-detection → ship. Catches layers 3 & 6 instantly.
2. **Doc↔runner contract tests** — assert each phase's `ArtifactFilename()` == the name its agent doc instructs (shared constant or doc-parse).
3. **Adversarial cases** in the auto-respond matrix (tokens that must NOT match).
4. **Reliable real-CLI gating** (fix/quarantine `TestRealTmux_*`) so alt-screen + codex behaviors are enforced.
5. **bash→Go parity tests** for any subsystem the port touched.

## Part 6: Lessons

1. **A port needs behavioral-parity tests, not just unit-coverage of the new code** — a whole subsystem (worktree provisioning) vanished with a green suite.
2. **Test the contract, not the code** — assert against the requirement/doc, or the test will lock the bug (Rule 9).
3. **Cross-seam contracts need explicit tests** — doc↔runner names, prompt-substitution vars, per-CLI rendering. Fakes hide them.
4. **Self-healing must distinguish slow from stuck** — progress/inactivity signals, not wall-clock (layer 1); and the signal must handle long single generations + alt-screen (layers 1, 7) — Stage-1 of ADR-0026.
5. **Defer integration testing and it never happens** — "Phase 2 (TBD)" became the home of every production bug.

## References

- Commits: `350e01a`, `90c09e6`, `fc315b7`, `28af2693`, `bd3a6bf`, `e7918e2`, `6817063`.
- ADRs: `docs/architecture/adr/0026-self-healing-review-layer.md`, `0027-commit-as-evidence.md`.
- `docs/TEST_PLAN.md` (Phase-1 plan + deferred Gaps).
- Source-of-truth: `go/internal/phases/<p>/*.go ArtifactFilename()`; `go/internal/guards/role.go` (`WorktreePhase`); `go/internal/core/worktree.go`; `go/internal/bridge/driver_tmux_repl.go` (capture scrollback); `go/internal/bridge/manifests/*-tmux.json` (auto-respond regexes + `tier_aliases`).
- Driver: meta-loop ran on `~/.local/bin/evolve-selfheal` (external, all 7 fixes).
