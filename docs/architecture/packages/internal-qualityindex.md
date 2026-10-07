# internal/qualityindex

> Decision: [ADR-0124](../adr/0124-code-review-phase.md). Design: [review-loop-and-quality-index.md](../review-loop-and-quality-index.md) §2–§3. The prose projection: `skills/quality-index/` (SKILL and COMPACT).

## Purpose

`internal/qualityindex` is the code home of the quality index that the code-review loop and the audit share. It holds:
- the ten dimension keys and the never-N/A set;
- the threshold resolver for `workflow.quality_index.thresholds`;
- the parsers for the `## Scores` and `## Review Plan` grammars, and their agreement check;
- `Qualifies`, the one qualification test.

It decides nothing about routing and reads no files.

## Design

- **The vocabulary is one ordered table** (`index`). `Keys()` returns a copy in index order, so a caller can never reorder or extend the vocabulary. `NeverNA` and `Known` read the same table.
- **Thresholds resolve in one place.** `ResolveThresholds(raw)` starts from `DefaultThreshold` (4) for every dimension, applies `"*"`, then the per-key entries. An unknown key or a value outside 1–5 is ignored with a warning, and the default stands. `policy` decodes the block and calls it; the deliverable gate reads the same result through `policy.QualityIndexThresholdsFor`.
- **The grammars are line grammars on reportdoc's visible section.**
  - `sectionEntries` takes `reportdoc.Section`, so a fenced example can never be parsed, and reads only `- key: …` lines; prose lines in a section are allowed. It strips `**` from an entry the way `reportdoc.Fields` does, so a bold key reads as its key: one tolerance for one report.
  - `ParsePlan` collects the `Inputs read` lines itself: none is a problem, the first one is validated (`planInputs`), and any more are one problem.
  - `splitValue` splits a value from its rationale at the first em dash, en dash or hyphen with a space on each side, the tolerance `reportdoc`'s finding headings already allow.
  - `scanDimensions` is the one walk both grammars share: it reports an unknown key, a duplicate, and every dimension that never appeared. A line that is present but malformed is reported once, not again as missing.
- **Problems are data.** Every parser returns its problems as correction-directive strings that name the dimension and the expected form. The caller decides what they mean: the deliverable gate turns each one into a `bad_grammar` violation.
- **`Agree`** checks that the plan's N/A set equals the Scores' N/A set. It reads only dimensions present in both, because a missing one is already reported by the parser.
- **`Qualifies`** returns the gaps in index order: a missing dimension, a forbidden N/A, or a score below its threshold. It is the only qualification test; the loop's resolved predicate, the audit cross-check and the shadow metric all call it.

## Invariants

- The skill's and COMPACT's dimension tables and "Never N/A:" lines list exactly `Keys()` and the never-N/A set, in order (`TestTheSkillProjectsTheIndexItsCodeDefines`).
- A configured threshold changes the bar; the literal 4 is only the default (`TestQualifies_EveryApplicableDimensionMustMeetItsThreshold`).
- Every malformed form named in the design's §2.2 and §3 is exactly one problem (`TestParseScores_EveryMalformedFormIsAProblem`, `TestParsePlan_EveryMalformedFormIsAProblem`).

## Findings

- None yet. The shadow waves measure how often reviewers score below the bar and how often the audit disagrees ([plan §8](../../plans/code-review-phase-2026-10.md)).
