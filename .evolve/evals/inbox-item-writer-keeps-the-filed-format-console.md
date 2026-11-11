---
score_cap:
  - criterion: "evolve inbox edit --set weight=0.4 on an item filed by evolve inbox add changes exactly one line in git diff"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2262_001_' ./acs/cycle2262"
  - criterion: "a criterion containing <id> (and & or >) survives a rewrite byte for byte, with no HTML escape"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2262_002_' ./acs/cycle2262"
  - criterion: "an edit and its undo return the filed bytes with no git diff"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2262_003_' ./acs/cycle2262"
  - criterion: "the lifecycle package keeps the filed format at the writer seam"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestUpdateItemJSON_KeepsTheFiledFormat' ./internal/inboxmover/lifecycle"
---

# Eval: the inbox item writer keeps the filed format

> This eval pins inbox item `inbox-item-writer-keeps-the-filed-format-console`. `evolve inbox add` files an item with two-space indent and no HTML escape. `rewriteItemJSON` rewrote it with `json.Marshal`: one line, sorted keys, `<id>` escaped. A curation or route stamp then showed as a whole-file diff. Source: cycle 2262 bug-reproduction and TDD contract, `go/acs/cycle2262/predicates_test.go`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-line-diff | weight stamp changes one line | 8/10 | `TestC2262_001_` |
| no-html-escape | `<id>` survives byte for byte | 8/10 | `TestC2262_002_` |
| round-trip | edit then undo leaves no diff | 6/10 | `TestC2262_003_` |
| writer-seam | `UpdateItemJSON` keeps the filed format | 6/10 | `TestUpdateItemJSON_KeepsTheFiledFormat` |
