# Flag-Consolidation Campaign — design-pattern-driven reduction to < 30 (2026-06-19)

> ## 🎯 TARGET UPDATED 2026-06-19 (post v20.0.0): ZERO operator feature flags
>
> New HIGHEST standing rule from the user: **"any feature flags are not allowed — use design
> patterns to solve cross-component functionality."** The target is no longer `<30` — it is the
> operator flag registry driven to **0 rows**. Everything below about cluster-consolidation,
> the ratchet, and anti-gaming still applies; the extensions for ZERO are:
>
> - **Persistent dials/gates/stages → config-as-code** (typed `.evolve/policy.json` structs +
>   `FooConfig()` resolver; reuse `FanoutConfig()/ObserverConfig()` shape). The `os.Getenv`/
>   `envchain` read is DELETED; value flows from `policy.json`.
> - **Per-phase `*_CLI`/`*_MODEL`/`*_PERMISSION_MODE` → profiles SSOT + `policy.Agents map[string]Pin`.**
> - **Test seams → Dependency Injection** (read moves into `_test.go`; reuse `sysexec.RunFunc`,
>   `gitexec.Git`, `bridge.engineFactory`, `loopCycleRunner`).
> - **Transient emergency (BYPASS_*, SKIP_*) → explicit CLI flags** (struct param, not env).
> - **Subprocess IPC handoffs → documented protocol** via split-const `"EVOLVE_"+"..."` +
>   `// SSOT IPC-protocol-allowed:` comment; removed from the registry (protocol, not a flag).
> - **Bootstrap locators (PROJECT_ROOT/PLUGIN_ROOT/WORKTREE_ROOT) → CLI flags** + split-const
>   env fallback; removed from the registry (documented bootstrap, not a feature flag).
> - **Hybrid gates:** persistent → `policy.json`; transient → CLI flags (Fowler config-vs-ops split).
>
> **Load-bearing cycle:** invert `config.Load` from reading `EVOLVE_*` env to a stdlib-only
> `RolloutInput` DTO built from `pol.RolloutConfig()` at `wireOrchestratorDeps` (keep `config`
> a leaf; reuse `parseStage`; delete `applyEnv` env reads + `legacyFlags`); atomic diff + parity
> contract test. **Sequence:** dead → per-agent → rollout-stages(+inversion) → phase-enable →
> budget/gc/quota → swarm/dispatch → workflow-defaults → recovery/latency(+DI) → bypass→CLI →
> IPC→split-const → bootstrap→CLI(last) → sweep to `FlagCeiling=0`.
> **Anti-gaming for ZERO:** "removing a config flag" MEANS deleting its read into a Config
> struct; split-const is ONLY for genuine IPC/bootstrap (with the justification comment).
> Plan of record: `~/.claude/plans/deep-dive-on-design-dazzling-russell.md`.

> **Campaign SSOT.** Supersedes the one-flag-at-a-time approach of
> `flag-reduction-campaign-2026-06-18.md` (which stalled: cycles 5–6 shipped nothing
> because the per-flag backlog was scraped dry). This campaign reduces by **cluster
> consolidation via design patterns**, not by deleting flags one at a time.
>
> Every cycle of the flag loop reads THIS doc as scout context. It defines the method,
> the priority order, the forcing function, and the integrity constraints.

## Goal

`go/internal/flagregistry/registry_table.go` currently has **262 registry rows**.
**Target: < 30 operator-facing flag rows.** That is ~232 rows removed.

This is not a delete-fest. The *capability* each flag controls stays; what is removed is
the **scattered `os.Getenv` override surface**. Per the project rule
`no_feature_flag_sprawl` (centralize config via Strategy/Specification/DI) and
`never_duplicate_centralize_via_design_patterns`, each flag becomes either a config-struct
field, a DI seam, or documented subprocess protocol — none of which is an operator flag row.

## Why cluster consolidation (the leverage argument)

The 262 rows are **not 262 independent knobs**. They are ~15 clusters of one subsystem
each. Retiring one flag per cycle needs ~232 cycles; consolidating one *cluster* per cycle
needs ~15. Uber's Piranha removed ~2000 flags by batch refactor, not one at a time
(see Research, below).

## Research — best practice (2026-06-19 web)

1. **Lean inventory limit** (Martin Fowler, *Feature Toggles*): "place a limit on the
   number of feature flags a system is allowed to have at any one time." → We encode this
   as a **ratchet test** (a "time-bomb" gate): `count <= CEILING`, CEILING ratcheted DOWN
   every cycle, terminal target 30. Progress becomes monotonic; regressions fail CI.
2. **Configuration Object + Builder** (Go config best practice): collapse scattered env
   reads into one config struct per subsystem — defaults in code, loaded once, optional
   single override source (`.evolve/policy.json`). "Centralizes configuration management
   into explicit objects rather than scattered environment variables."
3. **Piranha-style batch refactor** (Uber): automated cluster removal, not per-flag.
4. **Co-located config through CD** (Fowler): config lives with code and moves through the
   pipeline like any change — matches `.evolve/policy.json` + the registry SSOT.

