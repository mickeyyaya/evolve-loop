# Portable Core: Vendoring Evolve-Loop into Another Project

> **Status:** Authoritative file list for vendoring evolve-loop's runtime kernel into any project using `claude -p`. Lists required vs optional files, the trust-kernel invariants the core preserves, and a bootstrap outline. The full distribution is plugin-installable via marketplace; this doc is for operators who need to vendor a subset directly into their own repository.

## Contents

- [Audience](#audience)
- [What "portable core" means](#what-portable-core-means)
- [Required file set (minimal core)](#required-file-set-minimal-core)
- [Optional extensions](#optional-extensions)
- [Trust kernel invariants](#trust-kernel-invariants)
- [Bootstrap outline](#bootstrap-outline)
- [What is intentionally NOT in the core](#what-is-intentionally-not-in-the-core)
- [Version coupling](#version-coupling)

## Audience

Operators who:
- Use `claude -p` (Claude Code CLI) and want autonomous Scout → Builder → Auditor → Ship cycles in their own project.
- Cannot or will not install the full evolve-loop plugin from the marketplace.
- Need to vendor evolve-loop's runtime into a private monorepo, an air-gapped environment, or a non-Anthropic CLI harness with compatible primitives.

If you can install the plugin (most users), the marketplace path is preferable — it auto-updates with releases. This doc exists for the cases where direct vendoring is the only option.

## What "portable core" means

The minimal subset of evolve-loop that preserves these invariants:

| Invariant | Enforced by |
|---|---|
| Personas cannot spawn personas | `orchestrator.json:disallowed_tools` + `phase-gate-precondition.sh` |
| Each phase runs only after its predecessor produced an artifact | `phase-gate-precondition.sh` |
| Each role has its own tool permissions | `role-gate.sh` + `.evolve/profiles/<role>.json` |
| Only PASS audit + matching tree-SHA may ship | `ship-gate.sh` + audit-binding |
| Commit prefixes are bounded by scope | `commit-prefix-gate.sh` + `.evolve/commit-prefix-scope.json` |
| The audit ledger is a tamper-evident SHA chain | `verify-ledger-chain.sh` + `prev_hash`/`entry_seq` |
| EGPS predicates execute against ground truth, not prose claims | `audit-constitution-check.sh` + `validate-predicate.sh` |
| Personas don't duplicate trust-boundary boilerplate | `build-invocation-context.sh` (canonical shared prelude) |

Drop any one of these scripts and the corresponding invariant becomes unenforced.

## Required file set (minimal core)

### Kernel hooks (4 files)

Trust-kernel enforcement. These wire into the harness's `PreToolUse` / `Stop` hook slots and block forbidden actions deterministically.

| File | Role |
|---|---|
| `scripts/guards/phase-gate-precondition.sh` | Denies cross-phase tool calls before predecessor artifact exists |
| `scripts/guards/role-gate.sh` | Denies tool calls outside the active role's profile |
| `scripts/guards/ship-gate.sh` | Denies `git push origin main` unless audit-bound + verdict PASS |
| `scripts/guards/commit-prefix-gate.sh` | Denies commits whose prefix scope doesn't match changed paths |

### Lifecycle scripts (5 files)

Per-cycle state machine.

| File | Role |
|---|---|
| `scripts/lifecycle/ship.sh` | Atomic commit + push with ledger entry + audit-binding |
| `scripts/lifecycle/phase-gate.sh` | Phase-transition state machine (calibrate → research → … → learn) |
| `scripts/lifecycle/cycle-state.sh` | Reads/writes `.evolve/cycle-state.json` |
| `scripts/lifecycle/resolve-roots.sh` | Dual-root resolution (`PLUGIN_ROOT` for reads, `PROJECT_ROOT` for writes) |
| `scripts/lifecycle/role-context-builder.sh` | Role-filtered prompt context assembly (Layer B) |

### Dispatch scripts (7 files)

Subagent spawn + CLI adaptation.

| File | Role |
|---|---|
| `scripts/dispatch/subagent-run.sh` | The ONLY path through which personas may be spawned |
| `scripts/dispatch/run-cycle.sh` | Provisions per-cycle worktree, spawns orchestrator subagent |
| `scripts/dispatch/build-invocation-context.sh` | Canonical shared prelude (trust-boundary boilerplate + role notes) |
| `scripts/dispatch/resolve-llm.sh` | LLM/CLI selection from `.evolve/llm_config.json` + envelope |
| `scripts/dispatch/preflight-environment.sh` | Resolves `EVOLVE_WORKTREE_BASE`, sandbox availability |
| `scripts/dispatch/detect-cli.sh` | Identifies the active CLI (claude / gemini / codex / …) |
| `scripts/dispatch/detect-nested-claude.sh` | Auto-disables inner sandbox when nested |

### Verification scripts (3 files)

Audit-time integrity checks.

| File | Role |
|---|---|
| `scripts/verification/audit-constitution-check.sh` | Layer 4 constitutional checklist (P1-P6) |
| `scripts/verification/verdict-elevation.sh` | Layer 5 WARN→FAIL elevation on rule violations |
| `scripts/verification/validate-predicate.sh` | EGPS predicate executability + safety check |

### Observability (2 files)

Tamper detection + cycle health.

| File | Role |
|---|---|
| `scripts/observability/verify-ledger-chain.sh` | Validates SHA-chain integrity (`prev_hash` + `entry_seq`) |
| `scripts/observability/cycle-health-check.sh` | Detects reward-hacking / fabricated cycles |

### Profiles (6 files)

Per-role tool permissions. JSON schemas define `allowed_tools[]`, `disallowed_tools[]`, `parallel_eligible`, `output_artifact`.

| File | Role |
|---|---|
| `.evolve/profiles/orchestrator.json` | Read-only persona that sequences phases |
| `.evolve/profiles/scout.json` | Discovery + research; web-search permitted |
| `.evolve/profiles/builder.json` | Single-writer worktree; full edit/write |
| `.evolve/profiles/auditor.json` | Read-only repo + execute predicates |
| `.evolve/profiles/intent.json` | Intent capture (Layer A) |
| `.evolve/profiles/retrospective.json` | Failure post-mortem; writes lessons YAML only |

### Personas (8 .md files + 4 reference files)

The agent prompts that the dispatch layer spawns.

| Persona | Reference file | Role |
|---|---|---|
| `agents/evolve-orchestrator.md` | `agents/evolve-orchestrator-reference.md` | Cycle sequencer |
| `agents/evolve-scout.md` | `agents/evolve-scout-reference.md` | Discovery + task selection |
| `agents/evolve-builder.md` | `agents/evolve-builder-reference.md` | Implementation in worktree |
| `agents/evolve-auditor.md` | `agents/evolve-auditor-reference.md` | Single-pass review |
| `agents/evolve-intent.md` | — | Pre-scout intent capture (Layer A) |
| `agents/evolve-retrospective.md` | — | FAIL/WARN post-mortem |
| `agents/evolve-memo.md` | `agents/evolve-memo-reference.md` | PASS-cycle carryover memo (Layer P) |
| `agents/evolve-triage.md` | `agents/evolve-triage-reference.md` | Cycle-scope top-N selection (Layer C) |

### Configuration files (3 files)

Per-project tunables and scope manifests.

| File | Role |
|---|---|
| `.evolve/commit-prefix-scope.json` | Commit prefix → allowed-path map for `commit-prefix-gate.sh` |
| `.evolve/llm_config.json` | Tier-to-model mapping (consumed by `resolve-llm.sh`) |
| `AGENTS.md` (project root) | Cross-CLI invariants + 12 Core Agent Rules every persona reads |

**Total: 36 files** (4 guards + 5 lifecycle + 7 dispatch + 3 verification + 2 observability + 6 profiles + 12 personas/refs - the 4 reference files being counted separately + AGENTS.md + 2 .evolve configs). Plus the runtime artifacts created per cycle (`.evolve/runs/cycle-N/*.{md,json}`, `.evolve/cycle-state.json`, `.evolve/instincts/lessons/*.yaml`, the audit ledger).

## Optional extensions

Layers that compose on top of the core but are not load-bearing for the basic Scout → Build → Audit → Ship loop.

| Extension | Files | When to add |
|---|---|---|
| Plan review | `agents/evolve-plan-reviewer.md`, `.evolve/profiles/plan-reviewer.json` | When Scout misallocates effort and a CEO/Eng/Design/Security cross-check would catch it earlier |
| TDD-first | `agents/evolve-tdd-engineer.md`, `.evolve/profiles/tdd-engineer.json` | When the project's test discipline benefits from RED-phase test-first (failing tests encode acceptance) |
| EGPS Tester | `agents/evolve-tester.md`, `.evolve/profiles/tester.json` | When you need an independent predicate-author separate from Builder (anti-gaming) |
| Inspirer | `agents/evolve-inspirer.md`, `.evolve/profiles/inspirer.json` | When you want a "diff-aware reflection" persona between Scout and Builder |
| Evaluator | `agents/evolve-evaluator.md`, `.evolve/profiles/evaluator.json` | When you want a same-cycle counter-Auditor for cross-checking |
| Phase observer | `scripts/dispatch/phase-observer.sh`, `scripts/dispatch/phase-watchdog.sh` | When you need real-time stuck-loop detection during a cycle (gated by `EVOLVE_OBSERVER_ENABLED=1`) |
| Fan-out | `scripts/dispatch/fanout-dispatch.sh`, `aggregator.sh` | When read-only phases (Scout, Auditor) benefit from parallel sub-personas |
| Checkpoint/resume | `scripts/dispatch/resume-cycle.sh`, checkpoint-related hooks | When cycles routinely exceed 95% budget and partial work loss is unacceptable |

## Trust kernel invariants

These hold whether you vendor the core or install the full plugin. They are enforced by the kernel hooks listed above and operate BELOW the persona layer.

| # | Invariant | Failure mode if dropped |
|---|---|---|
| 1 | Personas cannot spawn personas | Same-instance cycles → reward hacking (cycle 102-111 incident) |
| 2 | Tree-SHA matches audit-binding at ship | Builder edits after audit, audit verdict no longer applies |
| 3 | Commit prefix scope matches changed paths | Mislabeled commits (cycle 70-72 incident) |
| 4 | Audit ledger is SHA-chained | Forged ledger entries (Gemini-forgery incident) |
| 5 | EGPS predicates are executable, not prose | AC-by-grep gaming (cycle 75 incident) |
| 6 | Personas don't duplicate trust-boundary boilerplate | Drift between persona text and kernel enforcement |
| 7 | Worktree-isolated Builder; single-writer per cycle | Concurrent writes corrupt cycle state |
| 8 | WARN-elevation when constitutional rules violated | Soft-pass on integrity defects (cycle 132-141 incident) |

Each invariant maps to a script in the minimal core. Removing the script removes the enforcement.

## Bootstrap outline

To wire the minimal core into a new project:

1. **Copy the 36 files** preserving the relative directory layout (`scripts/guards/`, `scripts/lifecycle/`, `scripts/dispatch/`, `scripts/verification/`, `scripts/observability/`, `.evolve/profiles/`, `agents/`, plus the 3 config files).

2. **Add `AGENTS.md` to the project root** with the cross-CLI invariants + 12 Core Agent Rules.

3. **Wire the kernel hooks into your harness's settings.**

   In Claude Code: `~/.claude/settings.json` (user) or `<project>/.claude/settings.local.json` (project):

   ```json
   {
     "hooks": {
       "PreToolUse": [
         { "matcher": "Bash|Edit|Write", "command": "bash scripts/guards/phase-gate-precondition.sh" },
         { "matcher": "Bash|Edit|Write", "command": "bash scripts/guards/role-gate.sh" },
         { "matcher": "Bash", "command": "bash scripts/guards/ship-gate.sh" },
         { "matcher": "Bash", "command": "bash scripts/guards/commit-prefix-gate.sh" }
       ]
     }
   }
   ```

4. **Initialize the per-project state directory:**

   ```bash
   mkdir -p .evolve/{runs,worktrees,instincts/lessons,profiles}
   cp <core>/.evolve/profiles/*.json .evolve/profiles/
   cp <core>/.evolve/commit-prefix-scope.json .evolve/
   cp <core>/.evolve/llm_config.json .evolve/
   echo '{"cycle":0,"phase":"calibrate","stop_reason":null}' > .evolve/cycle-state.json
   ```

5. **Smoke test** by running one cycle with `EVOLVE_BATCH_BUDGET_CAP=2.00` to cap cost:

   ```bash
   bash scripts/dispatch/evolve-loop-dispatch.sh --cycles 1 balanced "add a no-op task to verify wiring"
   ```

   Expect: `scout-report.md` → `build-report.md` → `audit-report.md` → `orchestrator-report.md` in `.evolve/runs/cycle-1/`, plus one commit on `main`.

6. **Verify the trust kernel** by attempting to bypass it (these should all fail):

   ```bash
   # Should be denied by ship-gate (no cycle commit):
   git push origin main

   # Should be denied by commit-prefix-gate (bogus prefix):
   git commit -m "bogus: this won't pass"

   # Should be denied by phase-gate-precondition (phase=calibrate, cannot reach build):
   bash scripts/dispatch/subagent-run.sh builder 1 .evolve/runs/cycle-1
   ```

If all three deny correctly, the kernel is wired. If any allow, the invariant is broken.

## What is intentionally NOT in the core

| Excluded | Why |
|---|---|
| `CHANGELOG.md`, `release-pipeline.sh`, `version-bump.sh` | Release tooling; vendor consumers ship their own releases |
| `docs/operations/*` incident reports | Historical record specific to evolve-loop the project |
| `skills/`, `plugins/`, `marketplace.json` | Plugin-distribution layer; vendoring bypasses this |
| 19 of the 27 phase-agent files | inspirer / evaluator / plan-reviewer / tdd-engineer / tester / operator / diagnose etc. are extensions |
| `docs/architecture/*` (except this file + `audit-constitution.md` + `posthoc-schema.md` + `control-flags.md`) | Internal design docs; vendor consumers don't need v8.61 archaeology |
| `acs/regression-suite/*` | Project-specific regression tests; vendor consumers write their own |

## Version coupling

The core is internally consistent at the level of a single release. If you vendor at version X.Y.Z, **do not mix files across versions** — kernel scripts and personas evolve together (e.g., `audit-constitution-check.sh` added in v10.9.0 requires v10.9+ `auditor.md`).

Verify alignment after vendoring:

```bash
grep -h '^# Version:' scripts/{guards,lifecycle,dispatch,verification,observability}/*.sh | sort -u
```

All scripts in the same vendored snapshot should report the same version line. Mismatches indicate partial vendor.

## References

- `AGENTS.md` (project root) — cross-CLI invariants + 12 Core Agent Rules
- `docs/architecture/audit-constitution.md` — Layer 4 P1-P6 checklist enforced by `audit-constitution-check.sh`
- `docs/architecture/posthoc-schema.md` — POSTHOC ground-truth-substitution contract
- `docs/architecture/control-flags.md` — every env-var and its default
- `docs/architecture/egps-v10.md` — EGPS predicate lifecycle
- `docs/architecture/sequential-write-discipline.md` — single-writer-per-worktree invariant
