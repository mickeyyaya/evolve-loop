# bridge — design

> **Status**: v0.1.0
> **Audience**: anyone adding a CLI driver, debugging bridge runs, or vendoring bridge into another project.

---

## 1. The problem

After **2026-06-15**, Anthropic split Claude subscription quota from programmatic credits:

- `claude -p <prompt>` (headless): bills the **programmatic credit pool** (full API rates)
- Interactive `claude` (REPL): bills the **subscription quota** (free until you hit the rate limit)

For agent orchestrators that drive Claude in tight inner loops, the post-cutover cost difference is significant: typical Max-20x = $200/mo subscription vs $5+/cycle on API rates.

`bridge` keeps the subscription path open by **driving the interactive REPL via tmux**: any caller can request "run this prompt, write this artifact" and bridge translates that into REPL keystrokes, polling, and artifact verification — invisible to the caller.

A prior prototype verified the billing path works on Claude Max-5x; this sub-project productionizes it.

---

## 2. Layered architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ bin/bridge                                                       │
│  - Dispatcher: subcommands (launch / probe / validate / report / │
│    add-rule / version / help)                                    │
│  - cmd_launch: arg parser → profile_load → manifest_load → driver│
└──────┬──────────────────────────────────────────────────────────┘
       │ sources
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ lib/                          (pure helpers, no I/O if possible) │
│  profile.sh         — parse agent profile JSON                   │
│  manifest-loader.sh — parse per-CLI manifest JSON                │
│  manifest-patcher.sh — append rules (used by bridge add-rule)    │
│  probe.sh           — capability tier resolution                 │
│  auto-respond.sh    — fallback prompt-detection (decide + tick)  │
└──────┬──────────────────────────────────────────────────────────┘
       │ sources
       ▼
┌─────────────────────────────────────────────────────────────────┐
│ drivers/                      (one per CLI; each defines         │
│  claude-p.sh                   drv_launch_<cli>)                 │
│  claude-tmux.sh                                                  │
│  codex.sh    (v2 stub, rc=99)                                    │
│  agy.sh      (v2 stub, rc=99)                                    │
└──────┬──────────────────────────────────────────────────────────┘
       │ shells out to
       ▼
