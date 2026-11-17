---
score_cap:
  - criterion: "two todos that edit different files of a leaf package both admit when the leaf is in file_cells"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit$' ./internal/admission"
  - criterion: "the section 5 guard tests hold: same file, whole-package edit, non-leaf package"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestAdmit_TwoTodosThatEditTheSameFileOfALeafPackageHoldTheSecond|TestAdmit_AWholePackageEditOfALeafHoldsAFileEditOfThatLeaf|TestAdmit_TwoTodosThatEditDifferentFilesOfANonLeafPackageHoldTheSecond)$' ./internal/admission"
  - criterion: "the never-conflict property and the whole admission package stay green"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./internal/admission/..."
  - criterion: "an empty file_cells list keeps package keys"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestAdmit_WithNoFileCellsTwoFilesOfALeafStillShareOneCell$' ./internal/admission"
  - criterion: "the policy key reaches the pool admission, the census and the wave shadow"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestCellAdmission_PolicyFileCellsLetTwoFilesOfCmdEvolveRunBesideEachOther|TestAdmissionShadow_PolicyFileCellsReachTheWaveShadow|TestSupplyWiring_TheCensusTakesTheFileCellsOfThePolicy)$' ./cmd/evolve"
---

# Eval: file-level edit keys for leaf packages in admission

> Pins plan K1 of `docs/plans/admission-cmd-evolve-cell-2026-11.md`. A leaf package named in
> `fleet.orchestrator.file_cells` gives each todo one key for each `.go` file it lists, so two
> items that edit two files of `cmd/evolve` run in two lanes. A whole-package edit, a non-leaf
> package and an empty list keep the package key. Source: cycle 2461, inbox item
> `admission-file-cells-for-leaf-packages`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| red-test | different files of a leaf both admit | 3/10 | `go test -run TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit ./internal/admission` |
| guards | same file, whole package, non-leaf hold the second | 4/10 | the three section 5 guard tests |
| property | the admission package stays green | 4/10 | `go test -count=1 ./internal/admission/...` |
| preservation | an empty `file_cells` keeps package keys | 5/10 | `TestAdmit_WithNoFileCellsTwoFilesOfALeafStillShareOneCell` |
| wiring | the key reaches the three production callers | 5/10 | the `cmd/evolve` tests of the policy key |
