# Eval: fix-cycle-cap-boundary-semantics

## Task
Fix the `>` vs `>=` inconsistency between the upfront check (SKILL.md) and the per-cycle check (phases.md) for `maxCyclesPerSession`.

## Correct Semantics
- **Upfront check (SKILL.md)**: `cycles > maxCyclesPerSession` → HALT. This correctly blocks requests exceeding the cap before any work starts.
- **Per-cycle check (phases.md)**: Should also use `>` not `>=`. The per-cycle halt fires "after exceeding" the cap, meaning: if `currentCycle > maxCyclesPerSession`. Using `>=` would halt at exactly the cap cycle, preventing the last allowed cycle from completing.

Example: maxCyclesPerSession=10, running 10 cycles.
- Upfront: `10 > 10` → false → allowed (correct)
- Per-cycle at cycle 10: `10 >= 10` → HALT before cycle 10 runs (WRONG — cycle 10 should run)
- Per-cycle at cycle 10: `10 > 10` → false → cycle 10 runs (CORRECT)
- Per-cycle at cycle 11: `11 > 10` → HALT (correct, but cycle 11 should never exist given upfront check)

## Acceptance Criteria

### Code Graders

1. **phases.md uses `>` for maxCyclesPerSession per-cycle check**
   ```bash
   grep -c "current cycle number > \`maxCyclesPerSession\`" skills/evolve-loop/phases.md
   # expected: 1
   ```

2. **SKILL.md uses `>` for upfront check (unchanged, verify)**
   ```bash
   grep -c "cycles argument > \`maxCyclesPerSession\`" skills/evolve-loop/SKILL.md
   # expected: 1
   ```

3. **phases.md no longer has `>=` for maxCyclesPerSession**
   ```bash
   grep -c "cycle number >= \`maxCyclesPerSession\`" skills/evolve-loop/phases.md
   # expected: 0
   ```

### Acceptance Checks

4. **Both files use the same operator for maxCyclesPerSession** — verify that the boundary semantics are now consistent (both `>`).

5. **warnAfterCycles in phases.md** — the `>=` for `warnAfterCycles` is intentional (warn starting at the threshold), leave it unchanged.
