# internal/retryladder

## Purpose

`internal/retryladder` classifies a failed phase try into one class: `infra`, `quota`, `halt`, `spec`, `capability` or `unknown`. It is component L1 of the retry ladder ([ADR-0142](../adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md), [spec §3](../retry-ladder.md#3-the-classifier)).

## Design

- `Classify` reads `Facts` and calls no model. The order is fixed: halt exit, benched quota, infra exit or teardown error, spec, capability, unknown.
- Exit 85 is also `ExitUnknownPrompt`. It is `quota` only with a bench, else `infra`.
- A floor failure is `spec` when it is protected or outside the lane scope. It is `capability` when it is inside the scope.
- `Fingerprint` is the sorted set of bracketed violation codes. Without codes it is a hash of the normalized text.

## Invariants

- The package does not import `internal/core`. Predicate `TestC1973_003` checks this.
- The normalizer copies the rules of `core/failure_digest.go`. A rule change must change both until a later cycle moves the shared rules into one package.
