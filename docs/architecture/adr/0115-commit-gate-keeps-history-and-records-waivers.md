# ADR-0115 — The commit gate keeps what a removed comment carried and records what it waived

- **Status:** Accepted (2026-10-01)
- **Follows:** [ADR-0111](0111-code-carries-no-comments.md) (code carries no comments). This ADR closes four follow-ups the comment campaign filed against the commit gate and the comment tooling.
- **Consumes inbox:** `history-bearing-comment-removal-needs-an-archive-entry`, `commit-gate-waiver-followups`, `comment-floor-skips-the-research-archive`, `commitgate-attestation-and-lane-defects`, `commentaudit-joins-the-protected-surface`.
- **Related:** [the code-comments convention](../../conventions/code-comments.md); package notes for [commitgate](../packages/internal-commitgate.md), [commentaudit](../packages/internal-commentaudit.md), [phases/ship](../packages/internal-phases-ship.md) and [scopedelta](../packages/internal-scopedelta.md).

## Context

ADR-0111 decision 7 said history is recorded before it is removed, and `commentaudit history` records it in `docs/history/code-comments/`. Only the comment-reduction rounds ran it. The tree still held about 3,338 history-bearing comment lines, so a console change that deleted such code landed with no archive entry, and git's diff was the only record.

The commit gate waives review for a proven comment-only change. The attestation of a waived commit carried no reviewer, and neither did a `--bypass-commit-gate` commit, so `git log` could not tell them apart. `commentaudit` and `commitgate` decide whether review runs, yet `scopedelta` classed a lane's edit to them as ordinary product code.

Continuation adoption archives an ancestor cycle's predicate packages under `docs/private/research/archived-<date>/`. Those `.go` files are added paths in the adopting build's diff, so the build comment floor counted their comments as the build's own. Under `comment_floor.stage=enforce` the builder would have had to edit the archive.

Three documents stated three different rules for a docs-only change: operating-policy.md said it may skip the simplifier, CLAUDE.md said it may skip the review fleet, and the commit gate required both review capabilities.

## Decision

1. **The commit gate refuses a removed history comment the change does not record.** `refuseUnrecordedHistory` runs `commentaudit.RemovedHistoryAcrossDiff`, the rule `commentaudit history` uses, so a group moved whole elsewhere in the change is not removed. It then runs `commentaudit.UnrecordedHistory`. A removed group is recorded when its fenced text, rendered by the same function the archive writer uses, occurs more often on its package's archive page after the change than before, one copy per removed group. Each unrecorded group is refused by `file:line` with the command that records it. The rule runs with the binary guard and the added-comment refusal, before the review waiver. The archive pages are `docs/**.md`, so a comment removal that records its history is still waived. Because a waived commit has no reviewer, the gate also refuses a change that rewrites the archive: every page under `docs/history/code-comments/` except the generated `README.md` index must keep its old text as a prefix (`commentaudit.RewrittenHistoryPages`), so a page only grows and a page is never deleted.
2. **`docs/` holds records, not code, for the added-comment rule.** `AddedAcrossDiff` and `check` skip every path under `docs/` on both sides of the diff, which serves the build floor, the commit gate and the CLI. A comment brought back from `docs/` into code counts as added. The removed-history rule still reads `docs/`. A group moved whole into an archive is stored, and deleting an archived file loses what it held.
3. **A waived review is recorded.** The attestation gains `review_waiver` (`"comment-only"`), written only when the waiver applied, so the golden layout of a reviewed commit is unchanged. Ship's `reviewTrailer` emits `Review-waived: comment-only` for it. The attestation schema has one home: ship decodes into `commitgate.Attestation` rather than a mirror struct, and its tests write attestations with the real `Marshal`, so the writer and the reader are tested together. `commentaudit` and `commitgate` join `scopedelta`'s signal surfaces, and `commentaudit` joins `guards.ProtectedSurfaceManifest` beside `commitgate`. The gate's logic lives there, so a loop lane cannot edit it.
4. **One pure-docs rule: the gate's.** A docs-only change needs the simplify and review capabilities. One `code-review-simplify` diff-review pass covers both. It may skip the architecture review. Only a proven comment removal needs no reviewer. CLAUDE.md and operating-policy.md now say this. Docs are the project's primary asset, so they keep a review. The console made this call in bypass mode. It keeps the stricter of the three rules, so it tightens review and loosens nothing.

## Alternatives considered

| Alternative | Why it was not chosen |
|---|---|
| The gate runs `commentaudit history` and writes the pages itself | The gate never mutates the tree it attests to. Writing pages would change the tree after its SHA was taken, and the section label is the author's to choose. |
| Match a record by `file:line` or by the whole rendered entry | A round records against its merge base and may land on a newer main, which moves the line numbers and anchors. The text is what must be kept. |
| Parse archive pages back into entries | That would be a second spelling of the page format to keep in step with the writer. Matching the writer's own rendering of the text needs none. |
| Exclude only the dated research archive, spelled once in `explanationdocs` (the inbox item's suggestion) | `commentaudit` imports only the standard library, while `explanationdocs` pulls in config, policy, profiles and signalcenter. No `.go` file under `docs/` is compiled, and the only ones present are archived predicates. |
| Filter archive paths in each caller (floor, gate, CLI) | Three copies of one rule. |
| Let pure-docs diffs skip all review | Docs are the primary asset, and the gate and the convention already require review. |
| Leave archive integrity to review | A proven comment-only change has no reviewer, so review could not catch a waived commit that cuts the archive. |
| Keep ship's own attestation struct | That gives the schema three homes: the writer's tags, its field keys and the reader's mirror. A key that drifts between them breaks nothing that any test ran. |

## Consequences

- A console change that deletes code with history comments must record them first. The refusal names the command. Deleting a whole Go file, or an archived file under `docs/`, counts as a removal.
- Loop lanes do not pass through the commit gate, so a lane that deletes history comments still lands them unrecorded. This is filed as a follow-up for the build floor or ship.
- `git log --format='%(trailers:key=Review-waived)'` lists every waived commit.
- A changed Go file that does not parse is refused by the history check (ExitFail; `commentaudit.IsUnparsable` classifies it, so the gate does not depend on the parser's error type). A failing `gofmt` now fails the Go lane instead of counting as formatted.
- The history archive is append-only for every console change. A correction to a recorded entry is appended as a new section.
- `go/internal/commentaudit/` is protected surface. Items whose fix touches it route to the console, as `commitgate` items already did.
- The attestation writer escapes every value, so any reviewer name writes valid JSON.

## Evidence

- Red-first tests for every behaviour, listed in the package notes. The end-to-end proof is `TestRefuseUnrecordedHistory_TheCommandTheRefusalNamesSatisfiesIt`: in a real repository the gate refuses, `commentaudit history` runs, and the gate passes. `TestCommitGate_GitLogTellsAWaivedCommitFromABypassedOne` ships two real commits and reads their messages.
- `TestCommentFloorFailures_AnAdoptedAncestorsArchivedPredicatesAddNoComment` drives the real `explanationdocs.ArchiveSupersededPredicatePackages` and then the floor under `enforce`.
