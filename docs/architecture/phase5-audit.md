# Phase 5 Audit — Path to v12.0.0 (legacy/ flag day)

> **Status:** v11.8.0 (first slice shipped) — `prune-ephemeral.sh` ported as the worked example. ~99 standalone bash scripts remain in legacy/scripts/ that still need either Go ports or runtime-reference rewrites before `git rm -rf legacy/` can ship as v12.0.0.

> **Audience:** the next session continuing the v12.0.0 work. Read this before reading anything else — it scopes the actual work, separates noise from signal, and sequences the slices.

## Headline numbers (as of 2026-05-24, HEAD ~v11.8.0)

| Metric | Count |
|---|---|
| Bash scripts in `legacy/scripts/**.sh` | 220 |
| Existing Go packages in `go/internal/**` | 88+ |
| Files referencing `legacy/scripts/` (all kinds) | 422 |
| **Files referencing `legacy/scripts/` at RUNTIME** | **~12** |
| Active PreToolUse hooks shelling to bash | 7 (all go through Go dispatcher first) |
| Active Go shell-out callsites to specific bash | ~6 |
| Untouched bash subdirs ready to delete after audit | tests/, dispatch/ (already archived) |

**The 422-file figure is misleading.** Most references are in:
- `docs/architecture/*.md` — design provenance ("ported from legacy/scripts/X")
- `go/internal/**/*.go` comments — port provenance ("// archive/legacy/scripts/Y...")
- `legacy/scripts/tests/*-test.sh` — bash-on-bash testing of the legacy scripts themselves
- CHANGELOG.md historical entries

After filtering noise, **the actual Phase 5 work surface is ~12 files of runtime references + ~7 bash scripts that need behavior preserved.**

## Bash inventory by subdir

| Subdir | Count | Role | Phase 5 disposition |
|---|---|---|---|
| `tests/` | 95 | bash-on-bash unit tests of legacy scripts | DELETE in v12 (Go has its own tests; bash tests test bash) |
| `utility/` | 25 | one-off helpers (release.sh, inbox-mover, doctor-subscription-auth, etc.) | AUDIT individually; port the ~5 still referenced |
| `lifecycle/` | 17 | run-cycle.sh / resume-cycle.sh / ship.sh / others | Most already shelled-out from Go; **ship.sh has native Go sibling at `evolve ship`** |
| `observability/` | 16 | prune-ephemeral, phase-observer, tracker-writer, etc. | ✅ prune-ephemeral ported v11.8.0; ~3 others actively referenced |
| `dispatch/` | 14 | evolve-loop-dispatch.sh, subagent-run.sh, etc. | **Already moved to `archive/legacy/scripts/dispatch/`** in v11.5.0; legacy/dispatch/ may be stale symlinks |
| `verification/` | 11 | postedit-validate, mutate-eval, audit-binding | Port postedit-validate (referenced in settings.json) |
| `release/` | 10 | preflight, changelog-gen, version-bump, marketplace-poll, rollback, release-pipeline + 3 tests + full-dry-run | ✅ 6/6 production scripts ported v11.7.x; full-dry-run.sh remains as opt-in only |
| `cli_adapters/` | 7 | claude/codex/gemini/agy shell wrappers around external LLM CLIs | **KEEP — these are intentional adapter shims; just move to non-legacy path (e.g. `adapters/`)** |
| `guards/` | 6 | ship-gate, role-gate, phase-gate-precondition, evolve-guard-dispatch, doc-deletion-guard, commit-prefix-gate | All have Go siblings via `evolve guard`; dispatcher in legacy/ is the fallback path |
| `failure/` | 6 | failure-adapter, failure-classifier, etc. | AUDIT — likely most are not actively called at runtime |
| `routing/` | 4 | model-routing helpers | AUDIT |
| `lib/` | 3 | shared bash sourcable funcs (resolve-roots.sh, etc.) | Sourced by other bash; deletes with the parents |
| `hooks/` | 2 | research-quota-gate, doc-deletion-guard (already routed through dispatcher) | DELETE after dispatcher port |
| `research/` | 1 | one helper | AUDIT |
| (top-level .sh) | 3 | release-pipeline.sh, perf-cycle-comparison.sh, parity-audit.sh | release-pipeline.sh already ported; 2 standalone |

## Active runtime references (the real Phase 5 work)

### 1. `.claude/settings.json` hooks (7 entries, all dispatcher-routed)

