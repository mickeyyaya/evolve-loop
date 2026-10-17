# ADR-0140: The type checker answers callers, and each query goes to its best measured backend

- **Status:** Proposed (2026-10-17).
- **Plan:** [code-index-backends-2026-10](../../plans/code-index-backends-2026-10.md). **Research:** [code-index-backends-2026-10](../../research/code-index-backends-2026-10.md). **Design:** [code-index-cli](../code-index-cli.md), section "Backends and the benchmark".
- **Relates to:**
  - [ADR-0138](0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md): this ADR fills the backend set that ADR-0138 decisions 2 and 3 opened. It reverses one rejected option of ADR-0138, the native Go index, because F2.13 removes the reason ("it adds a library to `go.mod`").
  - ADR-0138 decision 11 kept the trail-code MCP registrations for one wave. On 2026-10-17 the operator forbade MCP, and [PR #933](https://github.com/mickeyyaya/evolve-loop/pull/933) removes the registrations (open on 2026-10-17). When it lands, it closes ADR-0138 decision 11.
  - [ADR-0137](0137-one-bridge-factory-resolves-every-llm-cli-and-the-manifest-holds-its-facts.md): the backend set stays built and injected, with no `init()` registry.
- **Decision numbers.** The decisions below have the same numbers as D1 to D11 of the plan.
- **Evidence:**
  - F2.15: against a type-checked truth, trail-code gives callers a precision of 0.11 and a recall of 0.32. graphify gives 0.93 and 0.75, but only after a second call with the right file for 18 of 40 names. Without it, the recall is 0.24.
  - F2.19: the truth agrees with a grep-and-read check on 130 of 130 call sites of 10 symbols.
  - F2.20: a prototype of the type-checked index gives 1.00 and 1.00 for a package-qualified name, and 0.44 and 1.00 for a bare name.
  - F2.6, F2.12: graphify is about 10 times slower for each call than trail-code. The host load average was 12 to 22 during the runs, so these times are noisy.
  - The operator's rule of 2026-10-17: "Forbid any mcp use, we only use cli to commute with other services".

## Context

The operator asked if graphify beats trail-code for code look-up, through the `evolve code` CLI. We measured both against the Go type checker on 40 symbols. Neither tree-sitter graph gives a reliable `callers` answer on this repository. trail-code merges every symbol of the same name, and graphify drops calls through interfaces and needs a unique name.

The type checker is exact for static calls of a qualified name, and an independent grep check agrees with it. It is fast enough to run at each boundary. trail-code stays the backend of `find` and is the only backend for `map`. The capability route of ADR-0138 can already send each query to a different backend. The new rule forbids MCP, so each backend must be a CLI or an `evolve` verb.

## Decision

1. A new `gotypes` backend serves `callers` and `api` from a type-checked index.
2. The `gotypes` index uses the Go standard library only, and adds no library to `go.mod`.
3. The boundary builds the `gotypes` index after the trail refresh. A phase only reads it.
4. A qualified name gives an exact answer. For a bare name, each hit names its definition.
5. Each hit names its kind. A method reached only through an interface gets that interface's call count.
6. The default route is `["gotypes", "trail"]`. trail-code serves `find` and `map`.
7. `evolve code bench` measures each named backend against the truth, with a grep check of a sample.
8. A change to the default route needs a bench table in the change.
9. Every backend is a CLI or an `evolve` verb. No backend uses MCP.
10. The boundary refreshes only the route's indexes. graphify refreshes only inside a bench run.
11. graphify stays a bench-only run. An adapter is built only when a bench shows graphify best.

## Consequences

**Good:**
- `callers` becomes exact for static calls of a qualified name. By F2.15 and F2.20, the precision goes from 0.11 to 1.00, and the recall from 0.32 to 1.00. For a bare name, the recall is 1.00 and the precision 0.44, with each hit named by its definition.
- The type-checked index needs no Node, no Python and no outside binary.
- The bench turns each later backend choice into a measured claim, with a check of the truth itself.
- The agents see no change: the same four commands answer.

**Bad:**
- The boundary does one more step, about 5 s, with a peak memory of about 1.7 GB.
- The static answer does not give the concrete target of an interface call (F2.14). Decision 5 tells the agent so.
- `gotypes` serves Go only. A question about another language still goes to trail-code.
- The independent check covered 10 functions and no interface methods (F2.19).

**What changes this decision:**
- If the K3 usage log shows that agents ask for interface implementations, add a CHA or VTA mode (P3.8).
- If a later bench shows graphify best for a query type, at a latency that plan Q5 accepts, build its adapter.
- If the boundary step takes more than 15 s or fails on two waves in a row, the route goes back to `["trail"]`.

## Rejected

| Option | What it offers | Why it is not chosen |
|---|---|---|
| A. Keep trail-code only | No new code. It is fast (F2.12). | F2.15: a callers precision of 0.11. F2.9: a common name returns hundreds of wrong callers. |
| B. graphify as the first backend | One more graph, EXTRACTED and INFERRED tags, clusters. | F2.15: a callers recall of 0.75 only with the right file for 18 of 40 names, and 0.06 for interface methods. F2.6: about 10 times slower for each call. F2.1: a Python supply chain. |
| A graphify adapter now | A CLI-only graphify backend behind `Backend`. | YAGNI: graphify wins no query type today. The bench can run its CLI from outside. |
| graphify, gopls, codegraph or Serena through MCP | Resident servers and rich tools. | The operator's rule of 2026-10-17 forbids MCP. |
| The `gopls` CLI as the type-checked backend | The Go team's own tool. | P3.7: not installed, experimental, and it takes a position, not a name. F2.13 gives the same static answer from the standard library. |
| `go/callgraph` CHA or VTA now | Concrete targets of interface calls. | P3.8: it adds `golang.org/x/tools` to `go.mod`. No usage data asks for it yet. |
| A fixed latency ceiling for "best" | A simple rule for the bench. | No measure gives the latency that an agent accepts (F2.21). Plan Q5 measures it. |
