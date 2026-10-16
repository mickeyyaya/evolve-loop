# internal/phases/secretleakscan

## Purpose

`internal/phases/secretleakscan` is the native, in-process secret-leak-scan phase. It replaces the full-LLM phase, which booted a CLI for about twelve turns, with a deterministic regex scan over a unified git diff. Deterministic work belongs in code, not in LLM cycles.

## Design

- **Added lines only.** `ScanDiff` inspects the `+` lines of a unified diff, excluding the `+++` file header. Removed and context lines are ignored, because only newly introduced content can leak a secret.
- **Five detectors.** A PEM private-key header, an AWS access-key id, a GitHub token, a Slack token, and a generic `api_key`/`secret_key`/`private_key` assignment. Each targets a structurally unambiguous shape, so a clean diff does not trip it. A false positive makes the native phase noisier than the LLM phase it replaces.
- **Stable order.** The detector list is ordered, so identical input yields identical findings.
- **Location.** Each finding carries the post-image file and line. A `@@` header sets the new-file line, removed lines do not advance it, and a count of zero marks an unknown position.
- **Verdict.** `Verdict` maps findings to `PASS` or `FAIL`, the vocabulary the LLM variant produced. Downstream gates pattern-match the deliverable artifact, so the vocabulary must not change.
- **CRLF.** A trailing carriage return is trimmed, so CRLF diffs scan the same as LF diffs.

## Invariants

- **Only added lines can leak.** `TestScanDiff_IgnoresRemovedAndContextAndHeaderLines` pins this.
- **Identical input, identical findings.** `TestScanDiff_Deterministic` pins this.
- **Line numbers follow the new file.** `TestScanDiff_FindingCarriesPostImageFileAndLine` pins the location rules, including the no-newline marker and quoted paths.

## Findings

- `hasPrefix` duplicates `strings.HasPrefix`, and the file already imports `strings`. The helper is a candidate for removal. It is kept here, because this change does not alter the code's structure.
