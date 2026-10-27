# Retired ACS predicates: the advisor names no CLI (2026-10-27)

> Archived by the console lane for the inbox item `cli-neutral-task-assignment-and-guard` (component X19a of [the LLM-isolation plan](../../../../plans/llm-isolation-2026-10.md), [spec §28](../../../../architecture/llm-isolation.md#28-the-cli-neutral-task-assignment-x19)). X19a removes the `cli` field from the advisor plan. Thus the plan clamp has no catalog check, because that check needed a CLI. The predicate below ran a test of that check. It is archived here under the A2 retention rule ([the rule](../../../../architecture/acs-predicate-quality-gate.md#the-ledger-of-the-20-stale-names)). The rest of each package stays live in `go/acs/`.

## What was archived and why

| Predicate | It asserted | Why it is retired | What pins the contract now |
|---|---|---|---|
| `TestC440_005_GenuineCatalogMissStillClamps` | An advisor `{cli, tier}` proposal that the model catalog cannot resolve is cleared by the plan clamp. | The plan names no CLI after X19a, so the clamp has no `{cli, tier}` pair to look up. The router resolves the routed CLI and tier at dispatch. | `internal/router` `TestClampPlanModelRouting_ClampsOutOfEnvelopeTier` pins the tier clamp. `internal/phasecontract` refuses a plan entry with a `cli` key. |

## What was narrowed or repointed

| Predicate | Stale name | Action | Now |
|---|---|---|---|
| `TestC440_001` | `TestClampPlanModelRouting_SuffixedCLIHonoredViaBaseName` | narrowed | The predicate keeps the two `policy.BaseCLI` tests. The clamp reads no CLI, so no base name reaches it. |
| `TestC440_006` | `TestModelRouting_CatalogMissClampsUnderAuto`, `TestPhaseRequest_ModelRoutingFieldsOmitEmptyByDefault` | narrowed and repointed | `TestModelRouting_AutoApplies` and `TestPhaseRequest_CarriesANeutralAssignmentAndNoDriverOnTheWire` |
| `TestC463_001` | `TestComposePlanPrompt_ElicitsTierAndCLI` | repointed | `TestComposePlanPrompt_ElicitsATierAndNoCLI`. The prompt still asks for a tier, and it asks for no CLI. |
| `TestC463_013` | `TestReplayPlanFromResponse_ModelRoutingOverlayAppliesUnderAuto` | repointed | `TestReplayPlanFromResponse_OnlyTheTierOfAnAdvisorResponseReachesDispatch` |
| `TestC476_001` | `TestComposePlanPrompt_RealPersonaExistingExampleCarriesTierAndCLI` | repointed | `TestComposePlanPrompt_RealPersonaExistingExampleCarriesATierAndNoCLI` |
| `TestC476_002` | `TestRealPersonaFrontmatterOutputFormatEnumeratesTierCLI` | repointed | `TestRealPersonaFrontmatterOutputFormatEnumeratesTheTierOnly` |

## Notes

- The archived file keeps the original package clause and body of the predicate. It lives outside the Go module and never builds.
- `go/acs/regression/acsnames` reads `go/acs` only. It does not read this archive.
