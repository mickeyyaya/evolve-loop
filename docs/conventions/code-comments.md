# Code comments

Code explains itself. **New and changed code carries no comments** (the operator's rule, 2026-09-27). The reader learns the *what* from names, types and small functions. The reader learns the *why* from tests and `docs/`. New code can carry only two types of comments: the machine-read ones in the table below, and the package doc that `go/acs/regression/docgo` enforces.

Existing code has the same target (the operator's rule, 2026-09-30): **zero comments beyond those**. The comment-reduction workstream is the means to reach that target. This follows *Clean Code*: a comment is a failure to express the intent in code. Doc comments on nonpublic code are noise, and everything under `internal/` and `cmd/` is nonpublic by construction. The comment-reduction workstream removes existing comments. A feature change never deletes them piecemeal.

This rule applies to every Go file, test files included. It applies to every author: people, the console and every loop phase. [ADR-0111](../architecture/adr/0111-code-carries-no-comments.md) records the decision behind it, with the alternatives that were considered.

## What a comment may say

| Keep | Form |
|---|---|
| Toolchain and linter directives: `//go:build`, `//go:embed`, `//go:generate`, `//go:noinline` and the rest, `//nolint`, `//line`, `//export` | as the tool requires, on the line that it applies to |
| Generated-file headers (`// Code generated … DO NOT EDIT.`) | as the generator writes them |
| `// Deprecated:` notes, which staticcheck and gopls read, and the `// Output:` header of an Example test | as written. The expected-output lines under the header still count as comments. Thus an Example with output is refused until the rule reads those lines (no Example exists in the repo). |
| Machine-read markers: `// acs-predicate: …` waivers, `//apicover:ignore reason=…`, `// minimal:` shortcuts ([minimalism](../../skills/minimalism/SKILL.md)), and the `IPC-protocol-allowed` waiver of the envtaint regression gate. The waiver is spelled `SSOT IPC-protocol-allowed` or `SSOT §IPC-protocol-allowed`, and the gate reads it anywhere in a comment. | one line for new markers. Existing multi-line markers stay whole, also when their reason reads like history. |
| The package doc | one to three lines of at least six words (`go/acs/regression/docgo` enforces the floor). The doc says what the package is for. It can end with `See docs/architecture/packages/<dir>.md.` |

Until the workstream reaches it, existing code can still carry a one-line *why* of something that the code cannot show, when no test pins it. These are the permitted types: an ordering constraint, a concurrency or security invariant, a deliberate deviation, a workaround for a bug outside this repo. This is transitional. The workstream files each such comment as an inbox item to pin the invariant with a test. The change that adds the test also removes the comment. New code states the *why* as a test name or in the design notes of the package.

## What a comment must not say

- **A doc on an exported identifier.** The name and the signature state the contract. A behavior that needs a statement is a test. The design is in the notes of the package. `commentaudit verify` accepts the deletion of such a doc as a comment-only edit. `apicover -require-doc` only reports.
- **A pointer or a path.** `// See ADR-0100.` and a path that names live code (`go/acs/cycle1123`) point at knowledge. That knowledge belongs in the design notes of the package. A path is a reference, not history, so `commentaudit check` does not count it. But it is still a comment to remove.
- **History.** Write no cycle numbers, incident narratives, F-ids, wave, batch or round numbers, dates, PR numbers, commit SHAs, release versions, or "this used to…". The incident record, the ADR and the CHANGELOG hold the history.
- **What the code already says.** Rename the identifier or extract a function instead.
- **Commented-out code.** Git keeps it.
- **Test narration.** The name of a test states its intent. A comment above or inside a test is permitted only for a non-obvious fixture choice, or to correct a name that misleads.
- **File headers** (`// handler.go — …`). The design notes of the package hold what a file is for.
- **Stale facts.** Correct or remove a comment that no longer matches the code. Never keep it as it was.
- **ADR sub-item ids** (`ADR-0049 N14`). Point at the ADR itself.

String literals are code. A date or incident id inside an error message or a test assertion is not a comment. Comment work never touches it.

## What a deleted comment leaves behind

To delete a comment is half the job (the operator's rule, 2026-09-30: when you remove comments, the code must be lean enough to explain itself). A deleted comment can say *what* the code does or what a value *means*. If the code does not say it, the same change makes the code say it. Use the smallest edit that does this and keeps the behaviour:

- rename an unexported identifier to the name that the comment used;
- extract a well-named function, local variable or named condition for the block or expression that the comment explained;
- replace a magic number or string with a named constant;
- introduce a small named type where the meaning of a value was only in the comment.

The edit never changes the value of a JSON tag, a flag or env name, or a string literal. It also never changes an error message or the expected text of a test. (A name for a literal keeps its value.) The edit adds no comment. Rename an exported identifier only when the name misleads and every caller changes with it. History, restatements and design reasons need nothing: the design reasons are in the notes of the package.

## Where the knowledge goes

| Knowledge | Home |
|---|---|
| Why a package has its shape, its invariants and what it taught | its design notes, `docs/architecture/packages/<dir>.md`. The file name is the directory under `go/`, with `-` in place of `/` (`internal/phases/ship` → `internal-phases-ship.md`). Sections: Purpose, Design, Invariants, Findings. [The index](../architecture/packages/README.md) lists them. The dated module docs of the decomposition program are in `docs/architecture/decomposition/`. They link here and do not absorb these notes. |
| A decision and its alternatives | an ADR under `docs/architecture/adr/` |
| What happened and what it taught | an incident record under `docs/incidents/`, plus a row in `REGRESSION-COVERAGE-INDEX.md` |
| Measured findings about the pipeline | a report under `docs/research/` or `docs/reports/`. It links to the Findings section of each package and does not copy it. |
| What changed for users | `CHANGELOG.md` |
| The history that a comment carried (what `commentaudit check` counts as history), when the comment is removed | the [comment history archive](../history/code-comments/README.md), `docs/history/code-comments/<dir>.md`. `commentaudit history` appends to it in the change that removes the comment. The commit gate refuses a removal that the change does not record (enforcement below). |

When a change removes a comment with knowledge that is nowhere in `docs/`, the same change writes that knowledge into its home. An invariant that moves out of the code must have a test that pins it, or it must keep a one-line *why* in the code. A doc alone does not prevent a future edit that breaks it. A comment-only change cannot add a test, so it keeps the one-line *why*. The test that pins the invariant comes in a separate change.

When a change alters the design or an invariant of a package, it updates the design notes of the package in the same commit.

## Enforcement

`evolve comments` and `go run ./cmd/commentaudit` run the same `commentaudit.Main` with the same arguments, so their output and exit codes match.

- `evolve comments rank [dir]` lists directories by narrative and comment load.
- `evolve comments history -base <ref> [-label L] [-out DIR] [dir ...]` records the history that the change removes.
  - It records each removed comment group that carries history (the `check` rule), as it was, with its file, its line and the declaration below it. It appends them as a labelled section to `docs/history/code-comments/<dir>.md`. Without `-out`, it prints the section. A relative `-out` resolves against the repository root.
  - The anchor is the code below the comment, never another comment or a trailing one.
  - A group that shows up again whole elsewhere in the change is a move, and the command does not record it. In a file, the match is first by the full text and the code below it, then by the text alone. Across files, the match is by the text alone. Thus the same group, removed from one file and newly written in another, reads as a move.
  - The command records a group that is reworded, split or merged as it was.
  - Every run rewrites the page's `README.md` index from the pages. The command refuses a label that is already on a target page, so it records a change once.
  - The comment-reduction rounds run it once per round, from round 13. They land the archive with the removal ([plan](../plans/comment-reduction-2026-09.md), step 6). Every other console change runs it too, because the commit gate refuses a removal that the change does not record (below).
- `evolve comments strip [dir ...]` (default `.`) removes, in place, every comment that the table above does not allow.
  - One table in `equivalence.go` gives the allowed comments and how much of each comment it keeps. `verify` reads the same table. The command keeps:
    - the own line of a directive;
    - the whole paragraph of a `minimal:` or `Deprecated:` note, up to a blank `//`;
    - an `Output:` header and the rest of its group;
    - each comment group with the `IPC-protocol-allowed` marker, whole, because envtaint reads the group;
    - a whole generated file, with no change.
  - It also keeps a cgo preamble and one package doc for each package. That doc is the doc of `doc.go`, else of `<dir>.go`, else of the first non-test file that has one. (A package of test files only keeps none.)
  - That doc stays as written when it is at most three lines of at least six words. If not, it becomes its first sentences up to six words, wrapped to at most three lines. When no such form exists, the doc stays as written, and the command lists it on stdout as `package doc to write: <path>`.
  - An inline comment between two tokens becomes the space or newline that the language reads it as.
  - gofmt formats every rewrite (LF line endings). The command writes it atomically and keeps the mode of the file. `Equivalent` proves that the rewrite is comment-only. If a file fails the proof, the command leaves it alone, names it on stderr and exits 1.
  - Before you commit the result, run `history` against the same base, so that what the comments recorded stays.
- `evolve comments verify -base <ref> [dir ...]` proves that an edit changed only comments. It works from anywhere in the repo.
  - It compares position-free ASTs. It treats a cgo preamble (the comment above `import "C"`) as code.
  - It requires every directive and marker above to survive, still attached to the same code. It refuses added or deleted files. The deletion of any other comment, the doc of an exported identifier included, is comment-only.
  - A `dir` resolves against the working directory, else the repo root. The repo root itself means every change. A scope that matches no changed Go file fails, and a proof over zero files also fails.
- `evolve comments check -base <ref>` names every comment line with history that a diff adds, by the same move rule as `comments`. A bare `See ADR-NNNN.` pointer, the reference date layout of Go and the `INCIDENT` envelope of the observer are not history.
- `evolve comments comments -base <ref> [dir ...]` lists every whole-line comment that a diff adds, as `path: text`. It uses the one rule that the build floor and the commit gate share (`commentaudit.AddedAcrossDiff`).
  - Comments are Go scanner tokens on physical lines. Thus a `//` line inside a string literal is not a comment, and a `//line` directive hides nothing after it.
  - Not added:
    - the directives and markers above;
    - the package doc of a new file, or a rewrite of an existing one, while it stays within three lines or its old length. (A doc added to a file that had none is added.)
    - a comment moved within the scoped diff (a line that one changed file drops and another gains, a `git mv` included);
    - anything under `testdata/` (inputs), `vendor/` (third-party code) or a dot directory: the skip set that `rank` uses and that the go tool ignores;
    - anything under `docs/`, which holds records, never compiled code (the predicate packages that continuation adoption archives under `docs/private/research/`).
  - A comment brought from `docs/` into code is added. With a `dir` scope, a comment moved in from outside the scope counts as added.
  - It exits 1 when it lists any line, so it is the comment count for new and changed code. `dir` scopes it as it scopes `verify`.
- The commit gate (`evolve commit-gate run`) refuses a commit that adds a comment, by the rule in the `commentaudit comments` bullet above. It names each added line and writes no attestation. This binds the console and every operator, so the rule holds for code that no loop lane wrote.
- The commit gate also refuses a commit that removes a comment group with history, unless the change records it.
  - The `check` rule defines the history. The same `RemovedHistoryAcrossDiff` that `history` runs finds the group. Thus a group moved whole elsewhere in the change is not removed.
  - The change must add the text of the group to the page of its package under `docs/history/code-comments/`.
  - The gate names each unrecorded group by `file:line`, with the `commentaudit history` command that records it. It writes no attestation. Run the command that the refusal names, then stage the pages that it writes ([ADR-0115](../architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md)).
  - When you delete a Go file, or an archived one under `docs/`, you also remove its history.
  - The archive is append-only. The gate also refuses a change that deletes or rewrites any archive page other than the generated `README.md` index. Thus the old text of a page must stay its prefix. This holds even for a waived change. Append a correction as a new section.
  - Loop lanes do not pass through the commit gate; see inbox `loop-history-comment-removal-needs-an-archive-entry`.
- The commit gate needs no reviewer for a comment removal that it can prove.
  - The proof needs at least one Go file that is comment-only against `HEAD` by the same check as `verify`. The change must have nothing else but Markdown under `docs/`.
  - The gate lists the change itself, with renames split into a deletion and an addition. Thus neither `--files` nor a rename can hide a path. Every path must be a regular file.
  - These still need code-simplifier and a reviewer: a symlink, a deletion, a `testdata/` fixture, a docs-only change, or anything that touches code, tests or personas.
  - The attestation of a waived commit records no reviewer and `review_waiver: "comment-only"`. Thus ship adds no `Reviewed-by:` trailer, and adds a `Review-waived: comment-only` trailer. That trailer tells it apart from a `--bypass-commit-gate` commit in `git log`. A denied waiver logs the reason.
- `apicover -enforce -require-doc` lists the exported symbols of enforced packages that have no doc. It only reports. Its exit code counts untested symbols, not absent docs. An absent doc is the target, not a defect.
- Console reviewers flag every comment that a diff adds, if it is not machine-read and not the enforced package doc (`.claude/agents/architecture-reviewer.md`, the three-book audit). The loop auditor reports each one as a LOW, advisory finding (`agents/evolve-auditor.md`, the review checklist line).
  - For loop lanes, the build handoff floor counts them at build exit with the same rule (`comment_floor`, batch 0b).
  - In `shadow`, the compiled default, it WARNs on the phase log.
  - Set `policy.json` `comment_floor.stage` to `enforce`, and a build that adds a comment gets a correction before its audit.
  - This project turns it on when the TDD phase has its own comment floor. The reason: the floor scans the whole diff, and the builder must not edit the predicate files that TDD writes.
- The removal of existing comments runs as its own workstream: [comment reduction](../plans/comment-reduction-2026-09.md).
