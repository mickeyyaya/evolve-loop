# Domain-Phase Campaign — Failed-Cycle Forensics (cycles 1–4, clone batch)

**Date:** 2026-06-06 · **Branch:** `feat/domain-phase-catalog` · **Context:** first 4 cycles of the domain-phase-catalog campaign produced 0 ships; every failure was dissected with code-level evidence. The audit gate was healthy throughout — the disease is failure-information transport (consistent with the cycles-215-231 canonical retro).

## Defect table

| # | Defect | Evidence | Class | Fix |
|---|---|---|---|---|
| D1 | Grep-only doc-status ACS predicates (cycles 2+4, FAIL 0.95 both) | `acs/cycle-2/006,007`, `acs/cycle-4/007` audit Issues tables | content (auditor correct) | Predicates over doc tables must parse/assert the table state machine OR carry the sanctioned waiver `# acs-predicate: config-check` (the TDD prompt's own cycle-85 policy). Baked into spec §5 post-batch. |
| D2 | Persona mirror drift `account-reconcile` (cycle 3, FAIL 0.98) | ACS `cycle-3/002` red; audit H1 | content (auditor correct) | Author phase-dir `agent.md` → `cp` to `agents/evolve-<name>.md` → `cmp` proof in build report. Structural candidate: byte-equality check inside `evolve phases validate` (deterministic > discipline). |
| D3 | **Retro skipped on every FAIL** → no retrospective, no carryover, no lessons | `cycle-4/retro-usage.json` = `verdict SKIPPED, duration 0ms, cost 0` (same 2,3) | pipeline (keystone) | Clamp retro mandatory when audit ∈ {FAIL, WARN}. = retro migration step 3 (`failure-supervision-tree`). |
| D4 | Cycle FinalVerdict mislabeled `SKIPPED_UNKNOWN` (audit FAIL masked) | `orchestrator.go:815` `finalizeOutcome` passes through `lastVerdict`, which `orchestrator.go:1697` set to the *retro's* SKIPPED; comment at `:1738` cites ancestor cycle-148 | pipeline | Thread the audit verdict into CycleResult; FAIL cycles must label FAIL in batch summaries. |
| D5 | Operator carryoverTodos injection clobbered | No Go consumer of `carryoverTodos` (only `ports.go:70` + LLM prompt pointers `bedrock.go:111`); `orchestrator.go:~1755` WriteState writes the **cycle-start snapshot** at cycle end → mid-cycle injects erased | pipeline | Merge-on-write (re-read + union carryoverTodos before WriteState). Operator workaround: inject only between cycles, or use committed spec docs as the lesson channel. |
| D6 | evolve-verdict sentinel emitted 1-of-3 audits | cycle-3 report has `<!-- evolve-verdict: ... -->`; cycles 2/4 none — prose drift `**Status:** FAIL` vs `## Verdict: FAIL` (`sentinel.go:15` exists for exactly this) | pipeline | Deliverable contract should REQUIRE the sentinel for audit reports (ADR-0033 deferred item). |
| D7 | Bridge launch exit=80 + orphaned agent wrote unsupervised residue to clone ROOT (cycle 1) | `~/.claude.json` clone path `hasTrustDialogAccepted:false` → worktree pane hung on trust dialog; bridge declared launch-fail; agent later ran anyway | infra | Fixed in-session (trust the path before campaigns in fresh clones). Open hazard: bridge "launch failed" must kill the pane, not orphan it. |
| D8 | `--resume` DOA ("no checkpoint block"); agy builder 0-for-N (`ExitArtifactTimeout` 81, `bridge/exitcodes.go:31`), claude fallback rescued each; cost telemetry $0 in tmux mode | loop log + usage sidecars | infra | Checkpoint at every phase boundary (migration step 3); agy weak-signal auto-respond (migration step 6); tmux-mode cost metering gap known. |

## Causal chain of the zero-ship streak

```
audit FAIL (correct) ──> retro SKIPPED (D3) ──> no carryover/lessons
        │                                        │
        └─> FinalVerdict mislabeled (D4)         └─> next cycle relearns blind
                       │                              │
operator injects lessons ──> clobbered by stale WriteState (D5)
                       │
scout disk-forensics (cycle 4+) ──> the one channel that DID transmit
```

Convergence was finally achieved when (a) scout itself carried the lessons forward in its report, and (b) the TDD prompt's built-in waiver syntax was applied.

## Operator playbook (until structural fixes land)

1. Fresh clone campaigns: pre-trust the clone path in `~/.claude.json` BEFORE launch.
2. Treat committed spec docs (re-read every cycle) as the durable lesson channel; `state.json` injections are racy.
3. On a FAIL cycle: read `audit-report.md` Issues yourself — the batch summary label (`SKIPPED_UNKNOWN`) hides FAILs.
4. Don't commit to the clone root mid-batch (advancing HEAD breaks FF-merge of in-flight cycle worktrees).
5. `--resume` works only from graceful pauses; bridge-fatal aborts need `evolve cycle reset`.
