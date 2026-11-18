## 2026-11-18 — Tests that cannot fail, tests that pin code text, and fakes that differ

| Document | What it records |
|---|---|
| [research/test-quality-audit-2026-11.md](research/test-quality-audit-2026-11.md) | Research: a census of 33,528 test functions, with a command for each repeatable figure. The census finds tests with no assertion and self-asserting `apicover_named` files. It also finds skips that hide load errors, single-file source pins, source-text-only `acs` predicates and four fakes that differ from the real tool. One of the four is a production defect in the bridge. It gives refinements R1 to R8. |
| [plans/test-quality-audit-2026-11.md](plans/test-quality-audit-2026-11.md) | Plan: an analyzer with a baseline ratchet in shards and a metric fix for `apicover`. It adds verified fakes with contract suites, and test-only fix items with a recorded mutation check. Components T1 to T10 and P1. |
