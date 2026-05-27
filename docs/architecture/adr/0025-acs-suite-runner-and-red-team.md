# ADR-0025: Deterministic ACS suite-runner + standing red-team predicates

> Status: **Accepted** (2026-05-27). Restores the host-side EGPS predicate-suite runner that v12's
> flag-day deleted without a Go port, and adds a standing `acs/red-team/` suite that encodes past
> gaming incidents as live predicates. Companion to `skills/adversarial-testing/SKILL.md` (Google
> adversarial-testing methodology applied to evolve-loop).

## Context

Applying Google's adversarial-testing "report & mitigate" loop (encode each past failure as a standing
test) required wiring red-team predicates to fire every cycle. Tracing that wiring uncovered a
pre-existing gap:

1. **The suite runner was deleted, never ported.** `docs/architecture/egps-v10.md` and
   `agents/evolve-auditor-reference.md:84` both reference `legacy/scripts/lifecycle/run-acs-suite.sh`
   as the mechanism that globs `acs/cycle-N/*.sh` + `acs/regression-suite/cycle-*/*.sh` and writes
   `acs-verdict.json`. v12.0.0's flag-day deleted ~220 bash scripts including this one; **no Go port
   exists**. The auditor doc points at a dangling script.
2. **The Go audit phase only *reads* the verdict.** `go/internal/phases/audit/audit.go` reads
   `acs-verdict.json:red_count` and FAILs if missing — it never *generates* it. Generation was
   implicitly delegated to the auditor LLM agent, improvising against the dangling reference.
3. **No production Go code globs `acs/regression-suite/`** (or would glob `acs/red-team/`). The
   `go/acs/cycleNN/predicates_test.go` packages + `evolve acs run <pkg>` are a *parallel* go-test
   harness, not the live-cycle verdict generator.

So the regression suite's "every prior predicate runs every cycle" guarantee — load-bearing for the
EGPS gate hardened across the cycle 102–141 incidents (`[[project_incident_history]]`) — rested on an
LLM improvising against a deleted script.

## Decision

### 1. `evolve acs suite` — deterministic host-side runner

A new Go package `go/internal/acssuite` + CLI `evolve acs suite --cycle N [--root .] [--evolve-dir .evolve]`:

- Globs, in deterministic order: `acs/cycle-<N>/*.sh`, `acs/regression-suite/cycle-*/*.sh`,
  `acs/red-team/rt-*.sh`.
- Executes each bash predicate in its **own process group** with a per-predicate timeout
  (`DefaultTimeout=60s`); a hang is killed (process-group SIGKILL + `WaitDelay`) and counts RED, so a
  runaway predicate can never stall the cycle.
- Writes `acs-verdict.json` in the exact schema the audit + ship gates read
  (`red_count`/`green_count`/`verdict`/`red_ids`/`predicate_suite.total`), plus a `red_team_count`
  and per-result `is_red_team`. `red_count == 0 ⇒ PASS ⇒ ship_eligible`.

### 2. Standing `acs/red-team/` suite

Predicates that encode a past gaming incident as a live test, firing every cycle via §1. Each honors
`RT_REPO_ROOT` (fixture-testable) and SKIPs gracefully when preconditions are absent:

| Predicate | Asserts | Incident |
|---|---|---|
| `rt-001-ledger-role-completeness.sh` | last completed cycle has scout+builder+auditor ledger entries | cycle 102-111 (auditor never invoked) |
| `rt-002-no-batch-cycle-jump.sh` | `state.json:lastCycleNumber` ≤ max ledger cycle +1 (read from ledger, not state) | cycle 132-141 (batch 132→141 jump) |
| `rt-003-challenge-token-integrity.sh` | every agent_subprocess entry for the last cycle carries a non-empty challenge_token | cycle 102-141 (forged entries) |

Fixture-proven in `go/acs/redteam/redteam_test.go`: each FAILs on a fabricated attack, PASSes clean.

### 3. Wiring (this ADR) + rollout target

- **Now:** the auditor reference doc invokes `evolve acs suite --cycle "$cycle"` (was the dangling
  `run-acs-suite.sh`). The auditor already runs the suite + writes `acs-verdict.json`; pointing it at
  the real runner makes that step executable again and pulls `acs/red-team/` into every cycle.
- **Rollout target (future ADR):** make the **host** audit phase run `acssuite` to generate
  `acs-verdict.json` authoritatively (EGPS = execution-grounded — the verdict-of-record should be
  host-computed, not LLM-written). That changes the auditor's contract (it would emit only qualitative
  defects) and the audit-phase Go code, so it is sequenced separately behind the shadow→enforce
  machinery. Not done here to keep this change low-risk.

## Consequences

- **Positive:** the suite-running mechanism is executable Go again (not a dangling bash reference);
  red-team predicates fire every cycle; the runner is independently usable (`evolve acs suite`) and
  fully unit-tested; a hanging predicate can no longer stall a cycle.
- **Negative / risk:** until the rollout target lands, the auditor LLM still *invokes* the runner
  rather than the host doing it deterministically — an auditor that ignores its instructions could
  still skip it (mitigated: `rt-001` would catch a skipped auditor next cycle; the audit phase FAILs
  on a missing verdict).
- **Reversible:** the runner is additive (new package + CLI subcommand + new `acs/red-team/` dir);
  removing the doc line reverts to prior behavior.

## References

- `skills/adversarial-testing/SKILL.md` §9 — red-team catalogue (canonical)
- `docs/architecture/egps-v10.md` — verdict schema + the (now-restored) suite-run step
- `docs/incidents/cycle-102-111.md`, `docs/incidents/cycle-132-141.md` — the encoded incidents
- `[[project_incident_history]]` — why these guardrails exist
