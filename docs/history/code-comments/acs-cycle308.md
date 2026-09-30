# Comment history: `acs/cycle308`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle308/predicates_test.go:3` — above `package cycle308`

```text
// Package cycle308 materializes the cycle-308 acceptance criteria for the three
// committed top_n tasks (scout-report.md):
//
//	T1  inbox-promote-on-ship-missing — ReleaseCycleProcessing scoped-releases
//	    processing/cycle-<N>/ back to the inbox root; wired into BOTH the
//	    ship-success residual drain (postship.go) and the cycle-fail terminal
//	    (cmd_loop.go) so claimed items never strand (the cycle-124/234/240/...
//	    orphan class).
//	T2  companion-malformed-must-surface — a present-but-malformed
//	    triage-decision.json companion SURFACES a non-empty warning instead of
//	    silently falling through to the prose scanner; absent file / absent field
//	    stay silent (backward compat).
//	T3  cli-version-lifecycle-preflight — looppreflight captures a CLI version
//	    inventory into loop-preflight.json and WARNs on a version change vs the
//	    last batch (the claude 2.1.173 → 2.1.175 silent-drift incident).
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test: they invoke `go test -v` on the white-box package tests
// that call the real functions (ReleaseCycleProcessing, MalformedCommittedFloor
// Warning, captureVersionInventory, checkCLIVersionDrift) and assert on the real
// `--- PASS:` / `--- FAIL:` lines. A magic string in a .go file can neither make
// a release move a file, make a malformed companion surface a parse error, nor
// make a drift check WARN — so none is gameable by source editing alone. The
// cmd_loop.go wiring check (C308_003) is MIXED: a behavioral inboxmover run plus
// an auxiliary grep that the production call site exists (cycle-307 seam-trap).
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1 fail-release       → C308_001 (named PASS lines, inboxmover)
//	T1 ship residual      → C308_002 (named PASS line, ship)
//	T1 cmd_loop wiring    → C308_003 (behavioral + wiring grep)
//	T2 committed surface  → C308_004 (named PASS lines, triagecap)
//	T2 deferred surface   → C308_005 (named PASS lines, triagecap)
//	T3 inventory + drift   → C308_006 (named PASS lines, looppreflight)
```

### `go/acs/cycle308/predicates_test.go:171` — above `func TestC308_003_CmdLoopFailTerminalWiresRelease(t *testing.T) {`

```text
// C308_003 (T1 cmd_loop wiring): MIXED. The behavioral portion re-runs the real
// inboxmover release test; the auxiliary grep confirms the production call site
// is wired into cmd_loop.go's cycle-fail path — the cycle-307 seam-trap guard (a
// helper built but never wired). The grep alone is auxiliary: the behavioral
// inboxmover run carries the weight.
```

### `go/acs/cycle308/reproduce_cycle308_test.go:3` — above `package cycle308`

```text
// Package cycle308 contains the ACS predicates and bug reproduction verifiers
// for cycle 308. The three bugs targeted by this cycle are absence-of-API
// failures: the TDD test files (inboxmover_release_test.go,
// malformed_floors_test.go, versioninventory_test.go) fail to compile against
// the pre-fix source because the required functions did not exist at HEAD.
//
// This file verifies the post-fix API contract: if any of the required symbols
// are missing the predicate fails, which reproduces the original build failure
// class against the package-under-test.
```
