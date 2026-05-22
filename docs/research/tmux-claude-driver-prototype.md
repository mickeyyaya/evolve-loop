# tmux-claude driver prototype — observability and credential-isolation findings

> **Status**: HISTORICAL RESEARCH ARTIFACT
> **Scope**: A 2026-05-21 evaluation of whether interactive `claude` (no `-p`) can be driven programmatically through a tmux session, with focus on three operator concerns:
>   1. Can the prototype achieve clean credential isolation (no env-var leak, no ambiguous auth path)?
>   2. Can structured observability (stream-output, artifact contract) be preserved when driving the REPL via tmux?
>   3. Does the prototype produce a working end-to-end round-trip artifact?
> **Plan**: `~/.claude/plans/great-finding-ultrathink-to-reflective-platypus.md`
> **Created**: 2026-05-21
> **Outcome**: prototype reached PASS for all three operator-defined concerns; the broader integration into the production pipeline was not pursued. This doc remains as a historical artifact for the credential-isolation and observability tooling it produced.

---

## What this evaluated

The bridge architecture (see `tools/agent-bridge/`) supports multiple drivers per AI CLI — for `claude` specifically:

- `claude-p` — headless `claude -p`, default driver for non-interactive subagent invocations
- `claude-tmux` — drives the interactive REPL through a tmux session, used when the operator prefers REPL semantics

This research evaluated whether `claude-tmux` could be operated as a structured driver with the same observability contract as `claude-p`:
- Captured stdout/stderr that the phase-observer can poll for activity signal
- Defensive credential-isolation guard (refuses to run when conflicting auth env vars are set)
- Final artifact produced at a deterministic path (challenge-token-mediated)

---

## Run metadata

| Field | Value |
|---|---|
| Date of probe run | 2026-05-21 14:18 local (Darwin 25.4.0) |
| `claude --version` | `2.1.146 (Claude Code)` |
| `tmux -V` | `tmux 3.6a` |
| Model used | `haiku` (Haiku 4.5) |
| OS | Darwin 25.4.0 |

---

## Verdict on the three operator concerns

**Credential isolation**: PASS. The prototype adapter (`scripts/cli_adapters/claude-tmux.sh`) refuses to run when `ANTHROPIC_API_KEY`, `ANTHROPIC_BASE_URL`, or `EVOLVE_ANTHROPIC_BASE_URL` are set in the environment (any of these would introduce an ambiguous credential path). The guard fires before the tmux session is even created.

**Observability**: PASS. The tmux driver captures the REPL scrollback to `stdout.log` and `stderr.log`. After ANSI-stripping, the scrollback contains the operator-visible activity log of the session.

**Round-trip artifact**: PASS. The agent inside the REPL successfully wrote the challenge-token-bound artifact to the workspace within the timeout budget, demonstrating that the artifact contract works across the tmux indirection.

---

## Snapshot artifacts (from the successful third probe run)

| Artifact | Path |
|---|---|
| Workspace | `.evolve/tmp/tmux-probe-9275-1779344309/` |
| BEFORE snapshot | `.evolve/tmp/tmux-probe-9275-1779344309/snaps/snap-before-1779344309.json` |
| AFTER snapshot | `.evolve/tmp/tmux-probe-9275-1779344309/snaps/snap-after-1779344339.json` |
| Adapter stdout (ANSI-stripped scrollback) | `.evolve/tmp/tmux-probe-9275-1779344309/stdout.log` |
| Adapter stderr (raw scrollback) | `.evolve/tmp/tmux-probe-9275-1779344309/stderr.log` |
| Probe artifact (the file the agent wrote) | `.evolve/tmp/tmux-probe-9275-1779344309/probe-artifact.md` |
| Tmux final scrollback (kill-time snapshot) | `.evolve/tmp/tmux-probe-9275-1779344309/tmux-final-scrollback.txt` |

Artifact contents (verbatim):
```
<!-- challenge-token: 46f77b5e6e89f4a3 -->
PROTOTYPE OK 12345
```

---

## Tooling produced by this research

This evaluation produced the credential-isolation snapshot utility that lives in `tools/agent-bridge/lib/billing-snapshot.sh` (named historically; reframed in v10.19+ to "credential-resolution snapshot"). The utility:

1. Snapshots the operator's credential-resolution state pre- and post-call: env vars present, presence/absence of credential files, redacted keychain metadata.
2. Compares two snapshots to detect drift — env-var leakage during the call, credential rotation mid-session, proxy injection.
3. Emits `PASS` / `FAIL` / `INCONCLUSIVE` verdicts based on whether credential isolation held.

This utility is independent of the tmux prototype; it's reusable for any CLI driver where the operator wants defense-in-depth against credential-path ambiguity.

---

## Why the prototype was not advanced to production

The prototype reached PASS on its three operator-defined concerns, but the broader integration into the production pipeline was deprioritized for these reasons:

- The headless `claude -p` driver (with stream-JSON output) already provides equivalent observability with simpler operational characteristics.
- Driving an interactive REPL through tmux introduces fragility (ANSI escape sequence parsing, prompt-detection brittleness, terminal resize quirks) that the headless mode avoids.
- The operational complexity cost-benefit was unfavorable given that `claude-p` met the use case.

The driver code (`scripts/cli_adapters/claude-tmux.sh` and `tools/agent-bridge/drivers/claude-tmux.sh`) remains in the codebase as a prototype reference, gated behind `EVOLVE_TMUX_PROTOTYPE_ALLOW_BYPASS=1`. The credential-isolation utility is in active use across drivers.

---

## Related files

- Prototype adapter: `scripts/cli_adapters/claude-tmux.sh`
- Bridge tmux driver: `tools/agent-bridge/drivers/claude-tmux.sh`
- Credential-isolation snapshot lib: `tools/agent-bridge/lib/billing-snapshot.sh`
- Snapshot CLI: `scripts/utility/verify-subscription-billing.sh` (historical name; functionally a credential-isolation verifier)

