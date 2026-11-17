# Recommendation — split cmd/evolve

Citations: [assumptions-and-evidence.md](assumptions-and-evidence.md). Full design: `docs/plans/split-cmd-evolve-2026-11.md`.

## Options Compared

| Criterion | 1 — per verb | 2 — co-change clusters | 3 — domain groups |
|---|---|---|---|
| Relief (E1, E2) | low | high | medium |
| Move risk (E3) | high | medium | medium |
| Fit with existing pattern (E4) | weak | strong | strong |
| Packages | about 80 | about 6 | about 6 |
| Score | 2.00 | 4.20 | 3.60 |

## Recommendation

**Winner: Option 2.** The first group to move is `inbox` (33 touches, 12 outbound, 9 inbound; E1, E3). The plan gives the move order and a red test for each move. It answers Q2 of the parent plan: a few groups.

**Runner-up: Option 3.**

**Evidence that can flip the choice:** if the 30-day co-change shows `boundary` and `quality` apart from `wave`, Option 3 ties with Option 2. If CORE stays hot after step 4, the admission fallback K5 of the parent plan wins.
