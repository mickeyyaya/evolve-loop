# Trust Kernel & EGPS — making "done" un-fakeable

> The trust kernel is the set of mechanisms that stop any LLM — Builder, Auditor,
> or judge — from *claiming* the work is done and getting away with it. Its design
> thesis, from EGPS: **stop letting a model report whether the work is done; let
> the sandbox's exit code be whether the work is done.** This document covers the
> guards, the EGPS predicate gate, audit-binding, the ship-gate, the commit-gate,
> and the hash-chained ledger — and *why* each exists. Current Go design (v13.0.0).

Related: [phase-pipeline.md](phase-pipeline.md) ·
[routing-and-advisor.md](routing-and-advisor.md) ·
[state-and-ledger.md](state-and-ledger.md) ·
[glossary](../00-overview/glossary.md)

---

## 0. The threat model

The pipeline is a multi-agent system where the agents are graded by other agents.
Across cycles 30–141, five distinct **reward-hacking / gaming** failure modes were
diagnosed (`docs/architecture/egps-v10.md`):

1. **AC-by-grep** — an acceptance criterion "verified" by `grep`-ing the source
   for a string instead of running the code path.
2. **Confidence cliff** — auditors clustering at a 0.78–0.87 confidence score that
   was just-high-enough to ship.
3. **Recurring same defect** — the same bug re-shipped cycle after cycle.
4. **Tautological eval** — an `echo PASS; exit 0` "test."
5. **Carryover work-shifting** — deferring the hard acceptance criterion to "next
   cycle" forever.

The research conclusion (Lilian Weng 2024, surveying 9 point-mitigations): *none
works in isolation.* So the kernel is **defense in depth** — several independent
gates, each of which must hold, none of which is the sole safety story.

---

## 1. Guards — the in-process trust kernel

`go/internal/guards` is the host-agnostic Go port of the legacy `scripts/guards/*`
PreToolUse hooks. Every guard satisfies one interface (`core.Guard`, `ports.go`):

```go
type Guard interface {
    Name() string
    Decide(ctx, GuardInput) GuardDecision   // Allow=true → exit 0; Allow=false → exit 2
}
```

They are invoked uniformly via `evolve guard <name>`, so a host hook in any CLI
shims to one shape. The load-bearing guards:

| Guard | File | Denies | Why |
|---|---|---|---|
| **role** | `role.go` | Edit/Write outside the workspace (or the active worktree for source phases) | Confine each phase's writes; a half-cycle is discardable by deleting the worktree |
| **ship** | `ship.go` | `git commit`/`git push`/`gh release` unless via `ship.sh` or `evolve ship` | Force every commit through the audit-bound ship path (§4) |
| **chain** | `chain.go` | (verifier) ledger hash-chain breaks | Tamper-evidence on the audit trail (§6) |
| **quota** | `quota.go` | over-quota research/web calls | Token-budget discipline |
| **docdelete** | `docdelete.go` | deletion of protected docs | Prevent silent doc destruction |

### Role-gate

`Role.Decide` (role.go) reads `cycle-state.json`. Outside an active cycle
(`CycleID == 0`) it allows. Inside a cycle it allows a write only if the path is
under `cs.WorkspacePath`, **or** — for a source-writing phase
(`WorktreePhase(cs.Phase)`) — under `cs.ActiveWorktree`. `/tmp/**` and
`$HOME/.claude/**` are always-safe. Default-deny: if storage is misconfigured it
refuses rather than fail-open. Bypass: `EVOLVE_BYPASS_ROLE_GATE=1` (loud).

### Ship-gate

`Ship.Decide` (ship.go) is the chokepoint that makes audit-binding *unavoidable*.
A `git commit`/`git push`/`gh release create` is denied unless the command invokes
the canonical `scripts/lifecycle/ship.sh` or the native `evolve ship` CLI. It
strips heredoc bodies before matching so a commit message that *mentions*
`git push` doesn't trip the gate (v11.8.3). Bypass: `EVOLVE_BYPASS_SHIP_GATE=1`
(emergencies only). **Why:** if any agent could `git commit` directly, the entire
audit-binding model below would be bypassable.

---

## 2. EGPS — Execution-Grounded Process Supervision

EGPS (`docs/architecture/egps-v10.md`) is the verdict-contract change that
structurally eliminates the gaming signals. Every acceptance criterion compiles to
an **executable predicate** — a bash script at `acs/cycle-N/{NNN}-{slug}.sh` that
*runs the actual code path* and exits 0 (GREEN) or non-zero (RED). Banned inside a
predicate: grep-only checks, `echo PASS; exit 0`, network calls, `sleep ≥ 2s`,
writes outside its scratch dir, missing metadata headers.

