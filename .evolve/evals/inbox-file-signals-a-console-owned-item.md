---
score_cap:
  - criterion: "Filing an item whose files name a protected path emits exactly one INBOX_ITEM_FILED_CONSOLE with class=protected, the item id, its kind and the reason"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2531_001' ./acs/cycle2531"
  - criterion: "Filing a console-kind item emits the signal with class=kind"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2531_002' ./acs/cycle2531"
  - criterion: "Filing a lane-dispatchable item emits no such signal while a sibling console item does"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2531_003' ./acs/cycle2531"
  - criterion: "evolve events watch --channel inbox --until code=INBOX_ITEM_FILED_CONSOLE exits on a console-owned evolve inbox add"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2531_004' ./acs/cycle2531"
  - criterion: "A refused duplicate filing emits no second signal"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2531_005' ./acs/cycle2531"
  - criterion: "The code is registered and signal-codes.md lists it with the derived check passing"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2531_006' ./acs/cycle2531"
---

# Eval: Inbox filing signals a console-owned item

> Pins the filing-time signal INBOX_ITEM_FILED_CONSOLE. Before it, an item filed as console-owned
> (protected path, console kind or console route) emitted nothing, so the console learned of it only by
> polling wave supply. Every predicate drives the real `evolve inbox add` and `evolve events watch`
> binaries on a throwaway project root. Source incident: inbox item
> inbox-file-signals-a-console-owned-item (cycle 2531).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| protected-class | one signal, class=protected, id, kind, reason | 4/10 | `TestC2531_001` |
| kind-class | console kind gives class=kind | 5/10 | `TestC2531_002` |
| lane-silent | lane item emits nothing, console sibling does | 5/10 | `TestC2531_003` |
| watch-exit | events watch exits on the code | 4/10 | `TestC2531_004` |
| duplicate-silent | refused duplicate emits nothing | 6/10 | `TestC2531_005` |
| registry-and-doc | code registered, doc row, codes check green | 5/10 | `TestC2531_006` |
