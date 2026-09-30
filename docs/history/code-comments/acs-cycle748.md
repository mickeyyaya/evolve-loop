# Comment history: `acs/cycle748`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle748/predicates_test.go:3` — above `package cycle748`

```text
// Package cycle748 materializes the cycle-748 acceptance criteria for the sole
// committed top_n task push-ci-watch-remote-parity (triage-report.md ## top_n;
// scout-report's three selections were all out of this fleet lane's assigned
// set, so per R9.3 no predicates bind to them and no deferred-floor predicates
// exist).
//
// Task source: .evolve/inbox/2026-07-07T06-32-00Z-push-ci-watch-remote-parity.json
// (weight 0.90). Incident: main stayed red 2026-07-06 11:50→20:16+ across 8
// pushes — nothing in the loop reads GitHub CI's verdict on a push, and
// release preflight never checks the remote CI conclusion on the release
// commit (v22.0.0 was cut on red CI, release_hardening memory).
//
// AC map (1:1), derived from the inbox item's acceptance list:
//
//	AC1 failed CI run ⇒ critical inbox item naming the failing test
//	    (faked gh runner)                                → C748_001 + C748_002 (negative)
//	AC2 evolve release refuses to tag on non-green release-commit CI;
//	    explicit override exists and logs loudly         → C748_003 + C748_004
//	AC3 CI verdict appears in the cycle dossier          → C748_005
//	AC4 knobs live in policy.json; zero new env flags    → C748_006 + C748_007 (config-check)
//	AC5 go vet / -race / apicover -enforce green         → manual+checklist (auditor
//	    runs the repo-wide CI-parity gates on touched pkgs per ADR-0069)
//
// Each behavioral predicate shells `go test -race -count=1 -v -run '^<name>$'`
// over the unit-test contract in the target package, which EXERCISES the SUT
// (faked gh runner seams, real temp inbox dirs, real policy.json documents) —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
// C748_007 is the sole declared config-check (an ABSENCE assertion: the
// feature must introduce no EVOLVE_* env flag).
```

### `go/acs/cycle748/predicates_test.go:67` — above `func TestC748_001_FailedCIRunFilesCriticalInboxItem(t *testing.T) {`

```text
// AC1 (positive) — the incident twin: a push whose CI run FAILS yields a
// critical fix-forward inbox item that names the failing job/test (bounded log
// excerpt), driven through a faked gh-runner seam and a temp inbox dir.
```
