# Build Explanation — Cycle 2134

## Build Binding
- Cycle: 2134
- Base SHA: a06a61e1d75306d6b5978ef9d1eb5e77eed52073

## Summary
The offline default model of the agy `top` tier changes from Gemini 4 Argon (High) to Gemini 4 Argon (Medium). The pin tests and the routing table follow.

## Rationale
Decision D4 of the model tier policy plan asks for Medium at `top`, because High drains the Argon pool faster. The manifest line is the smallest change that sets the offline default.

## Changed Areas
- `go/internal/bridge/manifests/agy-tmux.json` — the `top` entry of `model_tier_map` now names Gemini 4 Argon (Medium).
- `go/internal/bridge/agy_model_names_test.go` — the pin test expects Medium at `top`.
- `go/internal/setup/setup_test.go` — the tier-model test expects Medium at `top`.
- `go/internal/bridge/parallel_serial_tests_test.go` — lists the pin test as serial, because it now swaps the catalog directory seam.
- `go/internal/setup/agy_claude_detect_test.go` — the three agy-claude detect tests isolate the live catalog through the project root.
- `docs/reference/model-routing.md` — the routing table shows Medium for agy at `top`.
- `docs/plans/model-tier-policy-2026-10.md` — the P1 row carries a Built marker.

## Design Decisions
The manifest stays the single source of the offline default. The change adds no code, flag or setting.

## Verification
The pin tests read an empty catalog directory. They pass in a root with a live catalog. The bridge, setup and clihealth packages pass in both roots. The cycle 2134 predicates pass.

## Compatibility
A live catalog still overrides the manifest. Pin support for that case is item P2 and stays open.

## Limitations
A live catalog that names Argon (High) for `top` still wins at dispatch until P2 lands.
