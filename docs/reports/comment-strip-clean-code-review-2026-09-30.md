# Does the code explain itself once its comments are gone? A Clean Code review (2026-09-30)

## Summary

The operator's goal (2026-09-30) is to remove every comment and refactor the code so that it explains itself. `commentaudit strip` removed 89,895 comment lines from 3,355 files of a copy of main in about six seconds, and the result builds and vets. Before any of it lands, eight reviewers read 64 of the stripped production files against Robert C. Martin's *Clean Code*, each file beside the comments it lost.

| Measure | Result |
|---|---|
| Files that read fully without their comments (CLEAN) | **0 of 64** |
| Files with one or two small refactors to make (MINOR) | 51 |
| Files whose intent is lost until refactored (NEEDS-REFACTOR) | 13 |
| Mean readability (5 = reads fully without its comments) | **3.45** (35 files at 4, 23 at 3, 6 at 2) |
| Removed comment groups classified | 901 |

What the 901 removed comment groups carried:

| Class | Share | Where it goes |
|---|---|---|
| Redundant: the code already said it | 296 (33%) | nowhere; nothing is lost |
| Recoverable intent | 243 (27%) | into the code: a rename, an extracted function, a named constant or an explanatory variable |
| An invariant or a warning only a test can keep | 176 (20%) | into a test named for the rule |
| Design knowledge | 163 (18%) | into the package's design note, `docs/architecture/packages/internal-<pkg>.md` |
| History (cycles, incidents, dates) | 23 (3%) | the history archive, `docs/history/code-comments/` |

**Two thirds of what was removed carried something the code does not yet say.** Stripping is therefore half of the job. For production code, the removal, the refactors, the invariant tests and the design notes land together, package by package, so main never holds code that lost its intent. Tests and the per-cycle predicate packages are different, since test names and cycle reports carry their intent; they can be stripped in bulk.

The sample is weighted toward risk: it takes the files that lost the most comment relative to their code. The whole tree is likely to read better than these numbers.

## Method

- **Sample.** The trial stripped every Go file in a detached worktree of main. From the 1,014 production files it changed (no tests, nothing under `go/acs/`), files with at least 20 lines and 15 removed comment lines were ranked by removed lines per line of code, keeping at most two per package: 64 files from 54 packages.
- **Rubric.** [`rubric.md`](comment-strip-clean-code-review-2026-09-30/rubric.md): names (ch. 2, smells N1–N7), functions (ch. 3, F1–F4), general smells (ch. 17: G5, G16, G19, G20, G25, G28, G29, G31, G34), error handling (ch. 7), and a classification of every removed comment group by what it carried (ch. 4, "explain yourself in code"). Each finding names a concrete refactor or test.
- **Reviewers.** Eight Claude reviewers, read-only, eight files each, reading each stripped file and its `git diff` against main.

## Findings

### The most frequent smells

| Smell | Findings | Typical case |
|---|---|---|
| N1, a name that does not reveal intent | 70 | `FailedAt` holds a list of records, not a time (`cyclestate`); `RetroPrefix` is matched with `Contains`, not as a prefix (`routingtest`) |
| G25, a magic number or string | 52 | the `"evolve-"` prefix spelled in four places; `"ship.lock"` without a constant owning it |
| G5, duplication | 41 | two nested-session detectors that disagree (`detectnested`, `sandbox`); `buildplanner` and `swarmplan` near-duplicate classifiers |
| G16, obscured intent | 37 | `if cerr != nil { return }` reads as a swallowed error once the comment saying why is gone (`cycle_worktree_teardown`) |
| N4, an ambiguous name | 26 | three maps named like `Signals` with different types (`core/phase.go`) |
| G20, a function name that does not say what it does | 21 | `FromFloor` runs `cfg.Count` lanes, not the floor (`fleetbudget`) |

### Defects the comments were covering for

Reading the code without its comments surfaced behaviour the comments described wrongly or hid. Each is to be confirmed with a test before it is fixed:

