# CLI-failure triage: verify or rule out quota first

When an LLM CLI "doesn't work", quota is one of the most common causes. A drained quota pool can look like a boot timeout, an artifact that never came, an escalation or a stalled pane. This runbook asks the CLI for its usage first and records the answer as evidence. It never infers the cause from a marker that is not there.

The loop does the same on its own (since 2026-10-06). Every failure path that classifies a CLI as one that does not work queries usage once, in a fresh session, and records the verdict. The steps below are the version for the operator. They also show where to read what the loop recorded.

## The verdicts

A usage query reads the `/usage` screen of the CLI through its manifest (`controls.usage.windows`) into typed windows. These rules control the reuse of a read:

- A screen read from before the start of the try that failed can never rule out the cause of that try. The loop queries again and does not trust that read.
- But the loop reuses a failed query inside the TTL. So it does not boot a CLI that is down again for every failure.
- The loop reuses an exhausted read only until its reset comes.
- A caller that stops its wait (a phase that ended, the deadline of a try) gets `unknown` for itself. The query still lands for every other caller.

Each window has a scope, a kind (`session`, `week` or `5h`), a percentage used and a reset. The query gives one of four verdicts for the routing family of the CLI that failed:

| Verdict | Meaning | What follows |
|---|---|---|
| `exhausted` | A **family-scoped** window is at or above the threshold of its manifest (100% used by default). The family-scoped windows are the session, the all-models week and the window of an agy group. | Quota is the **verified** cause. The loop benches the family until the window resets, with a cap of 24 h. |
| `healthy` | No family-scoped window of the family is exhausted. | Quota is **ruled out** for the family, the original cause stands (a boot timeout, a transient upstream error or a stall), so debug it. The loop still benches a classified exit-85 wall once, because a family screen does not show every wall. An exhausted **per-model** window, such as `Current week (Fable)` of claude, shows as a note in the detail. The note is "per-model window exhausted, which is no family cause but fails a phase on that model". It is never the verdict, because the phase that failed can have run on another model. Check the model of the phase before you act on the note, and see [model-level benches](../plans/model-level-benches-2026-10.md). |
| `unavailable` | The usage query itself failed: the CLI did not boot, asked for a login, or timed out. | This points to an **auth, install or network** cause, or to a family that has no subscription. |
| `unknown` | The CLI has no usage command, or its screen gave no window. Examples are a CLI whose manifest declares no `windows` (such as codex today) and a layout that drifted. | Quota is not verified and not ruled out. Capture the screen (step 2) and update the manifest. |

## Steps

1. **Ask for usage now.**

   ```
   evolve clihealth usage [--json] [<family>...]
   ```

   It prints the typed windows of each family, with these fields:
   - the scope, the kind, the percentage used and the reset;
   - the target: a family, or `family/model` for a per-model window;
   - if the window is exhausted;
   - if the window benches the family. Only an exhausted family-scoped window benches the family.

   The command is read-only (no bench, no record). It boots each CLI once in a throwaway workspace.
   - A window marked exhausted is your cause.
   - The command prints a family whose query fails on stderr, and exits 1. That failure is itself evidence of an auth, install or network cause.
2. **See the raw screen** when step 1 printed a note instead of windows:

   ```
   evolve bridge control <family> usage
   ```

   Then compare the screen with the `controls.usage.windows` of the family manifest in `go/internal/bridge/manifests/<family>-tmux.json`.
3. **Probe the CLI the same way as a phase launches it.**
   - `evolve doctor live <driver> [--model <model>]` submits one trivial task. On failure, it prints the usage verdict and the windows under the final pane (`--json` adds a `usage` field).
   - `evolve doctor boot <driver>` boots without a prompt. If quota is ruled out, a boot timeout is a real boot problem. The problem is a login, an install, a dialog that the auto-responder does not answer, or a cold start (see [the cold-start incident](../incidents/agy-cold-start-boot-timeout-2026-10-06.md)).
4. **Look at live panes.**

   ```
   evolve bridge sessions
   ```

   It lists every live phase pane with its CLI, model, busy state and progress age. It is read-only. Use it for a phase that is stalled, not failed.
5. **Check the benches.**

   ```
   evolve clihealth list
   ```

   It shows what the loop benched and until when. `evolve clihealth clear <family>` lifts a bench after you fix the cause.

## What the loop records

| Where | What |
|---|---|
| `<phase workspace>/usage-evidence.ndjson` | One line for each usage query that a failure triggered. The line has the driver, the trigger (`exit 80`, `exit 81`, `exit 85`, another non-zero exit, or `stall`), the verdict summary and the evidence with its windows. The prompt of the failure advisor includes the last five lines. The instructions of the debugger and retrospective personas tell them to read the file before they blame the code or the task. |
| Signal Center: `BRIDGE_USAGE_EVIDENCE` | The same verdict as one signal (fields `driver`, `cli`, `family`, `verdict`, `trigger`, `exit_code`, `cached`). It is WARN for `exhausted` and `unavailable`, and INFO for all other verdicts. |
| `.evolve/usage-windows.json` | The latest windows that each CLI showed. They come from the pre-wave usage probe, or from the usage query of a failure (with its error when the query failed). Within the TTL, the file is also the cache, so lanes do not repeat the queries of other lanes. |
| `.evolve/cli-health.json` | Benches, with the `usage_probe` benches from an exhausted window. |
| Loop log | `[loop] WARN: cli-update: <family> quota-exhausted …`, `… boot-timeout …; quota ruled out …`, the `bridge-boot` detail line `usage: …` of preflight, and `[usage-probe]` lines. |

**Where the loop runs the query.**

| Site | Trigger |
|---|---|
| The boundary CLI updater | Its `doctor live` probe failed. |
| Preflight's `bridge-boot` | A boot failed. |
| Every bridge launch that exits non-zero (not 127), before the chain falls back | The walk of the runner and the walk of the bridge chain both launch through the decorator. The walk of the router (the phase advisor) also launches through it. Every exit 85 with a classified wall benches the family that failed through `bridgechain.BenchOnEscalation`, once per report, for any verdict. That is the one rule that all three walks share. An `exhausted` verdict has already benched the family until the reset of the screen, so the wall adds no second strike. |
| The per-phase observer | A pane stall. It is evidence only and kills nothing. |
| `evolve doctor live` | The probe failed. It is read-only: no bench, no record. |

**The budget.**

- `cli_health.usage_evidence_timeout_s` (default 120 s) bounds the query. The loop caches the result per CLI for `cli_health.usage_evidence_ttl_s` (default 600 s). Both keys are in `.evolve/policy.json`.
- Concurrent failures of one CLI in a process share one in-flight query. A query for one CLI never waits on the query of another CLI.
- The loop queries a CLI that is down, drained or unreadable at most once per TTL per process. Down means a failed query, drained means an exhausted read before its reset, and unreadable means no window.
- The loop queries a CLI whose screen reads `healthy` again for each failed try that started after the last read. Each query costs one usage round trip. A healthy read from before a failure cannot rule its cause out.
- Across processes, the recorded windows are the shared cache. So at most the lane width of queries run at the same time.
- `EVOLVE_CLI_HEALTH=0` keeps the query, the workspace record and the signal. But the query then does not bench and does not write `.evolve/usage-windows.json`.
