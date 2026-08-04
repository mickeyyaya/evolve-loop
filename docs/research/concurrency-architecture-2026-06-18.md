# Concurrency Architecture — Sibling-Worktree Model (implementation spec)

> Spec for the remaining slices of the "concurrent `evolve loop` never collides"
> architecture. Slice 1 (runscope) is DONE on this branch's base. This doc is the
> reference for the evolve loop implementing Slices 2–6, and the seed for the
> Slice-6 ADR. Canonical model: **M independent `evolve loop` processes, each in
> its own git worktree of one repo, each with its own `.evolve/`**. Two layers:
> **Layer 1 = namespace** (branch/dir — DONE via `internal/runscope`), **Layer 2 =
> shared host runtime** (tmux sessions, LLM CLIs, ~/.codex, ports — the 2026
> industry "bigger killer that worktrees do NOT isolate").

## Hard rules for every slice
- **TDD red-first**; a regression test per fix; run `-race -tags integration`.
- **Clean code + design patterns**; **reuse, don't rebuild** (list below).
- A NEW `internal/<pkg>` MUST be enrolled in `go/.apicover-enforce` AND satisfy
  the acs `TestApicoverEnforce_CoversEveryInternalPackage`: every export named in
  a same-package test + godoc + each `.go` < 800 LOC + ≥85% coverage.
- Ship each slice through the gate to THIS branch only (`concurrency-arch-slices`);
  never main. One slice per cycle, in the dependency order below.

## Reuse (do NOT rebuild)
- `internal/runscope` — the Layer-1 naming value object (already on this branch).
- `internal/runlease` — per-run `.lease` heartbeat (`Write/Read/Fresh`, `DefaultTTL`).
- `internal/sessionrecord` — per-run tmux registry (`ReadAll`, `PathIn`, `RunScopeToken`).
- `internal/swarm/reap_runsessions.go` — `ReapRunSessions` (suicide-safe per-registry killer) + `ExecTmuxKill`.
- `internal/adapters/flock` — `WithPathLock` (cross-process file lock).
- `internal/atomicwrite` — crash-safe temp+rename.
- `internal/gc/discover.go` — the evidence-walk template (qualify a run dir by its registry file, never parse `cycle-N`).
- `cmd/evolve` `fleet --simulate` + `internal/cyclesimulator` + `internal/soakreport` — the soak substrate.

---

## Slice 2 — codex pretrust concurrent regression test (dependency order: 1st, trivial)
The `~/.codex/config.toml` read-merge-write race is ALREADY fixed on main
(`internal/bridge/codex_pretrust.go` wraps the RMW in `flock.WithPathLock`). This
slice adds the MISSING named regression test, no production change.
- New `internal/bridge/codex_pretrust_concurrent_test.go`: two goroutines each
  pretrust a DISTINCT worktree path against ONE `EVOLVE_CODEX_CONFIG_PATH` temp
  file concurrently; assert the final file is valid TOML and contains BOTH
  `[projects."…"]` entries (a lost update would drop one). Run under `-race`.
- Use the existing `flockFn` seam if needed to force contention deterministically.

## Slice 3 — sessionreaper: Tier-3 liveness orphan reaper (2nd; HIGHEST value)
Tier-1 (per-launch `defer tmuxCleanup`) and Tier-2 (per-cycle `reapCycleSessions`
→ `ReapRunSessions`) already exist on main. The gap: a run SIGKILL'd mid-cycle
leaks its sessions; today `internal/looppreflight/checks.go` only globs
`evolve-bridge-*` and WARNs (a latent footgun one step from killing live peers).
- New leaf pkg `internal/sessionreaper`:
  `ReapOrphans(ctx, evolveDir string, o Options) (Report, error)` — walks
  `<evolveDir>/runs/*/tmux-sessions.jsonl`; for each run dir reads its `.lease`;
  **if `runlease.Fresh(lease, now, ttl)` → skip (LIVE peer)**; else
  `swarm.ReapRunSessions(registryPath, o.Kill)`. `Options{Now func()time.Time;
  LeaseTTL time.Duration; Kill swarm.TmuxKiller}`. `Report{LiveRunsSkipped int;
  Orphaned []OrphanReap}`. Mirror `gc/discover.go`'s evidence-walk.
