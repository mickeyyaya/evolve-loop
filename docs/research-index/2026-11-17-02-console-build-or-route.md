## 2026-11-17 — Console work: build only what a lane cannot build

| Document | What it records |
|---|---|
| [research/console-build-or-route-2026-11.md](research/console-build-or-route-2026-11.md) | Wave 177 census: 180 of 288 items had a console route with no checked fact, 1 item was admissible, and the lanes ran at 18%. Practice from the Theory of Constraints, Team Topologies, Google SRE, SLSA and agent orchestration. |
| [plans/console-build-or-route-2026-11.md](plans/console-build-or-route-2026-11.md) | Plan: the console builds an item only for a protected path, a console-only act or an integrity defect of operating-policy §1.1. It starts no feature build while the supply starves or while more than one batch of its PRs is open. |
