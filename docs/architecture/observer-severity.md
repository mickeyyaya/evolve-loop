# Observer Severity Vocabulary

> System-wide severity definitions for every component that emits a verdict. The phase-observer was the first consumer, but the same vocabulary applies to future health checks, ledger integrity reports, eval rigor verdicts, and any other observability surface. Defined once, referenced everywhere.

## Levels

| Level | Numeric | Name in JSON | Default orchestrator response |
|---|---|---|---|
| INFO | 10 | `"INFO"` | Log only, no action |
| WARN | 20 | `"WARN"` | Note in phase report; phase continues; flag for operator review |
| INCIDENT | 30 | `"INCIDENT"` | Read `suggested_action.machine_readable`; act |

Numeric values are spaced by 10 to allow future intermediate tiers (e.g., `NOTICE=15`) without breaking existing comparisons. Do not change these values without bumping every consumer.

## When to use each

### INFO

Routine status updates that do not require action. Examples:
- Phase started / ended (heartbeat boundary markers)
- Periodic activity heartbeats
- Normal tool calls
- Phase metrics within expected ranges

Default consumer behavior: log to events.ndjson, do not surface in phase report.

### WARN

Anomalous behavior worth flagging in the phase report but not blocking. The phase will complete; orchestrator notes the WARN for operator visibility. Examples:
- Cost trajectory above baseline p95 (`cost_anomaly`)
- Tool error rate above threshold (`error_spike`)
- API rate-limit events crossing per-minute threshold (`throttled`)
- Cache hit rate degradation

Default consumer behavior: include in phase report's `summary.warns` array; verdict becomes `DEGRADED` if there are any WARNs but no INCIDENTs.

### INCIDENT

Confirmed accident requiring action. The orchestrator should read `suggested_action.machine_readable` and follow the recommended action_type.

**v9.5.0 update:** `INCIDENT`-severity observations are **kill-eligible** when the observer is running with `--enforce` (`EVOLVE_OBSERVER_ENFORCE=1`). Specifically `stuck_no_output` and `infinite_loop` rules — both INCIDENT severity — trigger the SIGTERM/grace/SIGKILL sequence inherited from `phase-watchdog.sh`. `WARN`-severity rules never trigger kill, even in `--enforce` mode. This preserves the principle that *observer informs; orchestrator/kernel acts* — but when the operator has explicitly authorized the observer to act on the most-confident rules, it does so atomically. Examples:
- Phase stuck — no events for >= threshold seconds (`stuck`)
- Infinite loop detected — same tool call N+ times in window (`infinite_loop`)
- Integrity breach detected (`ledger_chain_broken`, future)
- Security violation (`auth_failure`, future)

Default consumer behavior: phase verdict becomes `INCIDENT`; the action manifest is the orchestrator's primary input for the next step.

## Severity ladder & verdict

Per-phase verdicts roll up severity into a single tier:

| Phase verdict | Definition |
|---|---|
| `NORMAL` | Zero WARN or INCIDENT observations during the phase |
| `DEGRADED` | One or more WARN observations; no INCIDENTs |
| `INCIDENT` | One or more INCIDENT observations |

The orchestrator's decision matrix is symmetric: NORMAL = continue silently, DEGRADED = continue with note, INCIDENT = consult suggested_action.

## Implementation reference

- Canonical bash library: `scripts/lib/severity.sh`
- Constants: `SEVERITY_INFO=10`, `SEVERITY_WARN=20`, `SEVERITY_INCIDENT=30`
- Helpers: `severity_name_to_int`, `severity_int_to_name`, `severity_gte`

Example usage in any observer-class script:
```bash
source "$EVOLVE_PLUGIN_ROOT/scripts/lib/severity.sh"
if severity_gte "$current_sev" WARN; then
    echo "warn-or-worse — flag for operator"
fi
```

## Stability guarantee

The names (`INFO`, `WARN`, `INCIDENT`), numeric values, and verdict tiers (`NORMAL` / `DEGRADED` / `INCIDENT`) are part of the schema-version-1.0 contract. Breaking changes bump the schema_version field in every envelope.

Adding new severity-emitting components (future health checks, etc.): use this vocabulary; do not invent new tiers without a SCHEMA RFC.
