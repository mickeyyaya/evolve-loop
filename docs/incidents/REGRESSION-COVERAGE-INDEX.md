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

## Summary (sweep 2026-05-29)

| | Count |
|---|---|
| Incidents mapped | 13 |
| Distinct failure modes | 34 |
| ✅ covered | 11 |
| 🟡 partial | 9 |
| ❌ none | 9 |
| ⛔ untestable | 5 |

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

## Prioritized gap backlog

Highest value first (regression likelihood × blast radius × testability):

1. **ledgerverify anti-gaming** — assert a cycle whose ledger has zero phase entries is reported incomplete AND the orchestrator does not advance `lastCycleNumber`. (Builds directly on the cycle-137 verify fix; cheap; pins cycle-132-141.)
2. **cli_chain fallback** — boot-timeout (exit 80) advances to the next CLI; empty fallback list degrades gracefully with a clear error, no panic. (Pins cycle-121 + cycle-123; the "any CLI any phase" invariant.)
3. **observer nudge-before-kill** — soft-stall nudge emitted before hard SIGTERM in the nudge window. (Pins cycle-94-98 second mode.)
4. **ship audit-binding format** — canonical `## Verdict\n**PASS**` accepted, malformed rejected. (Pins cycle-62.)
5. **runner relative-root** — artifact path resolves absolute under a relative `--project-root`. (Pins cycle-119.)
6. **reset orphan-worktree** — SealCycle archives an orphaned worktree's state. (Pins cycle-31.)

## Untestable-by-unit (mitigate by design + docs, not a test)

- codex/ChatGPT vendor rate limit (cycle-128) — operator-account state; mitigate via CLI fallback + clear operator message.
- Interactive CLI modals as raw terminal state (cycle-122/123) — the *decision logic* is testable (auto-respond mapping) but the live modal render is not.

## Contract for new incidents

When an incident ships a fix, add a row here with the pinning test path::name, and flip its coverage to ✅. A fix without a regression row is incomplete (CLAUDE.md Rule 9 — tests verify intent). This index is the single source of truth for "is the pipeline getting more stable over time."
