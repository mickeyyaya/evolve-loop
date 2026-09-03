# 2026-09-03 — the auditor rewrites the builder's tree; the explanation review rejects what auditors write

## Impact
Cycles 1603, 1604 and 1605 (wave-20260902, lane cd3ae73e) each FAILed the
deterministic explanation gate — `build-explanation.json diff SHA256 does not
match the base-bound Build content` — after audit rounds whose auditor
narrative was PASS or a substantive NEEDS_CORRECTION. No LLM round could repair
the defect because the auditor itself had introduced it. In the same eleven-cycle
window (1596–1606) five more FAILs were the audit report's
`## Explanation Documentation` section being rejected on FORMAT: missing section
(1601, 1603), `report has duplicate evidence fields` (1604), `Evidence must cite
… with path:line evidence` on a line-range citation (1605), `requires concrete
Evidence` when the citations sat under other field names (1606). Together:
7 of 11 consecutive FAILs were the harness rejecting its own pipeline's output,
not the code under audit. Ship rate over the window: 0 / 11.

## Forensics (all evidence verbatim from the runtime plane)
The three worktrees survive as ADR-0076 salvage snapshots, so the diff digest
was recomputed with the exact seal algorithm (`explanationdocs.diffSHA256`):
no single-path exclusion reproduces any sealed digest — tracked CONTENT changed
after the seal. File mtimes (UTC) bracket who changed it:

| cycle | seal (build-explanation.json) | audit window | material paths rewritten |
|---|---|---|---|
| 1603 | 19:07:57 | 19:07:57 – 19:19:31 | `tdd.go` 19:10:34, `digest.go` 19:11:21, plus `go/evolve` built 19:10:46 and `go/internal/core/audit-report.md` written into the tree |
| 1604 | 23:20:52 | 23:20:52 – 23:34:33 (report) | `digest.go` and `tdd.go` both rewritten 23:36:46 — after the report, while the agent was still alive |
| 1605 | 00:49:24 | 00:49:24 – 00:58:13 | the material package directories `decisionsample/` and `policy/` carry post-seal mtimes: probe files created and removed |

