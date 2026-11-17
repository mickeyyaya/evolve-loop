# Option 4: verbs that register themselves

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

**What changes:** each verb file adds its row to `commands` from its own `init` function. `registry.go` keeps the type and a sort.

**For whom:** lanes that add a verb.

**Causal chain:** a new-verb item edits only its new file, not `registry.go` (E6). This removes the hot file that limits Option 1 (E5).

**Expected effect:** alone, none: the package cell still holds the lanes (E1, E2). With Option 1, new-verb items stop sharing `registry.go`.

**Cost and time to effect:** one small mechanical lane item. Help output needs an explicit sort, because `init` order follows the file name.

**Top risks:**

1. The help output changes. Detect: a golden test of `evolve help`.
2. It lands before Option 1 and gains nothing. Detect: the waiting count stays the same.
