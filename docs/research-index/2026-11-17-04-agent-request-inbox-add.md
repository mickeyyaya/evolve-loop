## 2026-11-17 — The pipeline files the inbox items that phase agents request

| Document | What it records |
|---|---|
| [plans/agent-request-inbox-add-2026-11.md](plans/agent-request-inbox-add-2026-11.md) | Plan: the request channel refused 11 `inbox-add` requests, and no item was filed. Four designs scored; a deterministic intake with no LLM judge wins. Decisions D1 to D13 add an executor for each kind and one shared intake. They add a stage for each kind, with `enforce` as the default. Each item that is not filed gets an ERROR signal, and each decision signal gets its reason. |
