# How to run and debug a cycle locally

> The fastest way to understand the loop is to run one and read what it left behind.
> This guide is the local-run recipe: the commands, where artifacts land, and how to
> read them. The lifecycle these commands drive is
> [architecture/phase-pipeline.md](../architecture/phase-pipeline.md).

## Build the binary

The Go binary is the only runtime. Build it from `go/`:

```bash
cd go && go build -o bin/evolve ./cmd/evolve
```

`EVOLVE_GO_BIN` (or the default `go/bin/evolve`) is the entrypoint dispatchers use.

## Run a loop or a single cycle

```bash
evolve loop                       # run autonomous cycles until a stop condition
evolve loop --budget-usd 5.00     # stop at $5 cumulative cost (rc=0; alias: --budget)
evolve loop --resume              # continue an unfinished cycle from its checkpoint
evolve loop --dry-run             # resolve the invocation and print the plan; run nothing
evolve cycle run                  # run exactly one cycle
```

Per-agent CLI / model overrides are **repeatable `agent=value` flags** (parsed in
`go/cmd/evolve/cmd_loop.go`):

```bash
evolve loop --cli auditor=claude-tmux --cli builder=ollama-tmux \
            --model auditor=opus     --model builder=llama3.1:8b
```

> A fresh `evolve loop` **refuses** (exit 2) if it detects an unfinished cycle,
> printing the resume‖reset fork: `evolve loop --resume` to continue, or
> `evolve cycle reset` to seal it. This is the unfinished-cycle guard — see
> [architecture/state-and-ledger.md](../architecture/state-and-ledger.md).

## Where the artifacts land

**`.evolve/runs/cycle-N/` IS the workspace IS the cycle root** — not a `workspace/`
subdirectory under it (`WorkspacePath = ".evolve/runs/cycle-N"`). Every phase report
is read by the gate from that directory **directly**:

| File | Written by | Read by |
|---|---|---|
| `scout-report.md`, `build-report.md`, `audit-report.md` | the phase persona | the gate / auditor |
| `acs-verdict.json` (`red_count`) | `evolve acs suite` / audit | ship's EGPS gate |
| `challenge-token.txt` | orchestrator | build/audit (anti-forgery stamp) |
| `<phase>-stdout.log` (raw) + `<phase>-stdout.clean.txt` | the runner | **you** |

Source changes go to the per-cycle **worktree** (`.evolve/worktrees/cycle-N`), kept
separate from the live `main` tree so a half-finished cycle is discardable by deleting
the worktree.

## Reading what happened

Read **`<phase>-stdout.clean.txt`**, not the raw `.log`. The clean file
(`EVOLVE_STDOUT_FILTER=on`, default) is ~8–20% the size of raw: stream-redraw noise
dropped, hook envelopes one-lined, tool-result payloads middle-truncated. The raw
`.log` is byte-for-byte unchanged (cyclecost and the observer read it), so it remains
the forensic source of truth when the clean view isn't enough.

The runner also logs the resolved dispatch inline to stderr —
`[runner] phase=… agent=… cli=… (source=…) profile=…` — so you can see *which* CLI
and model actually ran and why (this disambiguates "codex delegating to claude" from
"runner ignored profile.cli").

## The stall observer

A background watcher (`observer.Start`, ADR-0030, auto-spawned unless
`EVOLVE_OBSERVER_AUTOSPAWN=0`) watches each phase's stdout-log; if it stops growing
past `EVOLVE_OBSERVER_STALL_S` (600s) it emits a stall event and, when
`EVOLVE_OBSERVER_ENFORCE=1`, SIGTERMs the subagent. If a phase hangs, this is what
kills it — and a stall event in the log is your first clue.

## Sanity-check the environment

```bash
evolve doctor          # diagnoses CLI binaries, auth mode, worktree base, sandbox capability
evolve guard chain     # verify the ledger hash-chain is intact (read-only)
```

`evolve doctor` (`go/cmd/evolve/cmd_doctor.go`) is the first thing to run when a phase
"ran but produced nothing" — it surfaces a missing CLI binary or an ambiguous
credential path before you spend a cycle on it.

## The tests that pin this

- `go/cmd/evolve/cmd_loop_test.go`, `cmd_loop_reset_guard_test.go` — flag parsing and
  the unfinished-cycle refusal.
- `go/internal/phases/runner/stdout_filter_test.go` — the `.clean.txt` companion writer.
- `go/cmd/evolve/cmd_loop_live_e2e_test.go` — a real local loop invocation path.
