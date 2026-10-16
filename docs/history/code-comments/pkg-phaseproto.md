# Comment history: `pkg/phaseproto`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d6

### `go/pkg/phaseproto/wire_bench_test.go:10` — above `func benchRequest() core.PhaseRequest {`

```text
// Phase 4 task #19: structural perf benchmarks. NO LLM calls. Measures
// the in-process wire-protocol overhead a real cycle would incur once
// per phase boundary.
//
// Targets per parent plan §6 item 8: "hot-path performance, Go median
// ≤ bash median × 0.6". These benchmarks establish the Go side of the
// comparison; the bash side is measured separately via
// `time bash scripts/dispatch/cycle-simulator.sh` (see
// scripts/parity-audit.sh).
//
// Run: go test -bench=. -benchmem -run=^$ ./pkg/phaseproto/
```