- **Safety invariant (the whole point):** never reap a session whose owning run's
  `.lease` is Fresh. Registry = which sessions belong to a run; lease = whether
  it's alive. Defense-in-depth: `ReapRunSessions` re-applies the empty/
  out-of-namespace guard.
- Wiring (safe-default): (a) replace the `looppreflight` glob-WARN with
  `sessionreaper.ReapOrphans` (it can only ever kill dead runs); (b) add
  `evolve swarm reap-orphans [--dry-run]` operator backstop.
- Patterns: lease-as-liveness, Reaper Strategy (`swarm.TmuxKiller` injected),
  evidence-walk. TDD: fake `TmuxKiller` records `(reaper, killed)`; assert a
  fresh-lease run's sessions are NEVER killed; a stale/absent-lease run's ARE.

## Slice 4 — cliadmit: cross-process LLM-CLI admission control (3rd; opt-in)
M loops hammer the shared codex/claude/agy CLIs → rate-limit failures. No
cross-process cap exists (the `swarm/dispatcher.go` semaphore is in-process).
- New leaf pkg `internal/cliadmit`:
  `Acquire(ctx, cli string, max int, ttl time.Duration) (release func(), err error)`
  — a flock'd holder-set JSON (`{pid, heartbeat}[]`) under
  `$XDG_RUNTIME_DIR/evolve/cli-<name>.slots` (fallback `os.TempDir()` if unset);
  on Acquire: lock, prune holders whose heartbeat is older than ttl (self-healing
  for a crashed holder — lease-as-liveness), admit if `len < max`, else
  block-with-backoff. `max<=0` → unbounded → **byte-identical to today (safe
  default, no behavior change)**.
- Dial `EVOLVE_CLI_MAX_CONCURRENT_<CLI>` (e.g. `EVOLVE_CLI_MAX_CONCURRENT_CODEX=2`).
- Hook in `internal/bridge/driver_tmux_repl.go` just before session creation:
  `release, err := cliadmit.Acquire(...); defer release()`. A failed acquire
  degrades to "proceed uncapped + WARN" — admission control must NEVER block a
  phase outright. Consult skill `api-rate-limiting-throttling`.
- TDD: holder-set respects `max` under `-race`; stale holder pruned by ttl;
  `max<=0` is unbounded; release frees a slot.

## Slice 5 — fleet soak harness: the system-level proof (4th; depends on Slice 3)
- `evolve fleet soak --count N` (new `cmd/evolve/cmd_fleet_soak.go`) reusing the
  existing `fleet --simulate` substrate (real storage/ledger/lease, no-LLM
  phases), then assert the four invariants and render a `soakreport`-style table:
  1. **distinct branches** across N runs (collect each run's branch);
  2. **distinct + fully-reaped sessions** — after the fleet, every
     `tmux-sessions.jsonl` has pairwise-distinct names AND
     `sessionreaper.ReapOrphans` finds **zero** live orphans;
  3. **no cross-run reap** — an injected `TmuxKiller` fake records every
     `(reaper-run, killed-session)`; assert no reaper killed a session outside
     its own registry (the killer-B regression, under concurrency);
  4. **no torn shared-config** — `~/.codex/config.toml` (test path) is valid TOML
     containing every run's entry.
- The soak test runs under `go test -race`. This proves Slices 1–4 compose.

## Slice 6 — ADR + docs (5th; last)
- New `docs/architecture/adr/00NN-concurrent-evolve-loop-sibling-worktrees.md`:
  the sibling-worktree model, the two layers, the `runscope` value object, the
  prior art (worktree-per-agent 2026 standard; stable-path+lane+run-id-in-sidecar
  — GitLab CI_CONCURRENT_ID / Jenkins @N / Temporal), and the explicit
  relationship to ADR-0049 (the *fleet* model — a DIFFERENT topology with one
  shared `.evolve/`; this is not that).
- Update `docs/operations/runtime-reference.md` with the new dials: `EVOLVE_LANE`,
  `EVOLVE_REAP_ORPHANS` (if used to gate Slice 3), `EVOLVE_CLI_MAX_CONCURRENT_*`.
- Register any new flags in `internal/flagregistry/registry_table.go`.

## Out of scope
- The fleet model (ADR-0049 S0–S7) — different topology. runscope's
  `cycle-<lane>-<N>` is collision-free under fleet too (same root → distinct N),
  so adopting fleet later needs no Layer-1 rework.
- Ports / `/tmp` — rarely collide in the sibling model; ADR note only.