| Where | What | Kind |
|---|---|---|
| `phases/ship/manifest.go:29-40` | a git error is ignored and the gate logs "all 0 bound path(s) covered", so under `enforce` it passes with nothing checked | fail-open gate |
| `phases/ship/gitops.go:110-120` | `detectColliders` trims a `git status` line before cutting its three-character prefix, so ` M go/x.go` becomes `o/x.go` | parsing bug |
| `phases/ship/manifest.go:34-39` | `diff --name-only` output is never unquoted, so a non-ASCII path reads as undeclared, a false block under `enforce` | parsing bug |
| `subagent` worker commands (`dispatchparallel.go:266`) | a subtask name from the profile is pasted unquoted into `sh -c`; the removed comment claimed it was regex-constrained, which holds for the role only | possible shell injection |
| `adapters/statemap/statemap.go:80` | a malformed or unreadable state file skips the revision check and is overwritten | data loss |
| `shipmanifest/shipmanifest.go:65` | the error from `continuation.ReadManifest` is dropped, so a corrupt manifest silently loses the prior attempt's paths | swallowed error |
| `phaseregistrar/registrar.go:39-44` | `Register` writes into the caller's `SandboxConfig` through a shared pointer | hidden side effect |
| `phaseconfig`, `ResolveStage` | `StageAdvisory.String()` is "advisory", which falls into the typo branch and becomes "off" | possible bug |
| `modelquery/lineage.go`, `dateRun` | a four-digit token such as "2048" in a model id is stripped as a year | possible bug |
| `recovery/outcome.go` | any non-empty `AbortReason` is read as a phase abort (`core/signal.go:69-72`), though the removed comment said it does not mean the phase died | possible mislabel |
| `detectnested` vs `adapters/sandbox` | the two nested-session checks read different variables; only one exempts `CLAUDECODE_TYPE=host` | two beliefs, one fact |
| `paths.RunWorkspace` | the removed comment called it the one home of the run-directory path; at least nine other sites build it | false single-source claim |

Dead code the comments were describing as live: `inbox.Envelope.Seq`, `PhaseConfig.SwarmWorkers`, `PhaseConfig.PromptBody()`, `RunScope.WorkspacePath()`, the `codex` and `agy` rows of `bareDriverMap`, `Realization.Ephemeral` and `SessionName`, `docsfloor.LabelArchitecture`, and `sysexec.CombinedOutput`, which also treats a non-zero exit as success.

### Design notes need homes

Removed design comments belong in `docs/architecture/packages/internal-<pkg>.md`, and 45 of the 54 sampled packages have no page by that name (counted against the tree on 2026-09-30). A few are covered by other design documents: the reviewers found most of what `modelquery`, `shipmanifest` and `core/carryover` lost already in their package or decomposition docs. The refactor pass creates each missing page as it lands its package.

### The tool had four defects, all found by this review or the tool's own review, and fixed before landing

| Defect | Fix | Test |
|---|---|---|
| a `// minimal:` or `// Deprecated:` note kept only its first line, leaving half a sentence (`contextfill.go:24`) | a note keeps its whole paragraph, up to a blank `//` | `TestStripComments_KeepsAWholeNoteParagraphAndDropsWhatFollowsIt`, `TestStripComments_AWhitespaceOnlyCommentLineEndsANote` |
| a shortened package doc ended in a colon that introduced a removed list (`gopkgpattern.go:1`) | a colon-ended first paragraph ends with a period | `TestLeadingSentences_EndsAtASentenceBoundaryNotAnAbbreviationOrAColon` |
| the first-sentence cut split at "e.g. ACS" | a sentence ends at a period followed by a capital, unless the word before it is an abbreviation | the same test |
| a line quoting the `IPC-protocol-allowed` marker survived alone, as a fragment (`subagent/recursion.go:12`); the envtaint scan reads the marker per comment group | a group that carries the marker is kept whole | `TestStripComments_KeepsAGroupThatCarriesTheIPCMarkerWhole` |

The tool's review also made its rewrite atomic (a temporary file renamed over the original, keeping the file's mode) and pinned idempotence and gofmt's LF line endings.

## Decision

1. **Production code lands package by package**, each landing holding the comment-only removal (proven by `commentaudit verify`, recorded by `commentaudit history`) and, in the same change, the refactors this review's rubric calls for: recoverable intent into names, functions and constants; invariants into tests; design notes into the package page. The defects above are confirmed with a failing test and fixed in the package's landing, or filed when they sit on a protected surface.
2. **Test files and `go/acs/` predicate packages are stripped in bulk**, since test names and the cycle reports carry their intent. Predicate packages that pin comments are updated or archived in that landing.
3. **The `minimal:` marker** is a human note, not a tool directive. Under the zero-comment goal, the six in production code (`contextfill`, `core/workspace_guard.go`, `core/worktree_clean.go`, `deliverable`, `fleet/starvation.go`, `phases/audit/defect_ledger.go`) move into their package design notes, and the minimalism skill records a ceiling there; the gate's allow list drops it in the same change.

## Artifacts

- [`verdicts.tsv`](comment-strip-clean-code-review-2026-09-30/verdicts.tsv): every sampled file with its verdict, readability and the classes of what it lost.
- [`group-00.md`](comment-strip-clean-code-review-2026-09-30/group-00.md) to [`group-07.md`](comment-strip-clean-code-review-2026-09-30/group-07.md): the eight reviews verbatim, with every finding's line and refactor.
- [`rubric.md`](comment-strip-clean-code-review-2026-09-30/rubric.md): the rubric the reviewers applied.
