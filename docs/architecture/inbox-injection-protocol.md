# Inbox Injection Protocol (v9.5.0+)

> **Layer**: Operator API · **Component**: Triage · **ADR**: [0007](../adr/0007-inbox-injection-protocol.md)

Operators inject tasks into the carryoverTodos backlog via a file-based API. Tasks land in `.evolve/inbox/` and are ingested by the Triage agent at the start of the next cycle.

## Why inbox + Triage ingestion?

Direct writes to `state.json:carryoverTodos[]` race the orchestrator (the reconcile pass drops schema-mismatched entries) and require operators to fill 8 required fields manually. The inbox pattern provides:

- **No race window** — Triage reads inbox once at phase start; mid-cycle injections defer to the next cycle.
- **Schema enforcement** — Triage validates at ingestion; bad entries are rejected with WARN, not silently dropped.
- **Audit trail** — every ingestion writes a ledger entry; processed files are archived.
- **Per-project isolation** — `.evolve/inbox/` lives under `$EVOLVE_PROJECT_ROOT`; no cross-project visibility.

## CLI

```bash
bash scripts/utility/inject-task.sh \
  --priority HIGH|MEDIUM|LOW \    # required
  --action "task description" \   # required, non-empty
  [--weight 0.85] \               # float in [0.0, 1.0]; tie-breaks within priority class
  [--evidence-pointer "url"] \    # auto-synthesized as inbox-injection://<ts> if absent
  [--note "operator context"] \   # free-form
  [--id custom-id] \              # auto-generated as user-<epoch>-<hex8> if absent
  [--dry-run]                     # validate + print JSON, do not write
```

| Option | Required | Notes |
|---|---|---|
| `--priority` | yes | `HIGH`, `MEDIUM`, or `LOW` (case-insensitive) |
| `--action` | yes | Non-empty task description |
| `--weight` | no | Float in [0.0, 1.0]; tie-breaks within priority class; default 0.5 |
| `--evidence-pointer` | no | URL or path; auto-synthesized as `inbox-injection://<ts>` if absent |
| `--note` | no | Free-form operator context |
| `--id` | no | Custom id; auto-generated as `user-<epoch>-<hex8>` if absent |
| `--dry-run` | no | Validate + print JSON, do not write |

Exit codes: `0` success · `10` validation error · `11` id collision · `12` filesystem error.

## Per-cycle flow

```
operator  ──► inject-task.sh  ──► .evolve/inbox/<ts>-<rand>.json

(next cycle begins)

Triage Step 0 ──► list .evolve/inbox/*.json
               ──► validate each (reject malformed to rejected/cycle-N/)
               ──► transform to reconcile-compatible schema
               ──► append to working carryoverTodos
               ──► move to processed/cycle-N/
               ──► write ledger entry
               ──► continue with top_n decision
```

Injections during an active cycle are deferred to the next cycle by design — Triage reads inbox once at phase start.

## Directory structure

```
.evolve/inbox/
├── .keep                                    # tracked; directory marker
├── 2026-05-13T10-00-00Z-a4f2e8b1.json      # pending injection (tracked)
├── processed/
│   └── cycle-27/                           # gitignored; archive of ingested files
│       └── 2026-05-13T...json
└── rejected/
    └── cycle-27/                           # gitignored; archive of validation failures
        └── 2026-05-13T...json
```

## Cancelling a pending injection

```bash
rm .evolve/inbox/2026-05-13T10-00-00Z-a4f2e8b1.json
```

No separate cancel CLI — the file-based protocol is self-documenting and inspectable.

## Schema reference

See [agents/evolve-triage-reference.md](../../agents/evolve-triage-reference.md) for the full inbox JSON schema, ingestion algorithm, reconcile-compatible schema transformation, and error codes.

## Out of scope

- HTTP API endpoint (overkill; security surface)
- Mid-cycle re-read of inbox (deterministic phase-start read is intentional)
- TTL-based auto-archive (operator runs `rm` manually)
- Integration with external trackers — use inbox as ingestion layer in a follow-up
- Batch multi-priority injection — v1 is one task per CLI invocation
