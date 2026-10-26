---
id: no-broad-pkill
statement: "Never kill processes by a command pattern on the shared host. Kill only the process ids that you started."
layer: L0
order: 2
audience: [all]
enforced_by: []
source: "memory feedback_no_broad_pkill_shared_host"
status: active
added: 2026-10-26
evidence: "on 2026-10-11 the lane cl-cover-changed ran pkill -f apicover and pkill -f go test while other lanes ran the same commands (memory feedback_no_broad_pkill_shared_host)"
---
Why: Many agents share one host. A pattern matches the processes of other agents too.
