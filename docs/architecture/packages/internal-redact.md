# internal/redact

> Plan: [incident knowledge base](../../plans/incident-knowledge-base-2026-11.md), decision D13.

## Purpose

`internal/redact` is the one home of the secret patterns. The pane trust boundary and the incident evidence bundle both call it. Two pattern lists can drift apart, so the repository keeps one.

## Design

- **Leaf.** The package imports the standard library only. `panetrust` imports it.
- **`Secrets`.** It replaces each secret-shaped string with `Token`. It also replaces the value of a `key: value` or `key=value` pair when the key names a credential. A second call on the result changes nothing.
- **`EntropyRuns`.** It returns each run of 32 or more base64 characters with an entropy of 4.0 bits or more for each character. It exempts pure hex of 40 or 64 characters, which are commit and digest values. This rule does not use the pattern list, so it catches a random token that no pattern names.
- **RE2.** All patterns are RE2, so no input causes a backtracking blowup.

## Invariants

- The enforced apicover gate lists the package, and `apicover_named_test.go` names each export.
- `panetrust.RedactSecrets` and `redact.Secrets` return the same text for the same input.
