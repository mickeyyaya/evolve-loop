# Plan: the skills-drift gate always grades with the worktree's own generator

- **Status:** implemented in the console lane `cl-skills-drift-peer` (branch `fix/skills-drift-host-stamp`, base `9b47ca6ae`), opened 2026-10-09. Fix round 1 replaced the first design (option B, below) with option A after an architecture review.
- **Incident:** cycle 1841, wave 83. The third skills-drift false red, after cycles 1828 and 1840.
- **Lessons:** `inst-L1841a`. Its consumption record: `.evolve/inbox/consumed/lesson-inst-L1841a-skills-drift-host-stamp.json`.
- **Prior plan:** [skills-drift-worktree-generator-2026-10.md](skills-drift-worktree-generator-2026-10.md) (#821). This plan replaces its trigger (D1, D2, D3) and closes its first two limits.

## 1. Incident

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-10-08 22:18 | The host binary `runtime/go/bin/evolve` is built at `8f46ad383`. | `evolve version` = `evolve dev (8f46ad383ab2, built 2026-10-08T22:18:21Z)` |
| 23:13 | Cycle 1841 (`route-verbs-refuse-mid-wave`) audits PASS on base `8f46ad383`. | `signals.ndjson` seq 137 |
| 23:15 | Ship fails with `GIT_FLEET_REBASE_NEEDED`. Peer cycle 1842 shipped `9b47ca6ae` to main. | `ship-error.json` |
| 23:27 | The identity carry declines: `ORCHESTRATOR_COMPOSED_GATE_DECLINED composed-tree gates not green (apicover)`. The cycle re-audits. | `signals.ndjson` seq 142 |
| 23:45, 23:58, 00:13 | Three audits: the auditor writes PASS each time. The skills-drift gate forces FAIL with 32 stale `commands/*.md`. | `audit-fail-reason.json`, seq 155, 211, 265 |
| 00:13 | `ORCHESTRATOR_AUDIT_REPAIR_DECLINED`: the `code-audit-fail` budget is spent (2/2). | seq 266 |
| 00:17 | The cycle seals FAIL. No `pipeline-defect-*` inbox item is filed. | seq 273 |

`9b47ca6ae` changes `go/internal/skillcheck/commands.go` (`RenderCommandStub`) and regenerates all 32 `commands/*.md`. The diff of cycle 1841 (`9b47ca6ae..925c58d12`) touches 6 paths. It does not touch the generator or `commands/`.

## 2. Root cause

- `skillsDriftCheckDefault` (`go/internal/phases/audit/skillsdrift.go:30` at `9b47ca6ae`) selected the worktree generator only when `core.ChangedWorktreePathsSinceBase(root, req.WorktreeBaseSHA)` contained a path under `go/internal/skillcheck/`.
- After the rebase, `WorktreeBaseSHA` is `9b47ca6ae`. The base contains the generator change, so the lane diff does not. The trigger stays off.
- The gate then called `skillcheck.Check(root)` (`skillsdrift.go:33`) in the orchestrator process. That process has the generator of `8f46ad383` compiled in. It reported the 32 new stubs of main as drift.
- The deeper fault: the trigger predicts "the host generator equals the worktree generator" from a path list. That prediction has three holes:
  - The base can carry the change (cycle 1841).
  - The generator imports `prompts`, `config`, `phasecontract`, `phasespec`, `profiles` and `atomicwrite` (`commands.go:10`, `skillcheck.go:18-23`). A change there changes the output, and no prefix sees it.
  - A host built from a dirty tree has the stamp of HEAD, not of its source.

## 3. Decisions

| # | Decision | Reason |
|---|---|---|
| A1 | When the tree carries the generator source (the directory `go/internal/skillcheck/`), the gate always runs the worktree's own `evolve skills check` (`worktreeSkillsDrift`, `core.WorktreeEvolveInvocation`). Else the host generator grades in-process, as before. | This is what CI does (`TestSkills_NoDrift` runs the tree's own generator). It removes the prediction, so no path list, import closure or build stamp can be wrong. One predicate (`carriesSkillsGenerator`) in one place. |
| A2 | A tree without the generator source (a test fixture, a project that is not evolve-loop) keeps the in-process grade with byte-identical output. | Such a tree has only one generator, the host's, so skew is not possible. A worktree run cannot start there, and it would turn a real FAIL into a skip WARN. |
| A3 | The #821 trigger (`changesSkillsGenerator`) is deleted. The gate does not call `core.ChangedWorktreePathsSinceBase` now. The build-floor reviewer (`changedFloorPaths`) still uses it. | Dead code. |
| A4 | After a clean worktree check, the host check still runs (`generatorDisagreement`). A disagreement is a WARN that names the count. | It tells the operator that the host binary is stale against the tree. It is never a FAIL. |
| A5 | The texts name the true cause: "the worktree generator `evolve skills check` did not run", "the worktree generator and the host generator disagree". | The old texts said "the lane changed the skills generator" and "the lane's generator". After A1 that is often false (cycle 1841 changed no generator). |
| B (rejected) | Option B: keep the lane trigger, and add a diff of `go/internal/skillcheck/` between the host build stamp (`version.Commit`) and the worktree, with a fail-safe for an unknown stamp. This was the first design of this lane. | It closed only the base hole. An import-closure change and a dirty-tree build still gave a false red or green. It needed a stamp seam, an exported git helper, a hex validator and a reason wrapper: about 55 more production lines than A, for less coverage. |

## 4. TDD protocol

