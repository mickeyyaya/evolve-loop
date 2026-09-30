# Comment history: `acs/cycle654`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle654/predicates_test.go:3` — above `package cycle654`

```text
// Package cycle654 materialises the acceptance criteria for the single
// triage-committed top_n task of cycle 654, infra-classifier-echo-veto (weight
// 0.95, operator-injected). It is the fix-of-record for lesson
// cycle-641-infra-incident-classifier-matches-echoed-prompt-keywords, which
// recurred byte-for-byte in cycle-642: a phase whose driver exited 0 and whose
// deliverable carried a PASS sentinel was discarded FAIL because the infra /
// escalation classifiers keyword-matched the agent's OWN echoed prompt text
// ("...missing rate limits.", the reviewer checklist) as a runtime rate_limit
// signal. The fix has three layers, one predicate each, plus a negative guard:
//
//   - AC1 (C654_001) cycleclassify source-of-truth veto gate: a PASS deliverable
//   - driver-exit-0 + prompt-echo infra_failure must NOT classify infrastructure.
//   - AC3 (C654_002) negative axis: a genuine runtime infra signal (non-zero exit,
//     non-echo excerpt) MUST still classify infrastructure — the fix is not a
//     blanket disable. GREEN today; the AC1 fix must keep it green.
//   - AC1' (C654_003) normalizer emit gate: the Classifier must not emit an
//     infra_failure INCIDENT for a line that is a verbatim echo of the injected
//     prompt; a genuine line absent from the prompt must still emit.
//   - AC2 (C654_004) escalation auto-responder: echoed prompt exhaustion text is
//     stripped before the exhaustion match; a genuine CLI quota banner survives.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…653 precedent).
// Each predicate shells `go test -run` over one RED regression test authored this
// cycle in the real system-under-test package, so every predicate EXERCISES the
// SUT (Classify over an on-disk workspace; Classifier.Stderr emit path;
// stripPromptEchoLines + matchExhausted) and asserts on behaviour — none is a
// source-grep. RED now: cycleclassify vetoes the echo (001); phasestream and
// bridge fail to compile (SetInjectedPrompt / stripPromptEchoLines absent — 003,
// 004). 002 is the currently-green guard. GREEN once Builder lands the three-layer
// fix. The Acceptance-Criteria-Summary line "go test -race on touched packages
// PASS; apicover clean" is dispositioned manual+checklist in test-report.md (a
// repo-wide toolchain gate the cycle audit already runs), not predicated here.
```
