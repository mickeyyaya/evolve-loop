# Comment history: `internal/acssuite`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/acssuite/acssuite.go:1` — above `package acssuite`

```text
// Package acssuite is the deterministic, host-side EGPS predicate-suite runner.
// It executes the Go predicate lane and writes acs-verdict.json conforming to
// the schema the audit + ship gates read (EGPS v11 — see ADR-0042; supersedes
// the bash run-acs-suite.sh of ADR-0025).
//
// The Go lane runs three scopes, each as a SEPARATE `go test -json -tags acs`
// (so a per-package compile error is a HARD error, never a silent PASS):
//   - ./acs/cycle<N>          this cycle's predicates (authored fresh)
//   - ./acs/regression/<sub>  curated durable predicates, every cycle
//   - ./acs/redteam           standing anti-gaming predicates, every cycle
//
// Each test maps to a Result via v.record. red_count == 0 ⇒ verdict PASS ⇒
// ship_eligible. A test that FAILs is RED; a t.Skip is SKIP (the TAP/automake
// convention, exit 77 in the Result) — an evidence-absent predicate (e.g. a
// runtime-only regression predicate on a fresh clone) is counted neither red nor
// green, so it cannot block the gate yet cannot fake a pass. CLI:
// `evolve acs suite --cycle N`.
```

### `go/internal/acssuite/acssuite.go:68` — above `FailingTests []string 'json:"failing_tests,omitempty"'`

```text
// FailingTests names the `--- FAIL:` tests found in this predicate's
// output — for a meta-predicate that shells an inner `go test`, these are
// the INNER failures, the exact identity the excerpt cap used to destroy
// (cycles 1107/1116/1123: head-truncated evidence left no failing test
// name anywhere on disk, making their false reds permanently
// unconfirmable). Deduped, bounded by maxFailingTests.
```

### `go/internal/acssuite/acssuite.go:87` — above `Flaky string 'json:"flaky,omitempty"'`

```text
// Flaky marks a predicate that was RED on the first run and GREEN on the
// single bounded retry (cycle-468): value "passed-on-retry". Visible in
// the wire JSON so a flake is never silently absorbed; the first-run
// failure evidence is retained deliberately (the flake's signature).
// The meaning is PINNED by acs/cycle468 (deterministic reds carry no
// flaky key) — retry outcomes for reds live in RetryOutcome instead.
```

### `go/internal/acssuite/acssuite.go:95` — above `RetryOutcome string 'json:"retry_outcome,omitempty"'`

```text
// RetryOutcome records what the bounded retry established about a red
// that STAYED red: "red-on-retry" (the retry ran and confirmed) or
// "retry-inconclusive" (the retry produced no result for this test —
// expired ctx, crash). Absent on greens, skips, and absorbed flakes.
// Without it a confirmed red was indistinguishable from a starved
// retry (batch-18 forensics gap).
```

### `go/internal/acssuite/acssuite.go:148` — above `Warnings []string 'json:"warnings,omitempty"'`

```text
// Warnings surfaces non-blocking anomalies (cycle-468: flaky predicates
// that passed on the bounded retry). Projection of Result.Flaky.
```

### `go/internal/acssuite/acssuite.go:151` — above `SuiteRoot   string 'json:"suite_root,omitempty"'`

```text
// SuiteRoot / ProjectRoot record which roots this verdict was minted
// under (cycle-1434: a verdict minted with the WRONG state root red'd 3
// predicates the correct-root run showed green, and nothing in the
// artifact said so). omitempty: verdicts written before these stamps
// stay byte-compatible, and readers treat absence as "unstamped", never
// as a mismatch.
```

### `go/internal/acssuite/acssuite.go:165` — above `ProjectRoot string`

```text
// ProjectRoot is the MAIN project root whose `.evolve/` holds the runtime data
// (history under .evolve/runs/, baselines, the current build-report) that
// predicates read via ${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}. When set, it is
// exported as EVOLVE_PROJECT_ROOT to each predicate so a suite run from a
// worktree (Root=worktree, post issue-#9 audit-cwd=worktree) still resolves
// `.evolve/` to main rather than the worktree (where `.evolve/` is absent).
// Empty → predicates inherit the caller's env. (issue #12)
```

### `go/internal/acssuite/acssuite.go:202` — above `lockCtx, lockCancel := context.WithTimeout(context.Background(), maxLockWait)`

