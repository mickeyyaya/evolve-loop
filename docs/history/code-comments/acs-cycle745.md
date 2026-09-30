# Comment history: `acs/cycle745`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle745/predicates_test.go:3` — above `package cycle745`

```text
// Package cycle745 materializes the cycle-745 acceptance criteria for the sole
// triage-committed top_n task token-resolver-boot-warn (triage-report.md
// ## top_n; the two other scout selections were dropped as out-of-fleet-scope,
// so per R9.3 no predicates bind to them and no deferred-floor predicates
// exist).
//
// AC map (1:1), derived from the top_n task text ("add boot-time WARN when
// Deps.TokenResolver is nil at construction") plus the scout AC summary
// ("regression test asserts this"):
//
//	AC1 nil TokenResolver at Engine construction ⇒ exactly one
//	    WARN line naming TokenResolver on the engine Stderr        → C745_001
//	AC2 wired TokenResolver ⇒ NO such WARN (negative / anti-noise) → C745_002
//	AC3 both production composition roots keep wiring a non-nil
//	    resolver, so the WARN never fires on a healthy boot
//	    (regression pin; pre-existing GREEN)                       → C745_003
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract, which EXERCISES the SUT (NewEngine against an injected
// Stderr buffer / the real composition-root constructors) — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green.
```

### `go/acs/cycle745/predicates_test.go:56` — above `func TestC745_001_EngineWarnsOnNilTokenResolver(t *testing.T) {`

```text
// AC1 — the incident twin: constructing an Engine with a nil TokenResolver
// emits exactly one WARN line naming TokenResolver on the injected Stderr, so
// telemetry fail-open (the all-zeros first-batch incident) is loud, not silent.
```
