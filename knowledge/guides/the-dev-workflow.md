# The contributor dev workflow

> How a human (or an agent) makes a change land safely: isolate in a worktree, write
> the failing test first, build to green, run the commit-gate, ship via the one
> sanctioned path. Each step is gated by a trust-kernel guard — this is the same
> defense-in-depth the autonomous loop runs through, applied to manual work. The
> guards' design is [architecture/trust-kernel-and-egps.md](../architecture/trust-kernel-and-egps.md);
> the decisions behind them are [evolution/decision-digest.md](../evolution/decision-digest.md).

## The loop

```
worktree isolation → TDD (RED first) → build (→ GREEN) → /commit (attestation)
                                                              → evolve ship --class manual
```

### 1. Isolate in a worktree

Do feature work in a dedicated git worktree, never on the live `main` tree. Inside a
cycle the **role-gate** (`guards/role.go`) enforces this mechanically: it reads
`cycle-state.json` and denies any Edit/Write outside the workspace or the active
worktree (`/tmp/**` and `$HOME/.claude/**` are always-safe; default-deny if storage
is misconfigured). A half-finished change is then always discardable by deleting the
worktree.

### 2. TDD — RED first

Write the test (or the EGPS predicate — see
[write-an-eval-and-predicate.md](write-an-eval-and-predicate.md)) **before** the
implementation, and confirm it FAILs. Tests are named for the *behavior* they pin,
never `TestC<N>_*`; a passing test that would still pass if the invariant were broken
is a no-op and gets rejected in review (`go/docs/testing.md`).

### 3. Build to green

Implement the minimum to turn RED → GREEN. Keep the change in the worktree; the
**tree-diff leak guard** (`guards/treediff`) snapshots the main tree before a
source-writing phase and re-checks after — any newly-dirty main-tree path is a write
that escaped the sandbox and aborts before anything is recorded as success.

### 4. Commit-gate — produce a fresh attestation

Bare `git commit` / `git push` / `gh release` are **denied** by the **ship-gate**
(`guards/ship.go`) unless they go through `evolve ship`. So interactive commits run
through `evolve ship --class manual`, which requires a fresh commit-gate attestation:
`.commit-gate/attestation.json` whose `tree_state_sha == sha256(git diff HEAD)` after
`git add -A`. Produce it with **`/commit`** (code-simplifier + code-reviewer +
language reviewer + lint + targeted tests → writes the tree-SHA-bound attestation).
See [skills/commit/SKILL.md](../../skills/commit/SKILL.md).

If the attestation is missing or stale (the reviewed tree ≠ the staged tree), ship
refuses with `COMMIT_GATE_MISSING` / `COMMIT_GATE_STALE`. `EVOLVE_BYPASS_COMMIT_GATE=1`
skips it — routine use is a CLAUDE.md violation.

### 5. Ship via the one sanctioned path

```bash
evolve ship --class manual "feat(x): …"      # interactive commit; requires the attestation
```

Before committing, ship runs **self-SHA TOFU** (`verify.go:verifySelfSHA`): it
SHA-256s its own binary against `state.json:expected_ship_sha`; a same-version
binary with a different SHA is `SELF_SHA_TAMPERED` and blocks. The autonomous loop
uses `--class cycle` instead, which additionally enforces full **audit-binding**
(HEAD + tree match the audited entry) and the EGPS `red_count == 0` gate — the human
`--class manual` path skips audit-binding (you did the review) but never skips the
commit-gate or self-SHA.

## Why each guard exists (don't route around them)

| Guard | Stops | If you bypass it |
|---|---|---|
| role-gate | writes outside the worktree | a half-cycle is no longer discardable |
| ship-gate | direct `git commit`/`push` | the entire audit-binding model is bypassable |
| commit-gate | committing un-reviewed code interactively | un-reviewed code reaches `main` |
| self-SHA TOFU | a tampered ship binary | the gate that runs every other gate is itself untrusted |
| ledger hash-chain | rewriting a past audit entry | a faked PASS-binding becomes undetectable |

Each has a loud bypass env var for emergencies; using one routinely re-opens a known
incident class — the [incidents/pattern-library.md](../incidents/pattern-library.md)
triage table maps each back to the failure it prevents.

## The tests that pin this

- `go/internal/phases/ship/commitgate_test.go` —
  `TestCommitGate_ManualMissingAttestation_Refuses`,
  `TestCommitGate_ManualStaleAttestation_Refuses`,
  `TestCommitGate_ManualValidAttestation_Ships`.
- `go/internal/phases/ship/*_test.go` — the `TestVerifyAuditBinding_*` family (the
  audit-binding refusals for the `--class cycle` path).
- `go/test/commitgate/commit-gate-test.sh` — the portable commit-gate runner over
  ephemeral repos.
