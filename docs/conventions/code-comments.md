# Code comments

Code explains itself. A comment exists only when the code cannot say something the reader needs. What the project learned belongs in `docs/`, not in the code.

This rule applies to every Go file, test files included, and to every author: people, the console, and every loop phase.

## What a comment may say

| Keep | Form |
|---|---|
| Toolchain and linter directives: `//go:build`, `//go:embed`, `//go:generate`, `//go:noinline` and the rest, `//nolint`, `//line`, `//export` | as the tool requires, on the line it applies to |
| Generated-file headers (`// Code generated … DO NOT EDIT.`) | as the generator writes them |
| `// Deprecated:` notes, which staticcheck and gopls read, and `// Output:` lines in Example tests | as written |
| Machine-read markers: `// acs-predicate: …` waivers, `//apicover:ignore reason=…`, `// minimal:` shortcuts ([minimalism](../../skills/minimalism/SKILL.md)), and the envtaint regression gate's `IPC-protocol-allowed` waiver (spelled `SSOT IPC-protocol-allowed` or `SSOT §IPC-protocol-allowed`), which it reads anywhere in a comment | one line for new markers; existing multi-line markers are kept whole, even when their reason reads like history |
| The package doc | one to three lines of at least six words (`go/acs/regression/docgo` enforces the floor): what the package is for, optionally ending `See docs/architecture/packages/<dir>.md.` |
| The doc of every exported identifier outside test files | one line stating the contract. Shorten a long doc; never delete one. That covers exported types, functions, const and var declarations, and methods of exported types. Packages in `go/.apicover-enforce` must also have a doc on every exported symbol, and that is the file's Definition of Done. Struct fields and `TestXxx` functions are not covered. |
| The *why* of something the code cannot show: an ordering constraint, a concurrency or security invariant, a deliberate deviation, or a workaround for a bug outside this repo | one or two lines, present tense |
| A pointer to the design that explains a non-obvious choice | `// See ADR-0100.`, at most once per decision |
| A path that names live code, such as `go/acs/cycle1123` | as written; a directory name is a reference, not history |

## What a comment must not say

- **History.** No cycle numbers, incident retellings, F-ids, wave or batch numbers, dates, or "this used to…". The incident record, the ADR and the CHANGELOG hold the history.
- **What the code already says.** Rename the identifier or extract a function instead.
- **Commented-out code.** Git keeps it.
- **Test narration.** A test's name states its intent. Comments above or inside a test are allowed only for a non-obvious fixture choice, or to correct a name that misleads.
- **File headers** (`// handler.go — …`). The package's design notes hold what a file is for.
- **Stale facts.** A comment that no longer matches the code is corrected or removed, never kept as it was.
- **ADR sub-item ids** (`ADR-0049 N14`). Point at the ADR itself.

String literals are code. A date or incident id inside an error message or a test assertion is not a comment, and comment work never touches it.

## Where the knowledge goes

| Knowledge | Home |
|---|---|
| Why a package is shaped the way it is, its invariants and what it taught | its design notes, `docs/architecture/packages/<dir>.md`, named by the directory under `go/` with `/` replaced by `-` (`internal/phases/ship` → `internal-phases-ship.md`). Sections: Purpose, Design, Invariants, Findings. [The index](../architecture/packages/README.md) lists them. The decomposition program's dated module docs live in `docs/architecture/decomposition/` and link here rather than absorb these notes. |
| A decision and its alternatives | an ADR under `docs/architecture/adr/` |
| What happened and what it taught | an incident record under `docs/incidents/`, plus a row in `REGRESSION-COVERAGE-INDEX.md` |
| Measured findings about the pipeline | a report under `docs/research/` or `docs/reports/`. It links each package's Findings section rather than copying it. |
| What changed for users | `CHANGELOG.md` |

When a change removes a comment that carried knowledge found nowhere in `docs/`, the same change writes that knowledge into its home. An invariant that moves out of the code must either be pinned by a test or keep a one-line *why* in the code. A doc alone does not stop a future edit from breaking it. A comment-only change cannot add a test, so it keeps the one-line *why*. The pinning test comes in a separate change.

When a change alters a package's design or an invariant, it updates the package's design notes in the same commit.

## Enforcement

- `go run ./cmd/commentaudit rank [dir]` lists directories by narrative and comment load.
- `go run ./cmd/commentaudit verify -base <ref> [dir ...]` proves that an edit changed only comments. It works from anywhere in the repo. It compares position-free ASTs and treats a cgo preamble (the comment above `import "C"`) as code. It requires every directive and marker above to survive, still attached to the same code. It refuses a deleted exported doc, and it refuses added or deleted files. A `dir` resolves against the working directory, else the repo root. The repo root itself means every change. A scope that matches no changed Go file fails, and so does a proof over zero files.
- `go run ./cmd/commentaudit check -base <ref>` names every history-carrying comment line that a diff adds. A bare `See ADR-NNNN.` pointer, Go's reference date layout and the observer's `INCIDENT` envelope are not history.
- The commit gate (`evolve commit-gate run`) needs no reviewer for a comment removal it can prove. That means at least one Go file comment-only against `HEAD` by the same check as `verify`, and nothing else but Markdown under `docs/`. The gate lists the change itself with renames split into a deletion and an addition, so neither `--files` nor a rename can hide a path. Every path must be a regular file. A symlink, a deletion, a `testdata/` fixture, a docs-only change, or anything touching code, tests or personas still needs code-simplifier and a reviewer. A waived commit's attestation records no reviewer, so ship adds no `Reviewed-by:` trailer, and a denied waiver logs the reason.
- `apicover -enforce -require-doc` lists the exported symbols of enforced packages that have no doc. It only reports; its exit code counts untested symbols, not missing docs. `verify` is what stops a comment edit from deleting a doc.
- Console reviewers flag every comment in a diff that breaks these rules. For loop lanes, the regrowth guard is planned as batch 0b of the workstream.
- The removal of existing comments runs as its own workstream: [comment reduction](../plans/comment-reduction-2026-09.md).
