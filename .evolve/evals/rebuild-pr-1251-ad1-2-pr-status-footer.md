---
score_cap:
  - criterion: "a project-scoped claude seat realizes --settings with prStatusFooterEnabled false"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2330_001 ./acs/cycle2330"
  - criterion: "the bypass acceptance stays in the same --settings data"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2330_002 ./acs/cycle2330"
  - criterion: "every manifest --settings value declares the footer off"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2330_003 ./acs/cycle2330"
---

# Eval: claude PR status footer off in the manifest --settings data

> Pins the behaviour of PR 1251 (plan component AD1-2 of architecture-decisions-2026-11): the claude manifest --settings data sets prStatusFooterEnabled false, so a cell does not poll gh for the footer. Source: inbox item rebuild-pr-1251-ad1-2-pr-status-footer, cycle 2330.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| realized-flag | RealizeFor claude-tmux project scope carries the key false | 6/10 | `go test -tags acs -run TestC2330_001 ./acs/cycle2330` |
| acceptance-kept | skipDangerousModePermissionPrompt stays true | 7/10 | `go test -tags acs -run TestC2330_002 ./acs/cycle2330` |
| manifest-walk | no manifest --settings value omits the key | 6/10 | `go test -tags acs -run TestC2330_003 ./acs/cycle2330` |
