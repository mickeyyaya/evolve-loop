# Build Explanation — Cycle 2495

## Build Binding
- Cycle: 2495
- Base SHA: 5db58d6b91d602b28eea4674bfd2dcbb3d18e095

## Summary
A review report whose only verdict line sits in a code fence now reads as the verdict that it states. This cycle resumes the cycle 2473 fix, which failed at ship with a moved HEAD.

## Rationale
The sentinel reader and the visible-line reader both skip fenced text. A fenced verdict gave no stated verdict, so the run was FAIL. A last read removes the fence marker lines and reads the verdict again. The severity count reads the same text as the verdict, so a fenced HIGH finding next to a fenced PASS stays FAIL.

## Changed Areas
- `go/internal/cli/agentcmd/verdict.go` — `statedVerdict` falls back to the report without fence marker lines, and `reportVerdict` counts severities on the text that gave the verdict.
- `go/internal/cli/agentcmd/agent_verdict_test.go` — the fenced-verdict tests state the new behavior.
- `go/internal/cli/agentcmd/agent_verdict_fence_test.go` — tests through the real agent run entry point.
- `go/acs/cycle2495/predicates_test.go` — the four cycle predicates over the named tests.
- `.evolve/evals/review-verdict-in-a-code-fence-reads-as-fail.md` — the eval graders of the task.
- `docs/private/research/archived-2026-11-18/superseded-predicate-packages/cycle2473/predicates_test.go` — the archived cycle 2473 predicates.
- `docs/private/research/archived-2026-11-18/unshipped-build-explanations/cycle-2473-01m7ndga4w4ckdsds6k98b6ehp.md` — the archived cycle 2473 explanation.

## Design Decisions
The fallback comes last, so a visible or sentinel verdict always wins. A fenced example finding next to a visible verdict stays uncounted.

## Verification
The agentcmd package passes with `go test -count=1`. The ACS suite for cycle 2495 reports red=0.

## Compatibility
No signature changes outside the package. A report with no verdict still fails.

## Limitations
A fenced example line counts as the verdict when no other verdict exists.
