# Plan: the skills-drift gate grades a generator change with the worktree's own generator

- **Status:** implemented in the console lane `cl-skills-drift`, opened 2026-10-09.
- **Tracked item:** `skills-drift-gate-grades-with-worktree-generator` (`.evolve/inbox`, created 2026-10-07).
- **Halt item:** `pipeline-defect-infra-systemic-cycle1840`. The console lands it separately.
- **Lessons:** `inst-L1828a`, `inst-L1840a`.

## 1. Incident

Two cycles took the inbox item `generated-skill-command-shadows-its-skill`:

| Cycle | Wave | Gate result | Worktree result | Outcome |
|---|---|---|---|---|
| 1828 | 77 | 30 drifted artifacts | `skills check OK` | FAIL, halt |
| 1840 | 82 | 32 drifted artifacts | `skills check OK`, ACS 4/4 PASS | FAIL `infrastructure-systemic`, ADR-0072 halt |

The fix of that item changes the generator `skillcheck.RenderCommandStub` (`go/internal/skillcheck/commands.go`). It then regenerates all `commands/*.md` stubs with the new generator. The stubs agree with the new generator. They do not agree with the old generator.

In cycle 1840, round 3, the auditor reported a correct deliverable. The skills-drift gate overrode the PASS with 32 `DRIFT` lines (`runs/cycle-1840/audit-fail-reason.json`). The gate-class repair round (a gate-forced FAIL gets one Build repair) did not help. Each repair round met the same skew.

## 2. Root cause

- Before this change, `skillsDriftCheckDefault` (`go/internal/phases/audit/audit.go:502` at base `195402329`) called `skillcheck.Check(root)` in the orchestrator process. The gate is now in `go/internal/phases/audit/skillsdrift.go`.
- `skillcheck.Check` renders the expected stubs and SKILL.md regions with the code that is compiled into the orchestrator binary (`017d3e817ef1` in cycle 1840).
- The orchestrator binary contains the base generator. It does not contain the lane's generator.
- Thus a lane that changes the generator gets a grade from the old generator. The grade is a false red for a correct lane (cycles 1828 and 1840).
- The same skew also gives a false green. A lane that changes the generator but does not regenerate the stubs keeps stubs that match the old generator. The gate passes them. CI then fails, because CI runs `TestSkills_NoDrift` with the tree's own generator.

CI is the truth: `TestSkills_NoDrift` (`go/cmd/evolve/cmd_skills_drift_test.go:25`) compiles the tree and runs its generator over the tree. The in-process gate is a fast copy of that check. The copy is correct only when the host generator and the lane generator are the same code.

## 3. Decisions

| # | Decision | Reason |
|---|---|---|
| D1 | When the lane changes a path under `go/internal/skillcheck/`, the gate runs the worktree's own `evolve skills check`. Otherwise the gate runs `skillcheck.Check` in-process, as before. | The in-process check is correct when the generator did not change. Acceptance 3 requires byte-identical output for those lanes. |
| D2 | The changed paths come from `core.ChangedWorktreePathsSinceBase`. It diffs from the cycle base (`PhaseRequest.WorktreeBaseSHA`) and adds untracked files. Without a base, it diffs from HEAD. | A builder can commit its work. A HEAD diff then misses a committed generator change. The build-floor reviewer had the same rule, so the rule moved into one exported function that both use. |
| D3 | The generator is the path prefix `go/internal/skillcheck/`, which includes `templates/`. | The derived-projection registry uses the same prefix form (`derivedArtifactSpec.ssotPrefix`, `go/internal/core/ship_recovery.go`). The tracked item names this prefix. |
| D4 | The worktree run uses the existing worktree-evolve invocation: `go run ./cmd/evolve <args>` in `<worktree>/go` with `EVOLVE_WORKTREE_ROOT=<worktree>`. One function in `core` builds that invocation. `regenerateDerivedArtifact` and the gate both use it. | The brief forbids a second subprocess path. `regenerateDerivedArtifact` already ran the worktree's own generator for `control-flags.md`. |
| D5 | The gate executes the invocation through the audit package's `runCmd` (`sysexec.RunFunc`). | `runCmd` is the one subprocess seam of the audit package. Tests replace it with a fake runner. |
| D6 | Exit 0 is clean. A non-zero exit with `DRIFT:` or `MANIFEST:` lines gives those offenders. A non-zero exit without such lines is a FAIL with one offender that carries the trimmed output. Only a runner error or the time limit fails open, with a loud WARN that says the lane is not graded. The gate checks the deadline first: on a timeout the real runner returns the exit of the killed process, not an error. | The lane's own binary prints the report. If such an exit failed open, a lane can reword the report or panic the check. The lane then gets a PASS while CI is red. `ciparitygate` `runGate` uses the same rule. |
| D9 | When the worktree check passes, the gate also runs the host check. If the host disagrees or cannot compare, the gate gives a WARN that names the count. | A generator change can disagree with the host for a good reason, so this is not a FAIL. But the lane grades itself, so the disagreement must never be silent. |
| D7 | An offender from a `DRIFT:` line is the artifact path (the first word after `DRIFT:`). A `MANIFEST:` line stays whole. | The in-process gate names artifact paths. The diagnostic keeps the same form. |
| D8 | The time limit of the worktree run is `ciparitygate.DefaultTimeouts().GoVet` (4 minutes). | The `go vet` gate compiles the same module. One home for the number. A warm build cache gives about 3 seconds. |

