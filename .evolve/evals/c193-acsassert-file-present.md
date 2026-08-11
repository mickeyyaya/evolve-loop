# Eval: acsassert FilePresent + skip-guard migration + cycle106 fix

> Cycle 193 HIGH task. Adds `acsassert.FilePresent(path string) bool` (pure bool,
> no TB, no Errorf), migrates 106 skip-guard sites in go/acs/ from the double-signal
> pattern `if !acsassert.FileExists(t, path) { t.Skip(...) }` to the clean
> `if !acsassert.FilePresent(path) { t.Skip(...) }`, and fixes the stale
> cycle106 v12.1.1 version-pin predicate that fails on current devel builds.

## AC-1: FilePresent added to acsassert with no Errorf [code]
```bash
grep -q "func FilePresent(" /Users/danleemh/ai/claude/evolve-loop/go/pkg/acsassert/assertions.go && \
python3 -c "
import re, sys
with open('/Users/danleemh/ai/claude/evolve-loop/go/pkg/acsassert/assertions.go') as f:
    src = f.read()
m = re.search(r'func FilePresent\([^)]*\)[^{]*{(.+?)^}', src, re.DOTALL | re.MULTILINE)
if not m: print('FilePresent function not found'); sys.exit(1)
body = m.group(1)
if 'Errorf' in body or 'tb.Error' in body:
    print('FAIL: FilePresent must not call Errorf'); sys.exit(1)
print('PASS')
"
```

## AC-2: No remaining FileExists-as-skip-guard in acs/ [code]
```bash
count=$(grep -rn "if !acsassert\.FileExists" /Users/danleemh/ai/claude/evolve-loop/go/acs/ 2>/dev/null | wc -l)
if [ "$count" -gt 0 ]; then
  echo "FAIL: $count remaining FileExists skip-guard sites"
  grep -rn "if !acsassert\.FileExists" /Users/danleemh/ai/claude/evolve-loop/go/acs/ 2>/dev/null | head -5
  exit 1
fi
echo "PASS: 0 remaining FileExists skip-guard sites"
```

## AC-3: acsassert tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./pkg/acsassert/... -count=1 2>&1 | tail -3
```

## AC-4: cycle106 tests pass (stale version pin resolved) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/cycle106/... -count=1 2>&1 | grep -E "ok|FAIL"
# Must show ok, not FAIL
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/cycle106/... -count=1 2>&1 | grep "^FAIL" && exit 1 || exit 0
```

## AC-5 (negative): FileExists still compiles and runs as assertion [code]
```bash
# cycle42 uses FileExists as a plain assertion — must still compile and pass
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/cycle42/... -count=1 2>&1 | grep -E "ok|FAIL"
```

## AC-6: All acs tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/... -count=1 2>&1 | grep "^FAIL" && exit 1 || echo "PASS: all acs tests pass"
```
