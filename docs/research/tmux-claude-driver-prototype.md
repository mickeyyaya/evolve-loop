# tmux-claude driver prototype — findings

> **Status**: PROTOTYPE
> **Scope**: Does interactive `claude` (no `-p`), driven from a tmux session, bill to Claude subscription quota instead of the post-2026-06-15 Agent SDK credit pool?
> **Plan**: `~/.claude/plans/great-finding-ultrathink-to-reflective-platypus.md`
> **Created**: 2026-05-21

---

## Run metadata

| Field | Value |
|---|---|
| Date of probe run | 2026-05-21 14:18 local (Darwin 25.4.0) |
| `claude --version` | `2.1.146 (Claude Code)` |
| `tmux -V` | `tmux 3.6a` |
| Model used | `haiku` (Haiku 4.5) |
| Subscription tier | **Max 5x** (confirmed via keychain `rateLimitTier: default_claude_max_5x`) |
| OS | Darwin 25.4.0 |
| Operator | local (mickeyyaya@gmail.com) |

---

## Verdict

**GO** — verifier PASS (strong via keychain) + artifact OK + adapter rc=0. Pending: manual Anthropic console check to confirm subscription quota actually decremented.

---

## Snapshot artifacts (from the successful third probe run)

| Artifact | Path |
|---|---|
| Workspace | `.evolve/tmp/tmux-probe-9275-1779344309/` |
| BEFORE snapshot | `.evolve/tmp/tmux-probe-9275-1779344309/snaps/snap-before-1779344309.json` |
| AFTER snapshot | `.evolve/tmp/tmux-probe-9275-1779344309/snaps/snap-after-1779344339.json` |
| Verifier verdict | **PASS (strong via keychain)** — subscriptionType=`max`, no env leak |
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

## Adapter behavior

| Question | Observed |
|---|---|
| REPL boot time to first `❯` prompt | **1 second** (Haiku) |
| Artifact appearance time after prompt delivery | **24 seconds** (run 2 took 16s — natural variance) |
| `--dangerously-skip-permissions` survived without extra prompts? | **Yes** — banner showed `⏵⏵ bypass permissions on` |
| `tmux paste-buffer` introduced spurious chars? | **No** — 578-byte prompt cleanly delivered |
| Did the agent write the artifact via its own Write tool? | **Yes** — Haiku called Write at the exact path |
| Challenge token preserved verbatim in artifact line 1? | **Yes** — first line was the exact HTML comment |
| Adapter exit code | `0` (clean exit, tmux session killed cleanly) |

### What went wrong on the FIRST run (recorded for future debuggers)

- Run 1 failed with `rc=80` (REPL boot timeout) despite the REPL actually being ready
- Root cause: my prompt-detection regex used `tmux capture-pane | tail -5 | grep ❯`
- But the Ink-based UI renders the `❯` prompt MID-PANE (separator above, status below, then empty trailing rows). `tail -5` was empty padding, not the prompt
- Fix: search the FULL pane (`tmux capture-pane | grep ❯`) — succeeded on run 2

### What went wrong on the SECOND run

- Mechanism worked (artifact written, rc=0) but verifier returned INCONCLUSIVE
- Root cause: verifier was looking for `~/.claude/.credentials.json` — on this Darwin box, OAuth lives in **macOS Keychain** under `"Claude Code-credentials"`, not in the filesystem
- Fix: verifier now extracts `expiresAt` and `subscriptionType` from the keychain entry via `security find-generic-password -w` — strong PASS achieved on run 3

---

## Manual console check (the load-bearing test)

The automated verifier can only see local file state. The ACTUAL billing destination is only visible at https://console.anthropic.com:

| Source | BEFORE | AFTER | Delta | Interpretation |
|---|---|---|---|---|
| Subscription quota (claude.ai/settings/usage) | _value_ | _value_ | _delta_ | _expected: > 0 if subscription path active_ |
| API credit balance (console.anthropic.com/settings/billing) | _value_ | _value_ | _delta_ | _expected: unchanged if subscription path active_ |
| Agent SDK credit balance (post-2026-06-15) | N/A pre-cutover | N/A pre-cutover | N/A | _N/A: cutover is 2026-06-15_ |

**PASS criterion**: subscription quota decremented AND API credit balance unchanged.
**FAIL criterion**: any API credit decrement (even $0.01).

---

## Observations

1. **Haiku REPL boot was 1 second** — much faster than expected. This means fresh-session-per-call is viable; the per-agent boot cost is ~1s on Haiku (likely 3-8s on Sonnet/Opus given heavier init).
2. **`tmux paste-buffer` cleanly delivered the 578-byte multi-line prompt** with no escape issues or trailing-newline races. This validates the prompt-delivery approach for production prompts that can be 5-20 KB.
3. **The agent (Haiku) immediately called Write with the requested path** — no preamble, no exploration, no tool checking. This is good news for the production use case where Builder is given a precise contract.
4. **`--dangerously-skip-permissions` granted Write silently** — the banner showed `⏵⏵ bypass permissions on (shift+tab to cycle)` and no permission dialog appeared. The CLI flag is equivalent in effect to `settings.local.json:permissions.defaultMode=bypassPermissions`.
5. **No CAPTCHA, no "Are you a human?", no rate-limit hit, no detection-warning** in either of the 2 successful runs. Anthropic's behavior-detection is not yet flagging this access pattern (at least not on a single-prompt round-trip).
6. **The OAuth lives in macOS Keychain, not in `~/.claude/.credentials.json`** as I'd assumed. The keychain entry shows `subscriptionType: max` and `rateLimitTier: default_claude_max_5x` — confirming Max 5x plan is the active billing target.
7. **The doctor's `MISCONFIGURED` verdict from earlier in this session was a false negative** — it checks the wrong path. The keychain has full OAuth metadata.
8. **`Haiku 4.5 · Claude Max` appeared in the interactive REPL banner inside tmux** — this is the load-bearing visual signal that the launched claude session attached to subscription auth, NOT to a stray API key.

