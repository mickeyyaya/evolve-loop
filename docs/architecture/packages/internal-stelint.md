# internal/stelint

> The rules it checks: [the ASD-STE100 house rules](../../conventions/ste100-writing.md). The plan it serves: [the STE rewrite plan](../../plans/ste100-docs-rewrite-2026-10.md). Its command: `evolve docs ste-lint`.

## Purpose

`stelint` checks text against the measurable ASD-STE100 rules. It checks Markdown documents and the log and error text in Go code. It reports findings and never blocks: the verb exits 0 without `--strict`, and the build floor prints one WARN line.

## Rules

| Rule | What it finds |
|---|---|
| `STE-SENTENCE` | a sentence of more than 25 words, or more than 20 words in a numbered item |
| `STE-PARAGRAPH` | a paragraph of more than six sentences |
| `STE-WORD` | a phrase from the house list of words to replace, matched as whole words without regard to case |
| `STE-SKIPPED-GENERATED` | a notice, not a finding: a generator writes this file or region, so the lint does not check it |

## Design

- **One home for the word list.** `ParseWordTable` reads the table under the heading "House list of words to replace" in `docs/conventions/ste100-writing.md`. The Go code has no copy of the list. A standard without the table, or with a row that has no replacement, is an error.
- **Blocks first.** `Check` splits a document into paragraphs. A paragraph is a block of text lines between blank lines. A list item is a paragraph, and each table cell is a paragraph.
- **Three kinds of paragraph.** A numbered list item has the limit of 20 words. The standard calls it a numbered item: a step, or a rule or a decision that other text cites by its number. Other prose has the limit of 25 words. A heading (ATX or setext) is checked against the house list only, with no length rule.
- **Blockquotes.** A blockquote is prose. The splitter removes the `>` marks and reads the text as usual. If the first text of a blockquote starts with a quotation mark, the blockquote is a quoted passage, and the lint does not read it.
- **What the lint does not read.** It does not read YAML frontmatter, fenced code, HTML comments, HTML blocks or table separator rows. An HTML block starts with a block tag (`details`, `div`, `table` and the others in `blockTags`). An inline tag (`a`, `b`, `code`, `span` and the others in `inlineTags`) at the start of a line does not start a block.
- **The standard's own word table.** In `docs/conventions/ste100-writing.md` only, the lint does not read the table under the house-list heading. `LintFile` sets `Options.IsStandard` from the path.
- **A scanner removes the markup.** A code span is one technical word. A link keeps its text and loses its URL: the link text counts for length, but `STE-WORD` does not match it. An autolink and a bare URL are one technical word each. An inline HTML tag is removed.
- **Quotations.** A pair of straight double quotes or of curly double quotes is a quotation. A quotation is one word, the same as a code span, and `STE-WORD` does not match it. A quotation mark without a pair in its paragraph is plain text.
- **Sentences.** A sentence ends at `.`, `?` or `!` at the end of a token. Closing marks after the stop (`)`, `*`) do not change this, and a quotation that ends with a stop ends the sentence. A dot inside a token (`1.5`, `docs/a.md`) does not end a sentence. The last word of a house-list phrase that ends with a dot (`e.g.`, `etc.`, `vs.`, `et al.`, `incl.`) does not end a sentence.
- **Words.** A word is a token with a letter, a digit, a code span or a quotation in it.
- **Technical words.** A token with `=` (`key=value`), with `://`, or a bracket tag (`[runner]`) is one technical word. Technical words count for length, but `STE-WORD` does not match them. A word with `_` (an UPPER_SNAKE code) never matches, because no house-list word has `_`.
- **Generated text.** A file is generated when an HTML comment before its first text line holds `GENERATED`, `DO NOT EDIT` or `Code generated`. A region between `<!-- GENERATED:<name> BEGIN -->` and `<!-- GENERATED:<name> END -->` is generated too. The lint reports each file once, at the first marker, with `STE-SKIPPED-GENERATED`.
- **Go text.** `CheckGo` parses one file with `go/parser`. It checks the message argument of `fmt.Errorf`, `errors.New`, each `log` function, and each method named `Infof`, `Warnf`, `Errorf`, `Printf` or `Logf`. It also checks `fmt.Fprint`, `fmt.Fprintf` and `fmt.Fprintln` to a stream. A stream is an identifier or a field named `stderr`, `Stderr`, `stdout` or `Stdout`: `os.Stderr`, an injected `stderr`, or `opts.Stdout`.
- **What a Go message can be.** The argument can be a string literal, a concatenation of literals, or a constant of the same file with one definition. The lint does not check any other argument. A format verb (`%s`, `%-10s`, `%[1]d`) is one technical word, and `%%` is a percent sign. One message is one paragraph for `STE-SENTENCE` and `STE-WORD`.
- **Determinism.** The findings are sorted by line and then by rule. The same input gives the same output.
- **One home for each limit.** The limits 20, 25 and 6 are constants in `text.go`. `TestTheLimitsMatchTheStandard` reads the section "The lint" of the standard and fails when a number there is different.
- **The standard library only.** The package imports no package of this repository, like `docsfloor`, so every layer can use it.

