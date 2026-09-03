# ADR-0097 — read-only phases are fenced: the tree a phase hands downstream is the tree it was given

- **Status:** Accepted (2026-09-03).
- **Driving evidence:** [2026-09-03-auditor-mutates-the-worktree.md](../../incidents/2026-09-03-auditor-mutates-the-worktree.md)
  — cycles 1603/1604/1605 failed the deterministic build-explanation gate because the
  auditor's mutation probes rewrote the builder's material files during the audit; five
  further FAILs in the same eleven were the explanation-review parser rejecting the shape
  a careful reviewer writes. The [ship-rate research](../../research/ship-rate-harness-reliability-2026-09-02.md)
  proposal R3 ("a deterministic floor before the expensive judge") was re-targeted by this
  evidence: the build handoff floor already exists (`core/build_floor_reviewer.go`); the
  deterministic checks that were failing were failing on a tree the judge had altered.
- **Related:** [ADR-0092](0092-audit-repair-loop.md) (repair rounds), [ADR-0096](0096-repair-rounds-escalate-and-carry-findings.md),
  the explanation-documentation contract (`internal/explanationdocs`), CB.1 (every phase runs
  cwd=worktree; write permission is the `worktreePhase` axis).

## Problem

Write permission for a phase is one predicate — `Orchestrator.worktreePhase`: built-in
tdd/build, or a spec declaring `writes_source` — consulted by the role gate and by the
main-tree tree-diff guard. Nothing enforced it on the worktree. An agent with a shell can
`cp`, `sed -i`, `git checkout --` or `go build` into the tree it was told to inspect, and the
verifications that decide a ship (the explanation digest at audit, again at ship, the ACS
predicates) read whatever tree the last agent left. The profile's `sandbox.read_only_repo`
would have caught it, but the OS sandbox is `auto` and is not applied when nested — the wave
ran `sandbox=false`.

## Decision

1. **`core.PhaseRequest.WorktreeReadOnly`** is derived at every dispatch site from
   `!worktreePhase(phase)` — one predicate, both readings — and cleared on a remediation
   builder fix (which inherits the fenced gate's request as its template).
2. **`internal/treefence`** records the worktree as a git tree object through a throwaway
   index (`Take`) and puts it back (`Restore`): adds removed, modified/deleted/retyped paths
   written back with their mode; ignore rules respected, the real index untouched. The set
   it protects is exactly the set the explanation digest hashes.
3. **`phases/runner`** and **`phases/retro`** (which calls the bridge itself) each hold a
   `treefence.Fence` around their launch: taken before the first dispatch attempt, closed after
   the last, before the classify hooks — so the audit's explanation binding, the retro's
   verification and the ship all judge the builder's tree. A write is a `warning` diagnostic
   on the phase response that names the restored paths; an untakeable or failed fence WARNs
   and the phase proceeds (fail-open, loudly — the downstream gates stay armed and now name
   the cause). The fence never changes a verdict.
4. **Declared writers.** `bug-reproduction` and `test-amplification` author `.go` into the
   worktree and now declare `writes_source: true` (both were undeclared; the fence would have
   reverted their deliverables) — pinned by `phasespec.TestUserSpecs_SourceWritersDeclareWritesSource`.
   The `evolve campaign` study dispatch derives the flag from its phase config the same way.
5. **`reportdoc.ReviewFields`** parses the section once for both phase gates and reads it as reviewers write it: `Evidence`
   is list-valued, values lose surrounding backticks, `path:23-48` / `path:12:3` /
   `path#L7-L9` are citations of the first line, and `EvidenceOrBody` falls back to the
   section prose when no `Evidence` field exists. The substance rule (every reference cited
   at a concrete line; `Status` must be one of two words) is unchanged.

## Alternatives considered

- **Make the OS sandbox mandatory for read-only phases.** Right layer for defence in depth,
  wrong layer for the invariant: it depends on host and nesting, and a sandbox that silently
  did not apply is precisely what happened. Kept as defence in depth.
- **Verify the explanation digest before the audit dispatch only.** Moves the failure to the
  ship gate (which re-verifies) and lets a mutated tree ship if it did not. The tree must be
  restored, not just noticed earlier.
- **A native `build-floor` phase between build and audit** (the research's R3 as first
  drafted). The deterministic build handoff floor already exists in the E2 reviewer chain
  with a correction ladder, and the FAIL census pointed at audit-time mutation and the review
  section's shape — not at gates that fired too late. Superseded by this ADR.
- **Loosen the review gate to prose-only.** Rejected: the grounding rule (path:line for every
  material path) is what stops a reviewer from repeating path names; only the SHAPE is
  forgiven.

## Consequences

- Any phase without `writes_source` that legitimately writes into the worktree will have
  its writes undone and reported on the next run — loudly, with paths. That is the
  declaration being enforced, and the diagnostic is the signal to declare `writes_source`.
- Two `git add -A` passes into a temp index per read-only dispatch (sub-second on this
  tree); dangling tree objects are garbage-collected.
- `internal/treefence` is a new enforced package (`.apicover-enforce`,
  `apicover_named_test.go`); `WorktreeReadOnly` is a wire field (`worktree_read_only`).

## Wiring proofs
`internal/core/cyclerun_remediate_test.go::TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot`
(mutation-checked: red without the remediation clearing line),
`internal/phases/runner/worktree_fence_test.go` (three cases through `Runner.Run` with a
worktree-mutating bridge), `internal/treefence/fence_test.go`,
`internal/reportdoc/review_fields_test.go` (the three real sections, verbatim, under
`testdata/`), `internal/phases/audit/audit_test.go::TestValidateExplanationReview_ReadsTheSectionAsAuditorsWriteIt`,
`internal/phases/retro/retro_test.go::TestValidateExplanationReview_ListValuedEvidence`.
