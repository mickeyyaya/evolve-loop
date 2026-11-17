# Option 1: file keys for leaf packages

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

**What changes:** for a leaf package and an item with a file list, the edit key is the file. An item with no file list is "whole package" and conflicts with each file key of that leaf. Two file sets conflict only when they share a file.

**For whom:** lanes that edit different files of `cmd/evolve`.

**Causal chain:** a leaf has no reach (E2). Only the term "two edits meet" holds the lanes apart (E1). Splitting that term for leaves frees two items on two files, so lane utilization rises.

**Expected effect:** items held only by the `cmd/evolve` cell fall from 19 to 22 (E3) to fewer than 5. This rests on A2 and on the share of items that avoid `registry.go` (E5, E6), which is not measured yet.

**Cost and time to effect:** one lane item in `go/internal/admission/`. It shows its effect in the first wave after the operator sets the `file_cells` list (E7).

**Top risks:**

1. Two lanes add the same top-level name in two files.
   - Detect: an integration red with `redeclared` (A3).
   - Revert: empty the new `fleet.orchestrator.file_cells` list.
   - The key `fleet.orchestrator.admission` is not a stage switch. It sets the rule of the whole fleet. File keys apply in both modes (E7).
2. Most items edit `registry.go`, so the gain is small (E5, E6). Detect: the count in the plan's Verification section.