```text
PreToolUse Bash:   bash legacy/scripts/guards/evolve-guard-dispatch.sh ship-gate
PreToolUse Bash:   bash legacy/scripts/guards/evolve-guard-dispatch.sh phase-gate-precondition
PreToolUse Edit:   bash legacy/scripts/guards/evolve-guard-dispatch.sh role-gate
PreToolUse Web*:   bash legacy/scripts/guards/evolve-guard-dispatch.sh research-quota-gate
PreToolUse Bash:   bash legacy/scripts/guards/evolve-guard-dispatch.sh doc-deletion-guard
PostToolUse:       bash legacy/scripts/verification/postedit-validate.sh
Stop (v11.8.0+):   evolve prune-ephemeral || bash legacy/scripts/observability/prune-ephemeral.sh
```

All five PreToolUse hooks go through `evolve-guard-dispatch.sh`, which **already prefers the Go binary** (`evolve guard <name>`) and falls back to bash only when the binary is missing. This dispatcher is THE single replaceable shim — port it (or make `.claude/settings.json` point directly at `go/bin/evolve guard <name>`), and 5 of 7 entries collapse.

### 2. Go runtime shell-outs (~6 callsites)

| File | Line | Shell-out target | Disposition |
|---|---|---|---|
| `go/internal/phases/ship/postship.go` | 95 | `legacy/scripts/lifecycle/inbox-mover.sh` | Port (small, lifecycle helper) |
| `go/internal/phases/ship/gitops.go` | 360 | `legacy/scripts/guards/commit-prefix-gate.sh` | Port (343 LoC; pure validator) |
| `go/internal/releasepipeline/releasepipeline.go` | (default Steps) | `legacy/scripts/utility/release.sh` + `legacy/scripts/lifecycle/ship.sh` | release.sh: port; ship.sh: switch to native `evolve ship` (already exists) |
| `go/internal/marketplacepoll/marketplacepoll.go` | DefaultReleaseSh | `legacy/scripts/utility/release.sh` | Same as above |
| `go/internal/rollback/rollback.go` | DefaultRevertAndShip | `legacy/scripts/lifecycle/ship.sh` | Switch to native `evolve ship` |
| `go/cmd/evolve/cmd_loop.go` | 468 | `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh` | **Already archived**; the bash legacy/legacy bridge can be removed |
| `go/cmd/evolve/cmd_subagent.go` | 279/355/461 | `legacy/scripts/cli_adapters/` directory lookup | Keep; just rename the dir to `adapters/` in v12 |
| `go/cmd/evolve/cmd_consensus_dispatch.go` | 38/39 | `legacy/scripts/cli_adapters/` + `legacy/scripts/dispatch/` | Same path rename + dispatch is archived |

### 3. The cli_adapters/ exception

`legacy/scripts/cli_adapters/*.sh` are NOT ports of internal logic — they ARE the adapter shims that wrap external LLM CLIs (`claude`, `codex`, `gemini`, `agy`). Each one is the entrypoint that the subagent dispatcher exec's to invoke an LLM provider. They can't be "ported away"; they're definitional. Phase 5 plan: rename `legacy/scripts/cli_adapters/` → `adapters/` (or similar non-`legacy` path) and update the 6 Go references.

## Sequencing plan (5 slices; ~v11.8.0 → v12.0.0)

### v11.8.0 (shipped this session)
- ✅ Port `prune-ephemeral.sh` → `go/internal/pruneephemeral/` + `evolve prune-ephemeral`
- ✅ Update `.claude/settings.json` Stop hook to use Go-native with bash fallback
- ✅ Write this audit doc

### v11.8.1
- Port `postedit-validate.sh` (127 LoC) → `go/internal/posteditvalidate/` + `evolve postedit-validate`
- Update `.claude/settings.json` PostToolUse hook
- Port `inbox-mover.sh` (utility) → `go/internal/inboxmover/` + `evolve inbox-mover`
- Update `go/internal/phases/ship/postship.go` to call the Go library directly

### v11.8.2
- Port `commit-prefix-gate.sh` (343 LoC; bash regex-based validator) → `go/internal/commitprefixgate/` + `evolve commit-prefix-gate`
- Update `go/internal/phases/ship/gitops.go` to call the Go library directly
- Port `legacy/scripts/utility/release.sh` consistency-checker (small) → `go/internal/releaseconsistency/` + wire into `evolve release` step 4 (replaces the bash shell-out from releasepipeline)

