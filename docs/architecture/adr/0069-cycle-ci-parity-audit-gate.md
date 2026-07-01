# ADR-0069: Cycle CI-parity audit gate

**Status:** Accepted
**Date:** 2026-07-01

## Context / request

The autonomous cycle verifies its work with **scoped, untagged** commands in an
isolated worktree, but `main`'s CI runs **whole-repo, `-tags acs`** gates. A
cycle can therefore pass its own audit ("green locally") yet break `main` CI
("red in CI"). This class recurred repeatedly and forced a manual salvage every
time:

| Cycle | Correct work, but CI caught | Gate the cycle never ran |
|-------|-----------------------------|--------------------------|
| 426, 430, 436 | unnamed exported symbol | repo-wide `apicover -enforce` |
| 436 | `router → modelcatalog` import cycle | `go vet ./...` |
| 436 | unregistered / over-ceiling env flag | `-tags acs` acs-durable |

The pattern is not a bug in any one cycle — it is a structural gap between the
cycle's *verification scope* and CI's *gate scope*. (Prior art: the existing
gofmt and skills-drift CI-parity gates in the audit phase, cycles 339-341.)

## Decision

Add three **deterministic CI-parity gates** to the audit phase (`Classify`),
each running the EXACT whole-repo CI command against the cycle's worktree and
FAILing the audit on offenders — so a cycle that would break `main` CI never
reaches ship (a FAIL audit routes `audit → retro`, and the spine floor rejects
any later ship attempt):

1. `go vet ./...` — catches import cycles and vet defects.
2. `go test -count=1 -tags acs ./acs/regression/...` (acs-durable) — catches
   flag-registry / flag-ceiling / skills-drift regressions.
3. `apicover -enforce` scoped to the cycle's touched∩enforced packages — catches
   the AST-level UNCOVERED (unnamed-export) class.

### Approaches considered

- **(a) new deterministic gate phase** — the catalog gates (coverage-gate, …)
  are LLM `evaluate` phases; a deterministic one needs a new archetype + runner
  + routing + spine anchor, and its FAIL routes to retro anyway. Over-built.
- **(b) build-phase self-verify** — build runs *before* audit; there is no
  build→block edge, and ship already keys off the audit verdict. Reinvents a
  block path.
- **(c) orchestrator pre-ship hook** — a bespoke chokepoint duplicating what the
  audit verdict already does.
- **(d) extend the audit deterministic checks (CHOSEN)** — the gofmt/skills gates
  already establish the pattern (nil-able `func(req) ([]string, error)` hook, run
  against `req.Worktree`, FAIL-downgrade on offenders, fail-open on infra error).
  Three more hooks of identical shape is the smallest correct change, and the
  single `New(Config{})` seam (`NewDefaultWithStageCompact`) wires all defaults
  through one path (cycle-147 dual-source lesson).

## Consequences

- **SSOT with CI:** the gates run the same commands as `.github/workflows/go.yml`
  / `ci.yml` (and the `make lint` / `make test-acs-durable` targets). Drift
  between the cycle gate and CI would reintroduce the gap, so they must stay in
  lockstep.
- **Fail-open:** a gate that cannot *run* (missing toolchain) emits a WARNING and
  leaves the verdict unchanged — its own inability to run must not brick a cycle.
  Only a real non-zero exit FAILs. (`sysexec.Capture`, not `CombinedOutput`:
  `DefaultRunner` maps a non-zero exit to `(code, nil)`, so only the exit code
  distinguishes "found problems" from "could not start".)
- **Scoped cost:** the apicover gate is a no-op unless the cycle touched an
  enforced package (`internal/changedpkgs` ∩ `go/.apicover-enforce`), keeping it
  O(change). FALSE-GREEN (coverage-dependent) is left to CI, matching the
  `acs/regression/apicover` completeness/correctness split.
- **Hooks are nil in `New(Config{})`** (the unit-test path) so the audit
  package's own `go test` never recursively forks the go toolchain; wired only in
  the production `NewDefaultWithStageCompact`.

Key files: `go/internal/ciparity/` (pure intersection helper),
`go/internal/phases/audit/ciparity.go` (the hooks), `go/internal/phases/audit/audit.go`
(the seam). Closes the standing `cycle-audit-cycle-scoped-ci-gap` request.
