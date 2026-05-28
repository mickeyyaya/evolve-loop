# ADR-0029: CLI Fallback Chain + Per-Agent Overrides (any-CLI / any-model / any-phase)

**Status:** Accepted | **Date:** 2026-05-28 | **PR:** [#26](https://github.com/mickeyyaya/evolve-loop/pull/26) `9d02630` | **Supersedes:** N/A | **Builds on:** [ADR-0022 LaunchIntent→Realizer](./0022-launch-intent-realizer.md), [ADR-0027 commit-as-evidence + setup-onboarding](./0027-commit-as-evidence.md)

---

## Context

The evolve-loop runs a Scout → Triage → TDD → Build-Planner → Build → Audit → Ship → Retro pipeline through 7 pluggable LLM CLI drivers (claude-p/tmux, codex/tmux, agy/tmux, ollama-tmux). Pre-ADR-0029, each phase pinned exactly ONE CLI via `.evolve/profiles/<agent>.json`'s `cli` field. **Cycle 121 demonstrated this is a single point of failure:** the auditor profile pinned `cli: codex-tmux` and codex 0.134 hit a `ExitREPLBootTimeout (80)` REPL-boot bug, which killed the entire cycle even though three other CLIs (claude-tmux, agy-tmux, ollama-tmux) were registered and could have run the phase. See [cycle-121 incident report](../../incidents/cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md) for the full forensics.

User goal stated after cycle 121:

> "Allow any combination of LLM CLIs + any model to be adapted for any phase (even new customized user / LLM constructed phases) — always be executed in the pipeline."

The pre-ADR-0029 codebase had four gaps blocking this goal:

1. **No fallback path** when the primary CLI failed.
2. **CLI selection used the global env `EVOLVE_CLI`** (not per-agent), so an operator had to either edit profile JSON or globally set the CLI for ALL phases at once.
3. **No launch-time `--cli phase=cli` flag** — operators had to construct env vars manually.
4. **No capability probe** — a missing-binary CLI burned its full 60s boot timeout before the cycle gave up.

This ADR documents the chosen design: a **fallback chain** model, with per-agent env overrides, launch-time flags, and a startup capability probe — all defaulting to byte-identical pre-G behavior so operators opt in incrementally.

## Decision

Three planks land together as a single conceptual unit:

### Plank G1 — fallback chain + per-agent CLI env

**Profile schema additions** (`internal/profiles/profiles.go`):
```go
type Profile struct {
    ...
    CLI                string   `json:"cli"`                              // primary (existing)
    AllowedCLIs        []string `json:"allowed_clis,omitempty"`           // safety constraint (existing)
    CLIFallback        []string `json:"cli_fallback,omitempty"`           // NEW: ordered alternates
    CLIFallbackOnExit  []int    `json:"cli_fallback_on_exit,omitempty"`   // NEW: trigger codes (default [80,127])
}
```

**Resolution helper** (`internal/phases/runner/cli_chain.go`):
```go
func resolveCLIChain(agentName string, env map[string]string, prof *profiles.Profile) cliChain
```
Returns `{primarySource, candidates, triggers}`. Primary is picked by precedence:
1. `EVOLVE_<AGENT>_CLI` (per-agent env)
2. `EVOLVE_CLI` (global env)
3. `profile.CLI` (on-disk per-agent config)
4. `"claude-tmux"` (final default)

Candidates = `[primary] + dedup(profile.CLIFallback - primary)`. Triggers = `profile.CLIFallbackOnExit` or `[80, 127]` default (`ExitREPLBootTimeout` + `ExitMissingBinary`).

**Dispatch loop** (`internal/phases/runner/runner.go`): each attempt is logged + ledger-recorded; on a trigger exit, advance to next candidate; on a non-trigger exit (e.g. a legitimate FAIL verdict, `ExitSafetyGate`), surface as-is. **A legitimate model FAIL never silently retries on a different CLI** — the chain only catches CLI-level integration bugs.

### Plank G2 — `--cli` / `--model` launch flags

Repeatable flags on `evolve loop`:
```
--cli scout=agy-tmux --cli auditor=claude-tmux \
--model auditor=opus --model builder=llama3.1:8b
```

Each flag pair translates to `EVOLVE_<AGENT>_CLI` / `EVOLVE_<AGENT>_MODEL` in the cycle env (via `phaseEnvAgentKey`'s dash→underscore upcase, matching `envchain.PhaseEnvKey`). Flags beat inherited shell env. Malformed pairs reject with exit 10.

This is **syntactic sugar over G1's env override** — operators experimenting with combos per-run don't need to construct env vars by hand.

### Plank G3 — startup capability probe

`probeAvailableCLIChain` runs `exec.LookPath(<binary>)` for each candidate's binary BEFORE the dispatch loop. Missing-binary CLIs are **DEMOTED to the end** of the chain (not deleted). If ALL candidates are missing, the original primary still attempts (so the bridge surfaces real `ExitMissingBinary 127` to the classifier).

Reduces a missing-CLI's failure time from ~60s (boot timeout) to milliseconds.

## Considered Alternatives

### Alt A — Single retry inside the existing single-CLI dispatch (rejected)

> *On exit 80 / 127, retry the SAME CLI once with longer timeouts.*

**Why rejected:** retrying the same broken CLI doesn't help when the CLI's bug is deterministic (codex's modal-`›` collision is reproducible — retrying produces the same outcome). Wastes 60s × N attempts per cycle.

### Alt B — Global `EVOLVE_CLI_FALLBACK=cli1,cli2,cli3` env override (rejected)

> *Single env var enumerates the fallback chain for ALL phases.*

**Why rejected:** different phases legitimately want different fallback chains. The scout phase + audit phase have different CLI sweet spots (scout = fast/cheap = haiku/agy; audit = adversarial-diverse = opus/claude). A single global chain forces a one-size-fits-all compromise. The per-profile field makes the dimension visible.

### Alt C — Auto-rotate through ALL registered drivers (rejected)

> *On any failure, try every registered driver in some default order.*

**Why rejected:** silently routes any failure to a different CLI, masking misconfigurations. An operator who set `cli: codex-tmux` deliberately would have a cycle ship on claude-tmux without ever knowing codex was broken. The explicit `cli_fallback` list makes the operator's intent visible + auditable.

### Alt D — Use the dynamic-routing/PhaseAdvisor system instead (rejected)

> *Extend the existing ADR-0024 PhaseAdvisor to advise CLI selection in addition to phase selection.*

**Why rejected:** scope mismatch. PhaseAdvisor handles WHICH phases to run (skip optional, route around failure). WS-G handles WHICH CLI runs a chosen phase. Different axes. Layering the routing decisions through the same agent would couple two unrelated concerns.

### Alt E — Reorder boot loop ONLY (codex Fix B), no chain (rejected as full solution)

> *Fixing the codex boot-loop tick-order alone removes the immediate cycle-121 failure.*

**Why rejected as a STANDALONE fix:** it addresses one CLI's specific bug but leaves the structural SPoF intact. The NEXT CLI integration bug (whether codex 0.135 or some future agy/ollama issue) would kill the cycle the same way. Fix B is shipped IN ADDITION to the chain — both layers of defense.

## Consequences

### Positive

- **No more "one CLI bug kills the cycle"** — cycle 121's failure mode is structurally fixed, not patched per-CLI.
- **Operators can experiment without editing profiles** — `--cli phase=cli` is repeatable on the command line.
- **Missing CLIs fail fast** — milliseconds, not 60s.
- **Per-agent visibility** — the `cli_fallback` list in profile JSON documents the operator's failover intent.
- **User-defined phases inherit the same routing** automatically — they have profile JSONs and use the same runner code path.
- **Default triggers `[80, 127]` are CONSERVATIVE** — only "the CLI literally couldn't run" promotes; real model failures still surface as authoritative.

### Negative

- **Per-attempt log files overwrite earlier ones** — the FINAL attempt's `<phase>-stdout.log` / events file wins; primary-attempt forensics are lost. Mitigated by ledger-per-attempt records (which name the attempted CLI + exit code) preserving the audit trail.
- **Operators must understand the agent-vs-phase naming distinction** — `cli` keys use agent names (e.g., `builder`, `tdd-engineer`), `model` keys (today) use phase names (e.g., `build`, `tdd`). The G2 polish follow-up makes `--model` accept both.
- **Cycle cost goes up if multiple CLIs run** — a chain that exhausts before succeeding bills against multiple subscription planes. Bounded: 60s × N candidates worst case.
- **Default `claude-tmux` final fallback assumes claude is installed** — operators without claude must set `EVOLVE_CLI` or an explicit profile primary.

### Neutral

- Existing profiles without `cli_fallback` set have **single-element chains** — byte-identical to pre-G behavior.
- The `core.Bridge` port surface is unchanged — fallback lives in the runner, not the bridge.

## Implementation Notes

### Why the trigger list is operator-tunable

Different operators have different risk tolerances. A homelab operator might accept `EVOLVE_ARTIFACT_TIMEOUT (81)` as a fallback trigger (assume the model hung, try the next CLI). A production operator wants `81` to surface so the failure-adapter sees it. The `cli_fallback_on_exit` per-profile field lets each agent encode its own policy without code change.

### Why demote-don't-delete in the capability probe

If ALL candidates are missing (e.g., a fresh host with no CLI binaries), deleting them all would yield an empty chain, and the runner would have nothing to attempt. By keeping the primary at the END of the chain, the bridge gets a real `ExitMissingBinary` to surface, not a silent skip — and the operator sees the failure with a fix hint instead of debugging an empty cycle.

### Why the new fields live on `profiles.Profile`, not `config.RoutingConfig`

`RoutingConfig` is per-loop (one cycle invocation), while CLI selection is per-agent (every cycle). `Profile` is the natural home — same file the operator edits to change a phase's CLI today.

### Forward compatibility

When new CLIs are added (e.g., a hypothetical `mistral-tmux`), they need only:
1. A driver registered via `Register()` in `internal/bridge/driver.go`.
2. A manifest at `internal/bridge/manifests/<cli>.json`.
3. An entry in `cliBinaryFor` map in `cli_chain.go` (so the capability probe can resolve the binary).

After that, ANY profile can name them in `cli` or `cli_fallback` and `--cli phase=mistral-tmux` Just Works.

## Validation

**Unit + integration tests:** 30 new tests across `cli_chain_test.go`, `runner_fallback_test.go`, `cli_probe_test.go`, `cmd_loop_perphase_test.go`. All green; full module passes `gofmt -s` / `go vet` / `go test -race -short`.

**Live verification:** cycle 122 (post-merge) launched with multi-family routing — scout/audit on agy-tmux (Gemini), tdd/build on codex-tmux (OpenAI), triage/build-planner/retro on claude-tmux. First dispatch log proves the G2 plumbing path: `phase=scout cli=agy-tmux (source=env(EVOLVE_SCOUT_CLI))`. Subsequent phases confirm per-agent attribution across all three families.

## References

- **Plan:** `~/.claude/plans/lexical-booping-hamster.md` (WS-G section was added post cycle-121 surfacing)
- **Cycle-121 incident report:** [../../incidents/cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md](../../incidents/cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md)
- **Codex 0.134 root-cause dossier:** [../../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md](../../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md)
- **Ollama control-surface dossier:** [../../../knowledge-base/research/ollama-control-surface-2026.md](../../../knowledge-base/research/ollama-control-surface-2026.md)
- **Builds on ADR-0022** (LaunchIntent→Realizer) — the per-CLI realization layer this ADR's chain dispatches into.
- **Builds on ADR-0027** (setup-onboarding) — the `allowed_clis` envelope constraint that gates which CLIs can appear in a profile's chain.
- **Related ADR-0024** (dynamic phase routing) — orthogonal axis; PhaseAdvisor chooses WHICH phases run; ADR-0029 chooses WHICH CLI runs a chosen phase.