The auditor runs this cycle's predicates **plus every prior cycle's accumulated
predicates** (the regression suite) and writes `acs-verdict.json`:

```json
{ "cycle": 40, "green_count": 54, "red_count": 1,
  "red_ids": ["cycle-32-001"], "verdict": "FAIL", "ship_eligible": false }
```

**`red_count == 0` IS the ship gate.** There is no scalar confidence anymore, and
no WARN level — any RED predicate is a FAIL. This is what each gaming signal maps
to: AC-by-grep is banned by the validator; the confidence cliff is gone (verdict
is an exit-code AND); a recurring defect re-fails its accumulated regression
predicate; a tautological eval is caught by the validator + a mutation-kill-rate
gate; deferred work stays an open RED predicate that ship refuses while it's red.

The kernel's role: the lifecycle integration. Build writes the predicates,
Audit runs them, **Ship enforces** `red_count == 0`, and post-ship the predicates
promote into the permanent regression suite. The verdict file is produced by
`evolve acs suite --cycle N` (writes `acs-verdict.json`; exit 2 on any RED).

> Research basis: Skalse et al. NeurIPS 2022 (impossibility of unhackable scalar
> proxies); Lightman et al. 2023 (process > outcome supervision); SWE-bench
> (`verdict = FAIL_TO_PASS ∧ no PASS_TO_PASS regression`).

---

## 3. Adversarial audit — breaking same-model sycophancy

An LLM auditing another LLM's work is prone to sycophancy, especially when it is
the *same model*. Two structural defenses (CLAUDE.md, default-on):

- **Adversarial framing** (`ADVERSARIAL_AUDIT=1`, default). The auditor prompt
  requires *positive evidence* for a PASS — absence of a failing test is not a
  pass.
- **Cross-family judge.** The auditor defaults to **Opus** while the builder
  defaults to **Sonnet** — different model families, so the auditor doesn't share
  the builder's blind spots. (Routing this across CLIs/families is the
  bridge's job — see [bridge-and-adapters.md](bridge-and-adapters.md).)

---

## 4. Audit-binding — Ship is bound to the *audited* tree

The single most load-bearing invariant: **ship may commit only the exact tree the
auditor approved.** If HEAD moved or the working tree changed after the audit, the
audit is stale and ship refuses. This is enforced in two halves.

### The orchestrator writes the binding entry

After a shippable audit (`PASS`/`WARN`), the orchestrator calls
`recordAuditBinding` (orchestrator.go), appending a rich ledger entry:
`role=auditor`, `kind=agent_subprocess`, carrying `git_head`,
`tree_state_sha = sha256(git diff HEAD)`, `worktree_tree_sha` (the worktree's
staged CHANGES tree), and `artifact_path` + `artifact_sha256` of
`audit-report.md`.

> **Why the orchestrator and not the auditor persona writes it** (root-cause fix,
> 2026-05-29): the Go orchestrator otherwise recorded audit as a bare `kind:phase`
> entry with no binding fields, so ship fell back to an ancient bash-era auditor
> entry and *every* cycle failed `AUDIT_BINDING_HEAD_MOVED`. And the auditor's own
> persona binds `HEAD^{tree}` = the *unchanged base* (the cycle's changes are
> uncommitted in the worktree at audit time), which can never equal the
> changes-commit tree → `INTEGRITY_TREE_DRIFT` every cycle (cycle-152). The
> orchestrator's `worktree_tree_sha` is the tree ship will actually commit, so the
> bind matches.

### Ship verifies the binding

`verifyAuditBinding` (`phases/ship/audit.go`) walks `ledger.jsonl` backwards for
the most recent `role=auditor kind=agent_subprocess` entry, then asserts:

1. auditor exit ∈ {0,1} (Unix-convention; 2+ is a real error);
2. `audit-report.md` exists and its SHA matches the ledger (artifact not mutated
   post-audit);
3. the report declares a recognizable verdict — `FAIL` refuses, dual `FAIL∧PASS`
   refuses, `WARN` ships unless `.evolve/policy.json` sets `workflow.strict_audit`;
4. **EGPS gate**: `acs-verdict.json:red_count == 0` (`checkEGPSGate`);
5. **cycle binding**: current `git rev-parse HEAD` == ledger `git_head` AND
   `sha256(git diff HEAD)` == ledger `tree_state_sha`;
6. **freshness**: artifact age < 7 days.

Each failure is a distinct `ShipErrorCode` of class `precondition` or `integrity`
(see [phase-pipeline.md](phase-pipeline.md) §6) — so the orchestrator can route a
stale-but-re-establishable precondition to the debugger while blocking a real
integrity breach.

