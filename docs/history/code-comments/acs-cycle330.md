# Comment history: `acs/cycle330`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle330/predicates_test.go:3` — above `package cycle330`

```text
// Package cycle330 materializes the cycle-330 acceptance criteria for the THREE
// committed top_n tasks (triage-report.md "## top_n"):
//
//	bridge-ratelimit-matches-agent-content — close the remaining residual of the
//	    soak-#4 cycle-314 content-vs-chrome false positive: a BARE unified-diff
//	    line ("+content"/"-content" WITHOUT a leading line number) carrying a
//	    quoted rate-limit banner must NOT drive the codex-tmux escalate rule when
//	    the pane is IDLE (the numbered-diff strip and the ADR-0047 busy idle-gate
//	    both miss it), while a genuine banner (CLI chrome, never diff-prefixed)
//	    still escalates. Scoped per triage to the bare-diff-exclusion sub-fix +
//	    TDD pin (footer/status-chrome scoping + persistence gating stay deferred).
//
//	interaction-error-branch-coverage — exercise the three uncovered error/edge
//	    branches in go/internal/interaction: ledgerPath's empty-phase "unknown"
//	    fallback (66.7% baseline), neutralize's second rune-cap on a multi-line
//	    Digest (83.3%), and appendLedgerLine's swallowed os.OpenFile error
//	    (77.8%). appendLedgerLine's json.Marshal error arm is unreachable (Outcome
//	    always marshals), so ~88.9% is its practical ceiling — its floor is 85,
//	    not 100.
//
//	phasecoherence-check-nil-guard-coverage — exercise the four uncovered guard/
//	    skip branches in phasecoherence.Check (81.4% baseline): the nil-AgentsFS
//	    error (distinct from empty-FS), the directory-entry skip (entry.IsDir),
//	    the nil-frontmatter skip (fm == nil), and the non-[]string toolsVal skip.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) — there is no load-bearing
// source-grep:
//
//   - The bridge gate RUNS the real decideAutoRespond truth-table suite in a
//     subprocess and asserts both the exit code and that the new bare-diff
//     PASS line is present; a magic string in a source file cannot satisfy it,
//     and an EMPTY repo (no bridge tests) cannot produce the PASS line.
//   - The coverage gates RUN the real package suites under -coverprofile and
//     assert on the measured `go tool cover -func` percentages. A magic string
//     cannot move a coverage number — only Builder's new tests can — so the
//     coverage gates are anti-no-op by construction. An EMPTY repo yields 0% and
//     fails every floor.
//   - coverFuncOutput Fatals (RED) if a suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis; a dedicated
//     -race gate per package adds the data-race axis with a distinct command verb.
//   - The per-function anchors (ledgerPath / neutralize / appendLedgerLine /
//     Check) pin the % movement to the TARGET dark branches, not to incidental
//     coverage elsewhere in the package.
//
// AC map (1:1 with the committed-task acceptance criteria; the "no production
// code beyond the scoped fix" / "tests not modified" criteria are dispositioned
// manual+checklist for the Auditor in test-report.md, not as a fragile git-diff
// predicate whose result depends on phase-commit timing):
//
//	bridge-ratelimit-matches-agent-content
//	  AC-A1 bare diff line (idle) → no escalate    } both → C330_001
//	  AC-A2 real banner → still escalates          }
//	interaction-error-branch-coverage
//	  AC-B1 ledgerPath empty-phase fallback covered  → C330_002 (ledgerPath >= 90%)
//	  AC-B2 neutralize multi-line rune-cap covered   → C330_003 (neutralize >= 90%)
//	  AC-B3 appendLedgerLine OpenFile error covered  → C330_004 (appendLedgerLine >= 85%)
//	  AC-B4 interaction suite stays green (-race)    → C330_005
//	phasecoherence-check-nil-guard-coverage
//	  AC-C1 four nil/skip branches covered           → C330_006 (Check >= 88%)
//	  AC-C2 phasecoherence suite stays green (-race)  → C330_007
//
// Floor binding (R9.3): internal/bridge, internal/interaction, and
// internal/phasecoherence are the THREE committed top_n tasks this cycle, so
// every floor/gate binds committed work. The triage-DEFERRED item
// evalgate-floorbinding-workspace-coverage gets ZERO predicates here — a floor
// on a deferred task would starve the committed ones (cycle-280 lesson).
```
