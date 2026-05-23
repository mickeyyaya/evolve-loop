# Migration: bash → Go (v11.0.0 cutover)

> **Status:** v11.0.0 introduces the Go binary as the **primary** runtime entrypoint. The bash scripts under `legacy/scripts/` continue to work and remain on disk. Set `EVOLVE_USE_LEGACY_BASH=1` to opt back into the bash-first dispatch.
>
> **Audience:** Operators upgrading from v10.x. New users should read [README.md](../README.md) first.

## TL;DR

| Question | Answer |
|---|---|
| Do I have to migrate now? | No. v11.0.0 keeps bash fully functional. The Go binary is the new default but not the only path. |
| Will my existing cycle state break? | No. `.evolve/state.json`, `cycle-state.json`, `ledger.jsonl`, and `instincts/*.yaml` use identical schemas in both runtimes. |
| Will my custom hooks break? | No. `.claude/settings.json` hook commands continue to invoke the same `legacy/scripts/guards/*.sh` files at the same paths. |
| Will the bash scripts be removed? | Not in v11.0.0. Physical relocation to `legacy/scripts/` is scheduled for v11.1.0 with a deprecation window. |
| What if Go misbehaves? | Set `EVOLVE_USE_LEGACY_BASH=1` to force the bash-first dispatch. This is the rollback hatch. |

## What changed in v11.0.0

| Surface | Pre-v11 (bash-first) | v11.0.0 (Go-first) |
|---|---|---|
| Plugin manifest | `agents/*.md` + `skills/*/` only | Adds `binaries[]` array; Go binary tier-1 primary |
| `evolve cycle run` | Did not exist | Drives one full cycle through 8 phases in-process |
| `evolve phase <name>` | Did not exist | Subprocess phase runner (raw JSON) |
| `evolve serve-phase <name>` | Did not exist | Envelope-framed phase runner (`phaseproto` wire) |
| `evolve doctor probe <tool>` | `legacy/scripts/utility/probe-tool.sh` | Same logic, better exit codes + JSON output |
| `evolve guard <name>` | `legacy/scripts/guards/*.sh` | Same predicates, in-process |
| `evolve ledger verify` | `legacy/scripts/observability/verify-ledger-chain.sh` | Same logic, faster |
| `evolve acs run --cycle N <pkg>` | `bash acs/cycle-N/*.sh` | Go test runner for ported predicates |
| `evolve loop` | `legacy/scripts/dispatch/evolve-loop-dispatch.sh` | Same dispatch semantics, native |

## Install the Go binary

```bash
# From source (~10s)
cd go && make build
./bin/evolve version

# Cross-arch artifacts via the release pipeline
cd go && make dist
ls bin/dist/
#   evolve-darwin-amd64
#   evolve-darwin-arm64
#   evolve-linux-amd64
#   evolve-linux-arm64
```

Drop the binary anywhere on `PATH`, or set `EVOLVE_GO_BIN=<path>`. The
plugin manifest (`.claude-plugin/plugin.json:binaries[].artifacts`)
already declares the expected layout.

## Behavioural parity

The Go orchestrator was built to produce **byte-identical** artifacts for the same fixture cycle:

- `.evolve/runs/cycle-N/scout-report.md`, `build-report.md`, `audit-report.md`, `ship-report.md`
- `.evolve/runs/cycle-N/acs-verdict.json` (EGPS gate)
- `.evolve/cycle-state.json` (phase transitions)
- `.evolve/ledger.jsonl` (timestamps differ; SHA chain reproduces)

Verify on your own fixtures:

```bash
bash legacy/scripts/parity-audit.sh --dry-run    # report-only, no spend
bash legacy/scripts/parity-audit.sh --simulate   # no-LLM smoke check
bash legacy/scripts/parity-audit.sh --full       # one real cycle each side (~$10-40)
```

See [legacy/scripts/parity-audit.sh](../legacy/scripts/parity-audit.sh) for the diff list.

## Performance baseline

Structural benchmarks (`cd go && go test -bench=. -benchmem ./...`):

| Operation | Go (Apple M4 Pro) | Bash baseline | Δ |
|---|---|---|---|
| Wire envelope encode | 1.1 µs | n/a (in-process) | — |
| Wire envelope decode | 5.1 µs | n/a | — |
| Ledger append (single entry) | 70 µs | 10–20 ms | ~150–280× |
| Ledger verify (100 entries) | 656 µs | ~1–2 s | ~1500–3000× |

LLM-bound paths (Builder, Scout, Auditor) are dominated by Claude API
latency in both runtimes; structural overhead is the win.

## Rolling back

If a Go-side bug bites you mid-cycle:

```bash
# Force the bash dispatch for one invocation
EVOLVE_USE_LEGACY_BASH=1 bash legacy/scripts/dispatch/evolve-loop-dispatch.sh

# Or persist for a session
export EVOLVE_USE_LEGACY_BASH=1
```

`EVOLVE_USE_LEGACY_BASH=1` reverts to v10.x behaviour — the bash scripts
remain canonical. The Go binary is not consulted.

Report the issue at <https://github.com/mickeyyaya/evolve-loop/issues>
with `cycle-state.json` and the failing log line attached.

## Deprecation schedule

| Version | Change | Status |
|---|---|---|
| v11.0.0 | Go binary tier-1 primary in plugin manifest. Bash scripts unchanged in place at `scripts/`. | SHIPPED |
| v11.1.0 | `scripts/` → `legacy/scripts/` physical move. `scripts/` became a backcompat symlink → `legacy/scripts/`. All existing references worked via the symlink. | SHIPPED |
| v11.2.0 | `scripts/` symlink REMOVED. Every in-repo reference (CLAUDE.md, AGENTS.md, hooks, agents/, skills/, docs/, Go source) now uses `legacy/scripts/...` directly. **Breaking change for operator integrations that hardcode `scripts/...` — they must update to `legacy/scripts/...`.** | SHIPPED |
| v11.3.0 (this release) | Native Go ship phase (`go/internal/phases/ship/{native,verify,audit,gitops,postship,statefile,dryrun}.go`) replaces the shell-out to `legacy/scripts/lifecycle/ship.sh` for cycle commits. Routed via `EVOLVE_NATIVE_SHIP=1` (default). Set `EVOLVE_NATIVE_SHIP=0` to revert to the bash shell-out (rollback path through v11.x). New CLI: `evolve ship [--class cycle\|manual\|release\|trivial] [--dry-run] "<msg>"`. 23-test parity gate (`go/internal/phases/ship/native_test.go`) mirrors `legacy/scripts/tests/ship-integration-test.sh`. | SHIPPED |
| v12.0.0 | Bash scripts removed entirely. Go-only. | DEFERRED — see [v12.0.0-roadmap.md](v12.0.0-roadmap.md) for the v11.4.0→v11.7.0 sub-release sequence (guards, EGPS predicates, dispatch, release-pipeline). |

## See also

- [docs/architecture/portable-core.md](architecture/portable-core.md) — what to vendor when adopting evolve-loop into another project
- [legacy/scripts/parity-audit.sh](../legacy/scripts/parity-audit.sh) — bash/Go diff harness
- [go/README.md](../go/README.md) — Go-side build, layout, and contribution notes
- [CHANGELOG.md](../CHANGELOG.md) — full v10.x → v11.0.0 changelog
