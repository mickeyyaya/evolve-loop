# Ship-Rate Harness Hardening + Live Pipeline Dashboard — Design Spec

**Date:** 2026-09-02
**Status:** design → implementation (this session; autonomous mode, decisions recorded here for operator override)
**Author:** console session (Fable 5.1, xhigh) + operator directive
**Research:** [ship-rate-harness-reliability-2026-09-02.md](../../research/ship-rate-harness-reliability-2026-09-02.md) (synthesis + ranked proposals), [ship-rate-harness-reliability-2026-09-02-sources.md](../../research/ship-rate-harness-reliability-2026-09-02-sources.md) (literature, 60 sources), [pipeline-dashboard-patterns-2026-09-02.md](../../research/pipeline-dashboard-patterns-2026-09-02.md) (UI patterns, ~45 sources)

## Operator directive (verbatim intent)

> Increase the successful ship rate for tasks in the pipeline; prevent/improve LLM agents doing exactly what was asked, considering some agents run on lower-capability models at lower thinking levels and must still get the job done through the harness. Investigate from an architecture perspective, research online with citations, and develop a live HTML dashboard for a human to monitor requests/tasks through the pipeline: what has been done, what went wrong, status updates.

Two sub-projects fall out of this, decomposed because they have independent deliverables:

| Sub-project | Deliverable | Path |
|---|---|---|
| **A. Ship-rate hardening** | research synthesis with citations + ranked, costed proposals; the top two smallest/highest-leverage fixes implemented (separate PR); the rest filed as inbox items with acceptance criteria | `docs/research/…`, `go/internal/subagent`, `go/internal/core`, `.evolve/inbox/` |
| **B. Live pipeline dashboard** | `evolve dashboard` — read-only, single-binary, stdlib-only local web UI over `.evolve/runs/`, `knowledge-base/cycles/`, `.evolve/inbox/` | `go/internal/dashboard`, `go/cmd/evolve/cmd_dashboard.go`, ADR-0095, guide |

---

## Engineering protocol (stated up front, applies to both sub-projects)

