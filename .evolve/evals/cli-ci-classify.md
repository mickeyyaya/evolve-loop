---
score_cap:
  - criterion: "evolve ci classify <run-id> prints the run line, the header and one row per failing test with its rule, its four evidence facts and its label, then retry-safe"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_013' ./acs/cycle1792"
  - criterion: "a failing test in a touched package is real, one also red on the base is pre-existing, and an untouched one green on --rerun is flake-evidence"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_014' ./acs/cycle1792"
  - criterion: "the labels come from one rule table (base-red, touched, rerun-green, recurred-on-main, default) and each row has its own test; recurrence excludes runs on the head and after the target run"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_015' ./acs/cycle1792"
  - criterion: "exit 0 only when every red is pre-existing or flake-evidence (or the run is green with failures []), 1 for real/unknown or an unfinished run, 2 when gh fails while gathering evidence, 10 on usage; --rerun is the only side effect"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_01[12678]' ./acs/cycle1792"
  - criterion: "CLAUDE.md's CI Failures section cites a runnable evolve ci classify and runtime-reference.md documents it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_019' ./acs/cycle1792"
---

# Eval: evolve ci classify

> Pins `evolve ci classify <run-id|pr:N|sha:H> [--json] [--rerun]`, cycle 1792 (inbox item cli-ci-classify). CLAUDE.md forbids retrying an unclassified CI red, yet the loop only detected red and the operator read `gh run view --log-failed` by hand. The predicates drive the real binary against a git fixture whose head touches one package and a fake `gh` that serves the target run, the base run and recent main runs, so every label is earned from evidence: touched (diff vs package dir), base red, recurrence on main (self and later runs excluded), and a green newer attempt after `--rerun`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| evidence-rows | one row per failing test with evidence and label | 8/10 | `go test -tags acs -run 'TestC1792_013' ./acs/cycle1792` |
| three-labels | real / pre-existing / flake-evidence from the acceptance | 9/10 | `go test -tags acs -run 'TestC1792_014' ./acs/cycle1792` |
| rule-table | one table, a test per row, consistent rule-to-label | 8/10 | `go test -tags acs -run 'TestC1792_015' ./acs/cycle1792` |
| exit-codes | retry-safe 0, real/unknown 1, gh I/O 2, usage 10, rerun only on request | 7/10 | `go test -tags acs -run 'TestC1792_01[12678]' ./acs/cycle1792` |
| ci-doc | CLAUDE.md cites the verb | 4/10 | `go test -tags acs -run 'TestC1792_019' ./acs/cycle1792` |
