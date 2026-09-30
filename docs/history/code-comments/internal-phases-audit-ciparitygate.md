# Comment history: `internal/phases/audit/ciparitygate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/audit/ciparitygate/apicover.go:17` — above `const (`

```text
// The two apicover gates' hard-FAIL sentences for an underivable change set
// on a cycle WITH an enforce list: git failed (no repo, bad baseRef, fleet
// .git/index.lock race), so neither gate can prove its set is empty — FAIL
// loud (err == nil) instead of the silent (nil, nil) no-op that shipped an
// uncovered export (cycle-581 D1 / D2). Deliberately NOT the three whole-repo
// gates' WARN (the severity asymmetry is the cycle-581 decision).
```

### `go/internal/phases/audit/ciparitygate/apicover.go:123` — above `run := scrubbedRun(g.run)`

```text
// CI-parity env scrub, the tier step's contract (tier.go): this `go test`
// runs in the lane process, whose EVOLVE_CYCLE_STATE_FILE / EVOLVE_FLEET
// flip core's env-sensitive tests — the step failed on every wave-2 audit
// (cycles 1673, 1676) until it ran under the same allowlist as the tier.
```

### `go/internal/phases/audit/ciparitygate/apicover_env_test.go:9` — above `func TestCoverageProfile_RunsGoTestUnderTheScrubbedEnv(t *testing.T) {`

```text
// TestCoverageProfile_RunsGoTestUnderTheScrubbedEnv — the apicover step's
// coverage run is a `go test` of the lane's own packages, spawned from the
// lane process exactly like the tier step, and it must carry the same
// scrubbed allowlist env. Inheriting os.Environ() leaks EVOLVE_CYCLE_STATE_FILE
// and EVOLVE_FLEET into core's env-sensitive tests and the step fails on every
// audit (wave 2, 2026-09-14: AUDIT_CIPARITY_GATE_STEP_FAILED cover_run on
// cycles 1673 and 1676 — the leak the ship gate had, #615, on the audit's
// spawn site). The tier step scrubs (TestTierAttempts_ScrubbedEnv…); this pins
// the coverage run to the same contract: an explicit env, allowlist only.
```

### `go/internal/phases/audit/ciparitygate/apicover_test.go:17` — above `func TestApicoverEnforce_InputsOrderAndSeverity(t *testing.T) {`

```text
// Test 26 (moved intent: audit/ciparity_unit_test.go:237-245 and the pre-move
// pins TestApicoverGates_MissingEnforceListWinsOverUnderivable /
// TestChangedSetUnderivable_SeverityAsymmetryBytes) — the shared prologue's
// order and the cycle-581 severity asymmetry: no module → (nil, nil); no
// .apicover-enforce + underivable → (nil, nil) (the read precedes the
// derivable check); enforce file + underivable → exactly the golden D1 / D2
// offender, ONE GATE_FAILED{cause=underivable} and NO CHANGESET_UNDERIVABLE;
// touched∩enforced empty → (nil, nil).
```

### `go/internal/phases/audit/ciparitygate/ciparitygate.go:1` — above `package ciparitygate`

```text
// Package ciparitygate is unit 14 of the component breakdown (ADR-0103): the
// audit phase's five CI-parity decisions — the deterministic gates that decide
// whether a cycle may ship. Each runs, against THIS cycle's worktree, the EXACT
// command .github/workflows runs (go vet ./..., the -tags acs durable suite,
// the -tags integration -race tier over the touched packages with its
// cross-lane serialized clean-env retake, apicover -enforce folded in-process
// over the touched∩enforced set, and the new-package graduation check), so a
// cycle can never ship green-locally / red-in-CI.
//
// One Gates owns the five decisions over four injected collaborators: the
// subprocess runner, the change-set Strategy (the host's locator — git and the
// build handoff never enter the leaf), a clock for the lock wait, and the
// Signal Center through an accessor read at every use. The hook contract is
// the host's, verbatim: ([]offenders, nil) → FAIL; (nil, err) → WARN, the gate
// could not run (fail-open); (nil, nil) → clean. The leaf never writes stderr;
// its ten failure modes are audit.warning WARNs under module audit, coded
// AUDIT_CIPARITY_*. Design: docs/architecture/decomposition/14-ciparity.md.
```

### `go/internal/phases/audit/ciparitygate/command_test.go:41` — above `func TestOffenderLines_DropsPassingTestChatter(t *testing.T) {`

```text
// Moved verbatim (audit/ciparity_unit_test.go:103-123) — the cycle-930/931/932
// false-FAIL diagnostic corruption: only line-anchored failure markers survive.
```

### `go/internal/phases/audit/ciparitygate/command_test.go:125` — above `func TestWholeRepoGates_ArgVectorsMatchTheGolden(t *testing.T) {`

```text
// Test 16 — every subprocess vector the five gates fork equals the golden
// captured on 8e8f080f, line for line. Absorbs the three moved scope tests
// (audit/ciparity_scope_test.go): the scoped tier runs ONLY the touched
// package with -race -count=1 -p 4 -parallel 4 -tags integration and never
// shells out to `go list`; a module-root change falls back to `go list ./...`
// and tests every non-acs package; apicover's three forks carry the scoped
// cover profile.
```

### `go/internal/phases/audit/ciparitygate/envexclusive.go:5` — above `type envExclusiveEntry struct {`

