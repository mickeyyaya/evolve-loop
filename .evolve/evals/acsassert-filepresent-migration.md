# Eval: acsassert-filepresent-migration

## Acceptance Criteria

### AC-1: FilePresent helper added to acsassert [code]
```bash
grep -q "func FilePresent(" /Users/danleemh/ai/claude/evolve-loop/go/pkg/acsassert/assertions.go
```

### AC-2: FilePresent has no Errorf (pure bool) [code]
```bash
# Verify FilePresent does NOT call tb.Errorf (unlike FileExists)
python3 -c "
import re, sys
with open('/Users/danleemh/ai/claude/evolve-loop/go/pkg/acsassert/assertions.go') as f:
    src = f.read()
# Extract FilePresent function body
m = re.search(r'func FilePresent\([^)]+\)[^{]*{(.+?)^}', src, re.DOTALL | re.MULTILINE)
if not m:
    print('FilePresent not found'); sys.exit(1)
body = m.group(1)
if 'Errorf' in body or 'tb.Error' in body:
    print('FAIL: FilePresent must not call Errorf'); sys.exit(1)
print('PASS: FilePresent has no Errorf')
"
```

### AC-3: No remaining FileExists-as-skip-guard in acs/ [code]
```bash
# After migration, no acs predicates should use if !acsassert.FileExists(t, ...) as a skip guard
count=$(grep -rn "if !acsassert\.FileExists" /Users/danleemh/ai/claude/evolve-loop/go/acs/ 2>/dev/null | wc -l)
if [ "$count" -gt 0 ]; then
  echo "FAIL: $count remaining FileExists-as-skip-guard sites found"
  grep -rn "if !acsassert\.FileExists" /Users/danleemh/ai/claude/evolve-loop/go/acs/ 2>/dev/null | head -10
  exit 1
fi
echo "PASS: No remaining FileExists skip-guard sites ($count)"
```

### AC-4: acsassert package tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./pkg/acsassert/... -count=1
```

### AC-5: cycle106 tests pass (stale v12.1.1 pin fixed) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/cycle106/... -v 2>&1 | grep -E "PASS|FAIL|ok"
# Must not contain FAIL
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/cycle106/... 2>&1 | grep -v "^ok" | grep -v "^\s*$" | grep "FAIL" && exit 1 || exit 0
```

### AC-6: Negative — old FileExists still works as assertion (not broken) [code]
```bash
# Positive assertion uses (acsassert.FileExists called without skip-guard) must still compile
# cycle42 uses FileExists as a direct assertion — verify it still compiles and passes
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/cycle42/... -count=1
```

### AC-7: All acs tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/... 2>&1 | grep "FAIL" | grep -v "^$" && exit 1 || echo "PASS: all acs tests pass"
```