## Limits

- An indented code block (four spaces, no fence) is read as text. The house documents use fences.
- A table without a leading `|` is read as text.
- Two straight double quotes that are not a pair (for example, two inch marks) make a false quotation. The lint does not check the words between them.
- A Go constant in another file of the package is not resolved, so the lint does not check that message.
- A writer with another name (`w`, `out`) is not a stream, so the lint does not check its text.

## Consumers

| Consumer | What it does |
|---|---|
| `evolve docs ste-lint` (`cmd/evolve/cmd_docs.go`, `cmd_docs_report.go`) | checks the scope, the named paths, or the files that changed since a base ref, and prints `path:line RULE message` lines and a summary line, or JSON. It finds the root from `--project-root`, the root variables or the git top level, never from the current directory. |
| the build handoff floor (`steLintWarn` in `internal/core/ste_floor.go`, from `docsFloorWarn`) | checks the changed documents in scope and prints `[ste-lint] WARN: <n> finding(s) in <m> file(s) (first: path:line RULE)` when it finds a problem |

`steLintFloorLines` returns the lines, and `steLintWarn` prints them. A changed document that the floor cannot read gets its own `[ste-lint] WARN: <path> was not checked` line. At the stage `shadow`, the floor counts every finding in a changed document, not only the findings that the change adds.

The floor reads the standard from the cycle worktree, and only when a document in scope changed. A worktree without the standard has no STE rule, so the floor prints nothing. A standard that does not parse gives a WARN that the lint did not run. The policy key `docs_floor.ste_stage` (`off`, `shadow` or `enforce`; the compiled default is `shadow`) controls the floor. Every stage only WARNs.

## Tests

| Behaviour | Tests |
|---|---|
| the three rules and their limits (25, 20 and 6 pass; 26, 21 and 7 fail) | `TestCheck_SentenceLengthIsTwentyFiveWordsInText`, `TestCheck_ANumberedStepHasTwentyWords`, `TestCheck_ParagraphHasSixSentences`, `TestCheck_HouseWords` |
| the limits agree with the standard | `TestTheLimitsMatchTheStandard` |
| sentence splitting | `TestCheck_SentenceSplitting`, `TestCheck_ACodeSpanIsOneWord`, `TestCheck_AQuotationIsOneWord` |
| headings, blockquotes and each exemption | `TestCheck_AHeadingIsCheckedForWordsOnly`, `TestCheck_ABlockquoteIsProse`, `TestCheck_Exemptions`, `TestCheck_LinkTextCountsAndTheURLDoesNot`, `TestCheck_OnlyTheStandardExemptsItsOwnWordTable`, `TestLintFile_ExemptsTheWordTableOnlyInTheStandard`, `TestCheck_GeneratedText` |
| the word table | `TestParseWordTable_*` (one reads the real standard) and `TestLoadStandard_*` |
| Go extraction: each call form, constants, concatenations, a variable argument | `TestCheckGo_*` |
| the scopes and the file reader | `TestInDocsScope_*`, `TestInGoScope_*`, `TestLintFile_*` |
| determinism, 20 runs | `TestCheck_IsDeterministic`, `TestCheckGo_IsDeterministic` |
| the verb | `TestDocs_*` and `TestDocsSteLint_*` in `cmd/evolve/cmd_docs_test.go` |
| the floor and the dial | `TestDefaultBuildFloorChecks_WarnsOnSteFindingsInTheChangedDocs`, `TestDocsFloorWarn_*` in `internal/core/ste_floor_test.go`, and `TestDocsFloor*` in `internal/policy` |

## History

- **2026-10-08:** fix round 1 after the J_0 review. A quotation is one word, and the lint checks link text and headings for words only. A blockquote is prose. The word-table exemption is for the standard only, and an inline tag does not start an HTML block. The `--go` mode knows injected streams, and three abbreviations join the house list.
- **2026-10-07:** the first version: batch S0 of the STE plan, and the `--go` mode of batch S10.
