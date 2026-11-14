# Build Explanation — Cycle 2374

## Build Binding
- Cycle: 2374
- Base SHA: 5da3551a32ee8caacaa852c5a6517f98bbda7744

## Summary
The inbox curator now reads the bodies of the merged branch of a merge commit. A `Closes-Inbox` id in those bodies is a deterministic consume of the open item with that id.

## Rationale
A merge commit hides the trailer of its branch commits, so the curator missed the landing of PR 1220. The item `reviewer-profiles-may-write-their-report` stayed open. Reading the branch bodies through `git rev-list --no-merges` fixes the cause and needs no new setting.

## Changed Areas
- `go/internal/inboxcurator/gitlog.go` — reads the parents of each commit and fills `Change.Closes` from the own body, or from the merged branch of a merge commit.
- `go/internal/inboxcurator/landing.go` — adds the `Closes` field and makes a landed `Closes-Inbox` id a deterministic consume.
- `go/internal/inboxcurator/merged_branch_fault_test.go` — covers the fault path of the merged branch read.
- `go/internal/inboxcurator/closes_landing_test.go` — pins the match, the PR 1220 replay and the octopus merge.
- `go/internal/inboxcurator/closes_trailer_grammar_test.go` — proves that a merged branch body and a commit body follow one trailer grammar, the grammar of `closesinbox`. The cases are fenced examples, bullet markers, `NONE` and prose after the marker.
- `go/acs/cycle2374/predicates_test.go` — holds the acceptance predicates of the cycle.
- `docs/architecture/inbox-curator.md` — states the change in the landing section.
- `docs/architecture/packages/internal-inboxcurator.md` — states `Change.Closes` and the changed match.
- `.evolve/evals/arch-decisions-ad2-2-the-curator-reads-the-bodies-of-the.md` — holds the graders of the task.

## Design Decisions
`rev-list` reads the branch, not `log`. A fault test can then fail the branch read alone. `none` and `Refs-Inbox` give no id, so only an explicit close consumes.

## Verification
The package has 100% statement coverage. The eight cycle predicates TestC2374_001 to TestC2374_008 pass.

## Compatibility
`Change` gains one field. Existing callers and the wire format do not change.

## Limitations
The curator reads only the `Closes-Inbox` trailer. A branch that names the item in prose gives the existing judgment proposal.

The curator reads the bodies of a merged branch as one text. An unclosed code fence in a newer branch body can therefore hide a `Closes-Inbox` claim in an older body of the same branch. The claim then gives no consume and the item stays open.
