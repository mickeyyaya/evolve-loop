# Build Explanation — Cycle 2547

## Build Binding
- Cycle: 2547
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
The agy model and routing documents now match `.evolve/policy.json`, `go/internal/bridge/manifests/agy-tmux.json`, and `evolve cli-routing show`.
[ADR-0150](../../architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md) and [ADR-0136](../../architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md) now link each other through `- **Amends:**` and `- **Amended by:**` lines.

## Rationale
`docs/reference/model-routing.md` still held the 2026-10-17 chains and Gemini 3.8 Flash names. Meanwhile `.evolve/policy.json` moved `deep` to `claude`, `agy`, `agy-claude` (`share_pct` 50) and pinned `fast` and `balanced` to Gemini 4 Hydrogen.
Updating the specification of record and adding dated notes to point-in-time plans and research records aligns every document without rewriting history.

## Changed Areas
- `docs/reference/model-routing.md` — updates `agy-tmux` models to Gemini 4 Hydrogen and Argon and aligns tier rows and bullets with `cli_routing`. Removes the stale `not yet` claim and states `share_pct` 50.
- `docs/architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md` — adds the `- **Amended by:**` link to ADR-0150 so readers see the updated `deep` and `top` chains.
- `docs/architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md` — adds the `- **Amends:**` link to ADR-0136 so the amendment relation goes both ways.
- `docs/operations/runtime-reference.md` — updates the four CLI routing table rows to state `default`, `tiers.deep` with `share_pct` 50, `tiers.top`, and Gemini 4 Hydrogen High for `builder`.
- `docs/architecture/model-routing-policy.md` — updates the `tdd-engineer` and `builder` spine table notes to match `Gemini 4 Hydrogen (High)` and `cli_routing.default`.
- `docs/explain/model-routing.html` — updates the checked-in table bullet to match `default`, `builder`, `tiers.deep`, `tiers.top`, and Gemini 4 Hydrogen High.
- `docs/plans/model-tier-policy-2026-10.md` — adds a dated note with the current `cli_routing` chains and `share_pct` 50 while preserving the historical plan text.
- `docs/research/model-tier-policy-2026-10.md` — adds a dated note with the current `cli_routing` chains and `Gemini 4 Hydrogen (High)` while preserving the historical research record.
- `docs/plans/bridge-factory-2026-10.md` — adds a dated note with the current `agy-tmux` tier models and `cli_routing` chains while preserving the historical plan.
- `docs/architecture/agent-bridge-factory.md` — adds a dated note that states the current `agy-tmux` models and points to `docs/reference/model-routing.md`.
- `docs/plans/model-currency-2026-10.md` — adds a dated note with the current `agy-tmux` Gemini 4 models and `cli_routing` chains while preserving C1–C5 history.
- `docs/plans/cli-routing-table-2026-10.md` — adds a dated note with the current `cli_routing` chains and `share_pct` 50 while preserving the L1/L2 design record.
- `docs/architecture/adr/0104-fallback-is-a-property-of-the-bridge-handle.md` — adds a dated note with the current checked-in `cli_routing` chains.
- `docs/architecture/adr/0119-one-routing-table-one-resolver.md` — adds a dated note with the current checked-in `cli_routing` chains and the ADR-0150 amendment.

## Design Decisions
- Keep historical plans, research, and earlier ADRs intact and append dated notes near the top, rather than rewriting point-in-time evidence.
- Update `docs/reference/model-routing.md`, `docs/operations/runtime-reference.md`, `docs/architecture/model-routing-policy.md`, and `docs/explain/model-routing.html` directly, because these files act as live specifications.

## Verification
`go test -count=1 ./internal/cliroute/...`, `go test -tags acs -count=1 ./acs/cycle2547`, and `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added` verify the chain sync, ADR links, and STE rules.

## Compatibility
Only documentation files change; runtime behavior, `policy.json`, and Go code remain unchanged.

## Limitations
The historical sections inside the dated 2026-10 plans, research, and ADRs keep their original observations under the dated header notes.
