# Current runtime contract

This is the current capability index. Historical ADRs explain decisions; an Accepted label alone does not prove a feature is enabled or wired through every entrypoint.

| Capability | Supported behavior | Production boundary | Behavioral evidence |
|---|---|---|---|
| Predicate evidence | Host execution and bound modern evidence required at Audit/Ship; legacy shipping requires re-audit | `phases/audit`, `phases/ship`, `acssuite` | [Evidence recovery](recovery-predicate-authority.md) |
| Isolation | Enforced supported profile read/write restrictions; required but unverified confinement refuses | `bridge`, `adapters/sandbox`, `looppreflight` | [Isolation recovery](recovery-isolation-policy.md) |
| Continuation adoption | Clean snapshot advances to a pinned main base before archive staging; the ancestor's explanation records and own predicate package are archived under one retention rule (A2); failures stop dispatch | `core/continuation_stamp.go`, `core/continuation_baseadvance.go`, `core/continuation_ancestor_predicates.go` | [Adoption boundary](continuation-adoption-boundary.md) |
| Resume | Shared terminal lifecycle, policy and checkpoint advancement | `core/resume.go`, fresh cycle lifecycle | [Resume recovery](recovery-resume-lifecycle.md) |
| Prompt delivery | Multiline bytes preserved through application-aware paste; transport failures stop before verification | `bridge/tmux.go`, shared REPL delivery | [Prompt transport](tmux-prompt-delivery.md) |
| Task contract | Selected acceptance/predicate inventory delivered to TDD, Build and Audit; sanitized previews name their original authority | `core/task_contract.go` | `TestDispatch_TaskContractReachesTDDBuildAndAudit`, `TestComposeTaskContract_SanitizedCriteriaAreNotClaimedVerbatim` |
| Lesson recall | Task/goal recall into Scout/TDD/Build/Audit, current-goal and latest-failure recall into advisor | `core/task_recall.go`, `core/routing_dispatch.go`, `phases/runner` | `TestDispatch_TaskRecallWithoutFailureHistory`, `TestAdvisorPlanInput_RecallsGoalWithoutHistory` |
| Reader swarm | Optional observations or synthesized reader output | `phases/swarmrunner` | Existing reader composition tests |
| Writer swarm | Live writer dispatch unsupported; explicit refusal before side effects, default shadow delegates | `phases/swarmrunner` | `TestDecorator_WriterLiveModesRefusedBeforeDispatch` |
| Fleet visibility | Per-run state/lease liveness, active lanes survive history limit | `dashboard/loop.go`, `collect.go` | `TestCollect_FleetLivenessSurvivesHistoryCap` |
| Retry adjudication | Bounded conservative TDD retry; deep adjudicator deferred | `core/audit_fail_decision.go`, ADR-0093 | Existing retry/resume tests; no claim of production adjudicator wiring |
| Family separation | The Claude floor holds six grading seats on `claude`. Since X6 (#1123) it is `cli_routing.floors` data, with a compiled default in `policy/routing_defaults.json`. It is not a kernel invariant. | `policy`, `cliroute` | `TestDeepTierFamilyArrangement` |
| LLM agent isolation (ADR-0154) | Proposed. X1 (#1121), X2 (#1122, stage `shadow`) and X6 (#1123) are on `main`. Each open gap is an expected-failure row with its owner component. | `internal/isolation` | `isolation.Rows`, [llm-isolation.md](llm-isolation.md) §14 |
| One agent with no cycle (ADR-0152) | `evolve agent run <profile>` runs one read-only spec phase and prints its report. It writes a ledger entry with cycle 0. | `cli/agentcmd` | [runtime-reference.md](../operations/runtime-reference.md) |
| Quality panel (ADR-0153) | `quality report` publishes the readings of PQ1, PQ4, CQ1 to CQ5 through the Signal Center. The boundary runs it after `sync-main`. | `qualitypanel`, `pipelinerates`, `codeshape` | [llm-quality-indexes.md](llm-quality-indexes.md) |
| CLI service contract (ADR-0156) | Proposed. No verb prints the versioned envelope yet, and `loop status <word>` starts a loop. | `cmd/evolve` | [cli-contract.md](cli-contract.md) |
| Control flags (ADR-0156) | The pipeline still reads its knobs and bypasses from the environment, and they reach the LLM children. X18 removes them. | `flagregistry`, `config`, `bridge` | [control-flags.md](control-flags.md) |

## Documentation lifecycle

Changes to these contracts require production-call-site evidence and tests through each supported entrypoint. Field parsing, an exported helper, or a passing mock callback alone is insufficient. Update the affected current guide in the same change. Archive superseded protocols; historical shell paths are not executable instructions.

Current guides: [trust](../concepts/trust-architecture.md), [learning](../concepts/self-evolution.md), [platforms](platform-compatibility.md), [resume](checkpoint-resume.md), [swarm](swarm-harness.md).

## Issue / gap / solution — September recovery

**Issue:** helper-level completeness was repeatedly mistaken for complete production behavior. The dashboard saw one live cycle; the memory contract named a nonexistent summary; writer swarm reported integration without transferring the authoritative tree; a sanitized acceptance preview was called verbatim.

**Gap:** tests exercised individual helpers, declared metadata, or a singleton cycle. They did not establish cross-entrypoint lifecycle/evidence parity or multi-lane liveness. Current-facing documentation retained shell-era guarantees despite a prior reconciliation.

**Solution:** enforce evidence and filesystem controls at launch/ship boundaries; share resumed closeout and policy; retrieve lessons through existing task-context dispatch; derive lane liveness separately; make incomplete writer modes refuse. Preserve task-contract work already landed in PR #534 and label transformed previews accurately. Archive five superseded guides and use this compact index as the current entrypoint.

## Outcome verification

The operator authorized two live verification waves after code review and tests. Each wave must report cycles attempted, validated task changes/landings, open PRs, failing CI jobs/classes, repair attempts, and intervention. Inspect diffs and acceptance results; a PASS label or a ship subprocess exit alone is insufficient. Stop on integrity blockers or the repository's two-unproductive-wave halt rule. Record missing dossiers and duplicate ledger events separately rather than hiding them in a success-rate denominator.
