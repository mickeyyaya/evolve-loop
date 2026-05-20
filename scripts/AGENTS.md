# scripts/ — Shell Discipline Guide

> **Directory purpose**: All pipeline shell scripts live here. This file documents
> the conventions every script author must follow: compatibility target, banned
> patterns, required idioms, and lifecycle subdirectory contracts.

## Compatibility target

**Bash 3.2** — macOS default shell since 10.3 (Panther). All scripts MUST run on
bash 3.2 without modification. Never rely on bash 4+ features.

## Banned patterns

| Pattern | Reason | Alternative |
|---|---|---|
| `declare -A` | bash 4+ associative arrays | Encode key-value in plain strings; use `grep`/`cut` to parse |
| `mapfile` / `readarray` | bash 4+ | `while IFS= read -r line; do … done < file` |
| `${var^^}` / `${var,,}` | bash 4+ case conversion | `echo "$var" \| tr '[:lower:]' '[:upper:]'` |
| `sed -i ''` | BSD/GNU incompatibility | Write to `${file}.tmp.$$` then `mv` atomically |
| `date -d` | GNU-only date flag | Use `date -u -j -f` on macOS; fallback: `gdate \|\| date -d \|\| date -j -f` |
| `grep -q … \| somecommand` on large input | SIGPIPE race under `set -o pipefail` | Use `[[ $var =~ pattern ]]` (bash ERE, no pipe) |

## Required idioms

### Error handling header

```bash
#!/usr/bin/env bash
set -uo pipefail
```

Use `set -uo pipefail`, NOT `set -e`. Orchestrator scripts capture sub-script exit
codes and must not abort on first error.

### Atomic file writes

Never write to a live path directly. Use a temp file and `mv`:

```bash
tmpfile="${target}.tmp.$$"
{
  echo "content"
} > "$tmpfile"
mv "$tmpfile" "$target"
```

The `$$` suffix makes the temp name process-unique. `mv` is atomic on POSIX
filesystems (same-device rename). Crash-safe: a partial write leaves `target`
unchanged.

### Tree-state SHA (audit binding)

Use `git diff HEAD` output hash, NOT `git stash` or working-tree-only hashes.
Untracked files do not appear in `git diff HEAD` — intentional. ship.sh uses
the same model so audit-binding is consistent.

```bash
tree_sha=$(git diff HEAD | sha256sum | cut -d' ' -f1)
```

### Cross-platform date (ISO-8601 UTC)

```bash
# Prefer gdate if available (GNU coreutils on macOS via Homebrew)
if command -v gdate >/dev/null 2>&1; then
  ts=$(gdate -u +%Y-%m-%dT%H:%M:%SZ)
elif date -u -j -f '%s' "$(date +%s)" '+%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
  ts=$(date -u -j -f '%s' "$(date +%s)" '+%Y-%m-%dT%H:%M:%SZ')
else
  ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
fi
```

## Subdirectory contracts

| Subdirectory | Purpose |
|---|---|
| `scripts/dispatch/` | Subagent spawning: `subagent-run.sh`, `run-cycle.sh`, `evolve-loop-dispatch.sh` |
| `scripts/lifecycle/` | Phase transitions: `ship.sh`, `phase-gate.sh`, `phase-gate-precondition.sh`, `run-regression-suite-slice.sh` |
| `scripts/guards/` | Kernel hooks: `role-gate.sh`, `ship-gate.sh`, `commit-prefix-gate.sh`, `doc-deletion-guard.sh` |
| `scripts/failure/` | Failure classification: `failure-adapter.sh` |
| `scripts/observability/` | Ledger + metrics: `verify-ledger-chain.sh`, `tracker-writer.sh` |
| `scripts/verification/` | Eval quality gates: `mutate-eval.sh`, `lint-acs-predicates.sh`, `validate-predicate.sh` |
| `scripts/research/` | Knowledge-base search: `kb-search.sh` |
| `scripts/utility/` | Shared helpers: `probe-tool.sh`, `diff-complexity.sh`, `doctor-subscription-auth.sh` |

## Kernel scripts

Scripts in `scripts/guards/` are Tier-1 kernel hooks. They run in privileged
shell context as PreToolUse hooks (`.claude/settings.json`). Kernel scripts:

1. Source `scripts/utility/resolve-roots.sh` to split `PLUGIN_ROOT` (reads) from
   `PROJECT_ROOT` (writes). Never hardcode a single `REPO_ROOT`.
2. MUST NOT be called by Builder personas — Builder profiles deny execution of
   guards directly.
3. Changes require an accompanying audit justification in the commit body.

## Probe before declaring unavailable

Before declaring any CLI or tool unavailable, run the probe helper:

```bash
bash scripts/utility/probe-tool.sh <tool>
```

or manually check:

```bash
command -v <tool> || which <tool> || ls /usr/local/bin/<tool> ~/.local/bin/<tool>
```

False-negative "tool not found" reports without a probe trail are a recurring
audit defect (see `/insights`).

## Related files

- `AGENTS.md` (repo root) — cross-CLI invariants + bash 3.2 mandate (Rule 11)
- `CLAUDE.md` — banned/required shell pattern table
- `.evolve/profiles/AGENTS.md` — per-role permission profile schema
