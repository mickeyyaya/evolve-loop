# Build Explanation — Cycle 2330

## Build Binding
- Cycle: 2330
- Base SHA: 5a85ad72d5ac83606e2c2c6e21a286d5a877a6a8

## Summary
The claude manifest `--settings` data now sets `prStatusFooterEnabled` false next to the bypass acceptance. A project-scoped claude seat no longer polls gh for the PR status footer.

## Rationale
The key joins the one existing `--settings` JSON value. This adds no flag and no second configuration surface.

## Changed Areas
- `go/internal/bridge/manifests/claude-tmux.json` — the project scope `--settings` value now carries `prStatusFooterEnabled` false, and the note says so.
- `docs/architecture/packages/internal-bridge.md` — the clean-boot seat paragraph shows the new `--settings` value.
- `go/acs/cycle2330/predicates_test.go` — four predicates from the TDD phase pin the behavior.
- `go/internal/bridge/realizer_wiring_test.go` — the golden launch command pins the new `--settings` value.
- `.evolve/evals/rebuild-pr-1251-ad1-2-pr-status-footer.md` — the eval for the task.

## Design Decisions
Data in the manifest, not Go code. The realizer passes the value through unchanged.

## Verification
The four C2330 predicates pass (4/4 PASS). The ACS suite reports green=266 red=0.

## Compatibility
The bypass acceptance key is unchanged. Other CLIs are not touched.

## Limitations
The footer stays on for seats without project scope.
