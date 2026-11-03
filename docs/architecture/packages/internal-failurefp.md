# internal/failurefp

## Purpose

`internal/failurefp` is the one home of the failure-reason normalizer. `Normalize` removes the tokens that change from cycle to cycle for the same defect. The blocker breaker of `internal/core` and the retry ladder of `internal/retryladder` call it. Thus one failure gets one normal form in the two callers.

## Design

- **One function.** `Normalize` removes the `cyclestate` detail span first. Then it applies the folds in a fixed order, and it trims the surrounding whitespace.
- **The folds.** Each fold is anchored narrowly, so two different defects never get one normal form.
  - An audit verdict-conflict token `narrative=PASS` becomes `narrative=<verdict>`.
  - A `cycle=2071` field becomes `cycle=N`. A longer word that ends in `cycle` keeps its digits.
  - A `cycle-2049-01m5…` path suffix becomes `cycle-N-<ulid>`. This fold runs before the cycle-number fold, because that fold reads only the first ULID digits.
  - A cycle-numbered path and the prose "cycle 1946" become `cycle-N`.
  - A finding count (`2 finding(s)`, `12 findings`) becomes `<n>`. A check-failure count stays, because it can name a different set of checks.
  - A retry denominator (`attempt 2/3`, `retry 1 of 4`) becomes `<n>/<n>`.
  - A go-test duration in decimal seconds becomes `<dur>`. An integer-second token such as `-timeout 300s` stays, because it is a setting.
- **The callers hash the text.** `core.fingerprint` hashes the phase, the class and the normal forms. `retryladder.Fingerprint` hashes the normal form when the reason has no bracketed violation code.
- **A leaf package.** The package imports only `cyclestate`. Each caller imports it, so neither caller imports the other for a text rule.

## Invariants

- **One normalizer.** No other package keeps a copy of a fold. `TestFingerprint_HashesTheOneSharedNormalizer` and `TestFailureIdentity_BreakerAndEscalationUseTheOneSharedNormalizer` pin that each caller uses `Normalize`.
- **Each fold rewrites only its token.** `TestNormalize_FoldsEachPerCycleToken` pins each fold and the tokens that stay.
- **A detail never moves the identity.** `TestNormalize_DropsTheDetailSpan` pins that a reason with a detail span has the normal form of the plain reason.
- **The research.** The AD3-1 research measured 32 digests. Cycles 2071 and 2072 differed only in `cycle=N`. Cycles 2049 and 2051 differed in a ULID path and a finding count. Each pair now shares one fingerprint.

## Findings

- None.
