# Recommendation: admission splits the coarse cmd/evolve cell

Citations: [assumptions-and-evidence.md](assumptions-and-evidence.md). The full plan: [admission-cmd-evolve-cell-2026-11](../../docs/plans/admission-cmd-evolve-cell-2026-11.md).

## Options Compared

| Criterion | 1 file keys for leaves | 2 one package per verb | 3 symbol-aware rule | 4 self-registering verbs |
|---|---|---|---|---|
| Meets the red test: two items on two files of `cmd/evolve` both admit (E1, E2) | yes | yes, after months | unproven (A2) | no, alone |
| Keeps admission pure (A1) | yes | yes | no | yes |
| Cost | one lane item | 1000 files (E4) | parser or LLM | one small item |
| Time to relieve the wave (E3) | first wave after `file_cells` is set | months | unknown | none alone |
| Reversible through policy data (E7) | yes, a `file_cells` list | no | no | no, a code revert |
| Weighted score (plan section 4) | 4.65 | 2.85 | 1.65 | 2.85 |

## Recommendation

**Winner: Option 1.** It is the only option that meets the red test with one small, reversible change. It changes only the term "two edits meet" for packages that have no reach (E1, E2). The red test is `TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit`.

**Runner-up: Option 4**, as a staged follow-up. Its score ties Option 2 (2.85), so the score does not rank it. The reasons are not scores. It is one small mechanical item. It needs no 1000-file move (E4). It removes the `registry.go` hot file (E5, E6) that limits the gain of Option 1.

**Evidence that can flip the choice:**

- If the share of waiting items that need `registry.go` is above half, Option 4 moves first or together with Option 1 (E5, E6).
- If integration reds with `redeclared` rise after `file_cells` is set, the shared-symbol risk outweighs the gain (A3). Empty the `file_cells` list. Do not touch `fleet.orchestrator.admission`: it is `enforce` and sets the rule of the whole fleet.

Not sourced: the share of the 19 to 22 waiting items that avoid `registry.go` and `parallel_serial_tests_test.go`. The plan's Verification section measures it first.
