# internal/reportdoc

## Purpose

`internal/reportdoc` parses the small, machine-checked sections of the agent-written Markdown reports. It reads the auditor's review fields, the findings and the verdict. It also holds the single home of two beliefs that other packages share: the rule for a safe relative path, and the no-follow file open.

## Design

- **Visible lines only.** Every extractor scans the visible stream. Fenced code, indented code and HTML comments are removed first. So a template or example inside a report cannot satisfy a contract, declare a verdict or raise a finding. The one exception is the fenced `tsv` block directly under `## Issues`, which is the issues table of the reference template.
- **One presence rule.** `HasSection` is the presence predicate that the deliverable contract and the phase gates share. A duplicated heading still counts as present, and `Section` rejects it as a parser finding.
- **Fields.** `Fields` reads allowed `- Key: value` lines without regard to case. Backticks around a value are stripped. List keys such as `Evidence` may repeat, and their values join with newlines. Single-valued keys reject a repeat.
- **Reasoning floor.** `RequireReasoning` is the one blocking rule that the audit and retro gates keep. A review's evidence must say something. Token or canned evidence is not a review. `ReasonedReview` is the one reading of a review section that every phase gate shares. Absence is never laxer than thinness.
- **Grounding.** `RequirePathLineEvidence` requires each cited path to name a line. Ranges and column forms (`:23-48`, `:12:3`, `#L7-L9`) count as the citation. `RequirePathLineEvidenceAt` also checks that the line exists in the current file, or in the base blob when the build deleted the path.
- **Review field evidence.** `EvidenceOrBody` falls back to the whole section when no evidence field exists. The fallback forgives only the field name. The reviewer must still cite each material path at a line.
- **Finding grammar.** Five auditor layouts are live. Three are headings: the severity sits in parentheses, after a dash, or is implied by the id letter. The other two are a table row and the reference template's TSV block. Outside `## Issues` a finding needs an explicit severity token. `FindingKey` is the cross-round key of a title: the lead clause, lower-cased, alphanumerics only, capped at 80 characters.
- **Verdict grammar.** A verdict is the canonical two-line `## Verdict` heading, or an inline form that has a colon. The colon is required, so a prose line such as "Verdict PASS is required" cannot read as a PASS on the ship gate. The sentinel of `phasecontract.ParseVerdictSentinel` is authoritative and is tried first by every consumer.
- **Safe paths.** `ValidReference` is the one check that a relative path is safe. It is shared with `explanationdocs`, whose own copy used to judge the same string differently.
- **No-follow open.** `OpenRegularNoFollow` opens a file with `O_NOFOLLOW` on darwin and linux. Other platforms fall back to a plain open, so each caller must check identity with `Lstat` and `SameFile`. Do not copy this helper into another package.

## Invariants

- Examples inside fences, indented blocks and HTML comments never raise a finding or declare a verdict. Pinned by `TestFindings_IgnoresFencedIndentedAndCommentedExamples` and `TestSection_UsesOnlyVisibleUniqueHeadings`.
- A duplicated visible heading is a finding, and `HasSection` still reports it present. Pinned by `TestSection_RejectsDuplicateVisibleHeading`.
- Absence is never laxer than thinness. Pinned by `TestReasonedReview_AbsenceIsNeverLaxerThanThinness` and `TestReasonedReview_GarbledBoilerplateWithoutEvidenceFailsTheFloor`.
- A prose line without a colon is not a verdict, and the verdict grammar matches the audit gate's fallback. Pinned by `TestVerdict_MatchesTheAuditGatesProseFallback`.
- The title key joins the same defect across rounds. Pinned by `TestFindingKey_*` in `findings_test.go`.
- The safe-path rule and the no-follow open have one home each. Pinned by `TestValidReference_SingleSourcedStrictness` and `TestOpenRegularNoFollow_RefusesFinalSymlink`.
