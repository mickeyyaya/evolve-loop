# Cycle 464 Dossier

**Goal:** GOAL: FLEET-AS-POLICY (S1-S3). OPERATOR PRIORITY OVERRIDE: this goal OUTRANKS every carryover/inbox item; triage MUST commit THIS work as top_n item 1 and defer the backlog. Make concurrent cycle execution a CONFIGURATION setting — `.evolve/policy.json` gains a `fleet` block and `evolve loop` natively runs disjoint parallel WAVES when set. Strict TDD (red first), clean code; follow the SwarmPolicy precedent exactly (policy block + resolved-config getter + closed vocab + fail-safe defaults). NOTE: cycle 459 previously diverted from this goal to backlog — do NOT repeat; the backlog is deferred by operator order.

WHY: today `evolve fleet` is per-invocation CLI flags with a HAND-WRITTEN --plan file. Building blocks exist: fleet.PlanCycles(todos,count) (internal/fleet/partition.go:17, ADR-0049 E) partitions todos into disjoint-file cycles; ship serializes via .evolve/ship.lock and recovers moved-main via GIT_FLEET_REBASE_NEEDED (gitops.go:452, ship_recovery.go:23); lease fencing (F6) isolates per-run tmux sockets.

SLICES (one per cycle):
S1 POLICY BLOCK: `Fleet *FleetPolicy` on policy.Policy (`json:"fleet,omitempty"`) with Count int (`count`), Concurrency int (`concurrency`, 0=>Count), PlanSource string (`plan_source`, closed vocab "triage"|"manual", default "triage"; unknown => fail-safe "manual"+WARN). FleetConfig() getter with defaults (absent block => Count=1, byte-identical sequential). Mirror SwarmPolicy (policy.go:781) + tests + apicover naming.
S2 WAVE SEMANTICS: when FleetConfig().Count>1, `evolve loop` runs each batch step as a WAVE: triage runs ONCE (single-writer), committed tasks partitioned via the EXISTING fleet.PlanCycles into <=Count disjoint-file specs (scope from triage-decision.json committed_floors when present, else the card's named target package), lanes execute concurrently (REUSE the fleet launcher internals — no parallel dispatch path), ships serialize on ship.lock, wave completes => next wave re-triages. --max-cycles counts WAVES. CLI `evolve fleet` flags remain per-invocation overrides.
S3 GUARDS: (a) DIRTY-CONTROL-PLANE PREFLIGHT: refuse a wave when tracked control-plane files (.evolve/policy.json etc. — reuse the ship verify-class control-plane list) are modified-uncommitted in the main checkout, with an actionable message (this exact failure killed an audit-PASSED cycle in fleet trial #1). (b) QUOTA-AWARE COUNT: consult the usage-probe; a quota-benched required family shrinks effective Count (min 1) with a WARN. (c) regression test: overlapping file scopes NEVER co-schedule.

CONSTRAINTS: config-driven, zero new env flags; reuse internal/fleet; go test -race green; apicover -enforce clean; absent fleet block => byte-identical sequential (golden); docs: control-flags.md + runtime-reference.md fleet block table.
ACCEPTANCE: policy fleet{count:2} makes `evolve loop --max-cycles 2` run 2 waves of 2 disjoint concurrent cycles with zero operator plan files; absent block => sequential byte-identical; dirty control plane refuses with actionable message; quota-benched family shrinks count with WARN.
**Final verdict:** PASS
**Run ID:** 01KWHHV951M15M581PB7MAZE7S

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | PASS |  | cycle completed; ledger walk deferred to future slice |
