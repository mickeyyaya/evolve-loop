# Build Explanation — Cycle 2561

## Build Binding
- Cycle: 2561
- Base SHA: 499c111b5f3a1ddb8e30fec6de050faf4d5233ce

## Summary
All agy model and routing documents now match `.evolve/policy.json`, `go/internal/bridge/manifests/agy-tmux.json`, and `evolve cli-routing show`.
[ADR-0150](../../architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md) and [ADR-0136](../../architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md) link each other, and `AGENTS.md` states the checked-in chains, `share_pct` 50, and Gemini 4 models.

## Rationale
Cycle 2547 updated `docs/reference/model-routing.md` and related docs, and cycle 2561 found that `AGENTS.md` still stated agy-first defaults, `Gemini 3.8 Flash`, and a stale `deep` chain.
Updating `AGENTS.md` to match `.evolve/policy.json` and keeping dated notes in historical records aligns every document without rewriting history.

## Changed Areas
- `AGENTS.md` — updates the `cli_routing` bullets to state `clis`, `default`, `agents.builder`, `tiers.deep` with `share_pct` 50, `tiers.top`, and Gemini 4 models.
- `.evolve/evals/agy-docs-behind-the-code.md` — defines the five score-cap criteria and test commands for `model-routing.md`, ADRs, and `AGENTS.md`.
- `docs/reference/model-routing.md` — updates `agy-tmux` models to Gemini 4 Hydrogen and Argon and aligns tier rows and bullets with `cli_routing` and `share_pct` 50.
- `docs/architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md` — adds the `- **Amended by:**` link to ADR-0150 so readers see the updated `deep` and `top` chains.
- `docs/architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md` — adds the `- **Amends:**` link to ADR-0136 so the amendment relation goes both ways.
- `docs/operations/runtime-reference.md` — updates the CLI routing table rows to state `default`, `tiers.deep` with `share_pct` 50, `tiers.top`, and Gemini 4 Hydrogen High for `builder`.
- `docs/architecture/model-routing-policy.md` — updates the `tdd-engineer` and `builder` spine table notes to match `Gemini 4 Hydrogen (High)` and `cli_routing.default`.
- `docs/explain/model-routing.html` — updates the checked-in table bullet to match `default`, `builder`, `tiers.deep`, `tiers.top`, and Gemini 4 Hydrogen High.
- `docs/plans/model-tier-policy-2026-10.md` — adds a dated note with the current `cli_routing` chains and `share_pct` 50 while keeping the historical plan text.
- `docs/research/model-tier-policy-2026-10.md` — adds a dated note with the current `cli_routing` chains and `Gemini 4 Hydrogen (High)` while keeping the historical research record.
- `docs/plans/bridge-factory-2026-10.md` — adds a dated note with the current `agy-tmux` tier models and `cli_routing` chains while keeping the historical plan.
- `docs/architecture/agent-bridge-factory.md` — adds a dated note that states the current `agy-tmux` models and points to `docs/reference/model-routing.md`.
- `docs/plans/model-currency-2026-10.md` — adds a dated note with the current `agy-tmux` Gemini 4 models and `cli_routing` chains while keeping C1–C5 history.
- `docs/plans/cli-routing-table-2026-10.md` — adds a dated note with the current `cli_routing` chains and `share_pct` 50 while keeping the L1/L2 design record.
- `docs/architecture/adr/0104-fallback-is-a-property-of-the-bridge-handle.md` — adds a dated note with the current checked-in `cli_routing` chains.
- `docs/architecture/adr/0119-one-routing-table-one-resolver.md` — adds a dated note with the current checked-in `cli_routing` chains and the ADR-0150 amendment.
- `docs/private/research/archived-2026-11-20/superseded-predicate-packages/cycle2547/predicates_test.go` — archives the unshipped cycle 2547 predicate package superseded by `go/acs/cycle2561`.
- `docs/private/research/archived-2026-11-20/unshipped-build-explanations/cycle-2547-01m7twbyy0wqew8kcnmgjqhvdp.md` — archives the unshipped cycle 2547 build explanation superseded by this cycle 2561 explanation.
- `go/internal/cliroute/model_routing_doc_test.go` — adds tests that verify `docs/reference/model-routing.md` and ADR-0136/ADR-0150 against `.evolve/policy.json` and reject drift.
- `go/internal/cliroute/agents_md_routing_test.go` — adds tests that verify `AGENTS.md` routing bullets against `.evolve/policy.json` and `agy-tmux.json` and reject drift.
- `go/acs/cycle2561/predicates_test.go` — adds the four cycle 2561 ACS predicates that execute the `internal/cliroute` doc-vs-code and drift tests.

## Design Decisions
- Keep historical plans, research, and earlier ADRs intact with dated header notes, and update live specifications (`AGENTS.md`, `docs/reference/model-routing.md`, `docs/operations/runtime-reference.md`, `docs/architecture/model-routing-policy.md`, `docs/explain/model-routing.html`) directly.
- Enforce doc-vs-code parity with table-driven Go tests in `go/internal/cliroute` instead of generating prose tables. This cycle adds both `model_routing_doc_test.go` and `agents_md_routing_test.go`, and they share one table-driven pattern.

## Verification
`go test -count=1 ./internal/cliroute/...` and `go test -tags acs -count=1 ./acs/cycle2561/...` verify chain sync, ADR links, and drift detection.
`evolve acs suite --cycle 2561` and `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added` verify the regression suite and STE rules.

## Compatibility
Only documentation files and Go tests change; production Go code and `.evolve/policy.json` remain unchanged.

## Limitations
The historical sections inside the dated 2026-10 plans, research, and ADRs keep their original observations under the dated header notes.
Content under `landing/` remains out of scope.