```text
// ADR-0080 P1: the suite execution is host-wide SINGLE-FLIGHT. Fleet
// lanes are separate processes each shelling full package suites; run
// concurrently they oversubscribe the host and turn long suites into
// false reds (batch-16: TouchedPackagesStayGreen red on 1166/1167/1169,
// green in the preserved worktree — an identical-fingerprint halt over
// infrastructure). Blocking is bounded by the caller's lifetime only:
// verification MUST run, serialized, never skipped.
// Bounded wait (review HIGH): a wedged HOLDER must degrade this lane to
// unserialized (WARN below), never deadlock the fleet — the whole degrade
// path exists for exactly that, and an unbounded Background ctx made it
// unreachable.
```

### `go/internal/acssuite/acssuite.go:274` — above `func predicateEnv(projectRoot, worktreeRoot string, changedPkgs []string) []string {`

```text
// predicateEnv builds the env exported to BOTH lanes. The dual-root pattern:
//   - EVOLVE_PROJECT_ROOT (STATE root) → MAIN, so predicates resolve `.evolve/`
//     runtime data to main even from a worktree (issue #12).
//   - EVOLVE_WORKTREE_ROOT (SOURCE root) → the cycle's worktree, so predicates
//     that validate a generated-from-source doc (e.g. `evolve flags check` /
//     `evolve skills check`) read the WORKTREE artifact the cycle commits — not
//     main's stale working copy. Without this, such a predicate red-fails correct
//     work because the doc only reaches main at ship, after audit (cycle-355).
//   - CHANGED_PACKAGES → the cycle's touched packages, so a predicate can scope
//     `go test` (cycle-200).
//
// With no extras it equals os.Environ() — the prior inherit behavior.
```

### `go/internal/acssuite/acssuite.go:443` — above `env := predicateEnv(opts.ProjectRoot, opts.Root, changed)`

```text
// opts.Root is the cycle's worktree (resolveACSSuiteRoot → active_worktree);
// export it as EVOLVE_WORKTREE_ROOT so source/doc predicates validate the
// committed worktree artifact, not main's stale copy (cycle-355 fix).
```

### `go/internal/acssuite/acssuite.go:477` — above `func retryFlakyReds(ctx context.Context, goExec func(context.Context, string, string, []string) (string, error), moduleD…`

```text
// retryFlakyReds is the cycle-468 bounded flake absorber: when a scope's first
// run produced >=1 RED and NO red is the synthetic egps/ parse-error red
// (any such red marks the whole stream untrustworthy and suppresses the
// retry entirely — a truncated stream is not retryable evidence), the
// scope is re-run EXACTLY ONCE. A red that passes on the retry
// flips to green with the visible Flaky="passed-on-retry" annotation (first-run
// evidence retained — the flake's signature); a red that stays red keeps its
// first-run result. Greens/skips and the result set's size are untouched (the
// retry can only flip existing reds, never add or duplicate results). Rationale:
// parallel_evaluate=enforce runs the -race predicate suites under concurrent
// evaluate-phase host load; contention flakes burned 4 verified-good cycles
// (444/447/466/467). The gate is not weakened: the retry is bounded to one,
// annotated on the wire, and surfaced as a verdict warning.
```

### `go/internal/acssuite/acssuite_adversarial_test.go:3` — above `import (`

```text
// acssuite_adversarial_test.go — cycle-281 test amplification.
// Targets uncovered branches: cycleNumFromDir (66.7%), goLaneTimeout (62.5%),
// predicateEnv (66.7%), goLanePatterns (50.0%), excerpt (75.0%),
// WriteVerdict (83.3%), changedPackagesForCycle (28.6%).
```

### `go/internal/acssuite/acssuite_adversarial_test.go:121` — above `long := "HEAD" + strings.Repeat("x", evidenceMax+50) + "TAIL"`

```text
// AMENDED with the tail-anchoring fix (false-red hardening, 2026-07-27):
// the ellipsis moved from suffix to PREFIX because excerpt now keeps the
// END of over-limit output — go-test failure detail accumulates at the
// tail, and head-keeping is what destroyed the failing-test identity of
// cycles 1107/1116/1123. Same truncation-marker contract, opposite anchor.
```

### `go/internal/acssuite/apicover_named_test.go:25` — above `func TestWriteVerdict_LandsAtVerdictFilename(t *testing.T) {`

```text
// TestWriteVerdict_LandsAtVerdictFilename names acssuite.VerdictFilename and
// pins the belief the const exists for: the writer and every external reader/
// retirement path (core/audit_round_artifacts.go, cycle-1603) agree on ONE
// spelling because WriteVerdict itself derives its destination from the const.
```

