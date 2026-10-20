# ADR-0149: a test-writing phase that states FAIL on its own work is retracted to its dispatch tree

- **Status:** Proposed (2026-10-20). The code is in the same PR. The merge of that PR accepts this ADR.
- **Inbox item:** `phase-authored-red-tests-poison-egps` (filed 2026-08-16 after the cycle-1493 halt).
- **Builds on:** [ADR-0097](0097-read-only-phase-worktree-fence.md), whose content fence (`internal/treefence`) this decision reuses.
- **Preserves:** [ADR-0088](0088-audit-chain-of-reasoning.md) and the adversarial-audit separation. The EGPS floor (`red_count == 0` to ship) does not change.
- **Specs:** [internal-treefence](../packages/internal-treefence.md), [internal-phases-runner](../packages/internal-phases-runner.md), [internal-phases-specrunner](../packages/internal-phases-specrunner.md), [internal-phasespec](../packages/internal-phasespec.md).

## Context

The item states the problem: "A pipeline phase's self-FAILed test artifact left in go/acs/cycle-N red-counts an innocent green cycle". In cycle 1493 the audit marked every criterion PASS and the suite had 11 greens. The one red was `TestAmplified_Cycle1493PredicatesAreDiscovered`. The test-amplification phase wrote it into `go/acs/cycle1493/` with a `$`-anchored `-run` pattern, so it was red on every tree. The phase stated FAIL on its own work, but the test stayed, and the EGPS red failed the cycle.

Today only a persona rule stops that shape in the next cycle.

The item asks for three properties. A self-FAILed phase test must not red a green cycle. A phase test that catches a real defect must still block ship. A builder red must not get a path out through the phase.

## Findings

**F1** `acssuite.Run` counts every red result in the cycle package as ship-blocking (`go/internal/acssuite/acssuite.go:118-119`, `v.ShipEligible = v.RedCount == 0`). The only demotion is the scope lint for out-of-scope meta-predicates (`go/internal/acssuite/scopelint.go:154-206`). No result carries an author.
*Implication:* a fix inside the suite needs an authorship record that the suite does not have.

**F2** The host never reads the verdict that test-amplification states. Its `phase.json` has no `verdict_from_sentinel`, so the structural classifier returns PASS (`go/internal/phases/specrunner/specrunner.go:98-139`). Cycle 1971 shows it: the report states WARN for a real assertion failure (`.evolve/runs/cycle-1971/test-amplification-report.md:41`), and `phase-timing.json` records the phase as PASS.
*Implication:* the stated verdict is a free signal that no code uses today.

**F3** The persona already gives the stated verdict a meaning. A real assertion failure stays on disk and the report says so (cycle 1971, "kept on disk"). A test that the phase itself broke is retracted and logged (`agents/evolve-test-amplification.md:40`).
*Implication:* FAIL is the phase saying "my own work is wrong", and WARN is "I found a defect".

**F4** `internal/treefence` already snapshots a worktree as a git tree object and restores it after a read-only phase (ADR-0097). The snapshot is the host's own record, taken before the agent starts.
*Implication:* the host can know exactly which paths a phase wrote, without the phase's own report.

**F5** The audit has a precedent for this shape. `probe_quarantine.go` keeps the auditor's own probe tests under `<workspace>/audit-probes/` and removes them from the tree before the EGPS suite runs (`go/internal/phases/audit/probe_quarantine.go:26-41`).
*Implication:* "move the writer's own files aside, keep them as evidence" is an accepted pattern here.

## Options

| Option | How it works | Meets acceptance 1 (cycle-1493 shape) | Meets acceptance 2 (a real catch blocks) | Laundering risk | Cost |
|---|---|---|---|---|---|
| A. Retract on a stated FAIL | The runner snapshots the worktree before a phase that opts in. When the phase states FAIL, the runner restores every path the phase wrote and keeps the phase's copies in the workspace. | Yes | Yes: WARN and PASS keep the files | None: the result is the tree before the phase (D3) | One snapshot and one diff for each dispatch of the phase |
| B. Authorship partition | Phase tests live in a separate build-tagged package. A WARN-level gate counts them, not the EGPS floor. | Yes | No: a real catch loses its power to block ship | High: a path or a tag is the only authorship record, and the builder can write to it | A new package convention and a new gate |
| C. Status quo | The persona rule, and the infra-systemic classifier after the burn. | No: it costs a full lane and a batch halt each time | Yes | None | One burned lane for each occurrence |