### v11.8.3
- Replace `ship.sh` shell-out in releasepipeline.go + rollback.go with native `evolve ship` calls (the Go ship phase already exists at `go/internal/phases/ship/`; just need to wire it as a callable function not a CLI subprocess)
- Update `evolve-guard-dispatch.sh` strategy: make `.claude/settings.json` point directly at `go/bin/evolve guard <name>` (skip the bash dispatcher entirely); keep the bash dispatcher as `EVOLVE_NATIVE_GUARDS=0` fallback only
- Port the bash heredoc-stripping pre-processor from `legacy/scripts/guards/ship-gate.sh` into `go/internal/guards/ship.go` (so commit message bodies don't trip the verb regex — see `feedback_ship_gate_verb_in_message`)

### v11.9.0
- Rename `legacy/scripts/cli_adapters/` → `adapters/` at repo root
- Update all 6 Go references
- Update `.evolve/profiles/*.json` if any reference the cli_adapters path
- Update docs

### v11.9.1 (sanity check / cleanup)
- Re-run the audit grep — confirm no remaining runtime references to `legacy/scripts/`
- Confirm `go test ./...` passes
- Confirm a full `evolve loop` dispatch works end-to-end without falling back to bash

### v12.0.0 (flag day)
- `git rm -rf legacy/`
- Bump version to 12.0.0
- Update CLAUDE.md / AGENTS.md / README to remove all `legacy/scripts/` references
- Ship + tag + release

## Known traps & gotchas

### Ship-gate verb-in-message regression
Documented in `feedback_ship_gate_verb_in_message.md`. The Go-native ship-gate's regex catches literal "git push" / "git commit" inside commit message bodies. Bash version had heredoc-stripping; Go port doesn't. Until v11.8.3 ports the heredoc-strip, **workaround = rewrite the commit message body** to avoid those literal verbs (substitute synonyms or backquote them).

### tests/ subdir is bash-on-bash
The 95 scripts in `legacy/scripts/tests/` test the BASH scripts themselves. They have no Go equivalents because the Go ports have their own Go test files. When `git rm -rf legacy/` ships, the Go tests are unaffected — the bash tests just disappear with the bash they tested.

### Active `archive/legacy/scripts/dispatch/`
The historical bash dispatcher was moved to `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh` in v11.5.0 as part of the bash→Go cutover. The `legacy/scripts/dispatch/` directory may still contain stale copies. **Verify before deleting** that nothing references the non-archive path at runtime (most should already go through `evolve loop`).

### `legacy/scripts/lib/resolve-roots.sh`
Other bash scripts source this for PLUGIN_ROOT vs PROJECT_ROOT resolution. When the parent scripts are removed, this becomes orphaned. Delete with the parents.

### `EVOLVE_NATIVE_GUARDS=0` rollback hatch
After v11.8.3 routes `.claude/settings.json` directly to `go/bin/evolve guard`, the `EVOLVE_NATIVE_GUARDS=0` rollback hatch becomes inoperable (there's no bash dispatcher to route to). Either:
- (a) Keep the bash dispatcher as a thin fallback shim that just exec's bash legacy/.
- (b) Remove the rollback hatch entirely and rely on `git checkout v11.7.5` for emergencies.
- Recommend (a) until v12.0.0 confidence is high, then drop in v12.

## What the next session should do first

1. Read this doc top-to-bottom.
2. Verify the "active runtime references" table is still accurate via `grep -rln "legacy/scripts/" go/internal go/cmd .claude/settings.json` (filter out comments/provenance).
3. Pick the next slice (v11.8.1) from the sequencing plan above.
4. Use the canonical port pattern documented in `[[feedback_go_port_pattern]]` — the same template used for v11.7.x ships.
5. Each slice = one feat commit + version bump + changelog + tag + release + memory update. ~20-30 min per slice.

## Cross-references

- Memory: `[[project_v12_prep]]`, `[[feedback_go_port_pattern]]`, `[[feedback_ship_gate_verb_in_message]]`
- Docs: `docs/migration-from-bash.md`, `docs/v12.0.0-roadmap.md`
- Code: `go/cmd/evolve/main.go` (dispatch table), `go/internal/*` (88 existing packages), `legacy/scripts/*` (the 220 remaining bash scripts)