### `go/internal/acssuite/evidence_tail_test.go:3` — above `import (`

```text
// evidence_tail_test.go — the evidence must carry the FAILURE, not the boot
// noise. Measured across all five red predicates of cycles 1107/1115/1116/1117
// and again on cycle-1123: evidence_excerpt was head-truncated at evidenceMax,
// a go-test run's first 600 chars are compiler/WARN noise, so "--- FAIL:" was
// present in NONE of them and for 1107/1116/1123 the failing test name is
// permanently unrecoverable — the reason those cycles' false reds could never
// be confirmed or refuted from disk. The excerpt must be TAIL-anchored (Go
// test output accumulates assertion detail and the FAIL line at the end), and
// a red Result must either name the failing inner tests or say why it cannot.
```

### `go/internal/acssuite/phantom_binding.go:3` — above `import "regexp"`

```text
// phantom_binding.go — a red whose bound test NEVER RAN is named as such.
//
// Cycle-suite predicates commonly delegate to named "binding" tests in a
// production package (`go test -run '^(Name)$' <pkg>`, then require the
// `--- PASS: Name` line). That bind is identity by NAME across a package
// boundary, and the name can silently stop resolving — a continuation cycle
// renames the tests, or a builder never writes them (cycle-1532). The predicate
// then reds FOREVER: `go test -run` on a pattern matching nothing exits 0 with
// "no tests to run", the assert reports "did NOT pass", and the red reaches the
// EGPS gate as a bare count nobody can act on. On a continuation chain that red
// is ABSORBING — the 1539-1546 streak's dominant class, cured in PR #486 by a
// 2-line binding repoint that took a console session to diagnose.
//
// The discriminator is deliberately NOT the "no tests to run" string alone:
// that only appears when NOTHING matched. The robust rule covers the partial
// shape too (siblings matched, the renamed one silently didn't): a name the
// predicate reports as did-NOT-pass that is ALSO absent from the failing set
// never ran at all. FailingTests is the authority because it is extracted from
// the FULL output before the excerpt cap (cycles 1107/1116/1123) — truncation
// cannot fake a phantom.
//
// Anti-gaming boundary, stated: classification NEVER changes the verdict. A
// phantom red is still red — skipping it would make deleting a test the way to
// green a gate. What changes is that the red now names its own cure.
```

### `go/internal/acssuite/phantom_binding_test.go:3` — above `import (`

```text
// phantom_binding_test.go — a red whose bound test NEVER RAN must say so, with
// the exact names.
//
// The 1539-1546 streak's dominant class (inbox
// phantom-binding-predicates-absorb-continuation-chains): cycle-1544's
// predicates bound BY NAME to tests a continuation cycle later renamed. The
// bound names stopped resolving, `go test -run` printed "no tests to run", the
// predicate red'd forever, and the red surfaced as a bare EGPS red_count no one
// could act on. The cure was a 2-line binding repoint (PR #486) — but nothing
// on disk said so; the diagnosis took a console session.
//
// Classification authority is FailingTests (extracted from the FULL output
// before the excerpt cap, precisely so truncation cannot destroy identity —
// cycles 1107/1116/1123): a name the predicate reports as did-NOT-pass that is
// ALSO absent from the failing set never ran at all. That covers both the
// all-phantom shape ("no tests to run") and the partial shape (siblings ran,
// the renamed one silently didn't). A bound test that exists and FAILS is not
// a phantom and must classify exactly as today.
```

### `go/internal/acssuite/phantom_binding_test.go:28` — above `const cycle1546PhantomOutput = '=== RUN   TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase`

```text
// The REAL cycle-1546 evidence shape, verbatim from its acs-verdict.json lanes:
// both bound names renamed, nothing matched, go test printed "no tests to run".
```

### `go/internal/acssuite/phantom_binding_test.go:136` — above `func TestParseGoTestJSON_RecordUsesFullOutputAndFailingSet(t *testing.T) {`

```text
// M5's kill, both halves. The classification must be computed from the FULL
// output with the FULL failing set — not from the truncated excerpt with no
// failing set. Half one: a bound name that also appears as `--- FAIL:` in the
// stream is a FAILING test, and the record path must not call it a phantom.
// Half two: the binding vocabulary buried beyond the excerpt cap must still
// classify — head+tail excerpting destroying identity is the exact class
// FailingTests was built to survive (cycles 1107/1116/1123).
```

### `go/internal/acssuite/red_evidence_test.go:3` — above `import (`

