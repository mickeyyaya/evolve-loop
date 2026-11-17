# Build Explanation — Cycle 2484

## Build Binding
- Cycle: 2484
- Base SHA: 295d242043582038be26009b7a4bc164683c7287

## Summary
The agy model notes gain one dated bullet. It records the review of the agy CLI changelog page and the verdict that no persona changes.

## Rationale
The practice item asks for a reading of the changelog page as data and a proposal under ADR-0133. One appended line meets both asks. The older lines stay unchanged.

## Changed Areas
- `skills/model-notes-agy/COMPACT.md` — adds the Cycle 2484 bullet for App v2.33.0 and CLI 1.5.9. The bullet names the SSH host connections, the /swarm command and the multiline prompt fix.
- `docs/explain/builds/cycle-2484-01m7nszbd9wj1snx2xr14rnzr3.md` — this explanation of the build.

## Design Decisions
The bullet follows the shape of the Cycle 2454 and Cycle 2458 bullets. It does not copy changelog text. It keeps the /swarm command unused, because no LLM starts or directs another.

## Verification
The eight predicates of cycle 2484 pass. The strict sentence lint reports zero findings for the notes.

## Compatibility
No persona file under `agents/` changes. No code changes.

## Limitations
The bullet records a verdict only. It does not change any prompt.
