# CLI-failure triage: verify or rule out quota first

When an LLM CLI "doesn't work", quota is one of the commonest causes. A drained quota pool can look like a boot timeout, an artifact that never came, an escalation or a stalled pane. This runbook asks the CLI for its usage first and records the answer as evidence. It never infers the cause from a missing marker.

The loop does the same on its own (since 2026-10-06). Every failure path that classifies a CLI as not working queries usage once, in a fresh session, and records the verdict. The steps below are the operator's version, and the places to read what the loop recorded.

## The verdicts

A usage query reads the CLI's `/usage` screen through its manifest (`controls.usage.windows`) into typed windows. A screen read before the failing attempt started can never rule its cause out. The loop queries again rather than trust it. A failed query, though, is reused inside the TTL, so a CLI that is down is not re-booted for every failure. An exhausted read is reused only until its reset comes. A caller that stops waiting (a phase that ended, an attempt's deadline) gets `unknown` for itself, and the query still lands for everyone else. Each window has a scope, a kind (`session`, `week` or `5h`), a percentage used and a reset. The query yields one of four verdicts for the failing CLI's routing family:

| Verdict | Meaning | What follows |
|---|---|---|
| `exhausted` | A **family-scoped** window (the session, the all-models week, an agy group's window) is at or above its manifest's threshold (100% used by default). | Quota is the **verified** cause. The family is benched until the window resets, capped at 24 h. |
| `healthy` | No family-scoped window of the family is exhausted. | Quota is **ruled out** for the family, and the original cause stands: a boot timeout, a transient upstream error or a stall. Debug that. A classified exit-85 wall is still benched once, because a family screen does not show every wall. An exhausted **per-model** window, such as claude's `Current week (Fable)`, is named as a note in the detail ("per-model window exhausted, which is no family cause but fails a phase on that model"). It is never the verdict, because the failing phase may have run on another model. Check the phase's model before acting on it, and see [model-level benches](../plans/model-level-benches-2026-10.md). |
| `unavailable` | The usage query itself failed: the CLI would not boot, asked for a login, or timed out. | This points at an **auth, install or network** cause, or at a family that is not subscribed. |
| `unknown` | The CLI has no usage command, or its screen yielded no window (a CLI whose manifest declares no `windows`, such as codex today, or a layout that drifted). | Quota is neither verified nor ruled out. Capture the screen (step 2) and update the manifest. |

## Steps

1. **Ask for usage now.**

   ```
   evolve clihealth usage [--json] [<family>...]
   ```

   It prints each family's typed windows: scope, kind, percentage used, reset, target (a family, or `family/model` for a per-model window), whether the window is exhausted, and whether it benches the family (only an exhausted family-scoped window does). It is read-only (no bench, no record) and boots each CLI once in a throwaway workspace.
   - A window marked exhausted is your cause.
   - A family whose query fails is printed on stderr and exits 1. That is itself evidence of an auth, install or network cause.
2. **See the raw screen** when step 1 printed a note instead of windows:

   ```
   evolve bridge control <family> usage
   ```

   Then compare the screen with the family manifest's `controls.usage.windows` in `go/internal/bridge/manifests/<family>-tmux.json`.
3. **Probe the CLI the way a phase launches it.**
   - `evolve doctor live <driver> [--model <model>]` submits one trivial task. On failure it prints the usage verdict and the windows under the final pane (`--json` adds a `usage` field).
   - `evolve doctor boot <driver>` boots without a prompt. A boot timeout with quota ruled out is a real boot problem: login, install, a dialog the auto-responder does not answer, or a cold start (see [the cold-start incident](../incidents/agy-cold-start-boot-timeout-2026-10-06.md)).
4. **Look at live panes.**

   ```
   evolve bridge sessions
   ```

   It lists every live phase pane with its CLI, model, busy state and progress age, read-only. Use it for a phase that is stalled rather than failed.
5. **Check the benches.**

   ```
   evolve clihealth list
   ```

   It shows what the loop benched and until when. `evolve clihealth clear <family>` lifts a bench once you have fixed the cause.

## What the loop records

| Where | What |
|---|---|
| `<phase workspace>/usage-evidence.ndjson` | One line per usage query a failure triggered: driver, trigger (`exit 80`, `exit 81`, `exit 85`, another non-zero exit, or `stall`), verdict summary and the evidence with its windows. The failure advisor's prompt includes the last five lines. The debugger and retrospective personas are told to read the file before blaming the code or the task. |
| Signal Center: `BRIDGE_USAGE_EVIDENCE` | The same verdict as one signal (fields `driver`, `cli`, `family`, `verdict`, `trigger`, `exit_code`, `cached`). It is WARN for `exhausted` and `unavailable`, and INFO otherwise. |
| `.evolve/usage-windows.json` | The latest windows each CLI showed: from the pre-wave usage probe, or from a failure's usage query (with its error when the query failed). Within the TTL it doubles as the cache, so lanes do not repeat each other's queries. |
| `.evolve/cli-health.json` | Benches, including `usage_probe` benches from an exhausted window. |
| Loop log | `[loop] WARN: cli-update: <family> quota-exhausted …`, `… boot-timeout …; quota ruled out …`, preflight's `bridge-boot` detail line `usage: …`, and `[usage-probe]` lines. |

**Where the loop runs the query.**

| Site | Trigger |
|---|---|
| The boundary CLI updater | Its `doctor live` probe failed. |
| Preflight's `bridge-boot` | A boot failed. |
| Every bridge launch attempt that exits non-zero (not 127), before the chain falls back | Both the runner's walk and the bridge-chain walk launch through the decorator, and so does the router's (the phase advisor's) walk. Every exit 85 with a classified wall benches the failing family through `bridgechain.BenchOnEscalation`, once per report, whatever the verdict. That is the one rule all three walks share. An `exhausted` verdict has already benched the family until the screen's reset, so the wall adds no second strike. |
| The per-phase observer | A pane stall. It is evidence only and kills nothing. |
| `evolve doctor live` | The probe failed. It is read-only: no bench, no record. |

**The budget.** The query is bounded by `cli_health.usage_evidence_timeout_s` (default 120 s) and cached per CLI for `cli_health.usage_evidence_ttl_s` (default 600 s), in `.evolve/policy.json`. Concurrent failures of one CLI in a process share one in-flight query, and a query for one CLI never waits on another's. A down, drained or unreadable CLI (a failed query, an exhausted read before its reset, no window) is queried at most once per TTL per process. A CLI whose screen reads `healthy` is queried again for each failing attempt that started after the last read, at one usage round trip each. A healthy read from before a failure cannot rule its cause out. Across processes the recorded windows are the shared cache, so at most the lane width of queries run at once. `EVOLVE_CLI_HEALTH=0` keeps the query, the workspace record and the signal, but the query then neither benches nor writes `.evolve/usage-windows.json`.
