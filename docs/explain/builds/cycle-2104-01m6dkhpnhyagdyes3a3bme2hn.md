# Build Explanation — Cycle 2104

## Build Binding
- Cycle: 2104
- Base SHA: cfe3cc795d244cd42a9907d3cba7ceaa1a47791e

## Summary
The codex plan_question rule now reads the last 14 screen rows instead of 9, so five-option and wrapped pickers stay inside its window.

## Rationale
With 9 rows the window had one row of slack over an 8-row picker. The wrapped five-option pickers, at 10 and 11 rows, pushed the anchor out and the rule went silently inert. The change edits one data value, which is the smallest fix and adds no code path.

## Changed Areas
- `go/internal/bridge/manifests/codex-tmux.json` — tail_lines 9 becomes 14 and the rule note records why.
- `go/internal/bridge/autorespond_planmode_headroom_test.go` — tests for five-option, wrapped, over-long and answered pickers.
- `go/acs/cycle2104/predicates_test.go` — ACS predicates that run those tests.
- `.evolve/evals/plan-mode-soak-preconditions.md` — eval for the task.
- `.evolve/evals/plan-mode-codex-tail-headroom.md` — eval for the headroom criteria.

## Design Decisions
The value 14 gives three rows of slack over the largest fixture of 11 rows. It stays inside the 30-row bound. The rule therefore reads the live screen only. The bottom anchor stays, so answered pickers do not fire again.

## Verification
The headroom tests and the existing plan-mode tests pass. The ACS suite for cycle 2104 reports red=0.

## Compatibility
The rule name, regex and policy are unchanged.

## Limitations
The live codex capture, the effort assertion and the /plan entry plumbing stay console work.