Sources:
- https://martinfowler.com/articles/feature-toggles.html
- https://launchdarkly.com/docs/guides/flags/technical-debt
- https://docs.devcycle.com/best-practices/tech-debt/
- https://www.kaznacheev.me/posts/en/clean-way-pass-configs-go-application/
- https://medium.com/@perederei/the-ultimate-guide-for-the-builder-pattern-in-go-6f65e2ecc0a6

## Taxonomy → design pattern (the decision table the builder applies)

Every flag falls into exactly one bucket. Classify BEFORE touching it.

| Bucket | How to recognize | Pattern | Result |
|---|---|---|---|
| **Subsystem config cluster** | N flags sharing a prefix that tune ONE subsystem (`FANOUT_*`, `OBSERVER_*`, `INACTIVITY_*`, `CHECKPOINT_*`, `BRIDGE_*`, `QUOTA_*`) | **Configuration Object** — one struct, defaults in code, loaded once from `policy.json` | N rows → 0 (capability via struct field); keep at most ONE rollout dial if operator-facing |
| **Per-phase agent config** | `*_CLI` / `*_MODEL` / `*_PERMISSION_MODE` per phase (intent/scout/build/builder/audit/auditor/tdd/plan) | **Profile SSOT** (already exists: `.evolve/profiles/*.json` + `phaseconfig`) | delete redundant env override; profile is the source |
| **Test seam** | read ONLY to inject behavior in tests (`*_TEST_*`, `*_GO_BIN_TEST`, `*_TEST_EXECUTOR`, `OBSERVER_TEST_KEY`) | **Dependency Injection** — inject func/iface in the test | delete flag; no production env read |
| **Path / dir override** | `*_DIR`, `*_PATH`, `*_BASE`, `*_PIDFILE` | route through ONE path resolver (`runscope` / `sourceRoot`) | delete per-call env read |
| **IPC handoff** | parent process SETS, child subprocess READS, one handoff (`FANOUT_WORKER_TOKEN`, `FANOUT_WORKER_NAME`, `FANOUT_PARENT_AGENT`, `DISPATCH_DEPTH`, `ADVISOR_DEPTH`) | **Protocol, not config** — fold into runscope/sessionrecord token-passing OR move to a documented `internal/protocol` env set NOT in the operator registry | row leaves the operator registry (reclassified), capability preserved |
| **Bypass / gate toggle** | `BYPASS_*` | **single policy gate** (`policy.json` decision) | collapse the family into one gate decision |
| **Deprecated** | `StatusDeprecated`, no Go reader, superseded | retire | delete row + dead-reader, keep a one-line CHANGELOG note |

### ⚠️ Integrity constraint (the ultrathink guardrail)
**Do NOT blindly delete.** An IPC handoff env var deleted as if it were config will break
subprocess spawning. A test seam deleted without DI will break the test. Classify each flag
into exactly one bucket above and apply that bucket's pattern. The flagreaders guard +
`go test ./...` + the acs predicate are the proof the capability survived the consolidation.

### 🚫 ANTI-GAMING RULE (cycle-8 audit lesson — HARD requirement)
Cycle 8 was REJECTED by the auditor (H1 HIGH) for **metric-gaming**: it dropped 17 FANOUT
rows but, for the 8 *config* flags (`CONCURRENCY`/`TIMEOUT`/`CANCEL_ON_CONSENSUS`/
`CONSENSUS_K`/`CONSENSUS_POLL_S`/`TRACK_WORKERS`/`CACHE_PREFIX`/`TEST_EXECUTOR`), it merely
HID the `os.Getenv`/`envchain` reads from the flagreaders guard via the split-const trick
(`"EVOLVE_" + "FANOUT_..."`) — the override surface stayed LIVE at runtime. Row count fell
without real consolidation; the registry-completeness invariant was silently broken.

**The rule, non-negotiable:**
1. **"Remove a config flag" MEANS "delete its `os.Getenv`/`envchain` read."** The value must
   come from a `Config` struct loaded ONCE (defaults in code, optional single `policy.json`
   source) — NOT from a still-live env read hidden from the guard. If `grep -rn
   'os.Getenv\|envchain' ` still finds the flag's read after your cycle, you did NOT
   consolidate it — you gamed it. That is a HIGH defect, the cycle will FAIL audit.
2. **The split-const pattern (`"EVOLVE_" + "..."`) is ONLY for genuine cross-process IPC
   handoffs** (parent sets env → child subprocess reads it once, e.g. `FANOUT_WORKER_TOKEN`/
   `WORKER_NAME`/`PARENT_AGENT`). NEVER use it to make a config read invisible to the guard.
3. **acs predicates MUST assert override-surface removal, not just registry-absence.** A
   predicate that only checks "row gone + guard passes" is gaming-blind (cycle-8 M1). Add a
   predicate that asserts the `os.Getenv`/`envchain` read for each consolidated config flag
   is GONE (e.g. `FileNotContains` the read call-site / asserts the Config struct field is
   the sole source). The auditor verifies behavior, not row count.
