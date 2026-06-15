# Design — Modularization, Dedup & Unified Phase I/O (evolve-loop Go)

> Detailed target contract + module boundaries for the campaign chartered by **ADR-0050**. This
> mirrors plan `happy-petting-wreath.md` and is committed so the contract is reviewable. The
> single chronological audit trail is `docs/architecture/decision-log-modularization.md`; the
> measured package inventory is `docs/architecture/audit-2026-06-15-package-map.md`.
>
> Date: 2026-06-15 · Status: accepted design, leaf→core execution · Gate: `EVOLVE_PHASE_IO`
> (`off`→`shadow`→`advisory`→`enforce`, default `off`).

---

## 1. The request / requirement

> *"Modularize and dedup the codebase; make every basic component (agent-bridge, logging,
> contract, output-verification, …) an independent dependency-injected module unit-tested in
> isolation with 100% of its public API exercised by edge-case + concurrency tests; re-architect
> phase I/O into one unified, isolated, typed envelope so each phase is a true pipes-and-filters
> stage; remove duplication onto shared leaf utilities; keep the trust kernel byte-identical."*

### Requirements

- **R1 (modularize).** Each basic component is a package with a minimal public API (1–3-method
  ports where it's a boundary), constructed with injected dependencies, unit-testable with fakes —
  no real subprocess/network/tmux/git in the fast tier.
- **R2 (dedup).** Git exec, env parsing, `.evolve` path layout, and console logging each route
  through one shared leaf (`gitexec` / `envchain` / `paths` / `log`), faked in tests.
- **R3 (coverage).** 100% of the public-API surface of each modularized package has a direct
  edge-case test; concurrency seams have stress tests; enforced by `cmd/apicover`.
- **R4 (phase I/O).** Phases communicate through one unified typed `PhaseInput`/`PhaseOutput`
  envelope; they read typed `Upstream` handoffs, never sibling artifacts off disk ad-hoc.
- **R5 (trust kernel).** Ship-gate, ledger hash-chain, and EGPS `red_count==0` gate stay
  byte-for-byte identical; every kernel-touching slice proves it with a byte-identity test.
- **R6 (rollout).** All behavior change is gated `EVOLVE_PHASE_IO`; `off` ⇒ byte-identical live
  loop. `shadow` compares; `advisory` runs old path as winner and compares; `enforce` switches.

---

## 2. Unified phase envelope — `internal/phaseio` (the centerpiece)

New **leaf** package; imports only `phasespec` (P2/P3). No imports of `core`/`router` (the
dependency points inward; `core` and `router` import `phaseio`).

### 2.1 `PhaseInput`
```
PhaseInput{
  // identity / roots
  Cycle int; RunID, GoalHash, ProjectRoot, Workspace, Worktree string
  Phase, PreviousPhase string
  // sealed config (getters only — no mutable map leaks)
  Env   sealedEnv     // typed env snapshot, read via accessors
  Spec  sealedSpec    // the phase spec
  // typed upstream — replaces ad-hoc disk reads
  Upstream Handoffs
  // sealed cycle inputs — getters only
  Cycle CycleInputs   // Goal/Strategy/CommitMessage/FleetScope/ChallengeToken
  // separate typed channels — replace the mutable Context dict + "## Correction" blob
  ErrorContext    *ErrorContext     // ship_error_code/class/stage/debug
  CorrectionState *CorrectionState  // Attempt (idempotency), Directive
  WorktreeWritable bool             // per-phase isolation expressed in the type
}
```
**Invariants:** sealed fields expose getters only — `TestPhaseInput_SealedContext_NoMutation`
proves a returned map/struct cannot mutate the input. Tolerant of fields a phase doesn't use (P5).

### 2.2 `PhaseOutput`
Today's `PhaseResponse` fields **plus**: mandatory structured `Verdict`; `*FailureBlock`
(mandatory on non-PASS); namespaced `Signals`; `WorktreeTreeSHA` (recorded — the seam for future
per-phase-worktree isolation). Each verdict-emitting phase must populate `Verdict` + (on non-PASS)
`FailureBlock` — composes with `EVOLVE_CONTRACT_GATE`.

### 2.3 `Handoffs`
Typed per-phase accessors `Scout()/Triage()/Build()/Audit()` (each returns `(view, present bool)`),
a `Generic(key)` plane for un-typed reads, and `Degraded() []string` listing read-misses.
**Built by reusing `router.Digest`** — the single authority on the on-disk handoff shape; `phaseio`
never adds a second reader. `TestHandoffs_AuditAccessor_AbsentReturnsFalse` pins the absent path.

### 2.4 Canonical on-disk envelope
`<Workspace>/handoff-<phase>.json`:
```
{ "payload": { …today's exact per-phase bytes… },
  "verdict": "...", "signals": {…}, "failure": {…} }   // promoted top-level
```
Postel-compatible: `router/digest.go` reads `payload.*` **with fallback to the legacy flat shape**.
`TestDigest_PayloadWrapped_EquivalentToFlat` is the golden-equivalence anchor — wrapped and flat
yield identical `RoutingSignals`.

---

## 3. Leaf utilities (dedup targets, P2)

| Leaf | Role | State | Migration |
|------|------|-------|-----------|
| `internal/gitexec` (new) | `Git{Dir, Run sysexec.RunFunc}` with `Capture/Output/Run/DirtyPaths/HEAD` + pure `PorcelainPath/PorcelainOldPath`. Isolates the git CLI. | to build (1.1) | leaf callers first (rollback, versionbump, changeloggen, swarm, cycleclassify, preflight); `core`'s 4 git files LAST (Phase 4.5). |
| `internal/envchain` | typed env registry (`BoolValue` etc.) | partially adopted (core done) | remaining `cmd/*` + leaves (1.3). |
| `internal/paths` | `.evolve` layout from a `Layout` | partial | `bridge/manifest.go`, `research/kb.go`, `ship/gitops.go` (1.4). |
| `internal/log` | unified logger; add `Console{Out,Err,Quiet}` + `Infof/Warnf/Errorf` + `Default()` (additive) | to promote (1.5) | non-core printers first; orchestrator's ~115 sites in Phase 4, per-section. |
| `internal/jsonio` (optional) | read helper | deferred (1.6) | only if a slice naturally touches a read cluster; no 277-site sweep. |

Each leaf is created **green with zero callers** (zero blast radius), then callers migrate one
package per slice. Each migration's RED test = "FakeExec/fake records the calls" — provable only
*after* the dependency is injected (the raw `exec.Command`/`os.Getenv` form couldn't test it).

---

## 4. Test infrastructure (Phase 0 deliverables)

### 4.1 `cmd/apicover` (stdlib only — `go.mod` has no `golang.org/x/tools`)
- **Enumerate** exported symbols via `go/ast`+`go/parser`: `FuncDecl` (methods keyed `Type.Method`),
  `TypeSpec`, top-level `var`/`const`; skip `integration`/`e2e`/`acs`-tagged files for the default
  measurement.
- **Two-signal uncovered check:** a symbol is "covered" iff it is **named in a `_test` AST** *and*
  shows **`>0%`** in `go tool cover -func`. Named-but-0% ⇒ flagged as a **false-green**.
- **`//apicover:ignore reason=...`** directive (non-empty reason required; tool prints the full
  ignore-list each run). `-require-doc` mode flags exported decls with a nil `Doc`.
- **Wiring:** `make apicover` target + a **warning-only** CI step mirroring the existing
  warning-only ≥85% gate (`.github/workflows/go.yml`).

### 4.2 `fixtures.StressN`
`StressN(t, n, k, fn)` — launch `n` goroutines, each doing `k` iterations, released simultaneously
by a closed-`start`-channel barrier + `WaitGroup`. Asserts the **invariant**, not "didn't crash"
(P7). Canonical pattern + behavior-named test-name shape documented in `go/docs/testing.md`.

---

## 5. Rollout stages (`EVOLVE_PHASE_IO`)

| Stage | Behavior | Proven by |
|-------|----------|-----------|
| `off` (default) | New code dormant; dispatch path byte-identical to pre-change. | golden test + real-cycle dry-run. |
| `shadow` | Assemble `PhaseInput`/`Handoffs`, write `phaseio-shadow-<phase>.json`, compare vs. what the phase read off disk; mismatch ⇒ WARN + ledger `kind:"phaseio_shadow_mismatch"`. No phase reads the new envelope. | soak: zero mismatch entries over N cycles. |
| `advisory` | Typed `CycleInputs`/`ErrorContext`/`Upstream` populated; old `req.Context`/disk path still wins, results compared per reader (retro→scout→triage→intent→ship→debugger→build; **audit LAST**). | per-reader `Test<Phase>_<Field>_FromCycleInputs_MatchesContext`. |
| `enforce` | New envelope is authoritative; circuit-breaker auto-demotes `enforce→advisory` after N blocks. | EGPS-unchanged + ledger byte-identity tests green; then default flip (3.10). |

---

## 6. Anchors in current code (verified 2026-06-15)

- Dispatch / shadow-hook seam: `internal/core/cyclerun_dispatch.go` `func (cr *cycleRun) dispatch`
  — `PhaseRequest` built at **lines 94–104** (assemble `PhaseInput` here when `cfg.PhaseIO>=shadow`).
- Retro map-clone hack: `cyclerun_dispatch.go:87–93` (Phase 3.5 `CycleInputs` target).
- Ship-error `ctxSnap` injection: `cyclerun_dispatch.go:218–221` (Phase 3.5 `ErrorContext` target).
- Post-run comparison point: the returned `dispatchResult`, consumed by `cyclerun_review.go` /
  `cyclerun_record.go`.
- Phase→role binding to collapse: `recordAuditBinding` + `recordBuildBinding` →
  `recordPhaseBinding(phase, output)` via a `phase→role` map (`audit→auditor`, `build→builder`,
  identity otherwise) — same bytes, gated on `TestRecordPhaseBinding_*_ByteIdenticalTo*`.

---

## 7. Per-module Definition of Done (Phase 5 gate)

1. `apicover` reports **0 uncovered exported symbols**.
2. No false-greens; package total ≥85% line.
3. Each exported func has ≥1 error-path/empty-input/boundary test (table-driven).
4. Every mutex/flock has its canonical stress test (Phase 2 guard passes).
5. Tests use `fixtures` fakes — no real subprocess/network/tmux/git in the fast tier.
6. Every `.go` file <800 LOC.
7. Godoc on every exported symbol (`apicover -require-doc` clean).
8. `gofmt` + `go vet` clean.

---

## 8. Out of scope / explicit non-goals

- Forcing 100% **line** coverage (we enforce public-API coverage; line is reported).
- Re-architecting `bridge`'s driver model beyond Lego-splitting any `driver_*.go` still >800 LOC.
- Changing EGPS gate *logic* — only the audit gate's input *source* moves, proven equal first.
- A blanket `jsonio` sweep (deferred unless a slice naturally touches a read cluster).
