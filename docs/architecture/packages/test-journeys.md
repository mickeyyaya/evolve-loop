# test/journeys

> Spec: [regression proof and supervision](../regression-proof-and-supervision.md), section 3. The model is in [internal/journeys](internal-journeys.md).

## Purpose

`test/journeys` holds `journeys.json`, the journey registry, and the guard test that keeps it true. It has no production code. It has only the package document.

## Design

- **The seed rows are fixes of 2026-11.** Each row names a test that exists today. The rows are `#1286` (cloud launch), `#1280` (concurrent resume), `F3.4` (a dead owner), `F3.8` (one lane for each family) and `#1250` (fenced verdict). The journey of F3.6 has no test yet, so it has no row. A row is never added for a test that does not exist.
- **`registryViolations` is the guard.** It loads the file. It lists each active row whose test does not resolve. It lists each active row whose test file is not on the protected surface (`guards.IsProtectedSurface`).
- **The guard has four cases.** The committed file has no violation. A row that names a missing test fails. An active row that loses its test fails, and a retired row does not. A row whose test file is not protected fails.

## Invariants

- **A row goes away only with a reason.** The guard test cannot see history. The commit gate can (`refuseRemovedJourneyRows` in [internal/commitgate](internal-commitgate.md)). It reads the file at `HEAD` and on disk, and it refuses a change that removes a row or empties a `retired` reason.
- **The file, this guard and each named test file are on the protected surface.** A lane cannot delete or weaken them. The console adds a row. A lane proposes a row as a held proposal.
- **A named test sits in a file that the default build compiles.** A test behind a build tag does not count.