The auditor transcripts name the mechanism: cycle 1603 defines
`restore() { cp /tmp/digest.go.bak internal/phaseio/digest.go; }` around an
in-place mutation probe; cycle 1604 logs `wrote reverted tdd.go; removed 253
bytes`. The auditor persona says "inspect but never edit", and the profile
declares `sandbox.read_only_repo: true` — but the wave ran with `sandbox=false`
(the OS sandbox is `auto` and is not applied when nested), so nothing between
the agent's shell and the builder's files existed. The audit's own explanation
check (`audit.go` `explanationCheckDefault` → `explanationdocs.Verify`) runs
AFTER the agent exits, on the polluted tree, and the post-phase `gofmt`
normalise then rewrote the half-restored files a second time (1604's 23:36:46).

The review-section failures are the parser's, not the reviewer's: cycle 1604's
section is a four-line `- Evidence:` list that reproduces the sealed digest
itself; `reportdoc.Fields` treated the repetition as a duplicate. Cycle 1605
cited `sampler.go:23-48` and `citationLine` rejected the `-`. Cycle 1606 wrote
its citations under `- Binding:` and `- Correction still required:` and
backticked the document path.

## Root cause
1. **No fence between a read-only phase and the worktree it is given.** Write
   permission was a predicate (`Orchestrator.worktreePhase`: tdd/build or a
   spec with `writes_source`) consulted by the role gate and the main-tree
   tree-diff guard; nothing enforced it on the WORKTREE, and the deterministic
   verifications that decide a ship (explanation digest at audit, again at
   ship) read whatever tree the last agent left behind.
2. **The explanation-review parser encoded one exact shape** (single Evidence
   line, `path:NN` only, no backticks, exactly those field names) for a section
   written by an LLM reviewer; five of eleven FAILs were shape, not substance.

## Fix (ADR-0097)
- `internal/treefence` — `Take` records the worktree as a git tree object
  through a throwaway index (tracked + untracked, ignore rules respected; the
  real index untouched); `Restore` diffs a fresh tree against it and undoes
  every difference (adds removed, modified/deleted/retyped written back with
  mode). Ignored paths (build outputs, the worktree's `.evolve/` scratch) are
  outside the fence, exactly as they are outside the digest.
- `core.PhaseRequest.WorktreeReadOnly` — derived at every dispatch site
  (`cyclerun_dispatch.go`, `resume.go`, `evaluate_batch.go`) from the ONE
  write-permission predicate; a remediation builder fix never inherits the
  fenced gate's flag.
- `phases/runner` and `phases/retro` (its own bridge call) — `treefence.Fence`
  opened before the first attempt, closed after the last, BEFORE the classify hooks (so the audit's explanation binding judges the
  builder's tree); a write is a `warning` diagnostic on the response naming the
  restored paths (report, dashboard, retro all see it); an untakeable or
  failed fence WARNs and the phase proceeds with the downstream gates armed.
- `bug-reproduction` and `test-amplification` declare `writes_source: true` (both
  were undeclared source writers the fence would have reverted; pinned by
  `phasespec.TestUserSpecs_SourceWritersDeclareWritesSource`).
- `reportdoc.ReviewFields` — one parse for both phase gates; `Evidence` is list-valued (repeats join; `Status` etc. still
  reject a repeat); values lose surrounding backticks; `path:23-48`, `path:12:3`
  and `path#L7-L9` count as citations of the first line; `EvidenceOrBody`
  falls back to the section prose when no `Evidence` field exists. The
  substance rule — every material path cited at a concrete line — is unchanged
  (1604 still owes a `tdd.go` citation; 1605/1606 were NEEDS_CORRECTION on
  merit).

## Regression coverage
| Failure mode | Pinning test |
|---|---|
| in-place mutation, revert, probe file, deleted builder file, chmod undone; ignored paths untouched; idempotent | `internal/treefence/fence_test.go::TestRestore_UndoesEveryKindOfWriteAndLeavesIgnoredAlone` |
| clean phase is a no-op; non-repo / empty path is an error; the real index is untouched | `…::TestRestore_CleanPhaseIsANoOp`, `TestTake_RejectsNonRepoAndEmptyPath`, `TestTake_DoesNotTouchTheRealIndex` |
| file→directory and directory→file type changes undone whatever order git lists them (removals before restores; a removal whose parent was already put back is a no-op); phase-created directories pruned; ambient `GIT_DIR`/`GIT_WORK_TREE` never redirect the fence (go review, 2026-09-03) | `…::TestRestore_UndoesTypeChangesInBothDirections`, `TestRestore_PrunesDirectoriesThePhaseCreated`, `TestTake_IgnoresAmbientGitEnvironment` |
| read-only dispatch restored + reported, verdict unchanged | `internal/phases/runner/worktree_fence_test.go::TestRun_ReadOnlyPhaseWorktreeIsRestoredAndReported` |
| source writers keep their writes, no diagnostic | `…::TestRun_SourceWriterIsNotFenced` |
| untakeable fence warns and proceeds | `…::TestRun_FenceUnavailableWarnsAndProceeds` |
| flag derived from the one predicate on the live loop; remediation fix never fenced (mutation-checked) | `internal/core/cyclerun_remediate_test.go::TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot` |
| 1604's four Evidence lines parse; tdd.go still owed | `internal/reportdoc/review_fields_test.go::TestFields_EvidenceIsListValued_Cycle1604` |
| 1605's range citation accepted; `:col`, `#L` forms; path-only still rejected | `…::TestCitation_AcceptsLineRangesAndColumns_Cycle1605` |
| 1606's backticked values; prose fallback; explicit field wins | `…::TestFields_StripsBackticksAroundValues_Cycle1606`, `TestEvidenceOrBody_FallsBackToTheSectionProse_Cycle1606` |
| single-valued keys still reject repeats | `…::TestFields_SingleValuedKeysStillRejectDuplicates` |
| both phase gates read the tolerant shape; substance rule intact | `internal/phases/audit/audit_test.go::TestValidateExplanationReview_ReadsTheSectionAsAuditorsWriteIt`, `internal/phases/retro/retro_test.go::TestValidateExplanationReview_ListValuedEvidence` |
| retro fences its own bridge launch; resume and evaluate-batch request builders derive the flag; the two source-writing user phases are declared (architecture review, 2026-09-03) | `internal/phases/retro/worktree_fence_test.go::TestRun_ReadOnlyWorktreeIsFencedAroundRetrosOwnLaunch`; `internal/core/cyclerun_remediate_test.go::TestDispatch_ReadOnlyFlagOnResumeAndEvaluateBatchSurfaces`; `internal/phasespec/writes_source_declared_test.go::TestUserSpecs_SourceWritersDeclareWritesSource`; runner proof reads the tree AT classify time |

## Not fixed here (follow-ups, inbox)
- ~~A missing `## Explanation Documentation` section (1601, 1603) is still a
  terminal audit FAIL~~ — FIXED the same day: `phasecontract.ExplanationDocumentation`
  is a conditional audit contract section the deliverable reviewer enforces while the
  explanation contract is active, so the correction ladder re-dispatches the auditor.
- The OS sandbox's `read_only_repo` stays environment-dependent; the fence is
  the policy-level floor, the sandbox remains defence in depth when it applies.
- The auditor still spends budget on in-place mutation probes; the persona's
  `go test -overlay` guidance is the cheaper, tree-preserving form.
