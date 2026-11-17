---
score_cap:
  - criterion: "The design document docs/plans/split-cmd-evolve-2026-11.md exists and is not gitignored"
    max_if_missing: 2
    evidence: "test -f docs/plans/split-cmd-evolve-2026-11.md && ! git check-ignore -q docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "The document scores at least 3 groupings, one '### Grouping' heading each"
    max_if_missing: 3
    evidence: "test \"$(grep -c '^### Grouping' docs/plans/split-cmd-evolve-2026-11.md)\" -ge 3"
  - criterion: "Every '### Grouping' section carries a numeric score (a line with 'score' and a decimal number), so a heading without a score does not pass"
    max_if_missing: 3
    evidence: "awk '/^### Grouping/{if(h&&!s)bad=1;h=1;s=0;next} /^#{1,3} /{if(h&&!s)bad=1;h=0} h&&tolower($0)~/score/&&$0~/[0-9]+\\.[0-9]+/{s=1} END{if(h&&!s)bad=1;exit(bad||!NR)}' docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "The document names the first group to move"
    max_if_missing: 3
    evidence: "grep -qiE 'first group to move' docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "The document gives the move order and a red test for each move"
    max_if_missing: 5
    evidence: "grep -qiE '^#+ .*(move order|order)' docs/plans/split-cmd-evolve-2026-11.md && grep -qiE '^#+ .*red test' docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "The document answers Q2 of the parent plan and links the parent plan by name"
    max_if_missing: 4
    evidence: "grep -q 'Q2' docs/plans/split-cmd-evolve-2026-11.md && grep -q 'concurrent-edit-layout-2026-11' docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "The document passes evolve docs ste-lint --strict"
    max_if_missing: 4
    evidence: "cd go && go run ./cmd/evolve docs ste-lint --strict ../docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "The whole deliverable set passes ste-lint --strict: the plan, every file under solutions/edit-layout-c6-split-cmd-evolve-design, and the build explanation (audit H1)"
    max_if_missing: 4
    evidence: "cd go && go run ./cmd/evolve docs ste-lint --strict ../docs/plans/split-cmd-evolve-2026-11.md ../solutions/edit-layout-c6-split-cmd-evolve-design ../docs/explain/builds/cycle-2460-01m7mkbemx8ymy3884twe3j09h.md"
  - criterion: "The ci row of census 3.1 is a census-method row: a touch count, a numeric solo count that does not exceed it, and top partners written as 'name count'; the D2 ci count equals its touch count (audit M1, rounds 1 and 2)"
    max_if_missing: 5
    evidence: "awk -F'|' '/^\\| ci \\| [0-9]+ \\|/ && $5 !~ /^ *[0-9]+ *$/{c=$3+0; ok=($4 ~ /^ *[0-9]+ *$/ && $4+0<=c && $5 ~ /^ *[A-Za-z]+ [0-9]+/); hasci=1} /^\\| pr and ci \\|/{split($3,a,\" and \"); d=a[2]+0} END{exit !(hasci && ok && d==c)}' docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "The top partner in the ci row is the top partner that the census method of 3.1 finds in a fixed window of the git history, so a hand-made partner cell does not pass (audit M1, round 2)"
    max_if_missing: 5
    evidence: "top=$(git log --since=2026-10-18 --until=2026-11-18 --no-merges --name-only --format='@@%h' -- go/cmd/evolve/ | awk 'function flush(k){if(seen)for(k in g)if(\"ci\" in g&&k!=\"ci\")p[k]++; delete g; seen=0} /^@@/{flush(); next} NF{b=$0; sub(/.*\\//,\"\",b); if(b!~/^cmd_/)k=\"CORE\"; else{sub(/^cmd_/,\"\",b); sub(/[_.].*/,\"\",b); k=b} g[k]=1; seen=1} END{flush(); m=0; for(k in p)if(p[k]>m){m=p[k]; t=k} print t}') && row=$(grep -E '^\\| ci \\| [0-9]+ \\|' docs/plans/split-cmd-evolve-2026-11.md | awk -F'|' '$5 !~ /^ *[0-9]+ *$/{split($5,a,\" \"); print a[1]}') && test -n \"$top\" && test \"$top\" = \"$row\""
  - criterion: "The plan names the six inbox-add requests that file the build items (audit M2)"
    max_if_missing: 4
    evidence: "test \"$(grep -oE 'inbox-add-[1-6]' docs/plans/split-cmd-evolve-2026-11.md | sort -u | wc -l | tr -d ' ')\" -ge 6 || grep -qE 'inbox-add-1.*inbox-add-6|inbox-add-1\\.\\.6' docs/plans/split-cmd-evolve-2026-11.md"
  - criterion: "At least 3 inbox build items for the groups are filed (inbox files or inbox-add request files that name split-cmd-evolve)"
    max_if_missing: 4
    evidence: "test \"$( (grep -l 'split-cmd-evolve' /Users/danleemh/ai/claude/evolve-loop/runtime/.evolve/runs/cycle-2460/requests/inbox-add-*.json /Users/danleemh/ai/claude/evolve-loop/runtime/.evolve/inbox/*.json 2>/dev/null) | wc -l | tr -d ' ')\" -ge 3"
  - criterion: "The cycle changes no Go file (a design cycle moves no code)"
    max_if_missing: 6
    evidence: "test -z \"$(git status --porcelain -- go/ | grep -v ' go/evolve$')\""
---

# Eval: Design for splitting cmd/evolve into verb-group packages (C6)

> Pins the three acceptance criteria of the inbox item edit-layout-c6-split-cmd-evolve-design (cycle 2460):
> a design that scores at least 3 groupings and names the first group to move, docs that pass
> ste-lint --strict and docs review, and one build item for each group. The deliverable is a document, so
> the graders are shell checks on the document and the filed items, not Go predicates. Every grader is RED
> before Build because docs/plans/split-cmd-evolve-2026-11.md does not exist yet. The model grader
> still reads the scores: a heading with a number that no census figure supports does not satisfy criterion 1.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| doc-exists | The design document exists and is not ignored | 2/10 | `test -f docs/plans/split-cmd-evolve-2026-11.md` |
| three-groupings | At least 3 `### Grouping` headings | 3/10 | `grep -c '^### Grouping'` |
| scored-groupings | Each grouping section has a numeric score | 3/10 | `awk` over the sections |
| first-group | The first group to move is named | 3/10 | `grep -i 'first group to move'` |
| order-and-red-test | Move order and red test sections exist | 5/10 | `grep` on the headings |
| q2-and-parent | Q2 answered, parent plan linked | 4/10 | `grep Q2`, `grep concurrent-edit-layout-2026-11` |
| ste-lint | The document passes `docs ste-lint --strict` | 4/10 | `go run ./cmd/evolve docs ste-lint --strict` |
| ste-lint-all | The plan, the solution set and the explanation pass `--strict` | 4/10 | `go run ./cmd/evolve docs ste-lint --strict` over all three paths |
| ci-count-sourced | The `ci` row of census 3.1 has a numeric solo count and `name count` partners, and the D2 `ci` count equals its touch count | 5/10 | `awk` over both tables |
| ci-partner-recomputed | The top partner of the `ci` row equals the top partner that the 3.1 method finds in a fixed git window | 5/10 | `git log` and `awk` |
| requests-cited | The plan cites `inbox-add-1` to `inbox-add-6` | 4/10 | `grep` |
| build-items | At least 3 build items filed | 4/10 | `grep -l split-cmd-evolve` over requests and inbox |
| no-go-change | No Go file changed | 6/10 | `git status --porcelain -- go/` |
