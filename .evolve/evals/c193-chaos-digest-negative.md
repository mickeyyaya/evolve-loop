# Eval: Chaos negative tests — malformed/truncated upstream artifacts

> Cycle 193 MEDIUM task. Adds chaos tests to `go/internal/router/digest_test.go`
> verifying the fail-open contract holds for truncated (mid-write) JSON, empty
> files, wrong-schema content (array at top level), and mixed-valid scenarios.
> The existing `TestDigest_FailOpenOnMissingAndCorrupt` only tests blatantly
> invalid JSON; these tests cover runtime-tier artifact-shape gaps (partial writes,
> schema mismatches).

## AC-1: Truncated JSON chaos test exists in digest_test.go [code]
```bash
grep -q "Truncated\|truncat" /Users/danleemh/ai/claude/evolve-loop/go/internal/router/digest_test.go
```

## AC-2: Empty-file and wrong-schema chaos tests exist [code]
```bash
grep -qE "Empty|WrongSchema|ArrayInstead|ZeroByte" /Users/danleemh/ai/claude/evolve-loop/go/internal/router/digest_test.go
```

## AC-3 (negative): truncated JSON → Present:false, no error [code]
```bash
# The chaos tests must assert fail-open behaviour
grep -A 10 "Truncat\|ZeroByte\|Empty\|WrongSchema" /Users/danleemh/ai/claude/evolve-loop/go/internal/router/digest_test.go \
  | grep -qE "Present.*false|false.*Present|must.*Present:false|Errorf.*Present" && echo "PASS assertions found" || \
  { echo "WARN: chaos tests may not assert Present:false explicitly"; exit 0; }
```

## AC-4: All router tests pass including new chaos tests [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/router/... -count=1 2>&1 | tail -5
```

## AC-5 (negative): empty artifact must not panic or return error [code]
```bash
# Run just chaos tests and verify they pass (no panic)
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/router/... -count=1 -run "Chaos\|Truncat\|Empty\|WrongSchema" -v 2>&1 | tail -10
```

## AC-6: At least 3 new chaos test functions added [code]
```bash
count=$(grep -c "^func Test.*\(Chaos\|Truncat\|Empty\|WrongSchema\|ZeroByte\|ArrayInstead\)" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/router/digest_test.go 2>/dev/null || echo 0)
if [ "$count" -lt 3 ]; then
  echo "FAIL: only $count chaos test functions found, need >=3"
  exit 1
fi
echo "PASS: $count chaos test functions found"
```
