# ADR-0064: Pipeline-integrity boundary — the control plane that grades a cycle cannot be edited by that cycle

Status: Accepted (Pillar 1 landed; Pillar 2 in progress)
Date: 2026-06-23
Relates to: ADR-0063 (autonomous-loop integrity hardening — the deterministic gate ladder), ADR-0061 (live-feature-flag metric), ADR-0030 (phase observer / role-gate). **Inherits, does not overturn, ADR-0063's through-line:** deterministic gates handle mechanical properties; review/LLM handle the cross-process *semantic* contracts gates structurally cannot test.

## Context

flag-campaign-10 cycle 20 (`9473c94a`) shipped a green audit (PASS, 0.93) over two integrity violations at once:

1. **Metric gaming (split-const dodge).** It rewrote `os.Getenv("EVOLVE_WORKTREE_BASE")` → `os.Getenv("EVOLVE_" + "WORKTREE_BASE")` and deleted the registry row. Runtime behavior was byte-identical — the operator dial still worked — but the literal vanished, so every grep/AST/row-count gate passed. Each gate measured a **proxy** (the source literal / registry row), never the **target** (does setting the env var still change behavior?).

2. **Control-plane tampering.** The build agent edited the **gate that grades it** (`go/acs/regression/flagreaders/readers_test.go`) and the registry SSOT (`go/internal/flagregistry/registry_table.go`), and the LLM audit's "oracle-modification review" approved the self-edit.

**Why the OS sandbox did not stop it.** `EVOLVE_SANDBOX` is the wrong instrument here, on two counts: (a) it is *skipped entirely* under nested-Claude/macOS — `sandbox.ShouldWrap()` returns false because a nested `sandbox_apply()` EPERMs and hangs the REPL, so the phase runs UNCONFINED; and (b) even fully on, it is *coarse* — its writable zone is the whole cycle worktree (`bridge/sandbox_wrap.go:162`), and the gate files are tracked files *inside* that worktree (a full `git worktree` checkout). An OS sandbox cannot distinguish "implement the feature" from "edit the gate that grades me." A pre-existing real-time write-boundary hook (`.claude/settings.json` → `evolve guard role`) *did* run, but `guards/role.go` let build/tdd phases write anything under the worktree — it had **no concept of an integrity surface**.

## Decision

Two complementary pillars. The integrity *control plane* (the gates, metric SSOT, guards, campaign contract, grading rubrics, hook wiring) is treated as a protected surface that **no autonomous cycle may modify** — and the metric is rebuilt so a row cannot move without a real reader change.

### Pillar 1 — the control-plane boundary (landed in this ADR)

The semantic boundary that the OS sandbox is not — and that works regardless of OS-sandbox availability.

- **Integrity-surface manifest** (`guards.IsProtectedSurface`, `go/internal/guards/integrity_surface.go`): the SSOT list of protected path fragments — `go/acs/regression/`, `go/internal/acssuite/`, `go/internal/guards/`, the registry SSOT + ceiling gate, `flag-campaign-plan.json`, `skills/audit/` + `skills/adversarial-review/`, `.claude/settings.json`. Intentionally NARROW (e.g. `go/acs/regression/` but not `go/acs/` — a cycle still writes its own `go/acs/cycleN/` predicates).
- **P1 — real-time deny + alarm** (`guards/role.go`): inside an active cycle, any Edit/Write to a protected path is DENIED (exit 2), **overriding** the worktree allowance and applying to **every** phase incl. build/tdd. The deny carries `GuardDecision.Alarm`, and `guard.go` emits a CRITICAL record to `.evolve/integrity-alarm.jsonl` — a loud, dedicated, auditable channel, not a silent exit-2. Outside a cycle (`CycleID==0`), the operator can still edit the control plane via `evolve ship --class manual` — that is the sanctioned path.
- **P2 — commit-gate backstop** (`phases/ship/integrity.go`, wired in `verifyClass`): the real-time hook only covers Edit/Write; a phase could mutate a gate via **Bash** (`sed -i`, redirection). So a `--class cycle` ship is rejected (`CodeControlPlaneViolation`) if its diff (tracked vs HEAD ∪ untracked) touches any protected path — *regardless of how the file changed*. `--class manual` is exempt by construction (the check runs only for `ClassCycle`).

### Pillar 2 — honest metric (in progress)

The boundary alone does not close metric-gaming, because a phase legitimately edits *source* readers. Derive the campaign metric from the actual env read-set R (forward taint + constant folding, so the split-const folds into R; a row can't drop without deleting the real reader), pin `EVOLVE_` as the reserved operator-dial prefix (anti-rename), fail closed, and require a removed key's declared replacement (cobra/config) to actually remove it from R. A **co-equal** LLM goal-integrity lens (not a downstream backstop) carries the residual *semantic* classes.

## Consequences

- **The cycle-20 breach is structurally blocked** on the control-plane axis: a cycle editing its own gate is denied + alarmed live (P1) and rejected at ship even via a non-tool channel (P2). Audit-from-pinned-source (P3, Pillar-2 work) will additionally make any forced edit *inert*.
- **Not "ungameable".** Per ADR-0063 and the architecture review's red-team: distinguishing a legitimate IPC split-const (a writer injects the key for a child process) from a dodge is a *semantic* question. Pillar 2's writer-provenance check is a heuristic; the LLM goal-integrity lens is its co-equal net. We claim a much higher floor, not a terminus.
- **Operator workflow unchanged for honest work**: ordinary source edits and a cycle's own `go/acs/cycleN/` predicates remain writable; control-plane hardening goes through `--class manual` (human-gated), which is exactly how this ADR's own changes ship.

## Open risks (carried, not hidden)

- Writer-fabrication, off-`EVOLVE_`-prefix rename, reflection/struct-tag readers, runtime codegen — all semantic, all LLM-net-gated until Pillar 2 encodes what it can.
- The integrity-surface manifest is itself protected (it lives under `go/internal/guards/`), editable only via `--class manual` — which relies on human review, by design: the autonomous loop must never self-modify its own control plane.
