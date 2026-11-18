# Build Explanation — Cycle 2486

## Build Binding
- Cycle: 2486
- Base SHA: 6b129b6dcf47551aaa0b840cabaaffae839b37b6

## Summary
A resumed run now keeps the retry ladder state that its correction loop raises. The next phase and the quota-pause checkpoint of the same run start from the raised rung.

## Rationale
The resumed correction loop raised the ladder on a copy of the cycle state. The copy was lost when the loop returned. The resume loop file is a protected path, so the caller cannot receive the state. A package-level store, keyed by Orchestrator and cycle, now keeps the final ladder of each resumed cycle. The dispatch policy and the quota pause adopt it.

## Changed Areas
- `go/internal/core/resume.go` — the corrector defers one call that keeps its final ladder in the package-level store, also on an error.
- `go/internal/core/resume_ladder_keep.go` — the guarded per-cycle store and the `adoptResumedLadder` method. A dispatch adopts a kept ladder once; later ladder progress of the same dispatch stays.
- `go/internal/core/resume_policy.go` — the dispatch policy adopts the kept ladder before it picks the rung of a phase.
- `go/internal/core/quota_pause.go` — the quota pause adopts the kept ladder before it writes the checkpoint.
- `go/internal/core/parallel_default_test.go` — the new tests that swap the quota checkpointer or write the ladder store are listed as serial.
- `go/internal/core/resume_ladder_progress_test.go` — tests that ladder progress made after the adoption survives the next dispatch and the quota pause.
- `go/internal/core/resume_ladder_state_test.go` — tests for the recorded rung, the raised rung reaching the next phase, the quota pause and the shadow stage.
- `go/acs/cycle2486/predicates_test.go` — the predicates of this task.

## Design Decisions
A package-level store keyed by Orchestrator and cycle keeps the signature of `correct` unchanged and leaves the protected resume loop untouched. A copy of the ladder is stored, so no caller shares a mutable pointer. The store records which dispatch adopted the snapshot, so the same dispatch never overwrites its own later progress. A consume-on-adopt design was rejected: the resume loop is protected, so a phase without a correction can swallow the snapshot.

## Verification
`go test ./internal/core` passes. The ACS suite for cycle 2486 reports green=266 red=0.

## Compatibility
The diff adds no callback and no new signature. A cycle without a kept ladder behaves as before. A shadow ladder still leaves each dispatch unchanged.

## Limitations
The fix covers the resumed correction loop only. The sequential correction loop already owned its state. The ladder progress of a dispatch without a correction does not reach the next dispatch. The store is never pruned, so each resumed cycle leaves one entry for the life of the process.
