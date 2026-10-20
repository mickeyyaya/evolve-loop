## 2026-10-20 — a defect that survives two builder rounds is fixed from the finder's spec (ADR-0146)

| Document | What it records |
|---|---|
| [research/finder-fix-ownership-2026-10.md](research/finder-fix-ownership-2026-10.md) | Research: the first repair round fixes about 71% of audit failures. It also surveys who finds, writes and approves a fix. Findings F2.1 to F2.4 and R1 to R12. |
| [plans/finder-fix-ownership-2026-10.md](plans/finder-fix-ownership-2026-10.md) | Plan: five designs with weighted scores. The finder writes the fix spec, a fresh builder applies it, and another check verifies. Decisions D1 to D8 and components F0 to F6. |
| [architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md](architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md) | Decision (Accepted): R1 retry, R2 effort, R3 a fix from the finder's spec, verified by a check that did not write it. |
