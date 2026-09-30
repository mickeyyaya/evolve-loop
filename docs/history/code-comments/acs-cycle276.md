# Comment history: `acs/cycle276`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle276/predicates_test.go:3` — above `package cycle276`

```text
// Package cycle276 materializes the cycle-276 acceptance criteria for the three
// committed top_n tasks (scout-report.md — "bridge abstraction hardening"):
//
//	T1  bridge-tmux-controller-fixture-tests — FakeTmuxController + a multi-frame
//	                                            fixture corpus unit-tests the 885-line
//	                                            driver_tmux_repl state machine; tmux.go
//	                                            stops being an all-0% drag-anchor.
//	T2  bridge-codex-boot-sync-gate          — codex's pre-REPL update-menu nag no
//	                                            longer swallows the injected prompt;
//	                                            shell-spill panes are classified fatal.
//	T3  bridge-profile-contract-symmetry     — the runner fast-fails with a path-named
//	                                            diagnostic on a missing profile instead
//	                                            of letting the bridge exit a terse 10.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test as a real subprocess — `go test -v` over the bridge /
// recovery / runner packages and a `go tool cover` total — and assert on the real
// `--- PASS: <name>` lines, sub-case counts, and the coverage numbers the
// builder's new in-package tests produce. A magic string in a .go file can neither
// produce a named PASS line nor move a coverage number, so none of these is
// gameable by source editing alone (the established cycle-274 pattern).
//
// Convention (cycle-274): the BUILDER authors the in-package unit tests named
// below (TestTmuxFixture*, TestCodexUpdateMenuDismiss, TestFatalPaneShellSpill,
// TestRunnerMissingProfileFastFail, …) alongside the production code; these ACS
// predicates GATE on those tests running + passing with the required adversarial
// diversity. RED at baseline: none of those tests exist yet → no PASS lines →
// every predicate fails for the right reason. The coverage predicate (C276_003)
// is RED against the measured baseline (tmux.go = 1/9 funcs covered: stripANSI).
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.a FakeTmuxController defined            → C276_001 (fixture cases can't pass without it)
//	T1.b >= 3 fixture sub-cases PASS           → C276_001
//	T1.c tmux.go no longer 0% across the board → C276_003 (controller funcs now covered)
//	T1.d boot-timeout negative case covered    → C276_002 (a timeout fixture case present)
//	T2.a TestCodexUpdateMenuDismiss PASS       → C276_004 (+ no-FAIL rider = T2.d)
//	T2.b TestFatalPaneShellSpill PASS          → C276_005
//	T2.c TestFatalPaneNoFalsePositive PASS     → C276_006
//	T2.d full bridge suite PASS                → C276_004 (anyFail rider over the bridge suite)
//	T3.a TestRunnerMissingProfileFastFail PASS → C276_008 (+ no-FAIL rider = T3.c)
//	T3.b TestRunnerMissingProfileDiagnostic PASS (output names the missing path) → C276_009
//	T3.c full runner suite PASS                → C276_008 (anyFail rider over the runner suite)
```

### `go/acs/cycle276/predicates_test.go:196` — above `func TestC276_001_TmuxFixtureCorpusDrivesStateMachine(t *testing.T) {`

```text
// --- C276_001 (T1.a, T1.b): the FakeTmuxController-driven fixture corpus runs
// the driver_tmux_repl state machine through >= 3 distinct cases ---
//
// Behavioral: the builder's `TestTmuxFixture*` tests drive runTmuxREPL with the
// new scriptable FakeTmuxController (per-method frame queue, panic-on-underrun)
// over multi-frame fixtures derived from the two cycle-274 wedge scrollbacks.
// Requiring >= 3 distinct passing cases is the scout's exact verifiableBy and
// proves the fake exists and exercises more than the happy path (a single
// positive case is gameable). The no-FAIL rider guards against the new tests
// regressing the existing bridge suite. RED: no TestTmuxFixture* PASS lines.
```

### `go/acs/cycle276/predicates_test.go:238` — above `func TestC276_003_TmuxGoControllerCovered(t *testing.T) {`

```text
// --- C276_003 (T1.c): tmux.go is no longer an all-0% drag-anchor ---
//
// Load-bearing/objective: the number is produced by REALLY running the bridge
// suite over the package with -coverprofile, so it can only move once the
// builder's FakeTmuxController methods (and/or real execTmux exercises) are
// actually covered by the new fixture tests. Un-gameable by source editing.
// Baseline (measured 2026-06-10): exactly 1 of 9 tmux.go funcs has >0% coverage
// (stripANSI=100%; the 8 execTmux methods are 0.0%). RED: covered count == 1.
```

### `go/acs/cycle276/predicates_test.go:307` — above `func TestC276_005_FatalPaneShellSpillClassified(t *testing.T) {`

```text
// --- C276_005 (T2.b): shell-spill panes are classified fatal ---
//
// cycle-274 observed codex self-update / brew-upgrade leaving the pane in a bare
// shell with `zsh: command not found` and zsh `quote>` / `bquote>` continuation
// prompts — but only `: command not found` was a seeded fatal signature; the
// continuation prompts slipped through, so the stop-reviewer burned the full
// maxExtends backstop on a dead shell. Behavioral: the builder's
// `TestFatalPaneShellSpill` proves the (extended) registry now classifies these
// as fatal — exercised either via recovery.Detect or the bridge fatalPaneVerdict
// integration. Requiring >= 2 sub-cases forces more than one spill signature
// (e.g. the quote>/bquote> continuation AND the command-not-found form). RED:
// test absent in both the bridge and recovery suites.
```