```text
// red_evidence_test.go — full-output persistence for RED predicates. Three
// batch-18 false-reds (cycles 1173/1175/1178) were undiagnosable after the
// fact because the 600-byte excerpt elides the MIDDLE of the inner go-test
// stream (the same lesson class as 1107/1116/1123, which FailingTests only
// partially fixed: names survive, assertions do not). A red's full stream now
// lands beside the verdict; the bounded retry's outcome is recorded in
// RetryOutcome — NOT Flaky, whose passed-on-retry-only meaning is pinned by
// acs/cycle468 (deterministic reds carry no flaky key and no warnings).
```

### `go/internal/acssuite/red_evidence_test.go:97` — above `if red.Flaky != "" {`

```text
// The cycle-468 pin: a deterministic red carries NO flaky key and adds NO
// warnings — RetryOutcome and the evidence file are the forensic surface.
```

### `go/internal/acssuite/scopelint.go:17` — above `type ScopeFinding struct {`

```text
// scopelint.go — demote out-of-scope meta-predicates instead of letting them
// false-fail cycles.
//
// Cycles 1115/1116/1117/1123 each FAILed on one predicate of the shape
// "go test <core+bridge+recovery> stays green". A whole-suite sweep samples
// everything in those packages — an auditor probe, a shared test fixture
// root, a concurrent lane — and reports any contamination as a builder
// regression. Whole-repo staleness is the regression suite's job; a cycle
// predicate re-sweeping UNTOUCHED packages is duplication with a false-red
// surface. Such predicates are demoted to SKIP (a skip counts neither red nor
// green, so a lint false-positive can never fail a cycle) with a loud
// EvidenceNote + verdict warning.
//
// Known conservative limitation: patterns assembled by string concatenation
// or returned from helper functions are invisible to the lint. That is the
// safe direction (a missed broad predicate merely stays un-demoted), and the
// demotion floor below bounds what a deliberately-broad authoring style could
// ever hide.
```

### `go/internal/acssuite/scopelint.go:43` — above `func LintPredicateScope(dir string, touched []string) ([]ScopeFinding, error) {`

```text
// LintPredicateScope parses the cycle predicate sources in dir and returns a
// finding per test function that references a Go package pattern outside
// touched. An empty touched set lints nothing — with no scope authority a
// wrong guess would demote a legitimate gate. Const indirection is resolved
// (cycle-1117's bridgePkg shape): package-level string consts/vars count as
// references in every function that names them.
```

### `go/internal/acssuite/scopelint.go:191` — above `var scopeLintChangedPackages = func(worktreeRoot string) []string {`

```text
// scopeLintChangedPackages derives the touched set for the lint from GIT, and
// git only — never the builder-written handoff. Two reasons, both from
// adversarial review: the handoff has been extinct since ~cycle-215 (reading
// it made the lint dead code on every live cycle), and it is agent-authored,
// so trusting it would let a builder shrink `touched` to demote sibling
// predicates (gate-weakening). Seam var so tests can inject.
```

### `go/internal/acssuite/scopelint_test.go:3` — above `import (`

```text
// scopelint_test.go — whole-suite meta-predicates are the false-red AMPLIFIER.
// Cycles 1107/1115/1116/1117/1123 each failed on ONE predicate of the shape
// "go test <core+bridge+recovery> stays green": any contamination anywhere in
// those suites (an auditor probe, the shared /tmp/p root, a sibling lane)
// reads as a builder regression. Whole-repo staleness is the regression
// suite's job (195 predicates, every cycle) — a cycle predicate re-sweeping
// packages the cycle never touched is pure duplication with a false-red
// surface. The lint demotes such predicates to SKIP (never RED — a lint
// false-positive must not be able to fail a cycle), loudly, in the verdict.
```

### `go/internal/acssuite/verdict_provenance_test.go:3` — above `import (`

```text
// verdict_provenance_test.go — cycle-1434 (ADR-0072 halt): a verdict minted
// under the WRONG state root red'd 3 predicates the correct-root run showed
// green, and the artifact recorded nothing about which roots it was minted
// under — the misdiagnosis was invisible from the file. Every verdict now
// stamps suite_root/project_root; readers treat ABSENCE as "unstamped"
// (pre-stamp verdicts stay honored), never as a mismatch.
```

## phase 3d r1 (subagent, failurelog, acssuite)

### `go/internal/acssuite/acssuite_golane_test.go:258` — above `mustMkdir(t, filepath.Join(goDir, "acs", "cycle5"))`

```text
// A Go module with an acs/ tree and a cycle5 package — but the run is cycle 9.
```