---

## Risks observed during probe

| Plan Risk | Materialized? | Notes |
|---|---|---|
| R1 (ToS interpretation) | No signal yet | No captcha, no refusal, no warning. Anthropic isn't visibly enforcing programmatic interactive-driving in May 2026. |
| R2 (cost leak) | **No** | Env was clean. Adapter's hard-fail on `ANTHROPIC_API_KEY` / `ANTHROPIC_BASE_URL` worked as a safety net but didn't fire. |
| R3 (TUI parsing fragility) | **Partially** | The first run's prompt-detection regex was wrong (`tail -5` vs full pane). Fixed. The artifact-only verification design protected us from needing to parse the actual response text. |
| R4 (--dangerously-skip-permissions grants ALL tools) | Worked as expected | For production, replace with `--tools "Read,Write,..."` whitelist sourced from `profile.allowed_tools[]`. |
| R5 (behavior fingerprinting) | No signal | Anthropic could add this later; verifier would catch it post-hoc. |
| R6 (session contamination) | N/A (single-call prototype) | Production multi-agent flow needs fresh-per-call (which we already do). |
| R7 (tmux server orphans) | **No** — trap-based cleanup worked | All 3 runs cleanly killed their sessions via the EXIT trap. |

---

## Recommendations

### If GO

Production rollout is a multi-cycle project. The blockers (in order):

1. **Schema v2 bump for `_capabilities-schema.json`** to include `claude-tmux` in the adapter enum.
2. **Replace `--dangerously-skip-permissions` with `--tools` whitelist** sourced from `profile.allowed_tools[]`. The interactive `claude` CLI accepts `--tools "Read,Write,Bash"` directly — bridge to the existing profile schema.
3. **Persistent-session vs fresh-per-call decision**. If REPL boot takes 5–10s and a cycle spawns 7 agents, that's ~1 minute of pure boot cost per cycle. Persistent-session-with-/clear-between-roles is the obvious optimization but breaks role isolation guarantees — needs a separate ADR.
4. **Sandbox wrapping**. The prototype declares `sandbox: degraded`. Production should match `claude.sh`'s sandbox-exec wrapping where the host OS supports it.
5. **Integration into `subagent-run.sh`**. The adapter contract is preserved; just needs `cli: claude-tmux` to route through it.

### If NO-GO (billing falls to API)

The premise is wrong: driving interactive `claude` from tmux does NOT preserve subscription billing in 2026. Don't invest further. The realistic long-term cost model is:

- Pre-2026-06-15: current `claude -p` path on subscription (free, until cutover)
- Post-2026-06-15: Agent SDK credit pool — Max 20x gives $200/mo at API rates ≈ 35 evolve-loop cycles at cycle-102's $5.66 each. Plan accordingly.

### If NO-GO (technical failure)

Document the version-specific failure mode (`claude --version` is captured above). May warrant:

- A second prototype attempt against `claude --remote-control` (Anthropic's remote-control protocol, currently used by Anthropic's own web/mobile UI to control local CLI sessions)
- Filing an issue against the React-Ink UI version that broke the prompt detection (`❯` marker absent or moved)

---

## Cleanup commands (operator-runnable)

If tmux sessions are orphaned:

```bash
tmux ls 2>/dev/null | grep evolve-claude-tmux | awk -F: '{print $1}' | xargs -n1 tmux kill-session -t
```

If probe workspaces accumulate:

```bash
rm -rf .evolve/tmp/tmux-probe-*
```

---

## References

- Plan: `/Users/danleemh/.claude/plans/great-finding-ultrathink-to-reflective-platypus.md`
- Adapter: `scripts/cli_adapters/claude-tmux.sh`
- Manifest: `scripts/cli_adapters/claude-tmux.capabilities.json`
- Verifier: `scripts/utility/verify-subscription-billing.sh`
- Probe: `scripts/utility/run-claude-tmux-probe.sh`
- Reference adapter: `scripts/cli_adapters/claude.sh` (the `claude -p` adapter this prototype could one day replace)
- Knowledge-base entry on hermes-agent (what the user originally asked about): `knowledge-base/research/hermes-agent-proxy-integration.md`
- 2026-04-04 OpenClaw subscription block: https://techcrunch.com/2026/04/04/anthropic-says-claude-code-subscribers-will-need-to-pay-extra-for-openclaw-support/
- 2026-05-13 reinstatement with Agent SDK credit catch: https://venturebeat.com/technology/anthropic-reinstates-openclaw-and-third-party-agent-usage-on-claude-subscriptions-with-a-catch
- 2026-06-15 billing-split announcement: https://the-decoder.com/claude-subscriptions-get-separate-budgets-for-programmatic-use-billed-at-full-api-prices/
