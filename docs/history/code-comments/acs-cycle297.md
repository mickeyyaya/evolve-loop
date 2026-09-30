# Comment history: `acs/cycle297`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle297/predicates_test.go:3` — above `package cycle297`

```text
// Package cycle297 materializes the cycle-297 acceptance criteria for the two
// committed top_n tasks (scout-report.md — soak batch #6 reliability fixes):
//
//	T1  worktreebase-relative-projectroot-guard — worktreeBase() guards
//	    EVOLVE_WORKTREE_BASE (env path) for absoluteness but NOT the default path.
//	    When the env var is unset and projectRoot is relative (e.g. "."),
//	    filepath.Join(".", ".evolve", "worktrees") = ".evolve/worktrees" is
//	    returned with a nil error. This is the last gap of the inbox defect
//	    swarm-tests-relative-worktree-base (cycle 296 only moved the env-var
//	    check). Fix: guard filepath.IsAbs(projectRoot) in the default branch too.
//	T2  cli-version-freeze-claude — defaultSelfUpdateEvidence switches on
//	    bin=="codex" only, so a host where claude is NOT brew-pinned silently
//	    passes the version-freeze readiness check. claude 2.1.173 self-updated
//	    mid-soak (removed `esc to interrupt`), breaking PaneBusy detection →
//	    exit=81 in cycles 286/288/289/291 (inbox HIGH claude-cli-version-freeze).
//	    Fix: add a claude clause checking ~/.claude/settings.json (analogous to
//	    codex's ~/.codex/version.json).
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test: they invoke `go test -v` on the white-box package tests
// that call the real (unexported) worktreeBase and drive the real freeze
// Specification through Run, then assert on the real `--- PASS:` / `--- FAIL:`
// lines. The functions under test are unexported, so an in-package white-box test
// driven by subprocess is the only way to exercise them — a magic string in a .go
// file can neither make worktreeBase return an error on a relative default path
// nor make the freeze check HALT for claude, so none of these is gameable by
// source editing alone.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1  worktreeBase(".") returns ("", err mentioning "absolute")  → C297_001
//	    + full internal/swarm suite stays green
//	T2  real defaultSelfUpdateEvidence("claude") + end-to-end HALT  → C297_002
//	    + full internal/looppreflight suite stays green
```