Option B fails acceptance 2 by construction. Option C fails acceptance 1. Option A meets all three, so it wins.

## Decision

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | A phase opts in with `"retract_on_fail": true` in its `phase.json`. `test-amplification` opts in. | F3: only a test-writing phase gives FAIL the meaning "my work is wrong". | Retract for every source writer: a build FAIL goes to a repair round on the same tree. A revert erases the work that the repair needs. |
| D2 | The trigger is the verdict that the phase states in its own report sentinel (`phasecontract.ParseVerdictSentinel`), not the routed verdict. | F2: the routed verdict of this phase is structural and is PASS. | Turn on `verdict_from_sentinel: enforce`: that changes the routing of the cycle, which is a separate decision. |
| D3 | The runner snapshots the whole worktree before the dispatch (`treefence.Take`). On a stated FAIL, it restores the snapshot (`Snapshot.Retract`). Added paths go, and changed or deleted paths come back from the snapshot. | F4: the snapshot is the host's record. The phase cannot change it. | Retract the paths that the report names: the report is the phase's self-report, so it is not proof of authorship. |
| D4 | Before the restore, the runner copies each path the phase wrote to `<workspace>/<phase>-retracted/`. When it cannot keep a copy, it restores nothing. | F5: the evidence must outlive the retraction. A failure leaves the cycle as it is today. | Restore without a copy: an unlogged deletion hides what the phase tried. |
| D5 | A retraction never changes the verdict. It adds one `retraction:` diagnostic to the response and one WARN line to the log. | The fence of ADR-0097 follows the same rule: the fence reports, and the gates decide. | Change the verdict to WARN or SKIPPED: the verdict then hides the retraction. |

## Why a builder red cannot leave through this path

The retraction can only make the tree equal to the tree before the phase. That tree has every builder and TDD file exactly as they were. Three cases cover each move of an attacker.

1. **The phase deletes or edits a builder red, then states FAIL.** The restore writes the builder red back byte for byte. Pinned by `TestRun_AStatedFailRetractsThePhaseWritesAndCannotLaunderABuilderRed`.
2. **The builder plants text in `build-report.md` that makes the phase state FAIL.** The result is the tree before the phase, with the builder's reds in it. At worst the cycle has no amplified tests. That equals a cycle without the phase.
3. **The phase states FAIL to hide a real catch.** The result is again a cycle without the amplified tests. A different agent on a different CLI still audits the build (ADR-0088).

So the retraction can only remove the phase's own contribution. It cannot remove a red that existed before the phase ran.

## Consequences

- The cycle-1493 shape no longer reaches the EGPS suite. The phase's own test goes, and the cycle is judged on the work of the builder.
- A real catch still blocks. A phase that states WARN or PASS keeps its files, and a red test in `go/acs/cycle<N>/` still counts. Pinned by `TestRun_AStatedWarnKeepsTheAmplifiedTestThatCaughtADefect`.
- Each dispatch of the phase costs one extra `git write-tree` before the agent and one after it.
- The audit sees the retraction in the response diagnostics, and the operator finds the phase's files in the workspace.

## Limits

- A phase that deletes a TDD predicate and states PASS or WARN is not retracted. That gap exists today and does not depend on this decision. The audit and the ship manifest gate (`go/internal/phases/ship/manifest.go`) are the controls for it.
- The trigger depends on the phase's honesty about FAIL. A phase that leaves a broken test and states WARN still reds the cycle, as today. The persona states the rule (`agents/evolve-test-amplification.md`).
- The retraction does not touch paths that `.gitignore` covers. The snapshot does not see them (ADR-0097).
- The retraction runs after the judge of the phase itself. The EGPS suite runs later, in the audit, so it sees the restored tree. A host effect of the phase sees the phase's writes.
- A restore that fails part way names the paths that it restored. The other paths stay as the phase left them.

## What changes this decision

A stated FAIL can hide a real defect that the audit then misses. If a cycle shows that, the trigger moves to a host check of the red output. If a second test-writing phase needs a different rule, the opt-in becomes a value with more than two states.

## Wiring proof

- `TestUserSpecs_TestAmplificationRetractsOnFail` pins the opt-in in the tracked `phase.json`.
- `TestNew_ThreadsRetractOnFailFromTheSpec` pins that `specrunner.New`, the one constructor of every catalog phase, passes the opt-in to the runner.
- `TestRun_AStatedFailRetractsThePhaseWritesAndCannotLaunderABuilderRed` runs the shared runner over a git worktree with the cycle-1493 shape.
