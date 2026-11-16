---
id: issue-gets-a-record
statement: "Record each issue, alert or mistake through the issue-record skill, with its logs, results, root cause, fix and regression test."
layer: L0
order: 23
audience: [all]
triggers:
  failures: [test-failure, ci-red, gate-refusal, warn-signal, incident-signal, selfheal-incident, wrong-claim, revert, operator-correction]
enforced_by: []
source: "operator request 2026-11-16; docs/plans/issue-record-skill-2026-11.md"
status: active
added: 2026-11-16
evidence: "docs/incidents/2026-11-16-tests-leak-evolve-ledgers-under-go.md: a WARN signal cost cycle 2419 about 3 hours, and no procedure recorded it"
---
Why: A lesson that is not written is lost. The record and its regression test stop the issue from coming back.