┌──────────────────────────────────────────────────────────────────┐
│ External tools: claude, tmux, jq, openssl                        │
└──────────────────────────────────────────────────────────────────┘
```

**Independence invariant**: nothing under `tools/agent-bridge/` `source`s files outside the sub-project. Verified by:

```bash
grep -rn '\.\./' tools/agent-bridge/ | grep -v '^tools/agent-bridge/' | grep -vE '(README|design|\.bats)'
```

Must return empty. This makes `tools/agent-bridge/` `git filter-branch`-extractable.

---

## 3. The two drivers

### 3.1 `claude-p` — headless

| Aspect | Detail |
|---|---|
| Underlying call | `claude -p "<prompt>" --model <m> --allowedTools <list>` |
| Billing path | **Programmatic credit pool** (post-2026-06-15) |
| Permission strategy | `--allowedTools` whitelist from `profile.allowed_tools[]` |
| Use case | When the operator explicitly accepts API-rate billing, or pre-cutover |
| Cost guard | Refuses to run if `ANTHROPIC_API_KEY` set or `ANTHROPIC_BASE_URL` set without explicit opt-in |

### 3.2 `claude-tmux` — interactive

| Aspect | Detail |
|---|---|
| Underlying call | `claude --model <m> --dangerously-skip-permissions` (interactive REPL inside a tmux session) |
| Billing path | **Subscription quota** (load-bearing claim, verified by prototype) |
| Permission strategy | `--dangerously-skip-permissions` covers the common case; **auto-respond fallback** for edge cases |
| Use case | Default for cost-conscious workflows post-cutover |
| Cost guard | Same as claude-p + requires `--allow-bypass` to acknowledge the bypass flag |

The mechanism (from the prototype, line-numbered):
1. `tmux new-session -d -s NAME` — detached pane (line ~84 in `drivers/claude-tmux.sh`)
2. `tmux send-keys "cd $workdir" Enter` then `claude … Enter`
3. Poll `tmux capture-pane | grep -F ❯` for **REPL prompt** (60s timeout, full-pane search — see "the prompt-detection fix" below)
4. `tmux load-buffer FILE; tmux paste-buffer` for the prompt (multi-line-safe)
5. Poll for artifact file (300s timeout) WHILE auto-respond ticks
6. Capture scrollback, send `/exit`, kill tmux session via trap

**The prompt-detection fix**: the prototype's first run failed because `tmux capture-pane | tail -5 | grep ❯` was looking at the wrong rows. The Ink-based UI renders `❯` mid-pane with horizontal separators above/below and empty padding rows after. Search the full pane.

---

## 4. Auto-respond — fallback policy engine

`--dangerously-skip-permissions` only suppresses **permission** prompts. It doesn't suppress:
- Auth re-checks ("Please log in again")
- Rate-limit warnings
- Model deprecation continuations
- Terminal-resize redraw prompts
- Anything else interactive that's not permission-gated

`lib/auto-respond.sh` is a **policy engine** that:
1. Reads the per-CLI manifest's `interactive_prompts[]` array
2. Polls the tmux pane between artifact checks
3. Matches the pane against each `regex`
4. Acts based on `policy`:
   - `auto_respond` → send the keys
   - `escalate` → bail rc=85 + write `escalation-report.json`
5. **Loop guard**: same pattern matching >5× in one run → bail rc=86 (the regex is probably too greedy)

The function is split into a pure `auto_respond_decide` (testable without tmux) and an effectful `auto_respond_tick` that wraps it with tmux I/O. T13 covers the pure layer.

### 4.1 Operator learning loop

When bridge bails rc=85, it writes `workspace/escalation-report.json`:

```json
{
  "schema_version": 1,
  "captured_at": "...",
  "cli": "claude-tmux",
  "pattern_name": "unknown_prompt_xxx",
  "reason": "escalate",
  "pane_tail": "...last 30 lines of pane...",
  "suggested_rule_template": { ... },
  "next_steps": [ "Run: bridge add-rule --escalation=..." ]
}
```

The operator:
1. Reads the report; identifies the prompt the agent is stuck on
2. Runs `bridge add-rule --escalation=<report> --regex=R --response=KEYS`
3. The new rule is appended to the manifest
4. Re-running the workflow auto-responds correctly

This is the **policy-learning loop**: bridge accumulates known prompts over time without manual code changes.

### 4.2 `response_keys` encoding

Comma-separated tmux key names:

| Encoding | tmux command | Semantics |
|---|---|---|
| `"Enter"` | `tmux send-keys T Enter` | Press Enter (e.g., redraw prompt) |
| `"y,Enter"` | `tmux send-keys T y Enter` | Type y then Enter (e.g., "continue?") |
| `"3,Enter"` | `tmux send-keys T 3 Enter` | Decline + push back (claude's option 3) |
| `null` | n/a | Policy must be `escalate` |

Why comma-separated and not `"\n"`: shell command substitution (`$(jq ...)`) strips trailing newlines, so `"\n"` alone gets eaten. Named keys are explicit.

---

## 5. Manifests

Per-CLI capability declared in `lib/manifests/<cli>.json`. The schema (v1):

```json
{
  "schema_version": 1,
  "cli": "claude-tmux",
  "binary": "claude",
  "binary_min_version": "2.1.0",
  "default_tier": "hybrid",
  "tier_dependencies": {
    "full":     ["claude", "tmux"],
    "hybrid":   ["claude", "tmux"],
    "degraded": ["claude"]
  },
  "prompt_marker": "❯",
  "default_model": "haiku",
  "default_args": ["--dangerously-skip-permissions"],
  "interactive_prompts": [
    { "name": "...", "regex": "...", "response_keys": "...", "policy": "...", "note": "..." }
  ],
  "stub": false
}
```

| Field | Purpose |
|---|---|
| `binary` | Executable name (looked up via `command -v`) |
| `default_tier` | The "best this CLI can be" (full / hybrid / degraded / none) |
| `tier_dependencies[tier]` | Other binaries that must also be present to qualify for that tier |
| `prompt_marker` | REPL-ready signal for capture-pane (claude-tmux only) |
| `default_args` | Args always passed to the underlying CLI |
| `interactive_prompts[]` | Auto-respond rules (see §4) |
| `stub` | When `true`, probe returns tier=none unconditionally |

---

## 6. Profiles

Per-invocation policy. The schema (v1):

```json
{
  "name": "agent-name",
  "model": "haiku",
  "allowed_tools": ["Read", "Write", "Bash(git status)"],
  "auto_respond": {
    "destructive_ops": false,
    "timeout_s": 600
  },
  "prompt_overrides": []
}
```

`prompt_overrides[]` is reserved for v2: per-invocation overrides on top of manifest's `interactive_prompts[]` (e.g., "for this specific cycle, also auto-respond to X").

---

## 7. Exit codes

| Code | Meaning |
|---|---|
| 0 | success — artifact written, no escalations |
| 2 | safety gate — `--allow-bypass` missing for claude-tmux |
| 3 | cost-leak guard — `ANTHROPIC_API_KEY` or proxy var set |
| 10 | bad flags or missing required arg |
| 80 | REPL boot timeout (claude didn't show `❯` in 60s) |
| 81 | artifact-wait timeout (artifact never written in 300s) — escalation report emitted |
| 85 | unknown / known-escalate prompt — escalation report emitted |
| 86 | auto-respond loop guard (same pattern matched >5×) |
| 99 | `--require-full` set, but CLI doesn't reach full tier |
| 127 | required external binary missing |

---

## 8. Non-goals

- Detection-evasion (header injection, fingerprint scrubbing, billing-id mutation)
- Multi-turn dialog driving (single-turn round-trip in v1)
- Persistent-session optimization (every `bridge launch` is fresh)
- v2 CLIs (codex, agy) — drivers are stubs returning 99

---

## 9. Future risks

| Risk | Mitigation |
|---|---|
| Anthropic adds typing-cadence fingerprinting | Detect via post-hoc billing check (see lib/billing-snapshot.sh) |
| Claude's REPL UI changes (Ink layout) | Manifest `prompt_marker` is configurable; pin claude version |
| Subscription terms restrict programmatic interactive use | Operator policy; bridge documents non-goal §8 |
| 2026-06-15 cutover changes interactive billing semantics | Verifier signals correctness; design.md tracks the rule |

---

## References

- `README.md` — install & quick example
- `docs/cli-reference.md` — full CLI surface
- `docs/adding-a-driver.md` — how to support a new CLI

