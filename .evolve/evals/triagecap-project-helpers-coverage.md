# Eval: triagecap-project-helpers-coverage

## Objective
Add tests for the uncovered branches in two package-private helpers in
`go/internal/triagecap/project.go`:
- `actionOf` (66.7%): the "no em-dash" fallback (`return strings.TrimSpace(rest)`)
- `reasonOf` (66.7%): the "regex returns no match" fallback (`return ""`)

These are pure string functions; tests are single-line table-driven calls
added to `project_test.go` (same package).

## Criteria

### C1: `TestActionOf_NoEmDash` passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/triagecap/... -run TestActionOf_NoEmDash -count=1 -v
```
Expected: `--- PASS: TestActionOf_NoEmDash`

### C2: `TestReasonOf_NoMatch` passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/triagecap/... -run TestReasonOf_NoMatch -count=1 -v
```
Expected: `--- PASS: TestReasonOf_NoMatch`

### C3: `actionOf` and `reasonOf` coverage each reach 100% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/triagecap/... -count=1 -coverprofile=/tmp/c-triagecap.out && go tool cover -func=/tmp/c-triagecap.out | grep -E "actionOf|reasonOf"
```
Expected: both functions show `100.0%`.

### C4: Full triagecap suite passes without regression [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/triagecap/... -count=1
```
Expected: `ok` with no `FAIL`.

### C5 (negative): existing `actionOf` em-dash path still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/triagecap/... -run TestProjectDecisionJSON_ParsesAllSections -count=1 -v
```
Expected: `--- PASS: TestProjectDecisionJSON_ParsesAllSections` (exercises the em-dash/reason-present paths to verify no regression).
