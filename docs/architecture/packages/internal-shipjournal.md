# internal/shipjournal

> Spec of record: [llm-isolation.md](../llm-isolation.md) §19.2. Writer: [internal-phases-ship.md](internal-phases-ship.md). Reader: [internal-selfheal.md](internal-selfheal.md).

## Purpose

`internal/shipjournal` is the one home of the format of `.evolve/ship-journal.jsonl`. The ship phase writes one entry for each landing. The incident detector reads the shipped cycles, and the catalog validator reads the loop landings for `revert-last-landing`.

The architecture review of X14 (2026-10-27) found two copies of the entry type and of the class `cycle`. A change of the writer then makes the validator refuse each revert with a reason that blames the agent. One leaf package removes the second copy.

## Design

- **The entry.** `Entry` has `sha`, `class`, `ts` and `cycle`. `Stamped` sets `ts` in `TimeFormat` (RFC 3339, UTC), and `Time` parses it back. An entry with no `ts` has no time.
- **The names.** `FileName` is the file name, and `Path(evolveDir)` is its path. `ClassCycle` is the class of a loop landing, and `ship.ClassCycle` is this value.
- **A leaf.** The package imports only the standard library. So `selfheal` reads the journal and does not import the ship phase and its dependencies.

## Invariants

- The writer stamp is the reader time: `TestEntry_TheStampOfTheWriterIsTheTimeOfTheReader`.
- The package does no I/O. The ship phase appends, and each reader reads the file itself.

## Findings

- None.
