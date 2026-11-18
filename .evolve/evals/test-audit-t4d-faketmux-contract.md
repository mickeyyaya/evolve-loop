---
score_cap:
  - criterion: "the contract table passes on faketmux and, where tmux is present, on real tmux"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestContract$' ./internal/faketmux/"
  - criterion: "capture-pane without -p prints nothing on faketmux, as on real tmux"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestContract/fake/capture_pane_without_p_prints_nothing$' ./internal/faketmux/"
  - criterion: "a carriage return moves the cursor to the line start on faketmux, as on real tmux"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestContract/fake/carriage_return_overwrites_from_the_line_start$' ./internal/faketmux/"
  - criterion: "resize-window refuses a bad size and reports the new size on faketmux, as on real tmux"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestContract/fake/resize_window_(sets_the_reported_size|refuses_a_bad_size_and_keeps_the_size)$' ./internal/faketmux/"
  - criterion: "the bridge jiggle flags -L 1 and -R 1 still resize by the adjustment on faketmux"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestContract/fake/resize_window_nudge_flags_move_the_size_by_the_adjustment$' ./internal/faketmux/"
---

# Eval: faketmux contract suite against real tmux (T4)

> Pins the faketmux part of T4 (D8, F3.7). `contract_test.go` holds one table of cases that runs on faketmux and, where tmux is on PATH, on real tmux with a private `-L` socket. Each expected value was recorded from tmux 3.8. Source incident: the scout of cycle 2509 found that faketmux printed for `capture-pane` without `-p`, dropped a lone carriage return, and made `resize-window` a no-op.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contract-green | whole table passes on both backends | 4/10 | `go test -run '^TestContract$' ./internal/faketmux/` |
| capture-no-p | `capture-pane` without `-p` prints nothing | 6/10 | `TestContract/fake/capture_pane_without_p_prints_nothing` |
| carriage-return | lone CR overwrites from the line start | 6/10 | `TestContract/fake/carriage_return_overwrites_from_the_line_start` |
| resize | bad size refused, new size reported | 6/10 | `TestContract/fake/resize_window_*` |
| jiggle | `-L 1` and `-R 1` keep working | 7/10 | `TestContract/fake/resize_window_nudge_flags_move_the_size_by_the_adjustment` |
