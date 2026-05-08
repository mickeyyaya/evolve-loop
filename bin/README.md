# Operator Entry Points

Thin wrappers exposing read-only observability scripts via short, discoverable names. Each wrapper exec's into `scripts/observability/<name>` after resolving the repo root (so they work from any cwd).

| Command | Wraps | Purpose |
|---|---|---|
| `./bin/status` | `scripts/observability/evolve-status.py` | Current cycle + recent ledger summary |
| `./bin/cost <cycle>` | `scripts/observability/show-cycle-cost.sh` | Per-cycle token + cost breakdown |
| `./bin/health <cycle> <workspace>` | `scripts/observability/cycle-health-check.sh` | Anomaly fingerprint for any cycle (read-only) |
| `./bin/verify-chain` | `scripts/observability/verify-ledger-chain.sh` | Tamper-evident ledger chain check |
| `./bin/preflight` (v8.50+) | `scripts/release/full-dry-run.sh` | Full pipeline dry-run: regression + cycle simulate + release-pipeline --dry-run |
| `./bin/check-caps` (v8.51+) | `scripts/cli_adapters/_capability-check.sh` | Resolved capability tier for an adapter (auto-detects CLI if no arg) |

All wrappers are read-only. None mutate state, commit, or push. They are safe to run at any time, including mid-cycle.

## Why a separate `bin/` directory

- **Discoverability**: `ls bin/` lists everything an operator can run; nothing more, nothing less.
- **Convention**: Unix `bin/` semantics — user-runnable tools, not library code.
- **Stability**: the wrapper paths are part of the operator API; the underlying `scripts/observability/` paths can move without breaking operator workflows.

## What's NOT here

- `install.sh` / `uninstall.sh` — top-level by convention; referenced from CI.
- `scripts/release-pipeline.sh` — operator-runnable but heavyweight; kept distinct.
- `scripts/utility/probe-tool.sh`, `scripts/utility/calibrate.sh`, etc. — ad-hoc helpers, not part of the curated operator surface.

## Adding a new wrapper

1. Create `bin/<name>` (no extension).
2. Make it `chmod +x`.
3. Use the standard preamble:
   ```bash
   #!/usr/bin/env bash
   set -uo pipefail
   _self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
   _repo_root="$(cd "$_self_dir/.." && pwd)"
   exec <runtime> "$_repo_root/scripts/<subdir>/<target>" "$@"
   ```
4. Add a row to the table above.
