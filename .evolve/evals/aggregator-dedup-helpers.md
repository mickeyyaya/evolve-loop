---
score_cap:
  - criterion: "Dead anyWarn variable removed from writeVerdict — no `_ = anyWarn` or standalone `anyWarn` tracking remains"
    max_if_missing: 7
    evidence: "test -z \"$(grep -n 'anyWarn' go/internal/aggregator/aggregator.go)\""
  - criterion: "scanFirstCapture helper exists and extractVerdict/extractScore delegate to it"
    max_if_missing: 7
    evidence: "grep -q 'func scanFirstCapture' go/internal/aggregator/aggregator.go"
  - criterion: "appendWorkerSections helper exists and at least one write* function delegates to it"
    max_if_missing: 7
    evidence: "grep -q 'func appendWorkerSections' go/internal/aggregator/aggregator.go && grep -q 'appendWorkerSections' go/internal/aggregator/aggregator.go"
  - criterion: "aggregator package test suite passes — no regressions"
    max_if_missing: 9
    evidence: "cd go && go test ./internal/aggregator/... -count=1"
  - criterion: "aggregator.go line count strictly lower than 466 (pre-refactor baseline)"
    max_if_missing: 6
    evidence: "test $(wc -l < go/internal/aggregator/aggregator.go) -lt 466"
  - criterion: "ACS predicates for cycle-336 pass"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle336/"
---

# Eval: DRY aggregator helpers — scanFirstCapture, appendWorkerSections, drop anyWarn

> Pins the cycle-336 harden/code-reduction refactor in `go/internal/aggregator/aggregator.go`.
> Three behavior-preserving simplifications are bundled because they target the same file
> and together produce a measurable net line reduction:
>
>  1. **Remove dead `anyWarn`**: `writeVerdict` tracks `anyWarn` through a switch but then
>     immediately suppresses it with `_ = anyWarn`. The variable is write-only and dead.
>     Removing it and collapsing the default case shortens the function by ~5 lines with
>     zero semantic change.
>
>  2. **Extract `scanFirstCapture`**: `extractVerdict` (open file, scan, regex match, return
>     string) and `extractScore` (same shape, different regex + parse) share identical
>     open-scan-match boilerplate. A shared `scanFirstCapture(path string, re *regexp.Regexp)
>     string` helper eliminates ~10 duplicate lines.
>
>  3. **Extract `appendWorkerSections`**: `writeConcat`, `writeVerdict`, `writePlanReview`,
>     and `writeCrossCLIVote` all contain an identical loop (iterate workers, trim basename,
>     fprintf heading, readFile body, append). Factoring into
>     `appendWorkerSections(b *strings.Builder, heading string, workers []string, readFile ...)
>     error` eliminates ~30 duplicate lines across four call sites.
>
> All three changes preserve exact observable behavior: existing tests cover every verdict
> path, every merge mode, score extraction, and worker concatenation.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dead-var-gone | `anyWarn` absent from aggregator.go | 7/10 | `grep -n anyWarn` returns nothing |
| helper-scan | `scanFirstCapture` function defined | 7/10 | `grep -q 'func scanFirstCapture'` |
| helper-append | `appendWorkerSections` function defined and called | 7/10 | `grep -q 'func appendWorkerSections'` + call site |
| no-regression | aggregator test suite green | 9/10 | `go test ./internal/aggregator/... -count=1` |
| net-reduction | file shorter than 466 lines | 6/10 | `wc -l` check |
| acs-gates | cycle-336 predicates pass | 5/10 | `go test -tags acs ./acs/cycle336/` |

## Negative / Edge Cases

```bash
# [code] Dead-var guard: anyWarn must not appear anywhere in the file
test -z "$(grep -n 'anyWarn' go/internal/aggregator/aggregator.go)"

# [code] scanFirstCapture returns empty string for file-not-found (no panic)
# Exercised implicitly by TestExtractVerdict_MultipleFormsRecognized with missing file path

# [code] appendWorkerSections propagates readFile errors to callers
# Exercised by TestAggregate_MergeReadFailure (returns ExitUsageErr on read error)

# [code] Verdict aggregation: FAIL beats WARN beats PASS — unchanged
cd go && go test ./internal/aggregator/... -run TestAggregate_Verdict -count=1 -v 2>&1 | grep -E 'PASS|FAIL|RUN'

# [code] Full suite — no regression across all merge modes
cd go && go test ./internal/aggregator/... -count=1
```

## OOD / Boundary Cases

```bash
# [code] writeLessons (not refactored) still deduplicates across workers
cd go && go test ./internal/aggregator/... -run TestAggregate_LessonsDedup -count=1 -v

# [code] writeCrossCLIVote uses "### Worker:" heading — appendWorkerSections must accept parameterized heading
cd go && go test ./internal/aggregator/... -run TestAggregate_CrossCLI -count=1 -v
```
