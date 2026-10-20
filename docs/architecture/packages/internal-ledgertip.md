# internal/ledgertip

> Owner of the format: [internal-adapters-ledger.md](internal-adapters-ledger.md). The second writer: [internal-subagent-subagentrun.md](internal-subagent-subagentrun.md).

## Purpose

`internal/ledgertip` holds the one encoding of `.evolve/ledger.tip`. Two packages write the tip, and `adapters/ledger` reads it back during `Verify`. All three use `Format`, so the file has one encoding.

The package is a leaf. It imports the standard library only.

## Design

- `Format(seq, lineSHA)` returns `<seq>:<lineSHA>` with no newline.
- `adapters/ledger` uses it in `appendLineAndReplaceTip` and in `compareTip`. `compareTip` compares the tip byte for byte, so a trailing newline is a tip mismatch.
- `subagentrun.AppendChainedLine` uses it for the tip of the subagent ledger writers.
- `cyclesimulator.writeLedgerTip` uses it for the tip of a simulated ledger.
- A reader of the tip trims white space (`subagentrun.tipChainLink`), so a tip that an old writer left with a newline still chains. `Verify` refuses that tip until the next append writes the tip again.

## Findings

- Before 2026-10-20 the subagent writer wrote `<seq>:<sha>\n`. When it made the last append, `Verify` reported `tip mismatch` (6 of 30 runs of the interleaved test). Pinned by `TestAppendLedger_AfterAFileLedgerAppendTheChainVerifies` and `TestFormat_IsSeqColonHashWithNoNewline`.