---

## 5. Ship classes, self-SHA TOFU, and the commit-gate

`evolve ship --class <X>` has four classes (CLAUDE.md), each with different
verification:

| Class | Verification |
|---|---|
| `cycle` (default) | Full audit-binding (§4) + EGPS `red_count==0` |
| `manual` | Skips audit-binding; requires a fresh **commit-gate** attestation; interactive y/N (CI: `EVOLVE_SHIP_AUTO_CONFIRM=1`) |
| `trivial` | Skips audit but `cycle_size_estimate=="trivial"` AND zero pipeline-critical paths touched |
| `release` | Skips audit (the release pipeline mutates files post-audit); logs RELEASE loudly |

**Self-SHA TOFU** (`phases/ship/verify.go:verifySelfSHA`). Before any commit, ship
SHA-256s its own binary and compares to `state.json:expected_ship_sha`, pinned
per plugin version. First run pins; a plugin-version change re-pins; but a *same
version with a different SHA* is `SELF_SHA_TAMPERED` (class `integrity`) — real
local tampering of the ship binary itself.

**Commit-gate** (`phases/ship/commitgate.go`, ADR-0027 commit-as-evidence). Because
bare `git commit` is ship-gate-denied, *interactive* commits go through
`evolve ship --class manual`, and ship performs the commit as an internal
subprocess that no PreToolUse hook can observe. So the commit-gate is enforced
*here*, at the real chokepoint: `--class manual` requires
`.commit-gate/attestation.json` whose `tree_state_sha == sha256(git diff HEAD)`
*after* `git add -A`. Produced by `/commit` (code-simplifier + reviewer + lint +
targeted tests). Missing/stale → `COMMIT_GATE_MISSING`/`COMMIT_GATE_STALE`. Bypass:
`EVOLVE_BYPASS_COMMIT_GATE=1` (routine use is a CLAUDE.md violation).

**Trivial-class critical paths** (`verify.go:verifyTrivial`). The trivial class
cannot touch agent personas, skills, kernel scripts, profiles, or the plugin
manifest — those always require full audit. A hit returns
`TRIVIAL_CRITICAL_PATHS`.

---

## 6. The hash-chained ledger

`ledger.jsonl` is an append-only, tamper-evident audit trail. Each entry
(`core.LedgerEntry`, `ports.go`) carries `prev_hash` + `entry_seq`; the
`FileLedger` adapter (`go/internal/adapters/ledger/ledger.go`) computes
`newHash = sha256(line)` and writes a `ledger.tip` of `<seq>:<sha>`. On `Append`,
the entry's `PrevHash` is set to the prior tip hash and `EntrySeq` to
`prevSeq + 1`; the genesis entry seeds `PrevHash = ZeroSeed`.

`Verify` walks the chain and asserts each line's `prev_hash` equals the SHA of the
preceding line — a single edited or removed entry breaks the chain at that point
and every entry after it. **Why this matters:** the audit trail *is* the evidence.
If an attacker could rewrite a past auditor entry to fake a PASS-binding, ship's
audit-binding (§4) would be defeated. The hash chain makes any rewrite detectable.
This is also why the on-disk ledger is **never rewritten** — even to fix a
legacy malformed entry, because that would cascade hash breaks (the
string-vs-int `cycle` field is absorbed by a defensive unmarshaler instead; see
[state-and-ledger.md](state-and-ledger.md)).

The `chain` guard (`guards/chain.go`) exposes `ledger.Verify` through the unified
guard interface so a host hook can assert chain integrity with
`evolve guard chain`.

---

## 7. How the gates compose (defense in depth)

A cycle ships only if **all** of these independently hold:

```
Build writes executable predicates  ───┐
Audit runs them → red_count == 0     ───┤
Audit declares Verdict: PASS|WARN    ───┤
Auditor cross-family (Opus vs Sonnet)───┼──→ orchestrator writes audit-binding entry
                                         │       (git_head + tree_sha + artifact_sha)
                                         ↓
Ship-gate forces commit through `evolve ship`
   → self-SHA TOFU (binary untampered)
   → audit-binding (HEAD + tree match the audited entry)
   → EGPS gate (red_count == 0, re-checked)
   → ledger hash-chain intact
   → atomic commit + ff-merge + push
```

No single gate is trusted alone. The EGPS predicates ground "done" in execution;
audit-binding grounds "what" was approved; the ship-gate grounds "how" it's
committed; the hash chain grounds the evidence those gates read. Remove any one and
a known incident class re-opens — which is exactly why the routing layer's
integrity floor refuses to let an advisor reach ship without a real build + audit
(see [routing-and-advisor.md](routing-and-advisor.md)).
