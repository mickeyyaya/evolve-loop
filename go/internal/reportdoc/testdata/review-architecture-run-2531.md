# Architecture Review: resolved-fixes deny-list in reportdoc.CountSeverities (round 2)

Scope: the staged diff against origin/main (5 files, +131/−5). `go/internal/reportdoc/severity_counts.go` was read in full, and so were its tests and the agentcmd tests. `visibleLinesIdx`, `Findings` and `issuesBounds` were read at their declarations. `agentcmd/verdict.go:64-80` was read to find who uses the count.

Round-1 status: the round-1 fail-open (an allow-list with sibling headings that were not counted) is fixed. Probes confirm that `Remaining issues`, `Findings (round 2)`, `Blocking Findings`, a file-name heading, `Not fixed`, `Suggested fixes`, `Unresolved issues` and bare `Fixes` all stay counted. The deny-list direction is correct.

## Architecture Review Verdict

### Three-book audit
- Test-driven: FAIL (one surviving mutant on the fail-open seam).
  - Behavior map: the resolved-heading grammar is pinned by `TestSeverityCounts_IgnoresOnlySectionsThatListResolvedFixes` (27 cases). The reportVerdict integration is pinned by `TestAgentRun_Round2FixedFindingOutsideFindingsKeepsPass` and `TestAgentRun_TaggedHighWithoutFindingsHeadingStillFails`.
  - The bug fix has its regression test: the round-1 brief says both new tests fail on the old code. That claim is not re-run here.
  - Mutation sweep: 8 mutants run, 6 killed. The control mutant (`withoutResolved` returns nil findings) was killed.
  - Killed: `l <= level` → `l < level`; removing `i < covered`; dropping `:` from the trim set; removing the `, checked` suffix option; dropping the tail of `keptVisible`.
  - Survivors: (a) `#{1,6}` → `#{1,7}`. This one is almost equivalent: a 7-hash line is deeper than any span, so it never closes one. The "seven hashes" case cannot tell the two apart. (b) Dropping the raw tail `keptRaw = append(keptRaw, rawLines[rawStart:]...)` survives. See H2.
- Clean code: PASS.
  - `headingOf` has 7 lines, `resolvedSpans` 20, `withoutResolved` 21 and `CountSeverities` 33. Nesting is 3 or less, parameters are 3 or fewer, and no function takes a flag argument.
  - The diff adds no comments.
  - Queries have no side effects, and the change adds no dead code.
- Design patterns: PASS.
  - Projecting the document to a filtered view and then re-running the existing `Findings` grammar reuses the one finding grammar and adds no second one. That is the right seam.
  - The two section grammars, `issuesBounds` and `resolvedSpans`, are accepted, as round 1 recorded.

## Findings

