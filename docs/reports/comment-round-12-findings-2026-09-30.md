# Comment reduction, round 12: what the editors found (2026-09-30)

Round 12 of the [comment reduction workstream](../plans/comment-reduction-2026-09.md) took 17 packages to zero comments (batches 79–84, PR #749). Six editor agents removed the comments, and a second pass per group rewrote the code so it says what the deleted comments said. Reading every comment closely turned up rules that only a comment stated, tests that read comments, and real defects. This report records each finding and the inbox item that carries it.

## Summary

| Kind | Findings | Where they went |
|---|---|---|
| A comment states a rule no test pins (a "kept why") | 9 | each is pinned by a test named for the rule, then the comment is deleted |
| A test or check reads comments or exact source text, so the comment cannot go | 7 | the test changes to read code shapes, or is retired |
| A defect in production code | 17 | bug items; two are security holes |
| A defect in a test (vacuous, untested behaviour, or reading the real environment) | 5 | test items |
| A stale design doc | 1 | a doc fix |
| Duplication, or production code only tests call | 4 | refactor items |
| **Total** | **43** | |

Every finding is filed. The last six were filed with this report, as `round12-leftover-pins-and-literals`.

## Security holes in `scopedelta`

Round 12's editor for `scopedelta` found two ways a lane's own record can let a protected change through. Both are in `scopedelta-label-and-closure-bypasses` (weight 0.7, security):

- **A protected path can ship under a non-boundary label.** `Account` re-derives a closure or in-scope class but never re-checks the boundary class, and `Validate` refuses a keep only when the record already says boundary. A producer that labels a protected path `discovered`, keeps it, declares that it tightens and gives any corroboration gets `OK()`.
- **Gate configuration counts as closure.** `goBuildMetadataRule` classifies `go/.apicover-enforce`, `go.mod` and `go.sum` as closure in any Go-touching cycle, and `Admissible` exempts closure from evidence. An enrollment edit that deletes other packages' lines ships uncorroborated.
- Related: `GamingSignals` divides by every entry, skipped ones included, so padding a record dilutes a majority.

## Findings and the items that carry them

| Finding | Package | Inbox item |
|---|---|---|
| The two scopedelta holes above, and the `GamingSignals` denominator | `scopedelta` | `scopedelta-label-and-closure-bypasses` |
| Journal write errors are dropped; the classify-before-ship order is pinned only by a comment; an unused parameter and a dead branch | `releasepipeline` | `releasepipeline-journal-errors-and-classify-pin` |
| Any stat error on the naming manifest passes; two fail-open rules pinned only by comments; a stale log line | `releasepreflight` | `releasepreflight-fail-open-pins` |
| The SSE write-timeout and subscribe-order rules are unpinned; the board lane-status overlay survives deletion in every test; the cycle cap needs a decision | `dashboard` | `dashboard-sse-and-board-status-pins` |
| `reviewers_run` is written without JSON escaping; deleted files reach eslint; a failing gofmt counts as formatted; `Run`'s context is ignored | `commitgate` | `commitgate-attestation-and-lane-defects` |
| A network or auth failure reads as "not present" (success); two vacuous tests | `rollback` | `rollback-fail-open-and-vacuous-tests` |
| The decision-surface pin hashes raw bytes, so five files cannot shed comments; a test reads a comment; a vacuous test; a stale doc line | `modelquery` | `modelquery-pin-hashes-code-not-comments` |
| Historical cycle predicates pin comment text or exact source | `go/acs` | `retire-comment-pinning-cycle-predicates` |
| Fake-CLI phase detection iterates a map; skillcheck counts a read error as drift; `TestSeal_RealLedgerCopy` reads the real ledger; `// Output:` counts as a directive everywhere | several | `small-round12-defects` |
| `retro.resolveCLI` and `llmroute` repeat one fallback chain | `phases/retro`, `llmroute` | `one-cli-resolver-for-phases` |
| The build floor's comment count includes TDD's predicate files, so enforcing it would stall lanes | `core` | `tdd-comment-floor-then-enforce` |
| Three unpinned whys (auditchain ×2, skillcheck); auditchain test-only helpers; duplicated literals in `ciparitygate` and `releasepreflight`; a cycle-1707 pointer to a deleted comment | several | `round12-leftover-pins-and-literals` |

## Lessons for the next rounds

- **A comment that states a rule is a missing test.** Round 12 found nine. Each is pinned by a test named for the rule before its comment goes.
- **A test that hashes or greps source text blocks the workstream.** The modelquery pin and the cycle predicates had to be deferred. Tests should read code shapes (an AST), never raw bytes or comment text.
- **Removing comments is also a code review.** Reading every comment closely found 17 production-code defects, two of them security holes, in packages no lane had touched for weeks.

## References

- PR #749: round 12 (`7ceb2a38c`).
- PR #750: the follow-up items filed through `evolve inbox add`.
- [The code-comments convention](../conventions/code-comments.md) and [the workstream plan](../plans/comment-reduction-2026-09.md).
