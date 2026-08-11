# Eval: claude-version-drift-warning

## Task
Extend `go/internal/looppreflight/freeze.go` to recognise `claude` as a self-updating CLI and emit a WARN or HALT when its installed version drifts from the version stored in `.evolve/pinned-cli-versions.json`. The file is created by the operator once after a known-good batch; absence = WARN (first run); presence + mismatch = HALT (version drifted).

---

## Criterion 1 — `defaultSelfUpdateEvidence` recognises `claude` [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  grep -n '"claude"' internal/looppreflight/freeze.go | head -5 \
  && echo FOUND || echo FAIL
```

Expected: at least one match showing `claude` is now handled.
Gaming fake: adding a comment with "claude" but not real logic.
Negative: before the fix, `grep '"claude"'` returns nothing.

---

## Criterion 2 — Freeze test exercises the claude evidence path [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -run TestFreeze -v 2>&1 | grep -E "claude|PASS|FAIL" | head -10
```

Expected: at least one test line references `claude` AND the test suite passes (no `FAIL` line).

---

## Criterion 3 — WARN when no pinned-cli-versions.json and claude is self-updating [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -run TestCLIVersionDrift -v 2>&1 | tail -10
```

Expected: test passes (no FAIL); the check behaviour under "no pin file" is WARN (not HALT).
Negative case: a test must assert that HALT is NOT triggered when the pin file is absent (fail-open).

---

## Criterion 4 — HALT when pinned version mismatches installed version [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -run TestCLIVersionDrift -v 2>&1 | \
  grep -i "halt\|HALT\|mismatch\|drift" | head -5 && echo FOUND || echo NOTFOUND
```

Expected: `FOUND` — at least one test scenario asserts HALT on version mismatch.

---

## Criterion 5 — Full looppreflight suite passes [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -count=1 2>&1 | tail -3
```

Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/looppreflight` (no FAIL).
