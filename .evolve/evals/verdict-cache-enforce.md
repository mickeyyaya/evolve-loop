# Eval: verdict-cache-enforce

## Task
Promote ADR-0048 Slice B (content-addressed audit reuse) from shadow to enforce.
Add `EVOLVE_VERDICT_CACHE=off|shadow|enforce` config dial. At enforce stage, when the
pre-loop cache probe finds a matching entry, skip tdd/build/audit and jump to ship.

## Acceptance Criteria

### AC1 — Config dial parses off/shadow/enforce [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/config/... -run TestConfig_VerdictCacheDial -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: `--- PASS: TestConfig_VerdictCacheDial`

### AC2 — Cache hit + enforce skips tdd/build/audit [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestVerdictCacheEnforce_SkipsToShip -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: `--- PASS: TestVerdictCacheEnforce_SkipsToShip`

### AC3 — Cache hit + shadow only logs, does not skip [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestVerdictCacheEnforce_ShadowLogsOnly -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: `--- PASS: TestVerdictCacheEnforce_ShadowLogsOnly`

### AC4 — Cache miss always runs full pipeline [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestVerdictCacheEnforce_MissRunsFull -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL)"
```
Expected: `--- PASS: TestVerdictCacheEnforce_MissRunsFull`

### AC5 — control-flags.md documents EVOLVE_VERDICT_CACHE [code]
```bash
grep -c "EVOLVE_VERDICT_CACHE" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/control-flags.md
```
Expected: `1` or more (at least one entry)

### AC6 — Full internal/config and internal/core tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/config/... ./internal/core/... -count=1 2>&1 | grep -E "^(ok|FAIL)" | head -10
```
Expected: all `ok` lines, no `FAIL`

## Negative Cases (gaming fakes)

- **Fake**: unconditionally jumping to ship on every cycle → AC4 fails (miss case)
- **Fake**: not adding the config dial → AC1 fails
- **Fake**: skipping in shadow mode → AC3 fails
- **Edge case**: corrupted cache file → existing degradation contract (miss-on-corrupt) preserved; test in verdictcache package tests
