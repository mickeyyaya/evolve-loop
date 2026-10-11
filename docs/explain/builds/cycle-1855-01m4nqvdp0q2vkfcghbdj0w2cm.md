# Build Explanation — Cycle 1855

## Build Binding
- Cycle: 1855
- Base SHA: ddd10f569d57b1b6b3c52cc66811ed3010eea0fc

## Summary
Removes the write-only FailReasons field from VerdictInputs in internal/coherence and its assignment in internal/core/failure_dossier.go, and verifies sentinel-less audit report coherence behavior through unit tests.

## Rationale
VerdictInputs.FailReasons was populated by failure_dossier.go but never read by CheckVerdictCoherence or any other caller, creating dead state in verdict validation. Removing this unread field eliminates write-only coupling between core and coherence.

## Changed Areas
- `go/internal/coherence/coherence.go` — removes unread FailReasons field from VerdictInputs struct.
- `go/internal/coherence/coherence_test.go` — adds unit test TestReadCycleVerdicts_AuditReportWithoutSentinel pinning sentinel-less audit report behavior.
- `go/internal/core/failure_dossier.go` — removes FailReasons assignment from VerdictInputs construction in buildFailureDossier.

## Design Decisions
Removing the field entirely rather than deprecating or reading it preserves minimal struct definition and avoids introducing unused inspection logic into CheckVerdictCoherence.

## Verification
Verified through cycle 1855 ACS predicate suite (TestC1855_001 to TestC1855_004). Also confirmed by internal/coherence and internal/core unit tests.

## Compatibility
VerdictInputs is internal to packages core and coherence. Removing the unused field does not affect public APIs or external behavior.

## Limitations
This change only affects VerdictInputs in coherence; CycleState and other failure diagnostic fields in cyclestate remain unchanged.
