# bridge — multi-CLI tmux-driven AI agent bridge

> **Status**: v0.1.0 — in development
> **What**: A standalone tool that drives interactive AI CLIs (Claude Code; v2: Codex, Antigravity) via tmux, preserving subscription billing instead of API metering.
> **License**: same as the host repo (until extracted to its own repo per plan v2).

`bridge` is an **independent sub-project**. It does not depend on its parent repository's runtime, scripts, or state. It can be vendored, extracted, or called from any consumer (CI, manual operators, other agent orchestrators).

---

## Why

After 2026-06-15, `claude -p` invocations bill against a programmatic credit pool, not the Claude Max subscription. Interactive `claude` (no `-p`) still bills the subscription. `bridge` wraps the interactive REPL with tmux so any caller can drive it programmatically while keeping the subscription billing path.

See `docs/design.md` for the full rationale.

---

## Install

```bash
bash install.sh                  # symlinks bin/bridge → $HOME/.local/bin/bridge
which bridge && bridge version   # smoke
```

Dependencies: `bash` 3.2+, `jq`, `tmux`, `openssl`, an active Claude CLI (`claude --version` ≥ 2.1.x).
macOS billing-verification additionally uses `security` (keychain) — no install needed.

---

## CLI surface

```
bridge launch     Run one subagent invocation (the main verb)
bridge probe      Detect available CLIs and capability tiers (JSON output)
bridge validate   Dry-run: parse profile, print resolved config, exit 0
bridge report     Re-print structured summary for a past workspace
bridge version    Print the bridge version
bridge help       Print help
```

Full reference: `docs/cli-reference.md`.

### Quick example

```bash
bridge launch \
  --cli=claude-tmux \
  --model=haiku \
  --allow-bypass \
  --profile=./profile.json \
  --prompt-file=./prompt.txt \
  --workspace=/tmp/bridge-run-$$ \
  --stdout-log=/tmp/bridge-run-$$/stdout.log \
  --stderr-log=/tmp/bridge-run-$$/stderr.log \
  --artifact=/tmp/bridge-run-$$/artifact.md
```

Exit codes: `0` success, `2` safety-gate, `3` cost-leak, `10` bad flags, `80` REPL boot timeout, `81` artifact timeout, `99` require-full not satisfied, `127` missing binary.

---

## Tests

```bash
bash tests/run-tests.sh --suite=unit          # fast, no network (~5s)
bash tests/run-tests.sh --suite=integration   # require claude + tmux (~2m)
BRIDGE_BILLING_TESTS=1 \
  bash tests/run-tests.sh --suite=billing     # opt-in, macOS-strong
```

---

## Layout

```
tools/agent-bridge/
├── bin/bridge                 # entrypoint
├── lib/                       # shared helpers (profile, probe, safety, verify, …)
│   └── manifests/             # per-CLI capability manifests
├── drivers/                   # per-CLI driver scripts
├── docs/                      # design, cli-reference, adding-a-driver
└── tests/                     # bats-core suites (unit / integration / billing)
```

---

## Adding a CLI

See `docs/adding-a-driver.md`. The contract: a `drivers/<cli>.sh` script with stable env-var inputs and the standardized exit-code table.

---

## Non-goals

- Detection-evasion (no header injection, fingerprint scrubbing, billing-id mutation)
- Multi-turn dialog driving (v1 is single-turn round-trip)
- Persistent-session optimization (every launch spawns a fresh tmux session in v1)
- v2 CLIs (Codex, Antigravity) — drivers are stubs in v1, returning exit 99

---

## References

- Plan: `~/.claude/plans/great-finding-ultrathink-to-reflective-platypus.md` (read-only design doc)
- Prototype validation: parent repo `docs/research/tmux-claude-driver-prototype.md`
- Anthropic 2026-06-15 billing-split: https://the-decoder.com/claude-subscriptions-get-separate-budgets-for-programmatic-use-billed-at-full-api-prices/
