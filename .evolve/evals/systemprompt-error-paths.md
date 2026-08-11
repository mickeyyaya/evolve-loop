---
score_cap:
  - criterion: "Five named error-path tests exist and pass in internal/systemprompt"
    max_if_missing: 8
    evidence: "cd \"$(git rev-parse --show-toplevel)/go\" && [ \"$(go test -count=1 -v -run '^TestResolve_(MalformedProfileJSON|AbsoluteSystemPromptFile|MissingSystemPromptFile|HyphenatedAgentName|EmptyAgent)$' ./internal/systemprompt/... 2>&1 | grep -c '^--- PASS: TestResolve_')\" -eq 5 ]"
  - criterion: "Full systemprompt suite still green (no regression)"
    max_if_missing: 7
    evidence: "cd \"$(git rev-parse --show-toplevel)/go\" && go test -count=1 ./internal/systemprompt/..."
  - criterion: "No production source modified — systemprompt.go untouched (test-only change)"
    max_if_missing: 6
    evidence: "! git -C \"$(git rev-parse --show-toplevel)\" diff --name-only HEAD | grep -q 'internal/systemprompt/systemprompt\\.go$'"
  - criterion: "Module still builds"
    max_if_missing: 6
    evidence: "cd \"$(git rev-parse --show-toplevel)/go\" && go build ./..."
---

# Eval: systemprompt error-path behavioral tests

> Permanent regression entry for the cycle-206 task `systemprompt-error-paths`.
> It pins five error/edge/malformed-input behavioral tests for
> `internal/systemprompt.Resolve` so the package's uncovered branches stay
> covered in future cycles: malformed profile JSON (no panic), an **absolute**
> `system_prompt_file` (the `filepath.IsAbs` true branch), a **missing**
> relative `system_prompt_file` (the silent `os.ReadFile` error leg), a
> **hyphenated** agent name (`tdd-engineer` → `EVOLVE_TDD_ENGINEER_SYSTEM_PROMPT`
> via `envchain.PhaseEnvKey`), and an **empty** agent string (the `agent != ""`
> guard skips the per-agent lookup; global `EVOLVE_SYSTEM_PROMPT` still applies).
> The deliverable is test code only — `systemprompt.go` (production) must not
> change. Source incident: cycle 206 (2026-06-03); the same coverage was
> attempted in cycle 204 but that cycle did not ship (cycle-205 reset).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| five-tests-pass | The 5 named `TestResolve_` error-path tests exist and pass | 8/10 | `go test -run '^TestResolve_(MalformedProfileJSON\|AbsoluteSystemPromptFile\|MissingSystemPromptFile\|HyphenatedAgentName\|EmptyAgent)$'` reports 5 `--- PASS:` lines |
| suite-green | Full `internal/systemprompt` suite still passes (no regression) | 7/10 | `go test ./internal/systemprompt/...` exits 0 |
| test-only-scope | `systemprompt.go` (production) is not modified | 6/10 | `git diff --name-only HEAD` excludes `internal/systemprompt/systemprompt.go` |
| build-clean | Module still compiles | 6/10 | `go build ./...` exits 0 |
