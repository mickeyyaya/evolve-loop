# internal/commentaudit

> The rule it enforces: [the code-comments convention](../../conventions/code-comments.md) and [ADR-0111](../adr/0111-code-carries-no-comments.md). The campaign it serves: [the comment plan](../../plans/comment-reduction-2026-09.md). Its command: `go run ./cmd/commentaudit`.

## Purpose

`commentaudit` measures a Go tree's comments, removes the ones the convention does not allow, records the history the removed ones carried, and proves that an edit changed only comments. The commit gate and the build floor use it to refuse an added comment, the commit gate uses it to refuse a removed history comment no archive page records, and the comment workstream uses it to reach zero.

## Commands

| Command | What it does |
|---|---|
| `rank [-n N] [dir]` | lists directories by narrative and comment lines |
| `verify -base <ref> [dir ...]` | proves every changed Go file changed only comments (`Equivalent`) |
| `check -base <ref> [dir ...]` | names every history-carrying comment line a diff adds |
| `comments -base <ref> [dir ...]` | names every comment a diff adds, by the rule the commit gate and the build floor share (`AddedAcrossDiff`) |
| `history -base <ref> [-label L] [-out DIR] [dir ...]` | records each removed comment group that carries history in `docs/history/code-comments/` |
| `strip [dir ...]` | removes, in place, every comment the convention does not allow |

## Design

- **One table of comments a tool reads.** `commentMarkerRules` in `equivalence.go` lists each marker's pattern and how much it spans: its own line (toolchain and linter directives, `acs-predicate:`, `apicover:ignore`), its paragraph (`minimal:`, `Deprecated:`), the rest of its group (an Example's `Output:`), its whole group (`IPC-protocol-allowed`, which the envtaint scan reads per group), or the whole file (a generated header). `verify`'s `directive` pattern is rendered from the table and `strip` reads the extents, so the two cannot disagree about which comment a tool reads.
- **Proof after every transform.** `Equivalent` compares the code-only AST (positions and comments dropped) and each directive's anchoring (the next code token, a blank line before it, whether a `Deprecated:` note opens its paragraph: the one note the table marks `anchorsParagraph`, since staticcheck and gopls read it only there). `verify` applies it to a diff; `strip` applies it to its own output and refuses a file that fails, so a defect in the removal can never land code changes.
- **`strip` plans edits against the original bytes and applies them once**, from the end backwards, then formats with gofmt. A whole-line comment takes its line; a trailing comment takes the space before it; a comment between tokens becomes the space, or the newline, the language reads it as.
- **`strip` lists what it could not decide.** A refused file goes to stderr and makes the command exit 1; a package doc it could not bring within the floor is listed on stdout as `package doc to write: <path>`.
- **One package doc per package.** `packageDocHolder` picks `doc.go`, else `<dir>.go`, else the first non-test file with a doc. The doc stays when it is at most `maxPackageDocLines` (3) lines of at least `MinPackageDocWords` (6) words, the floor `acs/regression/docgo` imports; otherwise its leading sentences up to that floor are wrapped to at most three lines, and when that is impossible the doc is left as written and reported in `StripReport.DocsToWrite` for a person.
- **Writes are atomic and keep the file's mode**: a temporary file in the same directory, renamed over the original.
- **The skip set is one predicate.** `isSkippedDir` (testdata, vendor, dot directories) serves `Rank`, `strip`, `history` and the added-comment rule through `isOutsideProjectCode`.
- **`docs/` holds records, not code.** The added-comment rule (`AddedAcrossDiff`, and `check` through the same `rule.acrossDiff`) skips every path under `documentationRoot`, on both sides of the diff: a predicate package that continuation adoption archives under `docs/private/research/archived-<date>/` is not a comment the build adds, and a comment brought back from there into code is. The removed-history rule keeps reading `docs/`: a group moved whole into an archive is stored, not removed, and deleting an archived file loses what it held, so it must be recorded ([ADR-0115](../adr/0115-commit-gate-keeps-history-and-records-waivers.md)).
- **What `history` writes is what the commit gate reads.** `UnrecordedHistory` takes the groups `RemovedHistoryAcrossDiff` found and returns those the change did not record: a group is recorded when its fenced text (`renderHistoryText`, the renderer `RenderHistorySection` uses) appears more often on its package's page (`historyPageName`, the name `writeHistoryArchive` uses, under `HistoryArchiveDir`) after the change than before it, one copy per removed group. Matching on the text rather than `file:line` keeps a record made against an older base valid. Each page is read once per check. An empty source has no history, so a zero-byte file is not a parse error.
- **The archive is append-only.** `RewrittenHistoryPages` names each changed archive page (a `.md` under `HistoryArchiveDir`, the `README.md` index aside, since `history` regenerates it) whose text before the change is not a prefix of its text after, a deleted page included. The commit gate refuses those pages even for a waived change. The rule lives here because this package owns the page format.
- **`IsUnparsable` classifies an error** as a Go syntax error, the one error a caller should treat as a defect in the change rather than a read fault, so callers never match the parser's error type.
- **This package is protected surface** (`guards.ProtectedSurfaceManifest`, beside `commitgate`). The commit gate trusts it for its review waiver and its refusals, so no loop lane may edit it ([ADR-0115](../adr/0115-commit-gate-keeps-history-and-records-waivers.md)).

## Tests

| Behaviour | Tests |
|---|---|
| what `strip` removes, keeps and refuses | `TestStripComments_*`, `TestStripCommentsKeepingPackageDoc_*`, `TestLeadingSentences_*` |
| the tree walk, the package-doc holder, atomic writes, the report | `TestStripDirs_*`, `TestPackageDocHolder_*` |
| the command line | `TestMain_Strip*`, `TestMain_History*` |
| history recording | `TestRemovedHistory_*`, `TestWriteHistoryArchive_*` |
| what the commit gate counts as recorded | `TestUnrecordedHistory_*` (the first runs `history` and checks its output satisfies the check) |
| the archive only grows; what counts as unparsable | `TestRewrittenHistoryPages_*`, `TestIsUnparsable_NamesASyntaxErrorAndNothingElse` |
| `docs/` is not code for the added-comment rule, and only the root `docs/` | `TestAddedAcrossDiff_ACommentArchivedUnderDocsIsARecordNotCode`, `TestAddedAcrossDiff_ACommentBroughtBackFromDocsIntoCodeIsAdded`, `TestAddedAcrossDiff_OnlyTheRootDocsDirectoryIsDocumentation`, `TestRemovedHistory_AGroupArchivedUnderDocsIsStoredNotRemoved` |
| equivalence (including which notes anchor their paragraph) and added-comment rules | `TestEquivalent*`, `TestAddedComments_*`, `TestAddedAcrossDiff_*` |

## History

- **2026-10-01:** `UnrecordedHistory`, `RewrittenHistoryPages`, `IsUnparsable` and the exported `HistoryArchiveDir` for the commit gate's history refusals; the added-comment rule stops counting `docs/`; the package joins the protected surface ([ADR-0115](../adr/0115-commit-gate-keeps-history-and-records-waivers.md)).
- **2026-09-30:** `history` and the comment history archive; `strip`, the marker table and the shared package-doc floor, after a Clean Code review of a stripped trial ([report](../../reports/comment-strip-clean-code-review-2026-09-30.md)).
