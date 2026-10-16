# internal/phases/flakererunscan

## Purpose

`internal/phases/flakererunscan` is the native flake-rerun-scan phase. It reruns a candidate test N times in code, where the former phase booted an LLM for about twelve turns. Deterministic work belongs in code, not in LLM cycles.

## Design

- **Tally, then flag.** `Rerun` calls the given function once for each run and tallies the passes and failures. A target is `Flaky` only when it recorded at least one pass and at least one failure.
- **Verdicts.** `Verdict` maps a result to one of four strings. `SKIPPED` means no run happened, `WARN` means flaky, and `PASS` or `FAIL` means every run passed or failed. The strings are the vocabulary of the LLM variant, and downstream gates pattern-match them, so they must not change.
- **Deterministic.** An identical function yields an identical result, so two campaigns over the same target diff to nothing.

## Invariants

- **Flaky needs both outcomes.** `TestRerun_FlakyAlternating` pins that a mixed run is flagged and records both outcomes.
- **A zero-run campaign is skipped, not passed.** `TestRerun_ZeroRunsSkipped` pins this.
- **Consistent outcomes are not flaky.** `TestRerun_StablePass` and `TestRerun_ConsistentFail` pin the PASS and FAIL verdicts.
- **Determinism.** `TestRerun_Deterministic` pins this.

## Findings

- None.