Red first. The red runs used an overlay of the gate at `9b47ca6ae` (`$SP/red-round1.txt`).

| Case | Test | Red on the old gate |
|---|---|---|
| (a) The 1841 replay: the base changes the generator, outputs regenerated | `TestSkillsDriftGate_Cycle1841ReplayBaseNewerThanTheHostPasses` | FAIL: 32 drifted artifacts |
| (b) The same shape, outputs not regenerated | `TestSkillsDriftGate_BaseGeneratorChangeWithoutRegenerationFails` | no offender (a false green) |
| (c) A change only in an imported package (`prompts.ParseFrontmatter`), with and without regeneration | `TestSkillsDriftGate_ImportedPackageChangeIsGradedByTheWorktreeGenerator` | both directions red: 32 false offenders, and a false green |
| (d) A tree without the generator source | `TestSkillsDriftGate_TreeWithoutTheGeneratorSourceIsGradedInProcess` | PASS before and after (byte-identical, 0 subprocesses) |
| (e) The #821 cases | `TestSkillsDriftGate_*` in `audit_skillsdrift_worktree_integration_test.go` | see below |

Changes to the #821 tests:

- `TestSkillsDriftGate_LaneWithoutGeneratorChangeIsGradedInProcess` asserted the in-process grade for a lane that does not touch the generator. A1 removes that path on purpose, so the test becomes case (d).
- `TestSkillsDriftGate_CommittedGeneratorChangeTriggersFromTheCycleBase` pinned that a lane without a base missed a committed generator change. Now the change is seen with or without a base (`…IsGradedByTheWorktreeWithOrWithoutABase`).
- `TestSkillsDriftGate_Cycle1840ReplayPasses` was red on main at `9b47ca6ae`. It edited the generator at the text `` (Skill tool id `evo:%s`)``, which `9b47ca6ae` removed. It now uses the stable anchor of the other #821 tests (`laneMarkerOld`). The warning text follows A5.
- The fixture builder `generatorLaneRepo` now uses `committedSkillsRepo`, which also builds the tree without the generator source for case (d).

Unit tests (no tag) pin `carriesSkillsGenerator` (the package directory, a sibling package, a file in place of the package, the name outside `go/internal`) and the reworded `generatorDisagreement`.

## 5. Patterns and forces

- **Single source with projection.** CI's `TestSkills_NoDrift` is the source. The in-process check is a projection that is exact only when both generators are the same code. The gate no longer guesses when that holds. It asks the source.
- **Remove the prediction, not patch it.** Each of the three holes in section 2 would need its own detector. Running the source removes all three.
- **Plain code.** One `if` on one predicate. No Strategy.
- **Ports and adapters.** The worktree run stays behind `runCmd` (`sysexec.RunFunc`).

## 6. Limits

- **Cost.** Each audit of a tree that carries the generator source runs one `go run ./cmd/evolve skills check`. Measured on this worktree with a warm `GOCACHE`: 0.57 s, 0.29 s and 0.28 s (3 runs, after one warm-up). A fresh copy of the tree with a warm build cache takes about 3 to 4 s (the fixture tests). The time limit is the `go vet` budget of `ciparitygate` (4 minutes).
- **A failed worktree run is a skip WARN.** A runner error or the time limit gives `skills-drift gate skipped (could not run): the worktree generator ... did not run: the lane is NOT graded ...`. CI `TestSkills_NoDrift` is then the only check. A non-zero exit without a report is still a FAIL (#821 D6).
- The lane's binary grades the lane: the worktree binary owns the comparator, the report format and the exit code. The host WARN (A4) shows a disagreement but does not stop it. The follow-up `skills-drift-gate-generator-dependency-closure` (residual a) owns the fix (a render verb, with the comparison on the host).
- `core.ChangedWorktreePathsSinceBase` is exported, but after A3 no package outside `core` calls it. A later refactor can unexport it.
- Other in-process gates that render with host code:
  - gofmt (`codequality.UnformattedGoFiles`): it uses the `go/format` of the host's Go toolchain. Its skew follows the toolchain version, not the repository commit.
  - The explanation verifier (`explanationdocs.Verify`), the solution contract (`core.SolutionViolations`) and the build floors (docs floor, protected surface, comment floor): policy gates. Host authority is intentional (lesson `cycle-1340-in-binary-gate-fix-cannot-grade-the-cycle-that-authors-it`).
  - `derivedArtifacts` (`core/ship_recovery.go`) has a lane-diff trigger too, but it regenerates with the worktree's own generator. It does not grade with host code.
  - The CI-parity gates (go vet, acs-durable, integration tier, apicover) and the composed-tree gates run the worktree's own toolchain.
- The composed-tree gate decline at 23:27:24 (`ORCHESTRATOR_COMPOSED_GATE_DECLINED`, apicover) was real, not host skew. The gate runs `make -C go apicover-enforce` in the worktree. A replay over the composed tree (`925c58d12`) fails one test, `TestSkillsDriftGate_Cycle1840ReplayPasses`, because of the stale anchor above. The required CI of main is red at `9b47ca6ae` for the same reason. This lane fixes the test.
- The ADR-0072 classifier did not file a `pipeline-defect-*` item for cycle 1841. `buildFailureDossier` (`go/internal/core/failure_dossier.go:74`) reads only the audit's failure block. The auditor wrote PASS each round, so there is no block. The builder's `infrastructure-systemic` WARN block is not an input. The follow-up `failure-dossier-reads-no-builder-declared-system-class` records this. It is a different seam.
