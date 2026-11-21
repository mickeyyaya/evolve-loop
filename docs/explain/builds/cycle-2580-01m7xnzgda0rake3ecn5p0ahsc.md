# Build Explanation — Cycle 2580

## Build Binding
- Cycle: 2580
- Base SHA: 5badf79df3dcdab8b63e85ad8bd64dbdf64221d6

## Summary
The headless agy manifest no longer carries its own tier table. It adopts the agy family table through `model_tier_map_from`.

## Rationale
One family keeps one declaration. The headless copy held a stale single model for every tier, and the family table in the tmux manifest is the maintained one. The loader already resolves `model_tier_map_from`, as codex does.

## Changed Areas
- `go/internal/bridge/manifests/agy.json` — removes `model_tier_map`, adds `model_tier_map_from: agy-tmux`, and updates the note.
- `go/internal/bridge/agy_tier_map_test.go` — relational test that the loaded agy table equals the family table.
- `go/internal/bridge/seat_launch_test.go` — `seatHome` now clears `CLAUDE_CONFIG_DIR` (set to empty, then unset), so seat-launch tests do not read the host Claude config directory.
- `go/internal/bridge/parallel_serial_tests_test.go` — registers the new test in the serial or parallel classification table.
- `go/acs/cycle2580/predicates_test.go` — acceptance predicates for this task.
- `.evolve/evals/agy-tier-map-single-source.md` — eval graders for this task.

## Design Decisions
No loader code changed. The existing `model_tier_map_from` seam is the single control for this behavior.

## Verification
The cycle ACS suite reports green=267 red=0. The profiles and setup packages pass.

## Compatibility
The agy manifest schema is unchanged. Loaded tier values now match the family table, not the old single model.

## Limitations
Some bridge tests fail from the host live model catalog overlay; they do not touch agy.json.
