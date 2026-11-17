## 2026-11-17 — concurrent edits: one entry, one file, so that parallel lanes do not meet in the same lines

| Document | What it records |
|---|---|
| [research/concurrent-edit-layout-2026-11.md](research/concurrent-edit-layout-2026-11.md) | Research: a 30-day census of 12,747 touches. 7 central lists take 330 touches, 2 generated aggregates take 164, and 2 big packages take 2,654 and serialize admission. 7 of the 9 conflicts of 2026-11-17 were independent additions. Current practice: fragment files, self-registration, disjoint modules, structural merge as a help. Findings E1.1 to F2.11, P3.1 to P3.11, refinements R1 to R6. Amended: lanes had no code conflict in the 149 runs of cycles 2300 to 2462; all 21 cell waits are on `cmd/evolve`. |
| [plans/concurrent-edit-layout-2026-11.md](plans/concurrent-edit-layout-2026-11.md) | Plan: four designs scored; one entry for each file wins. Decisions D1 to D8, components C1 to C8 with red tests. C6 makes the cell of K5 in the console-build plan smaller, and K5 stays. Amended: C1, C4 and C8 are withdrawn, and C6 goes first. |
