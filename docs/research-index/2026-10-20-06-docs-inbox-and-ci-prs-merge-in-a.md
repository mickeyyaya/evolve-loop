## 2026-10-20 — docs, inbox and CI PRs merge in a window; code merges at a boundary (ADR-0148)

| Document | What it records |
|---|---|
| [plans/pr-merge-latency-2026-10.md](plans/pr-merge-latency-2026-10.md) | Plan: the merge wait of PRs #979 to #1005, four designs with weighted scores, and components P1 to P5. P1 is shelved. The window stays off until `fleet.integration.stage` is `enforce` (ADR-0147). |
| [architecture/adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md](architecture/adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md) | Decision (Accepted): a window PR merges when green, also in a wave. A code PR merges at a boundary, and the console can call one early. |
| [architecture/packages/internal-mergewindow.md](architecture/packages/internal-mergewindow.md) | Package note: the merge window rule reads the CI routing table of `required.yml`. |
