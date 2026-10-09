# Build Explanation — Cycle 1852

## Build Binding
- Cycle: 1852
- Base SHA: f8fdcdd565c87ac4972b14419eb7c27333182467

## Summary
The landing queue resume now settles local `main`. For a stranded head whose intent commit `origin` holds, the resume fast-forwards local `main` to `origin` before it clears the flag. When `origin` lacks the commit, local `main` does not move and the record becomes `parked`. Plan row Q17 names the red test for the new step.

## Rationale
Before this change, the resume made the record `landed` and cleared the flag, but local `main` stayed behind `origin`. Each other candidate then failed its fast-forward check against the old `main` and composed again (PR #827 round-3 review). Settling `main` inside the resume, before the flag clears, removes that wasted composition for every candidate.

## Changed Areas
- `docs/architecture/fleet-landing-queue.md` — the resume section adds the settle step (fast-forward local `main` to `origin` before the flag clears), states that local `main` does not move when `origin` lacks the commit, and adds a refusal when local `main` is not an ancestor of `origin`.
- `docs/plans/concurrent-cycle-landing-2026-10.md` — plan row Q17 adds `TestLandingQueueCLI_ResumeSettlesLocalMainToOrigin` and keeps its seven existing tests.
- `docs/explain/builds/cycle-1852-01m4ftdg7tcycpvwteyaq6gx8w.md` — this explanation document.

## Design Decisions
The resume does the fast-forward itself, instead of waiting for the stranded cycle's `Landing.Resume`. A stranded cycle may never resume, and the other candidates run as soon as the flag clears. The ship push invariant (ship moves `main` only after its push lands) makes local `main` an ancestor of `origin`, so the fast-forward is always possible. If that invariant is broken, the resume refuses and the flag stays set, so no candidate composes on a diverged `main`.

## Verification
`go test -tags acs -count=1 ./acs/cycle1852` passes all five predicates: the settle sentence, the origin-lacks branch, the Q17 test row, and the ship push invariant.

## Compatibility
The landing queue is not yet implemented; this is a spec and plan amendment. No verb, flag, or state field changes.

## Limitations
The Q17 test is named but not written; it lands with the Q17 implementation.