**TDD protocol.** Red → green → refactor per unit. Every behaviour lands with a test that failed before the change. Unit tests use `t.TempDir()` project roots seeded with fixture artifacts shaped exactly like the live ones (schemas verified against `runtime/.evolve/runs/cycle-1605` on 2026-09-02). Server tests use `net/http/httptest`. Tests are `t.Parallel()`-safe (per-instance seams, never package vars). Run with `go test -count=1` (the module's test cache is blind to `../.evolve` reads).

**Design patterns and the force that requires each.**

| Pattern | Where | Force |
|---|---|---|
| **Reader / Repository** | `dashboard/collect.go` and one `read*` function per artifact family | ~15 artifact schemas, each best-effort and absence-tolerant; parsing must be isolated from rendering so a half-written file never breaks the page |
| **Immutable Snapshot (Memento)** | `dashboard.Snapshot` rebuilt whole on change | files are written concurrently by the loop via atomic rename; rebuilding a value and swapping a pointer avoids every torn-read and locking question; the dashboard never takes the loop's flock sidecars |
| **Observer via polling adapter** | `dashboard/sse.go` | browsers need change notification; the module is stdlib-only (vendored two-dep `go.mod`), so no fsnotify — a mtime fingerprint poll adapts "directory changed" into SSE events |
| **Facade** | `dashboard.New(root, opts).Handler()` | the `cmd_*.go` layer stays a 60-line flag parser like `cmd_soak_report.go`; all logic lives in `internal/` |
| **Strategy (config-injected)** | `subagent/modeltier.go` situation → tier from `.evolve/profiles/builder.json` `model_tier_overrides` | tier escalation is policy, not a Go literal; the profile already declares `audit_retry_2plus: deep` — the fix supplies the missing producer signal, not a new flag |
| **Single source with projection** | phase order taken from observed `phase-timing.json`, cycle paths from `core.RunWorkspacePath`, cycle-state path from `core.ResolveCycleStatePath` | the repo already carries phase order in three places (gap G9); the dashboard must not add a fourth belief |

**Clean-code limits.** Files < 800 LOC, functions < 50 lines, nesting ≤ 4, godoc on every export, ≥ 85 % line coverage, `apicover_named_test.go` naming and exercising every export, new package listed in `go/.apicover-enforce` in the same change (the build-phase graduation gate hard-fails otherwise). No file named `*_gate.go` or `*guard*.go` (protected-surface tripwire). `gofmt -l`, `go vet ./...`, `go test -count=1 ./...` green before any commit. Review fleet on every code diff: code-simplifier → architecture-reviewer ∥ go-reviewer; CRITICAL blocks.

**Security posture (dashboard).** Every rendered artifact is LLM-authored and therefore a prompt-injection / XSS channel. All content reaches the browser as JSON and is placed into the DOM via `textContent`, never `innerHTML`; server HTML is a static shell. Artifact reads go through `reportdoc.OpenRegularNoFollow` (symlink-safe), an allowlist on the file name (no separators, known extensions), and a 2 MiB cap. The server binds `127.0.0.1` by default and exposes no write endpoint. The loop's `.lock` sidecars are never opened.

---

## Sub-project A — Ship-rate hardening

### A.1 Measured state (runtime plane, 2026-09-02)

| Metric | Value | Source |
|---|---|---|
| Autonomous ship rate, cycles 1560–1605 (46 closed) | **9 / 46 = 19.6 %** vs SLO ≥ 60 % (`cyclehealth/outcome.go:20`, comment only) | ledger `role=ship` + dossiers |
| Current zero-ship streak | 1595–1605 = **0 / 11** | `recent-outcomes.md` |
| Failures landing at `audit` | 30 / 37 = **81 %** | dossier `failure.fingerprint` phase |
| Recent failures classified pipeline-/harness-owned (not task) | 6 / 10 `legit-rejection` carry `layer ∈ {pipeline-code, harness}` | `disposition.json` |
| Ship probability by audit rounds | 1 round **100 %** · 2 rounds 50 % · 3 rounds 17 % · 4+ **0 %** | `audit_dispatches` × outcome |
| Builder over-claim as contributing cause | 4 / 9 most recent FAILs (1604, 1605, 1597, 1595) | `audit-report*.md` |
| Attestation/SHA mismatch as contributing cause | 3 / 9 (1603, 1604, 1605) | `audit-fail-reason.json` |
| Build phase model/effort | `codex-tmux` · tier `balanced` · `effort_level: medium` on all repair rounds | `llm-calls.ndjson`, `profiles/builder.json` |

### A.2 Verified architectural gaps (source-read, not inferred)

- **G1 — escalation declared but unreachable.** `profiles/builder.json` declares `model_tier_overrides.audit_retry_2plus: "deep"`, but `subagent/modeltier.go:166` `activeSituation` returns `""` for every cycle > 1, and `subagent/run.go:266` never passes the repair-attempt count into `ResolveModelTierRequest`. Three tests guard the key's *value*; none guard that it *fires*. Every repair round re-dispatches at the identical tier and effort.
- **G2 — the auditor's findings never reach the builder.** `core/repair_eligibility.go:81` seeds the repair brief from `audit-fail-reason.json` only — deterministic gate strings. The HIGH findings with `path:line` evidence in `audit-report.md` (`### H1 (HIGH) — …`) are dropped. Cycle 1605's H1 survived three rounds; the auditor recorded the builder mistaking the gate string for a documentation nit. Cycle 1596's fourth-round builder received one truncated defect.
- **G3 — acceptance criteria are two hops from the prompt.** `build-prompt.txt` carries the task slug only; the builder must open `triage-report.md` then `.evolve/evals/<slug>.md`. Nothing checks comprehension.
- **G4 — tdd→build contract is unvalidated prose** (`agent-mailbox.md`); the ACS predicate names appear nowhere in the build prompt.
- **G5 — every deterministic check fires after the builder's full budget**, at audit: `apicover -enforce`, explanation-SHA binding, `Closes-Inbox` proof, caller-proof. Each over-claim costs a full audit round to detect.
- **G6 — the learning loop is write-only.** ~650 lesson YAMLs; `instinctSummary`, which three personas instruct agents to consult, has zero producers in Go; `.evolve/genes/` is empty.
- **G7 — `evalgate` fails open on ambiguity** (`evalgate/reviewer.go:11-18`); a selected slug shipped with no eval file (1605 H2).
- **G8 — no capability-aware prompt adaptation**; prompt bytes are identical across tiers.
- **G9 — phase order defined in three places** (`phaseorder.go:17`, `router.go:18`, `phase-registry.json`).
- **G10 — repair-round prompts are overwritten in place**; only verdict/report rounds are archived.
- **G11 — `policy.json` advertises pins it does not contain.**

### A.3 The diagnosis in one sentence

The harness verifies only at the end and repairs by repeating — same model, same effort, same prompt shape, minus the actual finding — while the contract sits two hops from a weaker model's prompt. The literature is unambiguous that this cannot converge: models do not self-correct without external, specific, execution-grounded feedback; repair gains concentrate in the first two rounds and then require changing the *inputs* (feedback, context, or tier); and interface design moves weak-model success as much as model choice (see research synthesis §2).

### A.4 Ranked proposals (impact × confidence ÷ cost)

| # | Proposal | Closes | Evidence | Cost | Disposition |
|---|---|---|---|---|---|
| **R1** | **Arm repair-round escalation.** Plumb `CycleState.AuditRepairAttempts` → `subagent.RunRequest` → `ResolveModelTierRequest`; `activeSituation` returns `audit_retry_2plus` when attempts ≥ 1. Add a sibling `effort_overrides` map to the profile so effort escalates with tier. Regression test: a round-2 build resolves to `deep`; round-1 stays `balanced`. | G1 | cascades (FrugalGPT, MoT), "gains concentrate in two rounds", architect/editor split | S | **implement now** |
| **R2** | **Repair brief carries the auditor's findings + reflection memory.** `seedAuditRepairContext` additionally parses `### <id> (<SEVERITY>…) — <title>` sections from the final `audit-report.md` (HIGH/CRITICAL first, capped), and prepends "round N−1 findings that persisted". Archive `build-prompt.round<N>.txt` / `tdd-prompt.round<N>.txt` beside the existing round archives. | G2, G10 | Self-Debug (+12 pp execution feedback vs +2–3 prose), Olausson (feedback quality is the bottleneck), Reflexion, AgentLens "blind retries" | S–M | **implement now** |
| R3 | **Build-exit deterministic floor (lint-on-edit at phase granularity).** At build→audit, run the cheap gates the auditor would run (apicover on touched pkgs, explanation-SHA, Closes-Inbox proof presence, new-export production-caller scan, clean/untracked-test check); RED re-enters build with the exact gate text, ≤ 2 micro-rounds, no audit dispatch. Extend the existing `buildGraduationCheck` seam. | G5 | SWE-agent lint-on-edit +3 pp / guardrails hasten recovery; hooks deterministic vs advisory | M | inbox 0.90 |
| R4 | **Inline the contract.** Build prompt includes acceptance criteria verbatim, ACS predicate names (from `go test -list` over `go/acs/cycle<N>`), and a schema-validated `handoff-tdd.json` replacing the mailbox as authority. | G3, G4 | SWE-agent ACI, Spec Kit / Kiro EARS testable criteria, Anthropic feature-list-as-data | M | inbox 0.88 |
| R5 | **Completion minted by the harness.** Builder reports per-criterion `{id, status ∈ met/partial/blocked, evidence}`; the checker writes `Closes-Inbox` only when every criterion re-verifies; `partial` is a first-class honest outcome routed to carryover without penalty. | over-claim class | Kalai (binary grading rewards guessing), Confident-and-Wrong, Anthropic `passes` flag | M–L | inbox 0.88 |
| R6 | **Capability-aware scaffolding.** Per-tier `prompt_scaffold` in profiles: balanced/fast tiers get a numbered DoD with mandatory evidence lines; deep stays terse. Measure effort per phase; small models are non-monotonic in effort. | G8 | SWE-agent "interface > model", effort non-monotonic study | S–M | inbox 0.80 |
| R7 | **Learning re-entry.** Produce `instinctSummary` (top-k lessons by file/keyword overlap with the selected task) and inject it where the personas already look for it. | G6 | Reflexion memory; Anthropic progress-file pattern | M | inbox 0.78 |
| R8 | **Verifier isolation.** Gates evaluate the committed tree in a detached worktree the builder cannot write; attestation SHAs minted by the harness only. | attestation class | Berkeley agent-eval checklist (conftest exploit), SWE-bench harness | M–L | inbox 0.75 (largely in place; audit the residual writable seams) |

Dropped as unnecessary: a new durable outcome ledger — `knowledge-base/cycles/cycle-N.json` already carries `final_verdict`, `commit_sha`, and `failure.fingerprint` durably (1840 files); the dashboard computes the ship rate from it.

### A.5 R1 + R2 design (implemented this session, separate PR)

**R1.** `subagent.RunRequest` gains `AuditRepairAttempts int` (additive, zero = no repair). `core` populates it from `cs.AuditRepairAttempts` at the single dispatch chokepoint that builds the request. `ResolveModelTierRequest` gains the same field; `activeSituation` returns `"audit_retry_2plus"` when `AuditRepairAttempts >= 1` (second dispatch onward — the key's name is historical), else the existing cycle-1 rule. Effort: `profiles.Profile` gains `EffortOverrides map[string]string` (same situation keys); `bridge/launch.go` resolves effort through the same situation. Envelope clamps (`ModelTierEnvelope.Max`) still apply. Tests: table-driven over attempts {0,1,2} × profile declares/omits key; a live-wiring test asserting the request built for a round-2 build carries attempts=1.

**R2.** New `core/audit_findings.go`: `parseAuditFindings(markdown) []auditFinding{ID, Severity, Title}` over `## Issues` headings (regexp `^###\s+([A-Z]\d+)\s*\(([A-Z]+)[^)]*\)\s*[—-]+\s*(.+)$`), tolerant of the two observed heading shapes. `seedAuditRepairContext` renders: gate reasons (existing) + "Auditor findings (final round)" HIGH/CRITICAL first, MEDIUM after, LOW omitted, capped by the existing `truncateFindings` budget + "Findings that persisted from the previous round" computed by title match against `audit-report.round<N-1>.md`. Round archival: the pre-dispatch seam that already retires `acs-verdict.json`/`audit-report.md` (`supersedePreviousAuditRound`) gains the symmetric archival of `build-prompt.txt`/`tdd-prompt.txt` as `.round<N>.` copies. Tests: parser table (both heading shapes, no Issues section, LOW-only), seed renders findings and persisted-set, archival wiring pin.

---

## Sub-project B — Live pipeline dashboard

### B.1 Goals / non-goals

**Goals.** A human opens one local page and, within 30 seconds, knows: is the loop alive and what is it doing right now; what is queued; for each recent cycle what was done, whether it shipped, and if not, exactly what went wrong (category, fingerprint, recurrence, cited findings, repair-round history, salvage pointer); and how the ship rate is trending. Updates arrive live without refresh.

**Non-goals.** No writes (no brake toggle, no inbox edits). No database. No auth. No build step, framework, or new dependency. No markdown renderer (escaped text with visual heading hints). No OTel export. No per-fingerprint detail page. Not a replacement for `evolve cycle timing` / `soak-report` (they remain the scripted surfaces).

### B.2 Architecture

```
evolve dashboard [--project-root P] [--addr 127.0.0.1:8090] [--snapshot]
        │  (cmd_dashboard.go: flags → dashboard.New(root, Options).ListenAndServe)
        ▼
go/internal/dashboard
  collect.go   Collect(root, now, cache) → Snapshot        ← Reader/Repository over:
  loop.go        LoopStatus  ← cycle-state.json (core.ResolveCycleStatePath), .lease (runlease.Read/Fresh), .evolve/loop-stop
  queue.go       []QueueItem ← .evolve/inbox/*.json (inboxbatch.LoadDir) + consumed/processing/retry counts
  cycle.go       CycleSummary ← runs/cycle-N/{run.json, phase-timing.json, llm-calls.ndjson, triage-decision.json}
  failure.go     Failure ← failure-decision.json, disposition.json, audit-fail-reason.json, audit-report(.roundN).md
  history.go     Trend + []FingerprintStat ← knowledge-base/cycles/cycle-N.json (dossier.ParseJSON; mtime cache)
  artifact.go    ReadArtifact(ws, name) ← allowlist + reportdoc.OpenRegularNoFollow + 2 MiB cap
  server.go      Server{root, opts}; Handler(): GET / · /cycle/{n} · /api/snapshot · /api/cycle/{n} · /api/artifact/{n}/{name} · /events · /static/
  sse.go         poll fingerprint (mtimes) → broadcast `event: snapshot` `id: seq`; `: ping` keepalive; Flush via http.ResponseController
  static/        index.html, app.js, app.css (embed.FS; DOM built with textContent only)
```

**Data flow.** Disk → `Collect` (pure over a root; every read best-effort, `readJSONArtifact`-style bool returns) → immutable `Snapshot` → JSON at `/api/snapshot` → `app.js` renders. `/events` pushes a tiny notice when the fingerprint changes; the client re-fetches. Detail page fetches `/api/cycle/{n}` (rounds, findings, phases) and lazily `/api/artifact/{n}/{name}` per tab.

**Liveness semantics.** `Running` = current cycle's `.lease` is `runlease.Fresh(…, DefaultTTL)`; never a PID check. `BrakeEngaged` = `.evolve/loop-stop` exists. State type (closed enum, Prefect-style type vs name): `running | pass | warn | fail | halted | incomplete`. `halted` = `failure-decision.level == system`; `incomplete` = no dossier and lease stale (paused/crashed); name shown beside it (e.g. `incomplete · paused (brake)`).

**"What went wrong" panel (per FAIL cycle).** Row 1 pills: category · level · action/fix_type (`failure-decision.json`). Row 2: fingerprint (mono) · `seen N× · first #A · last #B` · REGRESSED badge when the fingerprint reappears after a PASS cycle. Row 3: `disposition.root_cause.summary` + legitimacy + layer. Row 4: gate reasons (`audit-fail-reason.json`). Row 5: auditor findings of the final round (id · severity · title), then repair-round history `r1 FAIL (7) → r2 FAIL (3: 5 resolved, 1 new) → final FAIL (2)` diffed by normalized title. Row 6: salvage pointer + links to raw artifacts.

**Board page.** Header tiles (running / queued / pass / fail / halted, last-20 ship rate, all-time). Loop card (cycle, phase, elapsed, CLI·tier, lease age, brake). Queue (pending items by weight, kind, route). Cycles grid (rows = phases in first-seen order, columns = last N cycles, cell colour = verdict, `rN` glyph for audit rounds, click → detail). Trend strip (verdict per cycle, last 60).

### B.3 Data model (exported, minimal)

```go
type Snapshot struct { GeneratedAt time.Time; Root string; Loop LoopStatus; Queue QueueSummary; Cycles []CycleSummary; Trend Trend; Fingerprints []FingerprintStat }
type LoopStatus struct { Running, BrakeEngaged bool; CycleID int; Phase string; PhaseStartedAt, LeaseHeartbeat time.Time; ActiveWorktree string; CLI, Tier string }
type QueueSummary struct { Pending []QueueItem; Consumed, Processing, Retry int }
type QueueItem struct { ID, Title, Kind, Route string; Weight float64; FailureCount int }
type CycleSummary struct { ID int; State string; Verdict, CommitSHA string; StartedAt, EndedAt time.Time; Phases []PhaseRun; AuditRounds int; Tasks []string; Failure *Failure; Tokens int; HasWorkspace, HasDossier bool }
type PhaseRun struct { Phase, Verdict, CLI, Model, Archetype string; StartedAt, EndedAt time.Time; DurationMS int64; Attempt int; Tokens int }
type Failure struct { Category, Level, Action, FixType, Fingerprint, PreClass, Legitimacy, Layer, RootCause, Salvage string; GateReasons []string; Findings []Finding; Rounds []AuditRound }
type Finding struct { ID, Severity, Title string }
type AuditRound struct { Round int; Verdict string; Findings []Finding; Resolved, New, Carried int }
type Trend struct { Points []TrendPoint; ShipRateLast20, ShipRateLast50, ShipRateAll float64; Closed, Shipped int }
type TrendPoint struct { Cycle int; Verdict string; Shipped bool }
type FingerprintStat struct { Fingerprint, PreClass string; Count, FirstCycle, LastCycle int; Regressed bool }
```

### B.4 Error handling

Absent or unparsable artifact ⇒ field zero-valued, cycle still rendered; a `Warnings []string` on `Snapshot` lists what could not be read (surfaced in a collapsed footer, never a red page). A half-written JSON (rename in flight) is treated as transient: the previous snapshot stays until the next tick. SSE handler exits on `r.Context().Done()`; server `WriteTimeout` stays 0 (a non-zero value kills streams). Artifact endpoint returns 404 for names outside the allowlist, 413 over the cap.

### B.5 Testing

Unit (fixtures in `t.TempDir()`): `loop` (fresh vs stale lease, brake), `queue` (pending + lifecycle counts), `cycle` (phases order from timing, routing from llm-calls, rounds count), `failure` (both heading shapes, round diff, missing files), `history` (ship-rate windows, regressed fingerprint, cache hit on unchanged mtime), `artifact` (allowlist reject, symlink reject, cap). Server (`httptest`): every route 200 with fixture root, 404/413 paths, SSE emits `event: snapshot` after a fixture mutation within a shortened poll interval and stops on ctx cancel. Cmd: `--snapshot` prints JSON with the cycle ids; bad flag → 10. `apicover_named_test.go` names every export. Live check: run against `runtime/` (loop paused, 18 run dirs, 920 dossiers) and read the page.

### B.6 Documentation

ADR-0095 (read-only observability surface; stdlib-only; polling not fsnotify; no write endpoints — with the alternatives rejected). `docs/guides/pipeline-dashboard.md` (how to run, what each panel means, how to read the failure panel). `docs/operations/runtime-reference.md` operator-command bullet. `CHANGELOG.md` Added entry. `docs/research-index.md` links to the three research files.

---

## Decisions recorded for operator override

1. Both sub-projects proceed in this session under autonomous mode; the dashboard first (the explicit build request), then R1+R2 as a second PR, then inbox items for R3–R8.
2. The loop stays paused throughout; the dashboard is validated against the paused runtime plane's on-disk state.
3. Stdlib-only, polling, no markdown rendering, loopback-only, read-only — all chosen for security and for the vendored dependency policy; each is reversible later without touching the data model.
4. Ship-rate history comes from committed dossiers, not a new ledger.
