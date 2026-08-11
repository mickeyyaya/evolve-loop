---
score_cap:
  - criterion: "skillinventory.scan uses NewFromDir(projectRoot) not NewForProject — EVOLVE_PROMPTS_DIR does not affect results"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestBuild_HappyPath_WritesInventoryFile ./internal/skillinventory/"
  - criterion: "All 4 previously-failing skillinventory tests pass"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/skillinventory/ 2>&1 | grep -E '^(ok|FAIL)'"
---

# Eval: skillinventory test isolation fix

Pins the fix for `scan()` using `prompts.NewForProject(projectRoot)` — env-var sensitive — instead of
`prompts.NewFromDir(projectRoot)`. When `EVOLVE_PROMPTS_DIR` is set in the environment (or shell),
the old code silently scanned the real project's 22 skills instead of the temp-dir test fixture.

## Graders

### [code] All 4 failing tests now pass (primary)

```bash
cd go && go test -count=1 ./internal/skillinventory/ 2>&1 | tail -3
# expected: ok  github.com/mickeyyaya/evolve-loop/go/internal/skillinventory
```

### [code] EVOLVE_PROMPTS_DIR override does NOT leak into scan (isolation guard)

```bash
cd go && EVOLVE_PROMPTS_DIR=/tmp/nonexistent go test -count=1 -run TestBuild_HappyPath_WritesInventoryFile ./internal/skillinventory/ 2>&1 | tail -3
# expected: PASS (not affected by the env override)
```

### [code] Empty project root still produces error (regression guard)

```bash
cd go && go test -count=1 -run TestBuild_MissingProjectRoot_ReturnsError ./internal/skillinventory/ 2>&1 | tail -3
# expected: PASS
```

### [code] Negative: scan against a dir with no skills/ returns empty inventory, not error

```bash
cd go && go test -count=1 -run TestBuild_NoSkillsDir_EmptyInventory ./internal/skillinventory/ 2>&1 | tail -3
# expected: PASS (not 22-skill count)
```