## 4. TDD protocol

Red first. Each test drives the production entry point `skillsDriftCheckDefault` over a real git worktree copy of the repository, and a real `go run` of that copy's generator.

| Test | Proves | Red on the old code |
|---|---|---|
| `TestSkillsDriftGate_GeneratorChangeWithRegeneratedStubsPasses` | Acceptance 1 | FAIL: the host generator reports every stub as drift |
| `TestSkillsDriftGate_GeneratorChangeWithoutRegenerationFails` | Acceptance 2 | FAIL: the host generator reports no drift (a false green) |
| `TestSkillsDriftGate_LaneWithoutGeneratorChangeIsGradedInProcess` | Acceptance 3 | PASS before and after (a preservation test) |
| `TestSkillsDriftGate_Cycle1840ReplayPasses` | The cycle-1840 regression | FAIL: 32-style drift of every stub |
| `TestSkillsDriftGate_CommittedGeneratorChangeTriggersFromTheCycleBase` | A committed generator change triggers the worktree check | Review round 1 |
| `TestSkillsDriftGate_ReworkedReportProtocolStillFails` | A reworded report fails, not fails open | Review round 1 |
| `TestSkillsDriftGate_GeneratorChangeThatAgreesWithTheHostIsSilent` | No WARN when the host agrees | Review round 1 |

Unit tests (no tag) pin the decision (`changesSkillsGenerator`), the report parser (`skillsCheckOffenders`), the runner contract of `worktreeSkillsDrift` and the invocation (`core.WorktreeEvolveInvocation`). Mutation checks cover the prefix, the exit-code branch, the line prefixes and the environment key.

## 5. Patterns and forces

- **Single source with projection.** CI's `TestSkills_NoDrift` is the source of truth. The gate projects it. When the projection cannot be exact (the host has a different generator), the gate asks the source.
- **Strategy by data, not by type.** The two grading paths differ in one fact: did the generator change? A prefix test selects the path. A Strategy interface with one call site is ceremony, so the code uses a plain `if`.
- **Ports and adapters.** The subprocess stays behind `sysexec.RunFunc`. The decision and the parser are pure functions.

## 6. Limits

- The prefix covers only the `skillcheck` package. The generator also reads code in `phasespec`, `phasecontract`, `profiles`, `prompts` and `config`. A lane that changes how those packages render a SKILL.md region still meets the skew. A follow-up item records this gap. A dependency-closure check (`go list -deps`) closes it. Superseded: since [skills-drift-host-stamp-2026-10.md](skills-drift-host-stamp-2026-10.md) the gate always runs the worktree generator on a tree that carries its source, so no trigger is left to widen.
- An orchestrator binary that is older than the worktree base has the same skew for every lane. The loop rebuilds the binary at wave boundaries, so this gap is small. A follow-up item records it. Cycle 1841 hit this gap; [skills-drift-host-stamp-2026-10.md](skills-drift-host-stamp-2026-10.md) closes it.
- Other in-process gates that grade with host code: the explanation verifier (`explanationdocs.Verify`) and the solution contract (`core.SolutionViolations`). These gates are policy gates. Host authority is intentional for them: a lane must not grade itself with a gate that it changed (lesson `cycle-1340-in-binary-gate-fix-cannot-grade-the-cycle-that-authors-it`). The generator is different: its output is a projection, and CI grades it with the tree's own code.
- A compile error in the worktree's `cmd/evolve` is now a FAIL with the compiler output as the offender.
- Residuals from review round 1. The follow-up item `skills-drift-gate-generator-dependency-closure` records each one:
  - (a) The lane's binary owns the comparator, the report format and the exit code, not only the renderer. The host must own the comparison, and the worktree must supply only the rendered bytes (for example a `skills render --json` verb). This also makes a closure trigger safe.
  - (b) Keep the prefix trigger until (a) lands. Then widen it to the `go list -deps` closure.
  - (c) The rule "a projection maps to a generator source prefix" has two homes: `derivedArtifacts` `ssotPrefix` (`core/ship_recovery.go`) and `skillsGeneratorPrefix` (`skillsdrift.go`). One registry must hold both.
  - (d) The worktree run does not emit a Signal Center event when it cannot run. The `ciparitygate` `stepFailed` seam is the model.
  - (e) The context of the worktree run is not tied to the cycle, because the `PhaseRequest` seam carries no context.
  - (f) The rule "an empty base means a diff from HEAD" has a second home: `floorBase` in `core/comment_floor.go`, which also adds `--no-renames`. `ChangedWorktreePathsSinceBase` must be its one home.
