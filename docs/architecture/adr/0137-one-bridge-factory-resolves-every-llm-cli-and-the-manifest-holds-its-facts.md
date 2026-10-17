# ADR-0137: One bridge factory resolves every LLM CLI, and the manifest holds its facts

- **Status:** Accepted (2026-10-17, the operator approved the design set).
- **Plan:** [bridge-factory-2026-10](../../plans/bridge-factory-2026-10.md). **Research:** [bridge-factory-2026-10](../../research/bridge-factory-2026-10.md). **Design:** [agent-bridge-factory](../agent-bridge-factory.md).
- **Relates to:**
  - [agent-bridge-dispatch-contract](../agent-bridge-dispatch-contract.md): the dispatch invariants. This ADR extends the one-bridge rule from dispatch to every query.
  - [ADR-0119](0119-one-routing-table-one-resolver.md): one table decides which CLI runs. This ADR decides how every caller reaches that CLI.
  - [ADR-0136](0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md): its components Q1–Q5 query the CLIs, so they follow this ADR.
- **Evidence:**
  - B3, B4: the model list and the effort list run raw processes outside the bridge.
  - B5: the deep probe builds its own argv, and its agy probe passes no `--model`.
  - B7, B8: no factory resolves a driver name, and 19 sites build one by hand.
  - B1: the `/usage` path already works the right way: manifest data and one controller.

## Context

The operator asked that every query and action go through the agent bridge as one common layer, built as a factory. The audit found that the bridge already does this for dispatch, `/usage` and the `/model` picker. Other paths go around it. Each bypass repeats the start, the capture and the stop of a CLI. It also skips the sandbox and the pane hygiene of the bridge. A fact that a bypass learns, such as a new screen format, does not reach the code that needs it.

The usage path proves that the right shape works. Its CLI facts are manifest data, and one controller reaches the CLI. So the work is to move the other paths to the same shape, one at a time, with a guard after each move.

## Decision

1. `bridge.Resolve(set, family)` is the one factory for a family, over an injected `DriverSet`.
2. The factory reads only the manifest set.
3. Only the `bridge` package starts an LLM binary.
4. Every other package reaches a CLI through a bridge interface.
5. The facts of a CLI are manifest data, not Go branches on its name.
6. The model list, the effort list and the picker parse move into the manifest.
7. The deep probe runs through the driver of its family.
8. Each move adds a guard test that bans the shape it removes.

## Consequences

**Good:**
- A new CLI needs a manifest file, not changes in many packages.
- A screen change, such as the agy 1.4.0 `/usage` screen, is one data edit.
- Every probe gets the same sandbox, model and teardown as a dispatch. The deep probe no longer starts agy without a model, so it no longer uses quota from the Argon pool.
- The guards keep a removed bypass from coming back.

**Bad:**
- For a while, the old registry stays beside the factory. It is hidden global state (a Service Locator), so it feeds only the default `DriverSet` until a later step removes it.
- Each parse spec in the manifest needs a fixture of the real screen, and a screen can change.

**What changes this decision:** if a CLI needs logic that data cannot express, its driver keeps that logic in Go inside the bridge. The rule then still holds, because the code stays in the bridge.

## Rejected

| Option | What it offers | Why it is not chosen |
|---|---|---|
| Replace the registry and unify the driver interface at once | The cleanest end state: one interface and no globals. | Not chosen as one change: it touches every driver and every dispatch at once, so a defect stops all cycles. It is the end state that the later steps reach. |
| Add guards only, and keep the current call sites | Fast, and it stops new bypasses at once. | The current bypasses stay, with the hidden Argon spend of the deep probe. Its allow-list tends to grow. |
