# internal/ledgerscan

> Decision record: [ADR-0123](../adr/0123-ledger-durable-evidence-segments-incremental-verify.md), decision 5. Plan: [ledger restructure](../../plans/ledger-restructure-2026-10.md), component C8.

## Purpose

`internal/ledgerscan` reads `.evolve/ledger.jsonl` from the newest line to the oldest, in 64 KiB chunks. A caller that needs the newest row of one run stops at that row and reads only the tail of the file. It never writes, repairs or seals a row.

## Design

- **API.** `Reverse(path, Filter, visit)`, `Filter.Pass(line)` and `ContainsValue(value)`. `visit` gets each non-empty line without its newline and returns `stop`. The line is valid only during the call. A missing file returns an error that wraps `os.ErrNotExist`.
- **The caller decodes.** The scanner does not decode JSON. Each reader keeps its own decode, trim and malformed-line rule. `auditledger` trims each line and skips a line that does not decode. The composition reader does not trim and skips a line that does not decode. The scan never turns a malformed line into an error.
- **Lines across chunks.** The head of a chunk that has no newline is kept and put in front of the next older chunk. The first line of the file is sent last. A last line with no newline, or a truncated last line, is sent as bytes.
- **Prefilter.** `Filter.Needles` hold VALUE bytes only, never key bytes. A line passes when it holds every needle or holds a backslash. `encoding/json` matches keys without case and decodes escapes in values, so a key needle or an escaped value could be missed. A value written with an escape has a backslash, so it passes. `ContainsValue` returns nil for an empty value and for a value with U+FFFD or an invalid UTF-8 byte (one check; the match counts a bad byte as U+FFFD). The decoder turns a bad byte in a line into U+FFFD, so a needle with U+FFFD would not match the raw bytes. The caller decode always confirms a match, so a false pass costs time and never a wrong result.
- **One home for each predicate.** `auditledger` names `AuditorKind` and `AuditorRole`; `IsAuditorRow` and the needles both use them. The composition reader's `compositionSpec` yields both `match` and `filter`.
- **Path.** Callers take the ledger path from `paths.LedgerFileOf(projectRoot)`.

## Invariants

- **Read-only.** The only file call is `os.Open`.
- **The prefilter never drops a row the decode would match.** Pinned by `TestPrefilterNeverMissesADecodeMatch`, `TestPrefilterRefusesTheNeedleThatInvalidUTF8WouldFake`, `TestAuditorFilterPassesEveryRowTheAuditorPredicateAccepts` and `TestCompositionFilterPassesEveryRowTheCompositionPredicateAccepts`.
- **Old and new readers agree.** Pinned by `TestReadersMatchTheLegacyReaderOnARealShapedLedger` and `TestLatestCompositionVerdictMatchesTheLegacyReaderOnARealShapedLedger`. Both run on a fixture of 9000 or more rows with mixed kinds, a truncated last line, a malformed middle line, no trailing newline and a row longer than a chunk. Pass `-real-ledger-copy=<path to a copy>` to run them on a copy of the real ledger.
- **Early exit.** Pinned by `TestReverseEarlyExitReadsOnlyTheTail`.

## Findings

- A ship runs 3 to 7 whole-file passes. After slice 1 the audit lookups read the tail until the run row is found.
- Not yet migrated (later slices): `core.latestAuditEntry` (needs one path for production and fakes, and a decision to skip only an unterminated last line and to error with a byte offset on any other malformed line), `dossier`, `cyclehealth`, `phasecoherence`, `ledgerverify`, `releasepreflight`, `cmd/evolve` composition wiring. `Forward` lands with its first caller. Unifying the malformed-line rule across readers is a separate decision.