- **[HIGH] H1: a still-open finding inside a resolved-fixes section, or under any deeper or non-ATX heading after it, is not counted (fail-open)** — `go/internal/reportdoc/severity_counts.go:47-52`, the span end rule `isHeading && l <= level`. Before this change, `CountSeverities` counted every tagged line, so each case below is a new fail-open path. Probes (`go test -overlay` with a probe test, results printed by the test) all give `High:0`:
  - `## Round 1 fixes, checked\n- [HIGH] x fixed\n\n**Remaining issues**\n- [HIGH] a.go:1 open` (a bold pseudo-heading)
  - `## Fixed\n- [HIGH] x fixed\n### New findings\n- [HIGH] a.go:1 open` (a nested heading)
  - `## Fixed\n...\nFindings\n--------\n- [HIGH] a.go:1 open` (a setext heading)
  - `# Resolved\n## Findings\n- [HIGH] a.go:1 open` (a level-1 resolved heading hides the whole document)
  - `## Fixed\n- [HIGH] x fixed\n- [HIGH] a.go:1 still open after fix attempt` (a round-1 item that the check found NOT fixed)

  The last case is the most likely one. A round-2 reviewer who checks round-1 fixes writes the result of each check under the heading that this change exempts, and that result includes the fixes that failed. That is the exact layout that caused the incident.
  - Violated principle: fail loudly. The trust boundary (reviewer free text → commit gate) now fails open on a heading that the parser alone defines. No reviewer persona states the `resolvedFixesTitle` grammar: a grep for "round N fixes", "prior fixes" and "resolved findings" in `agents/` and `skills/` finds none.
  - Existing guards, and why this stays HIGH and not CRITICAL: `reportVerdict` (`agentcmd/verdict.go:74`) still fails a stated FAIL. A severity-summary table row is always read (probe `with-table` → `High:1`). So the fail-open needs a reviewer that states PASS and omits the table. The count is the check against exactly that inconsistent reviewer.
  - Fix:
    - End a resolved span at the next heading of ANY level (`isHeading` with no level test). Then a subheading such as `### Still open` or `### New findings` goes back to counting.
    - Also recognize setext headings, and a line made only of bold text, as span boundaries in `headingOf`, so that every visual section break closes the exemption.
    - Inside a span, keep counting a tagged line that contains `not fixed|still open|unresolved|regress`. As an alternative, put the contract "a round-N item that is not fixed is listed again under Findings" in ONE home, the review persona output contract, so that the persona and the parser share one rule.
    - Add each probe above as a `want High:1` row in the table test.
  - The level-1 case is cheap to close: exempt only headings of level 2 or deeper.

- **[HIGH] H2: a surviving mutant drops every structured finding after the last resolved section** — `go/internal/reportdoc/severity_counts.go:84` (`keptRaw = append(keptRaw, rawLines[rawStart:]...)`).
  - Mutant: replace that line with `_ = rawStart`. All reportdoc and agentcmd tests pass.
  - Under the mutant, a `### H1 (HIGH) — …` heading-form finding or a `## Issues` TSV row that comes AFTER a resolved section is not counted. Tagged lines are still counted through `keptVisible`, so every current case, which uses tagged lines after the span, hides the fault.
  - The production code is correct. The fix is a test: add `{"structured finding after a resolved section", "## Round 1 fixes, checked\n- [HIGH] x fixed\n## Findings\n### CR1 (HIGH) — nil deref\n", SeverityCounts{High: 1}}`, plus a TSV variant. The `tailraw` mutant gives 0 for these inputs, so the test kills it.
  - Principle: TDD. The tests exercise the behavior but do not pin it, and this is the fail-open direction.

### Scores (1–5)
- Scalability: 4/5
- Debuggability: 3/5 (a silently exempted HIGH leaves no trace. The count does not report the lines that it skipped.)
- Maintainability: 4/5
- Performance: 5/5
- Consistency: 3/5 (the exempt-heading grammar lives only in the parser. The personas do not state it.)

### Severity Summary
| Severity | Count |
|----------|-------|
| CRITICAL | 0 |
| HIGH     | 2 |
| MEDIUM   | 0 |
| LOW      | 0 |

### Commendations
- The deny-list inversion: an unknown heading now fails closed. This is the right default for a gate.
- The filtered-view projection re-uses `Findings` and adds no second finding grammar.
- The table-driven test names each heading class, and each row is a falsifiable contract.

### Declined to judge
- The almost-equivalent `#{1,7}` mutant: it is not reachable in a way that changes the count, so it is not a finding.
- The `issuesBounds` vs `resolvedSpans` grammar split: settled in round 1 as two different questions.
- `Findings()` does not apply the exemption, so the dashboard still lists fixed items. The package note documents this, and it is not a gate path.

### The one change
H1: end the resolved span at the next heading of any level, recognize bold-only lines and setext headings as section breaks, and keep counting a tagged line inside the span that says "not fixed" or "still open". Each of these closes a fail-open path that the old code did not have.

## Verdict

Verdict: WARN

### Recommendation: FIX_THEN_MERGE

<!-- evolve-verdict: {"phase":"architecture-reviewer","verdict":"WARN","schema_version":1} -->
