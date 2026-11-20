# Build Explanation — Cycle 2562

## Build Binding
- Cycle: 2562
- Base SHA: 499c111b5f3a1ddb8e30fec6de050faf4d5233ce

## Summary
The orchestrator persona (`agents/evolve-orchestrator.md`) now frames all decision sections as persona proposals with Go kernel decisions. It also attributes all ship, release, dispatch, carryover reconciliation, and failure-state scripts to the Go kernel.

## Rationale
The salvaged cycle-2551 snapshot rewrote the five decision sections and verdict table. However, four lines in Layer-P Memo Phase Contract and Path conventions still attributed `reconcile-carryover-todos.sh`, `subagent-run.sh`, and `code-audit-warn` recording to the persona. Attributing those scripts and failure-state writes explicitly to the Go kernel satisfies the cycle-2562 ACS predicates with minimal diff. Read-only `cycle-state.sh get` queries remain intact.

## Changed Areas
- `agents/evolve-orchestrator.md` — updates the decision sections and verdict table so the persona only proposes transitions while the Go kernel decides. Also updates the memo and path sections so the Go kernel owns dispatch, carryover reconciliation, shipping, and failure-state writes.
- `go/internal/prompts/orchestrator_persona_test.go` — adds `TestOrchestratorPersonaDecisionSectionsProposeOnly` to verify proposal-and-kernel framing and reject imperative ship, release, or state scripts.
- `.evolve/evals/orchestrator-persona-decision-sections-propose-only.md` — pins the four `score_cap` evaluation criteria to the `go/acs/cycle2562` predicates.
- `go/acs/cycle2562/predicates_test.go` — enforces line-level script attribution and decision-section framing across `agents/evolve-orchestrator.md`.

## Design Decisions
Read-only `cycle-state.sh get` examples and negated `Do NOT propose subagent-run.sh` guidance stay intact. The memo and path lines name the Go kernel as executor instead of deleting operational context.

## Verification
`evolve acs suite --cycle 2562` passes with `verdict=PASS` (`green=267`, `red=0`, `skip=53`, `total=320`). `go test -tags acs -count=1 -v ./acs/cycle2562` passes 4/4 predicates. `go test -count=1 ./internal/prompts/...` passes, `evolve skills check` passes, and `evolve docs ste-lint --strict` passes with zero findings.

## Compatibility
Persona prose and prompt tests remain compatible with `core.Orchestrator` and `go/internal/prompts` loaders.

## Limitations
Other persona files under `agents/` were not audited in this cycle. `agents/evolve-orchestrator.md` remains an archived non-LLM reference persona.
