# internal/cyclefacts

> Spec of record: [debug-cli-2026-11](../../plans/debug-cli-2026-11.md) decision D5. ADR: [ADR-0162](../adr/0162-the-console-reads-through-a-tested-read-only-debug-family-and-writes-no-scratch-scripts.md).

## What it does

This package holds one read for each fact file of a cycle. `ReadDecision`, `ReadDisposition`, `ReadFailReason` and `ReadDigestRecord` each return a `Read` with a `State`. So a caller can tell an absent file from a broken file.

## Who calls it

The debug family and `dashboard.readFailure` call these reads, so each fact file has one reader.

## Limits

- The package stays small, because `profiles` cannot import it if it grows (plan D5).
- It carries no package doc comment, because the hard rule forbids comments. It is listed in `.docgo-allow-missing` for that reason.
