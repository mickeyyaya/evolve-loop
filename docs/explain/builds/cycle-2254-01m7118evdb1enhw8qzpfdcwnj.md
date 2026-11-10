# Build Explanation — Cycle 2254

## Build Binding
- Cycle: 2254
- Base SHA: 6184cfa4a88ed9140ed28f3d9bb6cce5526375b2

## Summary
Each cycle that runs an inbox item is now a round. A failed round stamps a plan and its findings on the item. The next run reads them. At the ceiling, the item goes to the console or to quarantine and never returns to the retry pool.

## Rationale
The loop retried a failed item with the same plan. The fix stamps the plan on the item in the loop's own fields, so the author of the item cannot forge it. The existing failure-count bump is the single place that writes the stamps.

## Changed Areas
- `.evolve/evals/convergence-policy-v11-inbox-item-rounds.md` — the eval graders of the inbox item.
- `agents/evolve-triage.md` — triage reads the round stamps of an item.
- `docs/private/research/archived-2026-11-09/superseded-predicate-packages/cycle2240/predicates_test.go` — the archived predicate package of the superseded cycle 2240.
- `docs/private/research/archived-2026-11-09/unshipped-build-explanations/cycle-2240-01m6zj4mkkdt3yqsxqrrq73hm3.md` — the archived explanation of the unshipped cycle 2240.
- `go/acs/cycle2254/predicates_test.go` — the 13 acceptance predicates of this cycle.
- `go/internal/core/task_contract.go` — the task contract of a retried item shows the prior findings and plan as quoted data.
- `go/internal/core/task_contract_prior_run_test.go` — tests that the findings reach the next run and cannot forge a heading.
- `go/internal/cycleoutcome/cycleoutcome.go` — passes the ceiling policy and the failure reasons to the mover.
- `go/internal/inboxbatch/fieldowner.go` — registers `last_failure_findings` and `retry_plan` as loop stamps.
- `go/internal/inboxbatch/fieldowner_test.go` — covers the two new loop stamps.
- `go/internal/inboxbatch/item.go` — the item reads the two stamps as raw JSON, so a non-text value cannot break the load.
- `go/internal/inboxmover/lifecycle/apicover_named_test.go` — names the new exported symbols.
- `go/internal/inboxmover/lifecycle/failure_record_test.go` — tests the stamps of the failure record.
- `go/internal/inboxmover/lifecycle/item.go` — the bump writes the findings and the plan, and sheds the plan at the ceiling.
- `go/internal/inboxmover/lifecycle/quarantine.go` — release from quarantine clears the findings and the plan.
- `go/internal/inboxmover/lifecycle/release.go` — cuts the findings at a character boundary. A failed console route falls through to quarantine.
- `go/internal/inboxmover/lifecycle/round.go` — `PlanRound` schedules the rounds of an item.
- `go/internal/inboxmover/lifecycle/round_test.go` — table tests of the round plan and of the character cut.
- `go/internal/inboxmover/outcome.go` — carries the ceiling policy and the reasons into the release.

## Design Decisions
The second run raises effort and carries findings. The third run, or the final run at the ceiling, changes the plan. A system-level failure stamps nothing. A failed route to the console used to release the item to the inbox root. Now it takes the quarantine path, because the root is the retry pool.

## Verification
The ACS suite of cycle 2254 holds 13 predicates. The lifecycle, inboxbatch, inboxmover, core and repocontract packages pass with `-count=1`. The coverage predicate requires 100% on the round lifecycle functions.

## Compatibility
Items without a failure history render no extra text. The two new fields are optional and the old items load unchanged.

## Limitations
The plan name is advisory text for the next run. The runtime does not yet pick a model from it.
