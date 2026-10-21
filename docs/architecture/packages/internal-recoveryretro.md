# internal/recoveryretro

> Spec: [recovery-retrospective.md](../recovery-retrospective.md). Decision: [ADR-0151](../adr/0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md). Plan: [recovery-retrospective-2026-10](../../plans/recovery-retrospective-2026-10.md) (C1 to C3).

## Purpose

`internal/recoveryretro` renders two documents from records. A recovery document shows each try of an item that failed two or more times and then shipped. A judgment-lesson document holds one `cycle-<N>-judgment-<phase>` carryover todo. Each document has a sibling lesson YAML for the recall of `internal/research`. The package makes no LLM call.

## Design

- **The history is the dossier corpus.** `Recoveries` reads `dossier.Dossier` values only. `WriteShadow` loads them with `dossier.ReadDir` from `knowledge-base/cycles/` and `.evolve/dossiers-pending/`, for the cycles `N-HistoryWindow` to `N`. It reads the dossier of the current cycle first. It reads the history only for a PASS dossier with `tasks`, so a FAIL closeout reads one file.
- **A pass resets the count.** `failuresSinceLastPass` walks the dossiers in cycle order. A PASS dossier of the item clears the list, so an item that shipped and failed again starts a new count. `MinFailures` (2) failures make a recovery. A cycle that is in both directories counts once.
- **Pure render, one writer.** `RenderRecovery`, `RenderJudgmentLesson`, `recoveryLessonYAML` and `judgmentLessonYAML` are pure. `WriteShadow` is the only function that touches the disk.
- **Agent text is data.** `quote` puts a value in a code span and replaces each backtick with `'`. It joins the lines. `cell` also cuts a value to 240 runes (the dossier keeps about 200 for each reason) and escapes `|` for a table cell. The host text passes `evolve docs ste-lint --strict`, which the tests check with `stelint.Check`.
- **No error path in the YAML.** `lessonEntry.yaml` quotes each value with `strconv.Quote`. A Go double-quoted string is a valid YAML scalar.
- **Errors do not stop the other writes.** `WriteShadow` joins each error, returns each path that it wrote, and creates the directory only when it has a pair to write. An empty workspace path is an error.
- **The rungs come from the call log.** `laddersAbove` reads `llm-calls.ndjson` of the workspace and keeps each `<phase> <rung>` above `R0`, once. An absent log gives no rung. An unreadable log is an error, and the recovery pair is not written, because its rung line can not be true.

## Callers

- `core.cycleRun.writeShadowRetrospectives` (`go/internal/core/retrospective_shadow.go`), from `completeCycle` after the dossier write. It prints one WARN line on an error.

## Tests

The package has 100% statement coverage and is in `go/.cover-strict` and `go/.apicover-enforce`. The worked example of the spec (`inboxbatch-utf8-and-resolution`, cycles 1804 to 1809) is the main fixture.
