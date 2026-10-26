---
id: kill-guard-never-mutated
statement: "Never mutate or run a live kill guard. Inject the signal sender, so that a test cannot kill a real process."
layer: L0
order: 1
audience: [all]
enforced_by: []
source: "memory feedback_never_mutate_live_kill_guards"
status: active
added: 2026-10-26
evidence: "a mutant of the pgid <= 1 guard of the swarm group killer ran kill(-1, SIGKILL) and killed each user process twice on 2026-10-15"
---
Why: A kill guard protects every process of the host. A mutant or a fixture that runs it for real kills processes outside the test.
