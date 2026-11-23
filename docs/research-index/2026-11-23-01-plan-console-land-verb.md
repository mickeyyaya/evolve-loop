## 2026-11-23 — One verb lands a console change and refuses on a skipped step

| Document | What it records |
|---|---|
| [plans/console-land-verb-2026-11.md](plans/console-land-verb-2026-11.md) | Plan for `evolve pr submit`. It runs the steps B5 to B7 of the console check-in SOP in order and refuses on a skipped or stale step. It reuses the evaluator pattern of `wave checkin` and leaves `evolve land` unchanged. The plan scores four designs and finds that `record-pr` already waits for the host head, so the verb adds only a local head compare. |
