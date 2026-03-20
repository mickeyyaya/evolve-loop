# Experiment Journal (`experiments.jsonl`)

The experiment journal is an append-only log that records every Builder attempt — pass or fail. It serves as the loop's anti-repeat memory, preventing the Scout from re-proposing approaches that have already been tried and failed.

## Location

```
$WORKSPACE_PATH/experiments.jsonl
```

Each run maintains its own journal. On run completion, the journal is copied to `.evolve/workspace/experiments.jsonl` and seeded into the next run's workspace.

## Schema

Each line is a single JSON object:

```jsonl
{"cycle":22,"task":"add-operator-brief-spec-doc","attempt":1,"verdict":"PASS","approach":"create standalone doc with schema reference","metric":"7/7 eval checks passed"}
{"cycle":22,"task":"fix-auth-flow","attempt":1,"verdict":"FAIL","approach":"refactor middleware chain","metric":"TypeError: req.session undefined"}
{"cycle":22,"task":"fix-auth-flow","attempt":2,"verdict":"PASS","approach":"add session initialization before middleware","metric":"all tests pass"}
```

### Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `cycle` | number | The cycle number when the attempt was made |
| `task` | string | The task slug from the scout report |
| `attempt` | number | Attempt number (1-based, max 3 per task per cycle) |
| `verdict` | string | `PASS` or `FAIL` — matches the Auditor's final verdict |
| `approach` | string | 1-sentence summary of what the Builder tried |
| `metric` | string | The eval result or error message that determined the verdict |

## How the Scout Uses It

Before finalizing the task list (Phase 1), the Scout reads `experiments.jsonl` to:

1. **Avoid re-proposing failed approaches** — if a task slug + approach combination appears with `verdict: "FAIL"`, the Scout either skips that task or proposes a different approach
2. **Identify recurring failures** — if the same task slug appears with multiple FAIL entries across cycles, the Scout flags it as a stagnation pattern and adds it to `avoidAreas`
3. **Inform counterfactual analysis** — failed approaches provide evidence for the `alternateApproach` field in deferred task annotations

## How the Builder Uses It

After each attempt (Phase 2), the Builder appends one entry:

```bash
# Appended by Builder after each attempt
echo '{"cycle":22,"task":"slug","attempt":1,"verdict":"PASS","approach":"...","metric":"..."}' >> $WORKSPACE_PATH/experiments.jsonl
```

The Builder also reads existing entries for the current task to avoid repeating the same failed approach on retry attempts.

## Relationship to Other Logs

| Log | Purpose | Scope |
|-----|---------|-------|
| `experiments.jsonl` | Per-attempt record of what was tried and whether it worked | Task-level, approach-level |
| `ledger.jsonl` | Per-agent structured log entry (one per invocation) | Agent-level |
| `state.json failedApproaches` | Structured failure analysis with root cause reasoning | Persistent across sessions |
| `notes.md` | Human-readable cycle summaries | Cycle-level |

The experiment journal is the most granular — it captures individual attempts, while other logs aggregate at the cycle or session level.

See [architecture.md](architecture.md) § Shared Memory Architecture for the broader data flow.
