# Eval: autorespond-rate-limit-persistence

## Task
Add ≥2 consecutive-capture persistence requirement for `escalate`-policy interactive-prompt
rules (rate_limit, auth_recheck) in the bridge auto-responder. A single pane capture
containing rate-limit text MUST NOT escalate; two consecutive captures MUST.

## Acceptance Criteria

### AC1 — Single match does not escalate [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestAutoRespondRateLimitPersistence_SingleMatch -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: `--- PASS: TestAutoRespondRateLimitPersistence_SingleMatch`

### AC2 — Two consecutive matches escalate [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestAutoRespondRateLimitPersistence_TwoConsecutiveMatches -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: `--- PASS: TestAutoRespondRateLimitPersistence_TwoConsecutiveMatches`

### AC3 — Busy-pane skip still works (existing behavior preserved) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestDecideAutoRespond -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: all TestDecideAutoRespond subtests PASS (no regression)

### AC4 — Counter resets on non-match [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestAutoRespondRateLimitPersistence_ResetOnMiss -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: `--- PASS: TestAutoRespondRateLimitPersistence_ResetOnMiss`

### AC5 — Full bridge test suite passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 2>&1 | grep -E "^(ok|FAIL)" | head -20
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/bridge` (no FAIL lines)

## Negative Cases (gaming fakes)

- **Fake**: removing the persistence check and always escalating → AC1 fails
- **Fake**: never escalating → AC2 fails
- **Edge case**: pane has rate-limit text but paneBusy=true → AC3 verifies this is still blocked by the existing paneBusy gate (not a new false negative)
