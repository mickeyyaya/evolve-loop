# Eval: add-attempt-count-to-phase-timing

## Summary
Verifies that phase-timing.json now includes an `attempt_count` field reflecting how many
bridge launch attempts were needed for each phase, and that the doc is updated.

---

## Criterion 1 — phaseTimingEntry struct has AttemptCount field [code]

```bash
grep -n "AttemptCount\|attempt_count" go/internal/core/orchestrator.go | head -10
```

Expected: at least one hit in the struct definition (`AttemptCount int`) and one in the append call.

---

## Criterion 2 — attempt count is tracked in the retry loop [code]

```bash
grep -n "attempt\|AttemptCount" go/internal/core/orchestrator.go | grep -i "timing\|append\|phaseTimings" | head -5
```

Expected: the `phaseTimings` append now includes `AttemptCount: attempt` (or equivalent).

---

## Criterion 3 — phase-timing.json schema in docs updated [code]

```bash
grep -n "attempt_count\|AttemptCount" docs/architecture/phase-timing-and-diagnostics.md | head -5
```

Expected: at least one hit showing `attempt_count` is documented.

---

## Criterion 4 — all core tests pass [code]

```bash
cd go && go test ./internal/core/... 2>&1 | tail -10
```

Expected: `ok` line at the end, no FAIL.

---

## Criterion 5 — timing test exercises attempt_count=1 for single-attempt success [code]

```bash
grep -rn "AttemptCount\|attempt_count" go/internal/core/orchestrator_timing_test.go | head -10
```

Expected: at least one assertion on `AttemptCount` in the timing tests.

---

## Negative: a phase that ran once has attempt_count=1 (not 0) [code]

```bash
grep -n "AttemptCount.*1\|attempt.*== 1\|attempt_count.*1" go/internal/core/orchestrator_timing_test.go | head -5
```

Expected: at least one test verifying that a single-attempt phase records `AttemptCount=1`.

---

## Edge: a phase retried once has attempt_count=2 [code]

```bash
grep -n "AttemptCount.*2\|attempt_count.*2" go/internal/core/orchestrator_timing_test.go | head -5
```

Expected: at least one test verifying that a retried phase records `AttemptCount=2`.
