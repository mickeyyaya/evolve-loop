# Eval: dry-issemver-4pkg

## Task
Extract the byte-identical `semverRE` regexp and `IsSemver` function from four packages
(`changeloggen`, `versionbump`, `marketplacepoll`, `releasepipeline`) into a new
`go/internal/semvercheck` package. The four packages then delegate to the shared function,
removing their local copies. `changeloggen.IsSemver` keeps its thin exported wrapper
since external callers (`cmd_changelog.go`, `releasepipeline/bridges.go`) already use it.

## Acceptance Criteria

### AC1: semvercheck package defines IsSemver [code]
```bash
grep -n "^func IsSemver" /Users/danleemh/ai/claude/evolve-loop/go/internal/semvercheck/semvercheck.go
# Expected: one line found, exit 0
```

### AC2: semvercheck package defines the package-level regexp [code]
```bash
grep -n "^var semverRE\s*=\s*regexp\.MustCompile" /Users/danleemh/ai/claude/evolve-loop/go/internal/semvercheck/semvercheck.go
# Expected: one line found, exit 0
```

### AC3: versionbump no longer defines its own semverRE [code]
```bash
grep -c "^var semverRE" /Users/danleemh/ai/claude/evolve-loop/go/internal/versionbump/versionbump.go
# Expected output: 0
```

### AC4: marketplacepoll no longer defines its own semverRE [code]
```bash
grep -c "^var semverRE" /Users/danleemh/ai/claude/evolve-loop/go/internal/marketplacepoll/marketplacepoll.go
# Expected output: 0
```

### AC5: releasepipeline no longer defines its own semverRE [code]
```bash
grep -c "^var semverRE" /Users/danleemh/ai/claude/evolve-loop/go/internal/releasepipeline/releasepipeline.go
# Expected output: 0
```

### AC6: All four affected packages build cleanly [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./internal/semvercheck/... ./internal/changeloggen/... ./internal/versionbump/... ./internal/marketplacepoll/... ./internal/releasepipeline/... 2>&1; echo "EXIT:$?"
# Expected: EXIT:0 with no errors
```

### AC7: Tests pass for all four affected packages [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/semvercheck/... ./internal/changeloggen/... ./internal/versionbump/... ./internal/marketplacepoll/... ./internal/releasepipeline/... -count=1 2>&1 | grep -E "^(ok|FAIL)" | sort
# Expected: all lines start with "ok", no "FAIL"
```

### AC8: Negative — semverRE is not multiply defined across the four packages [code]
```bash
grep -rn "^var semverRE" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/changeloggen/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/versionbump/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/marketplacepoll/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/releasepipeline/ 2>/dev/null | wc -l | tr -d ' '
# Expected: 0 (gaming fake: keeping the var but with a different pattern — covered by AC2 asserting the canonical location)
```

### AC9: semvercheck IsSemver rejects non-semver strings [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/semvercheck/... -count=1 -run TestIsSemver -v 2>&1 | grep -E "PASS|FAIL"
# Expected: PASS (tests must cover reject cases: "v1.2.3", "1.2", "1.2.3.4", "abc")
```
