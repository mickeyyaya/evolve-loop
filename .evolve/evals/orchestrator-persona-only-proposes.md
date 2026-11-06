---
score_cap:
  - criterion: "The orchestrator persona frontmatter claims no verdict authority and says the model proposes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2200_00[12]' ./acs/cycle2200"
  - criterion: "The orchestrator persona body says the kernel decides and claims no verdict authority"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2200_003' ./acs/cycle2200"
  - criterion: "The kernel clamps every adjudicator proposal and the rendered skills stay in sync"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2200_00[45]' ./acs/cycle2200"
---

# Eval: orchestrator-persona-only-proposes

> Pins ADR-0164 ruling 2: an LLM agent proposes and deterministic Go decides. The persona of
> `agents/evolve-orchestrator.md` said otherwise (the retrospective persona is a protected path, a console follow-up), while
> `core.Orchestrator.decideAfterAuditFail` already clamps each proposal. Source: cycle 2200 and the
> inbox item `orchestrator-persona-only-proposes`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| persona-frontmatter | No verdict-authority claim, and "proposes" present | 6/10 | `go test -tags acs -run TestC2200_001,002` |
| persona-body | Prose says the kernel decides | 6/10 | `go test -tags acs -run TestC2200_003` |
| kernel-and-render | Clamp tests pass; `skills check` has no drift | 8/10 | `go test -tags acs -run TestC2200_004,005` |

## Graders
- [code] `grep -c "makes verdict-driven decisions" agents/evolve-orchestrator.md` prints 0.
- [code] `grep -c "verdict authority" agents/evolve-orchestrator.md` prints 0.
- [code] `grep -ci "proposes" agents/evolve-orchestrator.md` prints at least 1; `evolve skills check` exits 0.
