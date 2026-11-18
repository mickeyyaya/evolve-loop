# Build Explanation — Cycle 2509

## Build Binding
- Cycle: 2509
- Base SHA: 77b47d7b30c21d1c86c2f50515327a3fb9c732a3

## Summary
The fake tmux now matches real tmux 3.8 for three behaviors. It prints from capture-pane only with the -p flag. A lone carriage return moves the cursor to the line start. The resize-window command changes the reported window size.

## Rationale
The bridge tests trust the fake. Each mismatch let a test pass on the fake and fail on real tmux. A contract table now runs the same cases on both backends. The expected values come from tmux 3.8. The fix keeps state in the existing session directory, so no new storage layer exists.

## Changed Areas
- `go/internal/faketmux/faketmux.go` — resize-window now takes the value flags -L, -R, -U and -D, so the bridge jiggle nudge parses as an adjustment.
- `go/internal/faketmux/server.go` — capture-pane prints only with -p. resize-window validates sizes from 1 to 10000 and applies absolute sizes and nudges. new-session records the width.
- `go/internal/faketmux/session.go` — the screen renderer overwrites cells after a carriage return. New helpers read and write the window width and height.
- `go/internal/faketmux/format.go` — the format expander knows window_width and window_height.
- `go/internal/faketmux/faults_test.go` — two fault cases pass -p, because capture-pane without -p no longer prints.
- `go/internal/faketmux/contract_test.go` — the contract table for both backends. The real backend skips when tmux is absent.
- `go/acs/cycle2509/predicates_test.go` — the ten acceptance predicates of this cycle.
- `.evolve/evals/test-audit-t4d-faketmux-contract.md` — the eval graders of the task.

## Design Decisions
The width lives in a small file in the session directory. The height stays in the pane record, because capture already reads it there. A failed resize computes every value first and writes nothing, so the size stays unchanged.

## Verification
The contract suite passes on the fake and on real tmux. The ACS suite for cycle 2509 reports 272 green and 0 red.

## Compatibility
Callers that use capture-pane -p see no change. The bridge always passes -p. The default size stays 80x24.

## Limitations
The renderer handles only the carriage return and line feed controls. It does not model cursor movement escapes.
