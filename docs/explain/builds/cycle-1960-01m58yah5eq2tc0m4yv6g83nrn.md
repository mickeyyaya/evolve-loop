# Build Explanation — Cycle 1960

## Build Binding
- Cycle: 1960
- Base SHA: 93867b28385e91987d10db73c73b9892fdd011a2

## Summary
The fleet landing plan now records why the S1 fast-forward pre-check stays dropped, and a solution set scores the three ways to bring it back.

## Rationale
The core resets a fleet lane to its base after build, so ship sees HEAD equal to base with staged edits. Recovery then classifies HEAD against main and discards a lane at HEAD equal to base (E3), so a read-only probe saves a scan only if a staged-content classifier separates superseded lanes from rebase-needed lanes (A1, not measured). A probe that stages the lane or consumes the inbox runs before the audit-binding check and bypasses the post-audit tamper guard. Option 1 keeps S1 dropped and moves the saving to S5 and S6. Rejected: Option 2, because it saves a scan only with the recovery fix, which is a console change in a protected file. Rejected: Option 3, because its rebase path is the same recovery, so it needs the same classifier plus a new core seam, and costs more than one console cycle.

## Changed Areas
- `solutions/landing-s1-readonly-redesign-lane/assumptions-and-evidence.md` — lists the assumptions A and the evidence E that the options cite, each with a source.
- `solutions/landing-s1-readonly-redesign-lane/options/1-keep-s1-dropped-record-why.md` — the option that keeps S1 dropped and records the reasons in the plan.
- `solutions/landing-s1-readonly-redesign-lane/options/2-readonly-probe-plus-recovery-fix.md` — the option of a read-only probe with a change to recovery; it scores lower because it saves a scan only with a recovery fix that stays with the console.
- `solutions/landing-s1-readonly-redesign-lane/options/3-prescreen-in-core-before-ship.md` — the option of a pre-screen in core; it scores lower because its rebase path needs the same classifier fix and a new core seam.
- `solutions/landing-s1-readonly-redesign-lane/recommendation.md` — compares the three options, names Option 1 as the winner and states what evidence flips the choice.

The plan file `docs/plans/fleet-landing-at-scale-2026-10.md` also changes: the S1 row, the rollout step 2 and the stage 1 row now say that S1 stays dropped.

## Design Decisions
The plan records the decision in its own S1 row, so a later reader of the plan finds the reason without the solution set.

## Verification
`evolve solution check solutions/landing-s1-readonly-redesign-lane` prints OK. `evolve docs ste-lint` reports 0 findings on the plan. No Go file changes, so no Go test runs.

## Compatibility
No code, flag or policy key changes. The S5 and S6 rows of the plan are unchanged.

## Limitations
The test for an edits-only diverged lane (acceptance 2) does not exist, because acceptance 3 is met by the recorded reasons. The console item that classifies a lane on staged content remains open.
