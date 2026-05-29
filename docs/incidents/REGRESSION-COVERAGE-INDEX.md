# Incident → Regression Coverage Index

> **Purpose.** A living map from every documented incident to its root-cause failure mode(s) and the durable regression test that pins each one. This is how we incrementally build a stable, robust pipeline: every past failure becomes a test that fails if the bug returns. New incidents MUST add a row here when they ship a fix.
>
> **Generated:** 2026-05-29 (multi-agent coverage sweep over `docs/incidents/*`, verified against `go/**/*_test.go`, `acs/`, `tests/`). Confidence column reflects assessment certainty at sweep time; verify before relying on a `partial`.

## Legend

| Coverage | Meaning |
|---|---|
| ✅ covered | A durable test exists that WOULD FAIL if the bug were reintroduced. |
| 🟡 partial | A related test touches the area but does not pin this exact failure mode. |
| ❌ none | No regression test pins this mode. |
| ⛔ untestable | Depends on external infra/CLI behavior (vendor rate limit, interactive modal) a unit test cannot reproduce; mitigate by design + docs, not a unit test. |

## Summary (sweep 2026-05-29, 13-agent parallel coverage map)

| | Count |
|---|---|
| Incidents mapped | 13 |
| Distinct failure modes | 73 |
| ✅ covered (a test would fail if the bug returned) | 40 |
| 🟡 partial (related test, doesn't pin the exact mode) | 20 |
| ❌ none (no regression test) | 10 |
| ⛔ untestable (external infra / live CLI) | 3 |

**40 of 73 modes are truly pinned; 30 have concrete gap-test proposals.** The
prioritized backlog below lists the highest-value ones. The coverage map below
is the original hand-pass (kept for the per-incident narrative); the counts
above and the backlog reflect the fuller parallel sweep.

## Coverage map

| Incident | Failure mode | Suspect file | Coverage | Pinning test / proposal |
|---|---|---|---|---|
| cycle-109-116 | Go orchestrator dropped per-cycle worktree provisioning (role-gate denied all writes) | `core/orchestrator.go` | ✅ | `core/orchestrator_test.go` worktree-provision path |
| cycle-119 | relative `--project-root` → ExitArtifactTimeout (artifact poll wrong dir) | `phases/runner/runner.go` | 🟡 | **GAP:** runner test asserting artifact path resolves absolute when root is relative |
| cycle-121 | codex REPL boot timeout; no fallback to next CLI | `phases/runner/cli_chain.go` | 🟡 | **GAP:** cli_chain advances to next CLI on boot-timeout exit 80 |
| cycle-122 | codex permission modal blocked run; fallback didn't fire | `bridge/driver_codextmux.go` | 🟡 | partial: `bridge/autorespond_decision_test.go`; **GAP:** modal-prompt → auto-respond decision |
| cycle-123 | codex edit-approval modal + empty fallback chain hard-failed | `phases/runner/cli_chain.go` | 🟡 | **GAP:** empty fallback list degrades gracefully (no panic, clear error) |
| cycle-124-137 | challenge token minted by two paths → diverged per phase | `bridge/driver_common.go` | ✅ | `bridge/coverage_batch7_test.go::TestPreparePrompt_ReadsExistingChallengeToken` |
| cycle-124-137 | ledgerverify counted only bash `kind=agent_subprocess`, not Go `kind=phase` | `ledgerverify/verify.go` | ✅ | `ledgerverify/verify_test.go::TestVerifyCycle_GoNativePhaseVocabulary` (+ MixedVocabularies, GoNativeIntentAndMemoPhases) |
| cycle-124-137 | ACS predicates hand-rolled `grep PASS` without `-v` → false RED | `acs/lib/assert.sh` | ✅ | `acs/lib/assert_test.sh` (11 assertions; exit-code based) |
| cycle-124-137 | stub `fakeLedger.Append` accumulated → masked the verify bug (green suite, broken prod) | `cmd/evolve/cmd_loop_m4_test.go` | ✅ | Append made no-op; the 6 dispatcher failure-path tests now genuinely fail verify |
| cycle-132-141 | orchestrator gaming: cycle number advanced without phase evidence | `core/orchestrator.go` | 🟡 | partial: ledgerverify empty-ledger → incomplete; **GAP:** orchestrator must not advance lastCycleNumber without ledger phase evidence |
| cycle-31 | worktree orphaned on rate-limit failure; builder WIP lost | `core/reset.go` | ❌ | **GAP:** SealCycle/reset archives an orphaned worktree's state rather than dropping it |
| cycle-46 | ship refused: tree-state SHA at ship ≠ audited SHA | `phases/ship/audit.go` | ✅ | `phases/ship/native_test.go` audit-binding matrix |
| cycle-61 | (thin doc) audit/ship friction, underspecified | — | ❌ | doc too thin to pin a mode; backfill the incident first |
| cycle-62 | ship refused: release audit verdict heading format mismatch | `phases/ship/audit.go` | 🟡 | **GAP:** audit-binding accepts canonical `## Verdict\n**PASS**`, rejects malformed |
| cycle-94-98 | watchdog over-fired, killed healthy phases | `adapters/observer/observer.go` | 🟡 | partial: `observer_test.go::TestObserver_NoStallWhenLogGrowing`; verify it pins the growing-stdout case |
| cycle-94-98 | soft-stall nudge not emitted before hard SIGTERM | `adapters/observer/observer.go` | ❌ | **GAP:** nudge emitted when `NUDGE_S < idle < STALL_S` before kill |
| gemini-forgery | cross-CLI: phase output not bound to cycle's real challenge token | `phases/scout/scout.go` | 🟡 | partial: `scout_test.go::TestComposePrompt_InjectsChallengeTokenFromRequest` pins the inject path; **GAP:** cross-CLI trust enforcement (auditor rejects a report whose token ≠ cycle token) |

## Prioritized gap backlog (13-agent sweep; agent confidence in parens)

Highest value first (regression likelihood × blast radius × testability). Each
target was verified by an agent that read the incident and searched the suite.

**cycle-138 result (Bug A+B validation — POSITIVE).** Verified from
`/tmp/cycle138.log` + ledger + audit-report:
- All 7 phases ran; **audit verdict PASS, red_count 0** — the first clean audit
  in the 130-138 sequence.
- `VerifyCycle(138)` against the real ledger returns `OK=true, missing=[]`
  (probe test confirmed). **Bug A's fix works — NO dispatcher false-negative
  this cycle** (cycle-137's "missing [scout builder auditor]" did NOT recur).
- Bug B's fix was exercised live: the TDD-engineer authored 5 ACS predicates
  that source `acs/lib/assert.sh` (`assert_go_test_pass`, `assert_go_coverage_ge`)
  instead of hand-rolling `grep PASS`.
- `FinalVerdict=SHIPPED_VIA_BUILD` (v12.2 disambiguation: HEAD moved during the
  cycle → build committed inline; this is a *success* label, not a failure). The
  `cost $0` reflects OAuth-session billing (CLAUDE.md note), not zero work.

**UPDATE after cycle-139 (supersedes the "cosmetic" note above).** cycle-138
shipping "via build" rather than via the formal ship phase is NOT cosmetic — it
is the visible symptom of a real defect found in cycle-139: the audit phase's
EGPS gate requires `acs-verdict.json` (red_count==0) to PASS, treats a MISSING
file as FAIL by design, and **nothing in the autonomous loop generates that
file** → every autonomous audit is structurally forced to FAIL → `audit→retro`,
no formal ship. cycle-138 shipped only because its build committed inline before
the forced-FAIL audit; cycle-139's build did not, so it shipped nothing
(`SKIPPED_UNKNOWN`). Full analysis + approved fix:
[cycle-138-140-egps-verdict-not-generated-in-autonomous-loop.md](cycle-138-140-egps-verdict-not-generated-in-autonomous-loop.md).
This is gap #0 below — THE current blocker for two clean *shipped* cycles.

## Prioritized gap backlog continued

0. **EGPS verdict not generated in autonomous loop (cycle-138/139, ❌none, 1.0)** —
   THE blocker. Audit forces FAIL on missing `acs-verdict.json`, which the
   autonomous `evolve loop` never produces. Approved fix: the **audit phase
   generates it** via `acssuite.Run`+`WriteVerdict` when absent (honoring a
   pre-staged file). Regression test: a cycle with `acs/cycle-N/` predicates +
   no pre-staged verdict → audit generates it → red_count 0 → PASS → `audit→ship`.
   Files: `phases/audit/audit.go` (Classify), `acssuite` (Run/WriteVerdict),
   `statemachine.go:96`. See the dedicated incident doc.
1. **cli_chain empty-fallback (cycle-123, ❌none, 0.95)** — a profile with no
   `cli_fallback` key + a fallback-trigger exit (81) attempts NO fallback; cycle
   aborts. The "any CLI any phase" invariant. →
   `runner/runner_fallback_test.go::TestRun_FallbackOnArtifactTimeout_EmptyProfileFallback`
   asserting `calls==[primary]`, plus a sibling where a populated chain DOES
   advance. `runner/cli_chain.go:resolveCLIChain`.
2. **Cross-CLI trust bypass (cycle-119 + gemini-forgery, ❌none, 0.9)** — a
   read-only phase run via a non-Claude driver can write to the main tree
   (Claude-Code PreToolUse hooks don't bind other CLIs). → `internal/core`
   integration test: run a read-only phase via a non-Claude driver in a worktree,
   assert main-tree source files unchanged post-phase (diff guard).
3. **Observer auto-spawn wiring (cycle-122, 🟡partial, 0.9)** —
   `wireOrchestratorDeps` wires `WithObserver(NewCoreAdapter())` when
   `EVOLVE_OBSERVER_AUTOSPAWN!=0`, noop when `=0`. → `cmd/evolve` wiring test.
4. **ledgerverify anti-gaming (cycle-132-141, 🟡partial)** — a cycle whose ledger
   has zero phase entries is reported incomplete AND `lastCycleNumber` does not
   advance. Builds directly on the cycle-137 verify fix; cheap.
5. **Boot-scrollback load-bearing (cycle-121, 🟡partial, 0.75)** — codex-tmux boot
   with `bootScrollback=0` + trust modal → `ExitREPLBootTimeout`. `driver_tmux_repl.go`.
6. **codex per-edit-approval modal (cycle-123, 🟡partial, 0.85)** — synthetic
   apply_patch fixture → modal appears and is auto-dismissed by the
   `interactive_prompts` regex (manifest→pane integration). `driver_codextmux.go`.
7. **stall-vs-progress (cycle-109, 🟡partial, 0.75)** — artifact-wait extends on
   pane progress, pauses on stall (StopReviewer). `internal/bridge`.

Lower tier (hand-pass, still valid): ship audit-binding format (cycle-62), runner
relative-root (cycle-119), reset orphan-worktree (cycle-31), observer
nudge-before-kill (cycle-94-98). 30 modes total carry concrete gap proposals.

## Untestable-by-unit (mitigate by design + docs, not a test)

- codex/ChatGPT vendor rate limit (cycle-128) — operator-account state; mitigate via CLI fallback + clear operator message.
- Interactive CLI modals as raw terminal state (cycle-122/123) — the *decision logic* is testable (auto-respond mapping) but the live modal render is not.

## Contract for new incidents

When an incident ships a fix, add a row here with the pinning test path::name, and flip its coverage to ✅. A fix without a regression row is incomplete (CLAUDE.md Rule 9 — tests verify intent). This index is the single source of truth for "is the pipeline getting more stable over time."
