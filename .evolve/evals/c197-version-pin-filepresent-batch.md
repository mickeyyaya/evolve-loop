# Eval: c197-version-pin-filepresent-batch

## Goal
Fix the stale version pin in cycle106 (`TestC106_011_BinaryVersionIsV12_1_1`) and migrate
a batch of ACS test skip-guards from `acsassert.FileExists(t,path)+t.Skip` (anti-pattern:
calls `t.Errorf` then tries to skip) to `fixtures.FilePresent(path)+t.Skip` (pure boolean,
no test failure logged for a legitimately-absent file).

## Acceptance Criteria

### AC-1: cycle106 test suite passes [code]
```
cd go && go test ./acs/cycle106/... 2>&1 | tail -5
```
Must exit 0 and produce no FAIL line. The stale `TestC106_011_BinaryVersionIsV12_1_1`
must either be removed, skipped-when-absent, or updated so it no longer pins to "12.1.1".

### AC-2: FilePresent migration present in at least 8 ACS cycle files [code]
```
grep -rl "fixtures.FilePresent" /Users/danleemh/ai/claude/evolve-loop/go/acs/ | wc -l
```
Must print a number ≥ 8.

### AC-3: No remaining FileExists-as-skip anti-pattern in migrated files [code]
```
# For each migrated file, confirm no acsassert.FileExists followed immediately by t.Skip
python3 -c "
import os, sys
found = []
for root, dirs, files in os.walk('/Users/danleemh/ai/claude/evolve-loop/go/acs'):
    for f in files:
        if f.endswith('_test.go'):
            path = os.path.join(root, f)
            content = open(path).read()
            lines = content.split('\n')
            for i, line in enumerate(lines):
                if 'acsassert.FileExists' in line and i+1 < len(lines) and 'Skip' in lines[i+1]:
                    found.append(f'{path}:{i+1}')
if found:
    print('ANTI-PATTERN REMAINING:', found[:5])
    sys.exit(1)
else:
    print('CLEAN: no acsassert.FileExists+Skip anti-patterns remain')
"
```
Must print "CLEAN: no acsassert.FileExists+Skip anti-patterns remain".

### AC-4: Full test suite still green [code]
```
cd go && go test ./... 2>&1 | tail -3
```
Must exit 0 with no FAIL lines.

## Negative Cases

### NEG-1: Old anti-pattern would produce FAIL verdict on missing file [code]
This is an observability check — the old pattern called `t.Errorf` (marking test FAILED)
before `t.Skip`, meaning a test could appear as both FAILED and SKIPPED. The migration
eliminates this. Verify the anti-pattern is NOT present after migration (same as AC-3).
