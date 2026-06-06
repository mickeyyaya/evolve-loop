---
score_cap:
  - criterion: "cycle-230 audited resolution fix is on main (resolveACSSuiteRoot wired + autosolve/fallback tests pass)"
    max_if_missing: 6
    evidence: "cd go && grep -q 'func TestACSSuiteRootAutosolve(' cmd/evolve/cmd_acs_test.go && go test -count=1 -run 'TestACSSuiteRootAutosolve|TestACSSuiteRootFallback' ./cmd/evolve/..."
  - criterion: "cycle-229 two-tier naming lint bug-repro is on main and passes"
    max_if_missing: 6
    evidence: "cd go && grep -q 'func TestBugRepro_Cycle229_TwoTierNamingMissing(' internal/phasespec/bug_repro_cycle229_test.go && go test -count=1 -run 'TestBugRepro_Cycle229_TwoTierNamingMissing' ./internal/phasespec/..."
  - criterion: "cherry-picked cycle-230 ACS predicates remain git-tracked (gitignore-drop guard)"
    max_if_missing: 5
    evidence: "git ls-files --error-unmatch acs/cycle-230/001-auditor-doc-trim.sh acs/cycle-230/002-phase-naming-lint.sh acs/cycle-230/003-acs-suite-root-autosolve.sh acs/cycle-230/004-ledger-skip-source.sh"
---

# Eval: Land the triple-audited cycle-230 resolution fix (201f7cb)

> Pins the landing of branch `cycle-230` @ `201f7cb` — the triple-audit-passed
> phase-identity resolution fix (ACS suite `--root` autosolve from
> `cycle-state.json` active_worktree, `LedgerEntry.Source`, two-tier naming
> lint) — onto main. The branch sat unshipped for two cycles because of the
> I-10 workspace-vs-worktree write collision; this eval ensures the landed
> content never silently regresses or gets dropped by the `.evolve/*`
> gitignore. Source incident: campaign retrospective cycles 215-231
> (2026-06-06), migration step 1; cycle-230 audit↔ship recovery loop ×3.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| root-autosolve | resolveACSSuiteRoot tests present + passing | 6/10 | `go test -run 'TestACSSuiteRootAutosolve\|TestACSSuiteRootFallback' ./cmd/evolve/...` (existence-guarded) |
| naming-lint-repro | cycle-229 bug repro present + passing | 6/10 | `go test -run TestBugRepro_Cycle229_TwoTierNamingMissing ./internal/phasespec/...` (existence-guarded) |
| tracked-artifacts | cycle-230 predicates git-tracked | 5/10 | `git ls-files --error-unmatch acs/cycle-230/*.sh` (cycle-92 gitignore-drop guard) |
