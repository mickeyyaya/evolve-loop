# Architecture Audit — Package Map, Coupling & Dedup Register (2026-06-15)

> The "audit-all" deliverable for the modularization campaign (ADR-0050, plan
> `happy-petting-wreath.md` Phase 0.3). Every number below is **measured**, not estimated, against
> the worktree at `main @ 0f8de059`. Reproduction commands are recorded per section so the audit
> is re-runnable. This map drives the leaf→core ordering and the dedup slices.

## 0. Summary

| Metric | Value | Note |
|--------|-------|------|
| Internal packages | **126** | `go list ./internal/... \| wc -l` |
| Non-test LOC under `internal/` | **62,832** | excludes `*_test.go` |
| Files >800 LOC (non-test, excl `acs/`) | **0** | the 800-LOC ceiling is already met — Phase 4 god-splits are DONE (PRs #100/#101) |
| Largest file | `phases/ship/gitops.go` **758** | next: `releasepipeline.go` 749, `core/phase_advisor.go` 708 |
| Largest package (non-test LOC) | `internal/core` **7,826** | then `bridge` 7,094, `phases/ship` 3,263 |
| Exported interfaces (`internal/`) | **37** | the "ports" surface; core's 5 in `ports.go` |
| `sync.Mutex/RWMutex` fields (non-test) | **44** | Phase 2 stress-test surface |
| Files using flock/`WithPathLock` | **13** | safety-critical serialization |

**Headline:** the orchestrator god-file decomposition this campaign originally targeted (Phase 4)
**is already merged** — no file exceeds 800 LOC. The remaining work is genuine: **leaf extraction
+ dedup** (Phase 1), **concurrency-test backfill** (Phase 2), **unified phase I/O** (Phase 3), and
**100% public-API coverage** (Phase 5).

---

## 1. Package inventory — largest 30 by non-test LOC

> `for d in $(go list ./internal/...); do wc -l <non-test .go in $d>; done | sort -rn`

| LOC | Package | Responsibility (one line) |
|-----|---------|---------------------------|
| 7826 | `internal/core` | cycle orchestration, dispatch, ports — the central hub |
| 7094 | `internal/bridge` | agent-bridge: LLM driver subprocess + tmux REPL + manifest |
| 3263 | `internal/phases/ship` | ship phase: gitops, audit-binding, repair ladder, verify |
| 2693 | `internal/subagent` | subagent run/dispatch (native bridge entry) |
| 1745 | `internal/router` | dynamic routing, `Digest` (the handoff on-disk authority) |
| 1350 | `internal/swarm` | intra-cycle swarm: mergetrain, provision, partition |
| 1313 | `internal/looppreflight` | pre-batch readiness gate |
| 1099 | `internal/routingtest` | routing test fixtures (`HandoffFiles`) |
| 1064 | `internal/triagecap` | triage capability model |
| 955 | `internal/adapters/ledger` | **safety-critical** hash-chain ledger (flock) |
| 884 | `internal/phasestream` | phase event streaming |
| 858 | `internal/releasepipeline` | release pipeline (git-heavy) |
| 809 | `internal/phases/runner` | phase runner (printer-heavy) |
| 798 | `internal/interaction` | corrective interaction protocol (ADR-0045) |
| 761 | `internal/adapters/observer` | observer adapter (concurrent) |
| 708 | `internal/phasecontract` | deliverable-contract SSOT (ADR-0033/0034) |
| 658 | `internal/recovery` | phase recovery (ADR-0044) |
| 658 | `internal/cyclehealth` | cycle health telemetry |
| 653 | `internal/config` | `Config` — env→typed config (envchain target) |
| 642 | `internal/bridge/recipe` | per-CLI driver recipes |
| 629 | `internal/failurelog` | failure-lesson YAML store |
| 615 | `internal/phasespec` | **leaf** phase spec types (`phaseio` will import only this) |
| 598 | `internal/phaseobserver` | phase observer auto-spawn |
| 545 | `internal/releasepreflight` | release preflight (git) |
| 539 | `internal/modelquery` | model routing query |
| 528 | `internal/acssuite` | EGPS predicate harness |
| 520 | `internal/inboxmover` | inbox defect mover |
| 511 | `internal/guards` | **quota guard** (0 concurrency tests) + others |
| 509 | `internal/bridge/channel` | bidirectional channel (ADR-0037) |
| 507 | `internal/gc` | runtime artifact GC |

**Size distribution:** ≥1000 LOC: 9 pkgs · 500–999: 22 · 200–499: 50 · <200: 45.

---

## 2. Coupling map — fan-in / fan-out (direct imports)

> `go list -f '{{.ImportPath}}\t{{join .Imports " "}}' ./...` then invert.

| Package | Direct fan-in (importers) | Note |
|---------|---------------------------|------|
| `internal/core` | **33** | the hub; depended on widely. Its own fan-out = **19** internal packages. |
| `internal/phasespec` | 11 | the dependency-free leaf `phaseio` will sit beside. |
| `internal/bridge` | 10 | second hub. |
| `internal/envchain` | 8 | **partially adopted** typed-env leaf (Phase 1.3 completes it). |
| `internal/router` | 7 | owns `Digest` (handoff reader). |
| `internal/paths` | 2 | under-adopted (Phase 1.4). |
| `internal/log` | 2 | under-adopted (Phase 1.5 promotes to unified logger). |
| `internal/gitexec` | **0** | does not exist yet (Phase 1.1 creates it green, zero callers). |

**Implication for ordering (D1):** `core` (fan-in 33) and `bridge` (fan-in 10) are the hubs —
touched **last** in each dedup track, only after their leaf dependencies (`gitexec`, `envchain`,
`paths`, `log`) are adopted by the lower-fan-in packages. This keeps every hub file edited once.

---

## 3. Files >800 LOC

**None.** The 800-LOC ceiling (P1) is already satisfied repo-wide. Largest 5 (all under ceiling):

| LOC | File | Phase-4 disposition |
|-----|------|---------------------|
| 758 | `internal/phases/ship/gitops.go` | git-using; not split-required. |
| 749 | `internal/releasepipeline/releasepipeline.go` | git-using leaf (Phase 1.x gitexec candidate). |
| 708 | `internal/core/phase_advisor.go` | OPTIONAL split (Phase 4.6) only if touched for coverage. |
| 703 | `internal/bridge/driver_tmux_repl.go` | under ceiling (was 1089, split in PR #99). |
| 657 | `internal/router/router.go` | fine. |

---

## 4. Dedup register (the duplication surface)

> Counts are non-test occurrences. Commands recorded inline.

### 4.1 `gitexec` (Phase 1.1 + 4.5) — **isolate the git CLI**
`exec.Command("git", …)` appears in **16 files** (`grep -rlE 'exec\.Command\(\s*"git"'`); the
broader git-shelling set is **25 files**. The **4 `core` files** that still shell git (Phase 4.5,
migrated LAST):

- `internal/core/git_porcelain.go` · `correction_ladder.go` · `resume.go` · `worktree.go`

Leaf callers to migrate first (Phase 1.2–1.x): `rollback`, `versionbump`/`changeloggen`, `swarm`
(`mergetrain.go`, `provision.go`), `cycleclassify`, `preflight`, plus `releasepipeline`,
`releasepreflight`, `marketplacepoll`, `resolvellm`, `subagent`, the 5 `phases/ship/*` git files.

### 4.2 `envchain` (Phase 1.3) — **typed env registry**
Raw `os.Getenv` / `os.LookupEnv`: **161 sites across 56 files** (`grep -rhoE
'os\.(Getenv|LookupEnv)\('`). 8 packages already on `envchain`. First slices (no core risk):
`cmd/evolve/cmd_subagent.go`, `cmd_phase_observer.go`, `cmd_fanout_dispatch.go`,
`internal/config/config.go`, `internal/phases/ship/native.go`.

### 4.3 `log` unified logger (Phase 1.5 + 4.5) — **highest dedup value**
`fmt.Fprintf/Fprintln(os.Stderr, …)`: **164 sites**, of which **123 are in `internal/core`**
(`grep -rhoE 'fmt\.Fprint(f|ln)?\(os\.Stderr'`). Non-core printers migrate first (`phases/runner`,
`releasepipeline/bridges`, `bridge/manifest`, `verdictcache`, `clihealth`); core's 123 deferred to
Phase 4, bundled per-section.

### 4.4 `paths` (Phase 1.4) — **`.evolve` layout**
Targets: `bridge/manifest.go`, `research/kb.go`, `phases/ship/gitops.go` cycle-state path.

### 4.5 `jsonio` (Phase 1.6, OPTIONAL — do NOT force)
`os.ReadFile`: **561 sites** · `json.Unmarshal`: **314 sites**. Per the plan, a blanket sweep is
**low-signal/high-churn** — adopt only when a slice naturally touches a read cluster; log the
decision either way.

---

## 5. Existing-seam inventory (what's already a clean port)

`internal/core/ports.go` defines the 1–3-method ports the campaign preserves (P1):

| Port | Methods (shape) | Adapter |
|------|-----------------|---------|
| `Storage` | state read/write | `internal/adapters/storage` |
| `Ledger` | append + iterate | `internal/adapters/ledger` (hash-chain) |
| `LedgerIterator` | iterate | — |
| `Bridge` | dispatch a phase | `internal/bridge` |
| `Guard` | phase/role/ship gates | `internal/guards` |

37 exported interfaces exist across `internal/` total — these are the seams new modules accept
(`accept interfaces, return structs`). The phase envelope's `Handoffs` will be **built on**
`router.Digest` (the single on-disk-shape reader) rather than introducing a second seam.

---

## 6. Concurrency primitives (Phase 2 stress-test surface)

44 `sync.Mutex/RWMutex` fields + 13 flock/`WithPathLock` files. Packages holding a lock (stress
backfill candidates): `adapters/{flock,ledger,observer,storage}`, `bridge`, `checkpoint`, `core`,
`dispatchevents`, `fanoutdispatch`, `guards`, `interaction`, `log`, `phaseobserver`,
`phases/registry`, `phases/ship`, `swarm`.

**Verified gaps (the plan's Phase 2 priorities — confirmed by grep, not assumed):**
- `internal/adapters/ledger` — **0** `*Concurrent*`/`*NoRace*` tests on a safety-critical hash
  chain (S2.1, highest priority).
- `internal/guards` (`quota.go`) — **0** concurrency tests (S2.2).
- `internal/log/events.go` `SidecarWriter` — needs `TestSidecarWriter_ConcurrentEmit_NoTornLines`
  (S2.3).

A `TestEveryMutexHasStressTest` AST-walk (generalizing `acssuite/tagguard_test.go`) will enforce
this invariant going forward (S2.4).

---

## 7. Reproduction

All counts above regenerate from the worktree `go/` directory with the one-liners quoted per
section (`go list`, `find … -name '*.go' -not -name '*_test.go' … wc -l`, `grep -rlE/-rhoE`). The
fast test baseline at audit time: `make test` GREEN, 0 failures, 813 test files / 5,292 test
functions (fast tier ≈4,654; `acs/` 294; integration/e2e 344).
