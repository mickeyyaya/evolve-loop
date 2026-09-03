# Worktree fence for read-only phases + explanation-review tolerance — Design Spec

**Date:** 2026-09-03 · **Status:** implemented (ADR-0097) · **Supersedes:** the
"build-exit deterministic floor" draft for research proposal R3 · **Evidence:**
[incident](../../incidents/2026-09-03-auditor-mutates-the-worktree.md).

## What the evidence said
The FAIL census for cycles 1596–1606 (eleven consecutive FAILs) was: 2 substantive auditor
findings, 3 triage artifact-timeouts, 3 deterministic explanation-digest mismatches, and 5
explanation-review FORMAT rejections (one cycle carried both). The digest mismatches were
reproduced forensically: the auditor rewrote the builder's material files during the audit.
The deterministic build handoff floor the draft proposed already exists
(`core/build_floor_reviewer.go` in the E2 reviewer chain, with a correction ladder); the
draft was retired.

## Design
- **Single source with projection.** Write permission stays the one predicate
  `Orchestrator.worktreePhase`; the fence reads its complement through
  `PhaseRequest.WorktreeReadOnly`. The parser tolerance lives once in `reportdoc`; both phase
  gates (audit, retro) and `explanationdocs.ValidateReviewedHandoff` project it.
- **Strategy at the runner seam.** The fence wraps the dispatch attempts in `phases/runner`
  (shared by every LLM phase, including spec-driven user phases), before the classify hooks.
- **Fail loud, fail open.** Every restored path is a diagnostic on the response; an
  untakeable fence is a diagnostic; nothing is silently kept or dropped; verdicts are never
  changed by the fence.
- **TDD.** Fence tests on real temp repositories; runner tests through `Runner.Run` with a
  mutating bridge; core proof through `RunCycle` with a read-only remediable gate
  (mutation-checked); parser tests on the three real sections verbatim.
- **Clean-code limits.** New package `treefence` (one file < 200 lines, functions < 50
  lines, godoc on exports, `.apicover-enforce` + `apicover_named_test.go`); no `*_gate.go` /
  `*guard*` filenames.

## Non-goals
Routing a missing review section through the contract correction ladder; making the OS
sandbox mandatory; changing the grounding rule of the review.
