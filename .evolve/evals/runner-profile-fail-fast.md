# Eval: runner-profile-fail-fast

## Task
When `runner.Run` is called for a phase whose profile file does not exist on disk,
the runner must return a typed error naming the missing file path BEFORE dispatching
to the bridge. No bridge call must be made for a missing-profile phase.

## Acceptance Criteria

### AC1 — build passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1
```
Expected: exit 0, no compile errors.

### AC2 — runner tests pass (all) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -count=1 -short 2>&1
```
Expected: exit 0, `ok github.com/...runner`.

### AC3 — missing-profile test exists and passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -run TestMissingProfile -v -count=1 2>&1
```
Expected: exit 0, output contains `--- PASS: TestMissingProfile`.

### AC4 — error message names the missing file [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -run TestMissingProfile -v -count=1 2>&1 | grep -q "profile" && echo "NAMED" || echo "MISSING_NAME"
```
Expected: prints `NAMED` (the test itself asserts the error string contains the profile path).

### AC5 — negative: existing profile does NOT trigger profile error [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -run TestExistingProfileNoError -v -count=1 2>&1
```
Expected: exit 0, `--- PASS: TestExistingProfileNoError`.

### AC6 (edge) — bridge is never called for a missing-profile phase [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -run TestMissingProfileNoBridgeCall -v -count=1 2>&1
```
Expected: exit 0, `--- PASS: TestMissingProfileNoBridgeCall`.

## Gaming check
A trivial fake that always returns nil (skipping the profile check) would pass AC2
but fail AC3. The test in AC3 must exercise the real `runner.Run` path with a
deliberately absent profile JSON and assert the named-file error.
