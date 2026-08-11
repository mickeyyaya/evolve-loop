# Eval: marketplacepoll DefaultPull / DefaultReleaseSh behavioral tests

## Task slug
`marketplacepoll-defaultpull-releasesh-tests`

## Objective
`go/internal/marketplacepoll/marketplacepoll_test.go` gains behavioral tests for the
`DefaultPull` (git-checkout branch) and `DefaultReleaseSh` (all three branches: absent
script, failing script, succeeding script). These are the only uncovered paths in the
package. No production `.go` file is modified.

---

## Acceptance criteria

### AC-1 — only *_test.go touched [code]
```bash
git diff HEAD -- '*.go' | grep -E '^(\+\+\+|---) ' | grep -v '_test\.go' | grep '\.go' | grep -v '/dev/null' && echo "FAIL: production .go changed" && exit 1
echo "PASS: only _test.go changed"
```

### AC-2 — build stays green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... && echo "PASS: build green" || echo "FAIL: build broken"
```

### AC-3 — package tests green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/marketplacepoll/... -count=1 -timeout 60s && echo "PASS" || echo "FAIL"
```

### AC-4 — DefaultPull git-path covered [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -coverprofile=/tmp/mp202.out ./internal/marketplacepoll/... -count=1 && \
  go tool cover -func=/tmp/mp202.out | grep DefaultPull | grep -v "100.0%" && echo "FAIL: DefaultPull not at 100%" || echo "PASS: DefaultPull coverage improved"
```

### AC-5 — DefaultReleaseSh covered (≥ 1 new statement covered) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -coverprofile=/tmp/mp202b.out ./internal/marketplacepoll/... -count=1 && \
  pct=$(go tool cover -func=/tmp/mp202b.out | grep DefaultReleaseSh | awk '{print $3}') && \
  [ "$pct" = "100.0%" ] && echo "PASS: DefaultReleaseSh at 100%" || echo "RESULT: DefaultReleaseSh at $pct"
```

### AC-6 — at least one negative / error-path test added [model]
The new tests include `TestDefaultReleaseSh_ScriptExitsNonZero_ReturnsError` (or
equivalent), which calls `DefaultReleaseSh` with a script that exits non-zero and
asserts a non-nil error wrapping the failure message.
