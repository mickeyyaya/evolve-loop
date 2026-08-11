---
score_cap:
  - criterion: "A durable regression test pins that a stray unmatched backtick does not perturb ClassifyBadVerdict's classification"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestClassifyBadVerdict_UnmatchedBacktickDoesNotMisclassify$' -v ./internal/deliverable | grep -q '^--- PASS: TestClassifyBadVerdict_UnmatchedBacktickDoesNotMisclassify'"
  - criterion: "A tripwire test fails if the removed isQuotedEcho/insideStringLiteral adjacency heuristic is reintroduced"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestNoQuotedEchoRegression$' -v ./internal/deliverable | grep -q '^--- PASS: TestNoQuotedEchoRegression'"
  - criterion: "The buggy adjacency heuristic stays absent from the production classifier"
    max_if_missing: 6
    evidence: "! grep -qE 'isQuotedEcho|insideStringLiteral' go/internal/deliverable/salvage_instrument.go"
  - criterion: "The deliverable package suite stays green (no regression from the added tests)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/deliverable"
---

# Eval: salvage backtick regression guard

> Pins the closure of the cycle-1406/1407 salvage-instrument defect, in which
> `isQuotedEcho` (`go/internal/deliverable/salvage_instrument.go`) treated
> backtick *adjacency* as proof of a "quoted echo" without requiring the backtick
> run to close. A single unmatched backtick sitting in prose ahead of a report's
> own malformed verdict sentinel therefore produced a false "genuinely absent,
> not recoverable" classification — measurement poisoning of the very baseline
> the instrumentation layer exists to measure honestly.
>
> The production bug is already fixed: cycle-1438 scout verified live (grep,
> `git log`, full-package `go test`) that `isQuotedEcho` and its O(n²)
> `insideStringLiteral` helper no longer exist, having been replaced wholesale by
> the 4-shape precedence classifier at `salvage_instrument.go:123-167`. What was
> missing was any committed evidence: no test pinned the fixed behaviour against
> the *current* implementation, so the class of bug could be reintroduced
> silently and the carryover had no durable evidence trail. This eval is that
> trail. It caps the audit score whenever the regression coverage disappears —
> not merely whenever the bug returns.
>
> Note the shape of the first two evidence commands: they demand a real
> `--- PASS: <name>` line rather than trusting the exit code, because
> `go test -run` over a pattern matching nothing exits 0 with "no tests to run".
> An exit-code-only check would green a deleted test. Source incident: cycle 1407
> (`code-audit-fail` retro entry, carryover `todo-salvage-quoted-echo-backtick`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| backtick-invariance | Paired-control test proves a stray unmatched backtick changes no classification | 8/10 | `go test -run '^TestClassifyBadVerdict_UnmatchedBacktickDoesNotMisclassify$' -v` emits its `--- PASS:` line |
| symbol-tripwire | Guard test fires if `isQuotedEcho`/`insideStringLiteral` return | 7/10 | `go test -run '^TestNoQuotedEchoRegression$' -v` emits its `--- PASS:` line |
| heuristic-absent | The adjacency heuristic stays out of the production classifier | 6/10 | `grep -qE` over `salvage_instrument.go` finds neither symbol |
| no-regression | The touched package suite remains green | 5/10 | `go test ./internal/deliverable` |
