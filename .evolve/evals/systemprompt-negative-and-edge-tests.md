# Eval: systemprompt — negative & edge-case behavioral tests

## Task slug
`systemprompt-negative-and-edge-tests`

## Objective
`go/internal/systemprompt/systemprompt_test.go` gains three branch-gap tests for
`Resolve` / `profileDefault`. The package sits at 95% *statement* coverage, but three
behaviorally meaningful *branches* are exercised only by coincidence:
- absolute `system_prompt_file` (the `if !filepath.IsAbs(p)` FALSE leg — read verbatim,
  not joined against `profileDir`);
- an unreadable/missing `system_prompt_file` (the `os.ReadFile` error leg falls through
  silently to `""`, no panic, no error propagation);
- `agent == ""` (the per-agent env lookup is skipped; fall back to `EVOLVE_SYSTEM_PROMPT`
  or the profile default).
No production `.go` file is modified.

> Source incident: this eval exists because the TDD-Engineer prompt (Step 6b, cycle-131
> lesson) mandates a persistent regression eval for every predicate-dispositioned task.
> The task itself stalled in cycles 200/201/203 before the build phase (unrelated
> infrastructure: EGPS cwd bug fixed cycle-190, observer liveness fixed cycle-202); the
> task is valid and re-selected as the sole cycle-204 deliverable.

---

## Acceptance criteria

### AC-1 — >= 9 test functions in the file [code]
```bash
cnt=$(grep -c '^func Test' go/internal/systemprompt/systemprompt_test.go)
test "$cnt" -ge 9 && echo "PASS: $cnt test funcs" || { echo "FAIL: only $cnt (<9)"; exit 1; }
```
Expect: `PASS` (baseline is 6; this cycle adds the three branch-gap tests).

### AC-2 — build stays green [code]
```bash
cd go && go build ./... && echo "PASS: build green" || { echo "FAIL: build broken"; exit 1; }
```
Expect: `PASS: build green`.

### AC-3 — package tests green [code]
```bash
cd go && go test ./internal/systemprompt/... -count=1 -timeout 60s && echo "PASS" || { echo "FAIL"; exit 1; }
```
Expect: `PASS` (existing 6 + new 3 all green).

### AC-4 — absolute-path system_prompt_file test present [code]
```bash
grep -q 'func TestResolve_AbsoluteSystemPromptFile(' go/internal/systemprompt/systemprompt_test.go \
  && echo "found" || { echo "missing"; exit 1; }
```
Expect: `found` (exercises the `filepath.IsAbs` FALSE branch).

### AC-5 — unreadable/missing system_prompt_file test present [code]
```bash
grep -q 'func TestResolve_UnreadableSystemPromptFileFallsThrough(' go/internal/systemprompt/systemprompt_test.go \
  && echo "found" || { echo "missing"; exit 1; }
```
Expect: `found` (exercises the silent `os.ReadFile` error fallthrough).

### AC-6 — no production .go files were modified [code]
```bash
prod=$(git diff --name-only HEAD -- '*.go' | grep -v '_test\.go$' || true)
test -z "$prod" && echo "PASS: only _test.go changed" || { echo "FAIL: $prod"; exit 1; }
```
Expect: `PASS` (only `*_test.go` changed this cycle).

### AC-7 — negative test runs and PASSES (`-run Unreadable`) [code]
```bash
# Existence guard first: `go test -run <missing>` exits 0 ("no tests to run"),
# so the run alone cannot prove the test exists.
grep -q 'func TestResolve_UnreadableSystemPromptFileFallsThrough(' go/internal/systemprompt/systemprompt_test.go \
  || { echo "FAIL: negative test absent"; exit 1; }
cd go && go test ./internal/systemprompt/... -run 'Unreadable' -count=1 \
  && echo "PASS" || { echo "FAIL"; exit 1; }
```
Expect: `PASS` (unreadable file → `""`, no panic).
