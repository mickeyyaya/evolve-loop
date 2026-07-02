# Cycle 459 Dossier

**Goal:** GOAL: FLEET-AS-POLICY (user priority, 2026-07-02): make concurrent cycle execution a CONFIGURATION setting so the operator declares HOW cycles perform — `.evolve/policy.json` gains a `fleet` block and `evolve loop` natively runs disjoint parallel WAVES when it is set. Strict TDD (red first), clean code, design patterns: follow the existing SwarmPolicy precedent exactly (policy block + resolved-config getter + closed vocab + fail-safe defaults).

WHY: today `evolve fleet` is per-invocation CLI flags (--count/--concurrency/--plan with a HAND-WRITTEN plan file); parallel operation requires an operator to craft disjoint plans each wave. The building blocks exist: fleet.PlanCycles(todos,count) (internal/fleet/partition.go:17) partitions todos into disjoint-file cycles (ADR-0049 E); the ship path already serializes via .evolve/ship.lock and recovers moved-main via GIT_FLEET_REBASE_NEEDED (gitops.go:452, ship_recovery.go:23); lease fencing (F6) isolates per-run tmux sockets.

SLICES (one per cycle):
S1 POLICY BLOCK + RESOLUTION: add `Fleet *FleetPolicy` to policy.Policy (`json:"fleet,omitempty"`) with Count int (`count`), Concurrency int (`concurrency`, 0=>Count), PlanSource string (`plan_source`, closed vocab "triage"|"manual", default "triage"; unknown => fail-safe "manual"+WARN). FleetConfig() getter with defaults resolved (absent block => Count=1 == today's sequential loop, byte-identical). Mirror SwarmPolicy (policy.go:781) + its tests + apicover naming for every new exported symbol.
S2 WAVE SEMANTICS IN THE LOOP: at the composition root, when FleetConfig().Count>1, `evolve loop` executes each batch step as a WAVE: triage runs ONCE (single-writer), its committed tasks are partitioned via the EXISTING fleet.PlanCycles into <=Count disjoint-file cycle specs, lanes execute concurrently (reuse the fleet launcher internals — do NOT build a parallel dispatch path), ships serialize on ship.lock, wave completes => next wave re-triages. --max-cycles counts WAVES. CLI `evolve fleet` flags remain per-invocation overrides over the policy block (explicit beats config). Derive each task's file scope from its triage card packages (triage-decision.json committed_floors when present; else the card's named target package).
S3 GUARDS: (a) DIRTY-CONTROL-PLANE PREFLIGHT: refuse to start a wave when tracked control-plane files (.evolve/policy.json etc. — reuse the ship verify-class control-plane path list) are modified-uncommitted in the main checkout, with an actionable message (this exact failure burned fleet trial #1: an uncommitted policy edit poisoned ship staging and the ADR-0064 guard killed an audit-PASSED cycle at ship). (b) QUOTA-AWARE COUNT: consult the existing usage-probe; when a required CLI family is quota-benched, shrink the effective Count (min 1) with a WARN naming the reason. (c) partition-disjointness already validated by PlanCycles — add the regression test that overlapping file scopes NEVER co-schedule.

CONSTRAINTS: config-driven, zero new env flags; no new parallel dispatch path (reuse internal/fleet); go test -race green on touched packages; every new exported symbol named in a _test.go AST (apicover -enforce); absent fleet block => byte-identical sequential behavior (golden test); docs: control-flags.md + runtime-reference.md get the fleet block table.

ACCEPTANCE: policy fleet{count:2} makes `evolve loop --max-cycles 2` run 2 waves of 2 disjoint concurrent cycles with zero operator plan files; absent block => sequential, byte-identical; dirty control plane refuses the wave with the actionable message; quota-benched family shrinks count with WARN; go test -race + apicover clean.
**Final verdict:** PASS
**Run ID:** 01KWGYCXFQX6XVRG7WG8V93XEC

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | PASS |  | cycle completed; ledger walk deferred to future slice |
