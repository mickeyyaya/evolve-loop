# internal/binaryguard

## Purpose

`internal/binaryguard` rejects large compiled executables from a change set before they reach a commit. `internal/commitgate` calls it at the commit chokepoint. It is the repo-wide backstop: no compiled artifact of any kind can be added again without a refusal.

## Design

- **Two conditions, both required.** A file is an offender only when it is larger than the threshold and it begins with a known executable magic signature. Large text and data assets (fixtures, manifests) never trip the guard.
- **The threshold is 1 MiB** (`DefaultThresholdBytes`). It is far above any source file and far below a real compiled binary.
- **The signature table is named.** `magicSignatures` lists `elfMagic` (Linux), the Mach-O magics (32-bit, 64-bit, both reversed byte orders, and the universal "fat" binary) and `peMagic` (Windows `MZ`).
- **`Scan` reads only the widest signature.** `widestMagicLength` is 4 bytes (ELF and Mach-O; PE needs 2). So `Scan` never loads a multi-megabyte binary to classify it.
- **Offenders come back sorted by path**, so the refusal text is stable.

## Invariants

- **A deletion is not an offender.** A path that does not exist is skipped, because a change set lists deletions. A real read error (permission and so on) is returned, never skipped.
- **Both conditions are pinned.** `TestScan_RejectsSyntheticLargeExecutable` flags a large executable, and flags neither a large non-executable nor a small executable.
- **Every known format is rejected.** `TestScan_EveryKnownExecutableFormatIsRejectedWhenLarge` covers each named signature through `Scan`.
- **A read error fails the scan.** `TestScan_ARealReadErrorIsReturnedNotSkipped` pins it.
- **A missing file is skipped.** `TestScan_SkipsMissingFiles` pins it.
- **The offender order is stable.** `TestScan_OffendersComeBackSortedByPathWhateverTheInputOrder` pins it.
- **The head read is bounded.** No test pins the size of the read, so this property is unpinned.

## Findings

- The incident that created this guard is in the [comment history archive](../../history/code-comments/internal-binaryguard.md).
