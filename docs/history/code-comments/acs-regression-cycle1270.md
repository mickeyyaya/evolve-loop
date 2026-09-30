# Comment history: `acs/regression/cycle1270`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle1270/predicates_test.go:3` — above `package cycle1270`

```text
// Package cycle1270 materialises the cycle-1270 acceptance criteria.
//
// Scope note (read this before judging the predicates below). This lane is an
// ADR-0076 continuation of the cycle-1268 salvage snapshot
// (continuation-manifest.json: base 85b3d368, snapshot 8e221b83), so the bulk of
// all three triage-committed items is ALREADY IN TREE (`git diff --stat
// main...HEAD`: 32 files, +2911) and the scout report's "unimplemented" claims
// are stale against this branch. The TDD phase verified the live tree instead of
// trusting the report and pinned only what is genuinely open — the residuals the
// fault-localization phase ranked as suspects 5-7 — plus the deterministic
// build-floor blocker that phase ranked #1-#3.
//
// The three triage-committed items (`triage-report.md` ## top_n):
//
//	infra-teardown-predicate-single-source  → 004, 005      (residual: scan-root hole)
//	retro-fleet-worktree-dispatch           → 006, 007      (residual: absent positive join)
//	test-amplification-context-scope        → 008, 009      (residual: silent degradation)
//
// The blocker (001-003) is NOT a triage item. It is the recorded cause of
// cycle-1268's death (`.evolve/runs/cycle-1268/audit-fail-reason.json`:
// `./cmd/evolve: unit tests FAIL`) and this cycle's fault-localization phase
// re-verified it will re-fire here, because this diff touches
// go/cmd/evolve/cmd_worktree.go and the floor therefore runs that package again:
//
//	gitexec.AddWorktreeWithRetry retries on ANY non-zero exit with a real
//	time.Sleep ladder (2s + 4s). 33 cmd/evolve tests transitively reach
//	core.gitWorktree.Create over a t.TempDir() that is not a git repository —
//	a PERMANENT rc=128 — so each pays the full 6s. Measured: 33 x 6s = 198s of
//	pure backoff in a package the floor runs with -timeout 120s. Deterministic,
//	not a flake. ADR-0082:83's claim that "test tiers install a no-op sleep …
//	so no suite pays the ladder" is false on the transitive-dispatch axis: the
//	seam (core/worktree.go:129 worktreeAddRetrySleep) is unexported and in a
//	different package than the 33 tests that reach it.
//
// Pinning the blocker here is deliberate. Triage cannot have committed it (it
// was diagnosed AFTER triage, by the phase whose job that is), no predicate can
// pass while it stands, and the anti-goals below are the ones that make a
// "green" cycle a lie. It binds no DEFERRED floor: 001-003 target
// internal/gitexec + internal/core, which are already Task 1's own package set.
// `tokenopt-session-resume-on-retry` is ## deferred and carries ZERO predicates
// (R9.3).
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987/1255/1268
// precedent). Each predicate shells `go test -run '^(names)$' -v -count=1` over
// exactly ONE named package and requires a `--- PASS: <name>` line per test.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN. This is what makes these RED today.
//   - No source-grep is load-bearing anywhere in this file (the cycle-85 ban):
//     every assertion runs the system under test.
//   - Flaky-predicate-shape rules: every invocation names EXACTLY ONE package,
//     never ./..., and the ones naming ./internal/core and ./cmd/evolve (known
//     40s+ suites) are narrowed with -run, which the rule explicitly permits.
//     No wall-clock bounds, no literal PIDs, no bare `git`, no load generators.
//     003 in particular asserts on OUTPUT STATE (did the ladder announce?), not
//     on elapsed time — a duration bound would be exactly the banned shape, and
//     would also stretch arbitrarily under fleet contention.
```

### `go/acs/regression/cycle1270/predicates_test.go:132` — above `func TestC1270_001_PermanentWorktreeAddFailureCostsZeroBackoff(t *testing.T) {`

```text
// TestC1270_001_PermanentWorktreeAddFailureCostsZeroBackoff — edit locations 1-2.
//
// The shared retry loop must consult a transience predicate BEFORE it sleeps, so
// a permanent condition (`fatal: not a git repository`, rc=128) returns after
// attempt 1 instead of paying the 2s+4s ladder. The three required tests are one
// per axis, and each one is load-bearing on its own:
//
//   - PermanentFailureSkipsBackoff — the fix. Zero sleeps, ONE invocation, and
//     the final rc + git's own stderr still returned intact. The loud-fail
//     contract (gitexec/worktree.go:46-50) is preserved: refuted PR #400 is the
//     record of what silencing the alarm costs, and this predicate must not be
//     satisfiable by suppressing the error.
//   - TransientStillRetriesToBound — the NEGATIVE guard. A "fix" that simply
//     stopped retrying would pass the first test and re-break PR #401's
//     collision absorber; rc=255 lock-shaped must still ride the full bound.
//   - NilRetryablePreservesRetryEverything — the zero value stays usable, so no
//     existing caller silently changes behaviour when the field is added.
```

### `go/acs/regression/cycle1270/predicates_test.go:154` — above `"TestAddWorktreeWithRetry_RetriesTransientFailure",`

```text
// PR #401's contract must stay green through the change — "existing
// tests keep passing" is part of the criterion, not a courtesy.
```

### `go/acs/regression/cycle1270/predicates_test.go:164` — above `func TestC1270_002_CoreSuppliesTransiencePredicateAndHonestAnnouncement(t *testing.T) {`

```text
// TestC1270_002_CoreSuppliesTransiencePredicateAndHonestAnnouncement — edit
// location 4, plus the diagnosability half of edit location 5.
//
// core is the caller that must actually SUPPLY the predicate — a classifier that
// exists in gitexec but is never passed leaves the 198s tax exactly where it is
// (the "seam whose only caller is a test" shape the house rules ban). And
// OnRetry currently prints `after transient rc=%d` for failures it has not
// established are transient, which is how a permanent rc=128 came to be logged
// as contention 33 times; the announcement must not claim transience it has not
// classified.
//
// BuildFloorSelfCheckFailures_KeepsTailDiagnostic is the reason cycle-1268 was
// undiagnosable rather than merely broken: the floor truncates with head[:400],
// but Go writes the panic/--- FAIL lines at the TAIL, so the recorded reason was
// 400 bytes of `[engine] WARN: Deps.TokenResolver is nil` and nothing else. The
// operator was handed noise where the stack trace was.
```

### `go/acs/regression/cycle1270/predicates_test.go:280` — above `"TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError",`

```text
// Renamed 2026-08-24 (cycle-1551 fix): the gate's single-source
// predicate widened to IsOptionalSkippableError (infra teardown OR
// missing persona doc) and the equivalence proof was retargeted with
// it — same pin, wider predicate, still exactly one spelling.
```

### `go/acs/regression/cycle1270/predicates_test.go:288` — above `func TestC1270_006_MintedScratchCwdClearsTheFleetGuard(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// TASK 2 — retro-fleet-worktree-dispatch (triage ## top_n).
// Root cause fixed upstream (PR #401, a497ffe1); the item's own acceptance
// criterion is the REGRESSION TEST, which is what is still open.
// Fault-localization suspect #6 (0.40).
// ---------------------------------------------------------------------------
```

### `go/acs/regression/cycle1270/predicates_test.go:319` — above `func TestC1270_007_RetroFleetDispatchCarriesLaneWorktreeEndToEnd(t *testing.T) {`

```text
// TestC1270_007_RetroFleetDispatchCarriesLaneWorktreeEndToEnd — the item's
// literal acceptance criterion ("a fleet-mode dispatch test proving retro's
// BridgeRequest carries the lane worktree").
//
// The new test must join retro's mint to the bridge's guard through the value
// that actually travels: the worktree retroWorktree resolves must be a real,
// existing, workspace-owned directory that the fleet guard's own predicate
// accepts — not merely a non-empty string. A fabricated path would satisfy
// "non-empty" and still strand the lane at isDir().
//
// The four existing pins ride along because they are the properties a
// convenience fix would quietly trade away: a provisioned worktree must pass
// through VERBATIM (a fallback that fired unconditionally would strand every
// normal retro in an empty scratch dir with no repo), the fallback must never
// resolve to the shared main tree or the dispatching process cwd (the leak the
// guard closes, and the shape refuted PR #400 tried to ship), and with no owned
// workspace it must return "" rather than fabricate a path.
```

### `go/acs/regression/cycle1270/predicates_test.go:379` — above `func TestC1270_009_CoveringCorpusContractSurvives(t *testing.T) {`

```text
// TestC1270_009_CoveringCorpusContractSurvives — the regression floor under 008.
//
// Every one of these is a property a "just derive it on more paths" change can
// break, and each was paid for by an incident or an audit finding:
// fail-open when nothing is derivable, an exact truncation count that makes a
// trimmed corpus impossible to mistake for a complete one, and the sanitiser
// that neutralises attacker-influenced filenames before they are interpolated
// into a document agent.md declares AUTHORITATIVE (a backtick closes the code
// span; a newline injects top-level markdown into a code-writing agent's input).
//
// LeavesBenignPathsVerbatim is the OOD counterweight: a sanitiser that mangled
// ordinary Go test paths would pass the injection test and quietly corrupt every
// real corpus.
```