4. The verdict is on SUBSTANCE: did the override surface actually move into a Config
   object? Row count and guard-pass are necessary but NOT sufficient.

## Priority order (largest clusters first — most leverage per cycle)

Cycle N takes ONE item. Recommended order (scout re-confirms live counts each cycle):

0. **Build the ratchet gate** (cycle 1) — see Forcing Function below. Nothing else ships
   until the gate exists, because the gate is what proves every later cycle made progress.
1. **Deprecated ×4** — `EVOLVE_FORCE_INNER_SANDBOX`, `EVOLVE_INNER_SANDBOX`,
   `EVOLVE_PROFILE_WORKTREE_AWARE`, `EVOLVE_REINVOKE_CMD`. ALREADY VALIDATED no-Go-reader /
   superseded (prior cycle-6 analysis). Quick win, do early.
2. **`FANOUT_*`** (~13) → `fanout.Config` struct (Configuration Object) + IPC handoffs
   (`WORKER_TOKEN`/`WORKER_NAME`/`PARENT_AGENT`) reclassified to protocol.
3. **Per-phase agent config** (`BUILDER_*`, `BUILD_*`, `SCOUT_*`, `INTENT_*`, `AUDIT_*`,
   `AUDITOR_*`, `TDD_*`, `PHASE_*` — the `*_CLI`/`*_MODEL`/`*_PERMISSION_MODE` rows) →
   Profile SSOT. This is the single biggest bucket (~30+ rows).
4. **`BRIDGE_*`** (~8) → `bridge.Config` (dirs via runscope path resolver).
5. **`OBSERVER_*`** (~7) → `observer.Config`.
6. **`INACTIVITY_*`** (~5) → fold into `observer.Config` (same subsystem).
7. **`CHECKPOINT_*`** (~4) → `checkpoint.Config`.
8. **`BYPASS_*`** (~6) → single `policy.json` gate decision.
9. **`SWARM_*`, `PLAN_*`, `RESUME_*`, `SKIP_*`, `QUOTA_*`, `DISPATCH_*`, `CODEX_*`,
   `ACS_*`, `ARTIFACT_*`, `MODELCATALOG_*`** — each its own Configuration Object cycle.
10. Sweep the long tail of singletons into the nearest subsystem config or DI.

## Forcing function — the inventory ratchet (cycle 1 deliverable)

Add `TestRegistry_FlagCeiling` in the `flagregistry` package:

```go
// FlagCeiling is the Lean inventory limit (Fowler). Ratchet DOWN every cycle that
// removes rows; never raise it. Terminal target: 30.
const FlagCeiling = 262 // ratchet: lower this in the SAME diff that removes rows

func TestRegistry_FlagCeiling(t *testing.T) {
    n := len(All())
    if n > FlagCeiling {
        t.Fatalf("registry has %d rows, ceiling is %d — consolidate before adding flags", n, FlagCeiling)
    }
    // Anti-stall: once we hit target, this becomes the hard <30 assertion.
}
```

- **No-regression**: any cycle that *adds* a flag above the ceiling fails CI.
- **Monotonic progress**: each consolidation cycle lowers `FlagCeiling` to the new
  `len(All())` in the same diff — the gate records that the cycle actually reduced count.
- **Completion**: campaign done when `FlagCeiling < 30` and the suite is green.
- Pairs with the existing `TestAll_SortedByName` and the flagreaders guard.

## Per-cycle protocol (what each loop cycle does)

1. **Scout**: read this doc; enumerate live `len(All())` and the next-priority cluster;
   classify each flag in it by the taxonomy table.
2. **Triage**: scope to ONE cluster (or the ratchet gate on cycle 1).
3. **TDD/Builder**: apply the bucket's pattern; remove the rows; **lower `FlagCeiling`**;
   keep the capability (config struct / DI / protocol); update `control-flags.md` via
   `evolve flags generate`; add/adjust acs predicate.
4. **Audit**: flagreaders guard green, `go test ./...` green, ratchet lowered, no capability
   lost (the deleted flag's behavior still reachable via its new home).
5. **Ship**: one cluster per commit. Push branch via the gate.

## Out of scope / survivors (the < 30 that remain)
True operator rollout/safety dials stay as flags: e.g. `EVOLVE_DYNAMIC_ROUTING`,
`EVOLVE_ROUTER_REPLAN`, `EVOLVE_SWARM_STAGE`, `EVOLVE_EVAL_GATE`, `EVOLVE_CONTRACT_GATE`,
`EVOLVE_SANDBOX`, `EVOLVE_CYCLE_BUDGET`, `EVOLVE_MAX_CYCLES_CAP`, `EVOLVE_FLEET`,
`EVOLVE_BYPASS_SHIP_GATE` (kept as the single bypass), plus a handful more — target the
final set at < 30.
