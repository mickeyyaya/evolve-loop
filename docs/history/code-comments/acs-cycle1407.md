# Comment history: `acs/cycle1407`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1407/predicates_test.go:3` — above `package cycle1407`

```text
// Package cycle1407 materialises the acceptance criteria for this lane's two
// fleet-assigned tasks, both inside go/internal/deliverable:
//
//   - salvage-baseline-rate-report        (fleet id task-a-salvage-extraction-stage)
//   - salvage-classifier-quoted-decoy-case (fleet id task-b-decoy-sentinel-fixture)
//
// # Task A — why a summarizer, and why a CLI caller
//
// salvage_instrument.go has been WRITING .evolve/bad-verdict-baseline.jsonl
// since cycle-1389, and nothing has ever read it: grepping the tree for
// `bad-verdict-baseline` outside the writer and its own tests returns only the
// writer. The salvage-layer portfolio item gates its extraction/coercion stage
// on a measured recoverable-malformed RATE, so the gate is currently blocked on
// a number no code computes. This lane computes it.
//
// The summarizer is a pure function over an io.Reader, but a pure function
// whose only caller is a test is dead code (house rule 2 — a wiring proof is a
// REACHABILITY test, not a unit test). Predicate 003 therefore drives the REAL
// `evolve` binary end to end and 004 executes the invocation the docs publish.
// Scout listed a "CLI-facing surface" for Task 1 while triage's files={} set
// named only internal/deliverable + docs; the triage set is evidence, not an
// allowlist, so go/cmd/evolve/ is in scope for this lane — without it there is
// no production caller and 003 can never go green.
//
// # Task B — the fixture already passes, for the WRONG reason
//
// The scout AC ("quoted sentinel examples in prose must not register as
// Recoverable") was probed against HEAD before this contract was frozen:
//
//	ClassifyBadVerdict(cycle1298-quoted-decoys.md)
//	  -> recoverable=false pattern="" reason="evolve-verdict sentinel present but
//	     its payload is not recoverably malformed"
//
// Recoverable=false — so the literal AC is pre-existing GREEN (pinned by 005).
// But the REASON proves the classifier never reached the report's own tail
// sentinel: sentinelPayloadRE.FindStringSubmatch takes the FIRST match in the
// document, and the first match in that fixture is a QUOTED DECOY — an
// other-phase sentinel the auditor echoed into prose while describing the
// cycle-1298 F-1 bypass. The classifier reproduces, inside itself, the exact
// first-sentinel-wins defect the fixture was landed to document, and the exact
// class `.evolve/instincts/lessons/cycle-641-...yaml` names ("classifiers MUST
// exclude any span that is a verbatim echo of injected prompt/instruction
// text"). So the real, RED-able criterion is DECOY IMMUNITY, and 006 is the
// crux: with a genuinely malformed tail sentinel appended to the real corpus,
// the classifier must classify from THAT sentinel, not from a quoted decoy.
//
// 007 is the anti-overcorrection guard: "take the LAST match instead of the
// first" also satisfies 006, and is still wrong. A decoy quoted AFTER the real
// sentinel must be ignored too, so the fix has to be genuine quote-awareness.
//
// # Predicate strategy
//
// Every predicate calls the system under test and asserts on a return value,
// a process exit code, or emitted output. None is a source-grep of production
// code (the cycle-85 degenerate-predicate ban). 008's file-content half is a
// single-source-of-truth check (no re-typed fixture) that rides on top of an
// executed `go test` run whose named subtest must actually report PASS — a
// `-run` pattern matching nothing also exits 0, so the "--- PASS:" line is what
// rules the no-op out.
//
// Reachability probe (cycle-644 obligation, run before freezing 003/004's
// package-qualified pin): `cmd/evolve` importing `internal/deliverable` was
// compiler-probed with a throwaway blank import — `go build ./cmd/evolve/`
// exit 0. The import is buildable, not merely plausible.
```

### `go/acs/cycle1407/predicates_test.go:80` — above `const decoyFixtureRel = "go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md"`

```text
// decoyFixtureRel is the real cycle-1298 adversarial-review report: 5 quoted
// sentinel decoys in prose plus the report's own tail sentinel. It is read from
// its ONE canonical location under phasecontract/testdata — never re-typed —
// so this suite and phasecontract's sentinel_tailanchor_test.go stay bound to
// the same bytes.
```

### `go/acs/cycle1407/predicates_test.go:104` — above `func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int) {`

```text
// acsSubprocess wraps SubprocessOutput so a missing toolchain skips instead of
// red-failing on a bare export (same guard as cycle-1156's predicates).
```

### `go/acs/cycle1407/predicates_test.go:337` — above `func TestC1407_005_quoted_decoy_corpus_alone_is_not_recoverable(t *testing.T) {`

```text
// AC-B1 (pre-existing GREEN — pinned deliberately). The scout AC as literally
// written: the cycle-1298 corpus, whose 5 quoted sentinels are prose echoes,
// must not be classified Recoverable.
//
// Probed GREEN on HEAD before this contract was frozen, and recorded as such in
// test-report.md rather than presented as new work. It is kept because it is
// the property a fix to 006/007 could most plausibly break: an implementation
// that starts trusting sentinels more eagerly regresses right here.
```

### `go/acs/cycle1407/predicates_test.go:445` — above `func TestC1407_009_new_exported_symbols_pass_the_apicover_gate(t *testing.T) {`

```text
// AC-A5 (RED, house-rule floor — apicover graduation). ./internal/deliverable is
// already enrolled in go/.apicover-enforce (line 232), so the repo-wide ADR-0069
// gate requires every NEW exported symbol to be named AND exercised in
// internal/deliverable/apicover_named_test.go. Task A adds
// SummarizeBadVerdictBaseline and BaselineSummary; omitting them aborts a later
// build phase with an unenrolled-symbol failure that reads as unrelated.
//
// Executed, not grepped: this runs the package's own apicover gate for the
// enrolled package and requires exit 0.
```

### `go/acs/cycle1407/predicates_test.go:467` — above `tmp := t.TempDir()`

```text
// Reproduce CI's exact invocation (.github/workflows/go.yml:118-128):
// `apicover -enforce -cover coverage.func.txt <dir>`. The -cover profile is
// NOT optional — flagless, every named-but-unmeasured symbol reads as
// false-green (probed: 22 pre-existing false-greens, all of which vanish
// once the profile is supplied). A predicate that omitted it would demand
// Builder fix 22 unrelated symbols to go green: unsatisfiable by design,
// which is the cycle-644 trap this contract must not re-set.
```
