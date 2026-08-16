# Eval: role-prompt-duplication-trim

**Slug:** role-prompt-duplication-trim
**Phase target:** tdd + build

## Task

Trim repeated, non-load-bearing prose from the Scout, Builder, Auditor, and
Orchestrator prompt sources while keeping every phase gate, artifact contract,
and explicit stop condition available to the agent.

## Acceptance Criteria

### AC1 — authoritative prompt invariants remain present [code]

Add a Go test in the existing prompt/skill validation surface that loads all
four role documents and verifies their required artifact name, challenge-token
requirement, completion/stop condition, and references to the authoritative
phase contract still exist after trimming.

```bash
cd go && go test ./internal/skillcheck/... -run 'RolePrompt.*Invariant|Prompt.*Invariant' -count=1
```

Expected: `PASS` with a non-zero matching-test count.

### AC2 — duplicated inline material is replaced by on-demand references [code]

The test must compare a fixture/baseline and assert each changed role prompt is
smaller while its mandatory invariants above are preserved. It must fail if a
document is merely truncated past an invariant.

```bash
cd go && go test ./internal/skillcheck/... -run 'RolePrompt.*Invariant|Prompt.*Invariant' -count=1
```

Expected: `PASS`.

### AC3 — skill generation remains coherent [code]

```bash
cd go && go test ./internal/skillcheck/... -count=1 && go run ./cmd/evolve skills check
```

Expected: tests pass and skills check reports no generated-facts drift.

### AC4 — edge case: reference-tail stripping retains the compact contract [code]

```bash
cd go && go test ./internal/phases/runner -run '^TestRun_CompactPrompts_' -count=1
```

Expected: `PASS`; compact mode removes only the on-demand tail, not the role's
required contract text.
