# Option 2: one package for each verb

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

**What changes:** each verb moves into its own package under `internal/cli/<verb>cmd`. Package cells then split the lanes.

**For whom:** every lane that adds a verb, in the long run.

**Causal chain:** a verb in its own package has its own cell. Two verb items stop sharing `cmd/evolve`.

**Expected effect:** the full split of 1000 files (E4). No gain until the moves are done. Each move edits `cmd/evolve`, so each move itself serializes on the cell (E1).

**Cost and time to effect:** months of migration. Each new package is a non-leaf and has reach.

**Top risks:**

1. The migration blocks the wave it must free. Detect: the waiting count does not fall after a month.
2. Moved packages add reach that holds other lanes. Detect: `evolve wave supply` cells.
