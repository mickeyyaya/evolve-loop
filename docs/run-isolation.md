# Run Isolation Model

The evolve-loop supports multiple parallel invocations running concurrently against the same repository. Each invocation gets a unique run ID and isolated workspace directory to prevent data races on workspace files.

## Run ID Generation

At initialization, each invocation generates a unique identifier:

```bash
RUN_ID="run-$(date +%s)-$(openssl rand -hex 2)"
# Example: run-1773980593-1d5d
```

The run ID is passed to all agents via the `runId` field in their context blocks. It appears in ledger entries and workspace file paths for traceability.

## Workspace Scoping

Each run operates in its own workspace directory:

```
.evolve/runs/{RUN_ID}/workspace/
```

All workspace files (scout-report.md, build-report.md, audit-report.md, operator-log.md, handoff.md, etc.) are scoped to this directory via the `$WORKSPACE_PATH` variable. This prevents two concurrent runs from overwriting each other's workspace files.

## Directory Layout

```
.evolve/
├── runs/                          # Per-run isolation
│   ├── run-1773980593-1d5d/
│   │   └── workspace/             # This run's workspace files
│   └── run-1773980601-a3f2/
│       └── workspace/             # Another concurrent run's files
├── workspace/                     # Shared workspace (backward compat)
├── evals/                         # Shared — eval definitions
├── instincts/                     # Shared — learned patterns
├── history/                       # Shared — cycle archives
├── genes/                         # Shared — fix templates
├── tools/                         # Shared — synthesized tools
├── state.json                     # Shared — protected by OCC versioning
├── ledger.jsonl                   # Shared — append-only log
├── project-digest.md              # Shared — project structure cache
└── latest-brief.json              # Shared — last-writer-wins operator brief
```

## Shared vs Run-Local

| Category | Location | Concurrency Model |
|----------|----------|-------------------|
| Workspace files | `$WORKSPACE_PATH/` (run-local) | No contention — each run has its own copy |
| State | `.evolve/state.json` | OCC (optimistic concurrency control) with version field |
| Ledger | `.evolve/ledger.jsonl` | Append-only — concurrent appends are safe |
| Evals | `.evolve/evals/` | Written by Scout (one writer per cycle), read by Auditor |
| Instincts | `.evolve/instincts/` | Written during LEARN phase (serial within a run) |
| Operator brief | `.evolve/latest-brief.json` | Last-writer-wins — acceptable for advisory data |

## Run Seeding

On initialization, if the shared workspace (`.evolve/workspace/`) contains files from a previous session, they are copied into the new run's workspace:

```bash
cp -rn .evolve/workspace/* "$WORKSPACE_PATH/" 2>/dev/null || true
```

This ensures continuity — the new run starts with the previous session's handoff.md, builder-notes.md, and other carry-forward files.

## Cleanup

### Automatic Pruning
Run directories older than 48 hours are pruned at the start of each new invocation:

```bash
find .evolve/runs/ -maxdepth 1 -type d -name 'run-*' -mtime +2 -exec rm -rf {} \;
```

### Run Completion
After all cycles complete, the final workspace is copied to the shared location:

```bash
cp -rp "$WORKSPACE_PATH"/* .evolve/workspace/ 2>/dev/null
```

This ensures backward compatibility — tools that read `.evolve/workspace/` directly still work.

See [architecture.md](architecture.md) § Shared Memory Architecture for the broader data flow.