```text
// envExclusiveEntry is the SINGLE record for one env-exclusive package: the
// package, the evidence, and where its integration tests actually run instead.
// Every consumer — envExclusivePkg, the emitted WARN, the mixed-scope event,
// the whole-suite filter — projects from tierEnvExclusive; no prose
// restatement elsewhere is authoritative (the 2026-09-01 architecture review
// found three stale copies inside one diff — projections, not narration).
//
// SELECTION CRITERION (the only thing keeping this list a scalpel, pinned by
// TestEnvExclusive_EntriesDeclareNoCIBackstop): a package may be listed ONLY
// when the serialized retake cannot make red-twice trustworthy AND CI
// provides no backstop. A package whose integration tier CI covers belongs IN
// the lane tier. HISTORY: internal/core, cmd/evolve and internal/phases/ship
// were excluded 2026-07-19 (8e2afef0; contention false-REDs, cycles
// 930/931/932) and the serialized retake that properly cures that contention
// landed ONE DAY LATER (3c5ed711) — the never-revisited skip shipped
// cycle-1594's red to main for 2.5 days (20e839ee; #519; #518). Two notes for
// the next adjudicator: the July evidence over-attributed cmd/evolve (its
// fleet-soak suite is in-process fakes, no real tmux — cmd_fleet_soak_test.go;
// its ~69s is CPU), and a compile-only floor would NOT have caught 1594 (an
// assertion failure, not a compile failure) — running the tier is the point.
```

### `go/internal/phases/audit/ciparitygate/envexclusive.go:28` — above `backstop string`

```text
// backstop states where the package's integration tests actually run;
// rendered VERBATIM into the emitted WARN. One dishonest word here
// recreates the #483 defect: a gate asserting coverage that cannot occur.
```

### `go/internal/phases/audit/ciparitygate/helpers_test.go:3` — above `import (`

```text
// helpers_test.go — the leaf's fakes and fixtures: a scripted runner, a fake
// change-set Strategy, a recording Center, a go-module worktree, the goldens
// captured on 8e8f080f (testdata/*.golden.*) and their path templating.
```

### `go/internal/phases/audit/ciparitygate/helpers_test.go:87` — above `func golden(t *testing.T, name string) map[string]string {`

```text
// golden reads one `key<TAB>quoted` golden captured on 8e8f080f.
```

### `go/internal/phases/audit/ciparitygate/helpers_test.go:167` — above `func killedAtDeadline(fn sysexec.RunFunc) sysexec.RunFunc {`

```text
// killedAtDeadline makes a scripted runner faithful to a process the ctx
// deadline KILLED: it returns only once ctx is done — a real SIGKILL follows
// the deadline, never precedes it — so a 1 ns budget reaches the deadline arms
// deterministically. Without it, context.WithTimeout(…, 1ns) may arm a timer
// instead of expiring synchronously (two clock reads inside one tick), and an
// instant fake is observed before ctx.Err() is set: deadlineHit=false →
// retake_red instead of the deadline verdict (2/40 runs, 2026-09-14).
```

### `go/internal/phases/audit/ciparitygate/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under the audit phase (ADR-0103
// unit 14 §2): stdlib plus the eight named internal packages, never
// internal/core, internal/changedpkgs or the host package (the compiler is the
// cycle guard; this is the leaf-ness declaration — signalcenter/importgraph_test.go
// idiom).
```

### `go/internal/phases/audit/ciparitygate/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 14 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/phases/audit/ciparitygate/offenders.go:12` — above `func offenderMarkerLine(ln string) bool {`

```text
// offenderMarkerLine is the ONE home of "this line is a real failure marker"
// — shared by offenderLines (which additionally falls back to the last lines
// when nothing matches) and hasOffenderMarker (which must NOT inherit that
// fallback: the deadline-kill path degrades to WARN precisely when no marker
// exists, and the fallback would make every non-empty truncation look judged).
// Matching is LINE-ANCHORED on real failure markers — the old substring
// heuristics ("error"/"FAIL" anywhere in the line) kept PASSING tests' verbose
// chatter while the last-12 cap pushed the real `--- FAIL` lines out, so
// cycles 930/931/932 recorded verdicts citing 12 lines of noise with the true
// offender unknowable.
```

### `go/internal/phases/audit/ciparitygate/tier.go:30` — above `func tierArgs(pkgs []string) []string {`

```text
// tierArgs is the ONE `go test` vector of the tier; its order is an on-disk
// contract through the integration-tier.log header. Faithful to CI on the
// tier and flags (-race IS included — a genuine data race in a touched
// package must fail the gate; only -cover is dropped, a CI-only concern per
// ADR-0069).
```

### `go/internal/phases/audit/ciparitygate/tier.go:126` — above `func (g *Gates) retake(req Request, dir string, args []string, run sysexec.RunFunc, first attempt) ([]string, error) {`

```text
// retake is the red-first-attempt path. Under a live fleet the -race tier
// also starves for CPU/IO (cycle-943: one package took 469s then failed;
// green in isolation), so a single red is not yet evidence: RETAKE ONCE under
// a cross-lane exclusive lock (isolation on demand — the root cause is
// contention, and serialization removes it). Both attempts persist to
// integration-tier.log (state.json truncates; the artifact is the one-grep
// diagnosis) — attempt 1 BEFORE the lock is sought, so a kill during a
// contended wait never loses it. DELIBERATE trade-offs: worst-case gate
// wall-clock doubles (attempt 1 + a fresh TierAttempt budget — red paths
// only, and a false FAIL discarding a shippable cycle costs far more). The
// verdict is decideTier's.
```

### `go/internal/phases/audit/ciparitygate/tierscope_test.go:144` — above `func TestIntegrationTierScope_CoversCoreCmdShip_Cycle1594(t *testing.T) {`

```text
// TestIntegrationTierScope_CoversCoreCmdShip_Cycle1594 is the INSTANCE
// regression for the incident: the three packages whose fossilized skip cost
// 2.5 days of red main must stay in the lane tier scope.
```
