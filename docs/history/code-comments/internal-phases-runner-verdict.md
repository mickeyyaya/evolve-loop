# Comment history: `internal/phases/runner/verdict`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/runner/verdict/artifact.go:22` — above `type Snapshot struct {`

```text
// Snapshot is the (size, mtime) identity of the canonical artifact at one
// instant — the same key the bridge's stability window and baseline use, so
// the runner's and the bridge's notion of "unchanged" cannot drift. Only
// StatSnapshot mints one: the host takes it pre-dispatch and the engine judges
// it post-dispatch (the cycle-1550 stale-leftover gate).
```

### `go/internal/phases/runner/verdict/artifact.go:109` — above `func acsFloorRescues(phase, workspace, report string) bool {`

```text
// acsFloorRescues reports whether a teardown-time deliverable.Verify not-OK
// should be OVERRIDDEN by the deterministic ACS ground truth
// (verdict-incoherence family: cycles 603/921/924/931/3). report is the
// VERIFIED deliverable content (the bytes Verify read — Result.Content),
// never a fresh read: the rescue decision and the subsequent Classify must
// judge the SAME snapshot. True iff ALL hold:
//   - the phase is audit (the acs-verdict.json + coherence floor are audit-scoped);
//   - the acssuite verdict is PASS — a NON-LLM signal a session stall cannot corrupt;
//   - the report declares a PASS-class verdict sentinel (via the canonical
//     ParseVerdictSentinel, with its placeholder-echo guard — read by ReadCycleVerdicts);
//   - the report echoes THIS cycle's minted challenge token, read through
//     phasecontract.ChallengeToken (anti-gaming: a stale, forged, or cross-cycle
//     report cannot be laundered to PASS by the ACS verdict alone).
//
// This is precisely the (audit==PASS && acs==PASS) condition the ADR-0072
// coherence floor flags as incoherent — reusing coherence.ReadCycleVerdicts
// keeps a single definition of "both verdicts agree on PASS". It never
// manufactures a PASS: a malformed/verdict-less/token-missing report, or a
// non-ship-eligible suite, declines.
```

### `go/internal/phases/runner/verdict/artifact_test.go:3` — above `import (`

```text
// artifact_test.go — the on-disk helpers (ADR-0103 unit 11 §6 tests 28-31): the
// single-read decision, the forensic renderers, the (size, mtime) snapshot, the
// challenge-token reader.
```

### `go/internal/phases/runner/verdict/classify.go:49` — above `func (e *Engine) selectVerdictBytes(ctx context.Context, d Dispatch, r reconciliation) verdictSource {`

```text
// selectVerdictBytes applies the VERDICT SOURCE rule (ADR-0072 coherence).
// For a phase that HAS a deliverable contract, the on-disk report is the SOLE
// verdict source — the terminal pane (bres.Stdout) is never classified: the
// pane is bridge scrollback that can lose the real sentinel to a TUI `Write`
// collapse AND carry the contract's own prompt-echoed EXAMPLE sentinels, so
// classifying it fabricates a verdict the agent never emitted (cycle-603,
// recurring 877→921). SINGLE READ: the classified bytes ARE the verified
// bytes (Result.Content); the path is never re-read here.
//   - err != nil  → no contract (or an IO fault): the pane stays the source.
//   - res.OK      → contracted + well-formed: classify the VERIFIED bytes.
//   - !res.OK     → contracted + malformed/absent after the settle WAIT: a
//     COHERENT deliverable-production FAIL. The bytes still reach Classify (a
//     phase may derive a legitimate NON-SHIP verdict from partial content —
//     intent delta's "[intent-unchanged]" → SKIPPED); the ship guard then
//     stops a verification-FAILED deliverable from laundering a ship-eligible
//     verdict, and the contract codes are surfaced as diagnostics.
//
// The reconcile step already verified the deliverable, so its probe's bytes
// are reused instead of verifying — or reading — again. This ladder honours
// ctx (the agent exited 0; nothing more is coming).
```

### `go/internal/phases/runner/verdict/classify_test.go:3` — above `import (`

```text
// classify_test.go — the classify half (ADR-0103 unit 11 §6 tests 26-27): the
// fault-only unverified-deliverable signal, the ship guard, the violation
// trail and the clean-stdout companion.
```

### `go/internal/phases/runner/verdict/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under the runner (ADR-0103 unit
// 11 §2): stdlib plus the six named internal packages, never the host, never
// internal/log, the bridge, the env or the process streams (the compiler is
// the cycle guard; this is the leaf-ness declaration — the
// signalcenter/importgraph_test.go idiom). Direct imports only: coherence
// reaches policy transitively, which is fine.
```

### `go/internal/phases/runner/verdict/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 11 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/phases/runner/verdict/reconcile.go:47` — above `if core.IsInfraTeardownError(d.BridgeErr) {`

```text
// A bridge INFRA teardown — an artifact-wait timeout (exit 81) OR a
// transient failure (exit 80/85/86: quota, liveness-exhaustion) — ends the
// SESSION, but is not a verdict: the agent may have written its contracted
// deliverable before the teardown (cycle-254/255 timeout false-FAIL;
// cycle-835 quota false-FAIL). Reconcile against the deliverable: if it is
// on disk and well-formed, trust its verdict (via Classify) instead of
// synthesizing FAIL. Reconciliation can only UPGRADE toward the agent's
// real verdict, never downgrade a real one.
```

### `go/internal/phases/runner/verdict/reconcile.go:66` — above `func (e *Engine) reconcileTeardown(ctx context.Context, d Dispatch) (reconciliation, *core.PhaseResponse, error) {`

```text
// reconcileTeardown runs the CANCELLATION-IMMUNE ladder (WithoutCancel,
// deliberate): on THIS path a cancelled ctx is frequently the CAUSE of the
// teardown, not a reason to stop waiting — the tmux driver, on ctx.Err(),
// takes one final completion poll and otherwise exits ExitArtifactTimeout,
// laundering a finished session into a timeout; honoring cancellation here
// would re-open the cycles-824/825 false-FAIL class. Then the arms, in order.
```

### `go/internal/phases/runner/verdict/reconcile.go:74` — above `stale := staleOf(d)`

```text
// The verifier may WRITE (the contract gate salvages a sole recoverable
// bad_verdict and persists the repaired artifact — F22), so the
// pre-dispatch identity is read BEFORE the probe: a leftover the probe
// repairs must still be refused as a prior attempt's report (cycle-1550).
```

### `go/internal/phases/runner/verdict/reconcile.go:95` — above `func staleGate(d Dispatch, stale bool, s settled) settled {`

```text
// staleGate is the cycle-1550 refusal: a deliverable byte-identical to the
// pre-dispatch snapshot is the PRIOR attempt's report — well-formed,
// cycle-scoped challenge token and all — and reconciling it would re-grade a
// stale verdict as this session's work. Refuse BOTH reconcile doors (the
// well-formed fall-through and the ACS floor) and let the optional/fatal arms
// handle it as untrustworthy, with the cause on record. The refusal fires
// only on an OK probe; a stale but malformed leftover stays malformed.
```

### `go/internal/phases/runner/verdict/reconcile.go:118` — above `func (e *Engine) degradeOptional(d Dispatch, s settled) core.PhaseResponse {`

```text
// degradeOptional is the optional-phase soft-fail (Workstream D / cycle-120):
// no trustworthy deliverable, but an optional phase's successor is
// verdict-unconditional, so degrade to WARN and let the cycle advance.
```

### `go/internal/phases/runner/verdict/reconcile.go:157` — above `func (e *Engine) forensicFail(d Dispatch, roots phasecontract.Roots, s settled) (core.PhaseResponse, error) {`

```text
// forensicFail is the mandatory-phase hard-fail: no trustworthy deliverable
// (absent/malformed/unverifiable), enriched with the well-formedness
// violation when there is one — including the stale-leftover refusal, so the
// forensic dig sees WHY reconcile declined, not only that the bridge timed
// out. The event carries the roots the probe judged, the codes, the on-disk
// state and the cause (the retro's cycle-3 ask: a teardown false-FAIL used to
// record no reasoning).
```

### `go/internal/phases/runner/verdict/reconcile_test.go:3` — above `import (`

```text
// reconcile_test.go — the teardown reconcile arms (ADR-0103 unit 11 §6 tests
// 21-25): the arm ORDER, the forensic FAIL event, the optional degrade, the ACS
// deterministic floor and the reconcile trail.
```

### `go/internal/phases/runner/verdict/reconcile_test.go:401` — above `func TestReconcileTeardown_LeftoverRepairedByTheProbeIsStillRefused(t *testing.T) {`

```text
// F22: the verifier may now WRITE (the contract gate salvages a sole
// recoverable bad_verdict and persists the repaired artifact). The
// pre-dispatch identity is read before the probe, so a leftover the probe
// repairs is still refused as a prior attempt's report — cycle-1550's guard
// must not be defeated by our own rewrite.
```

### `go/internal/phases/runner/verdict/replay_test.go:3` — above `import (`

```text
// replay_test.go — ADR-0103 unit 11 §6 test 33: the host's response and probe
// goldens (captured on 8e8f080f through runner.New + Run, before any code
// moved) replayed through Judge with equivalent fakes over real temp dirs —
// byte-identical JSON, identical probe/sleep counts. Any drift in a moved
// literal shows here.
```

### `go/internal/phases/runner/verdict/settle.go:27` — above `func (e *Engine) settle(ctx context.Context, id Identity, phase string, roots phasecontract.Roots) settled {`

```text
// settle waits (bounded) for a contracted deliverable to become well-formed,
// re-probing up to SettleRetries times with SettleInterval between attempts.
// It serves BOTH the reconcile-on-teardown path (cycles 824/825: a next-phase
// context-cancel laundered into ErrArtifactTimeout fires while the deliverable
// is still being written) AND the clean-exit artifact-read path
// (cycle-603/899/921: a cleanly-exited agent idles while its `Write` flush
// lands).
//
// It retries ONLY while the report is contracted-but-not-yet-well-formed
// (err == nil && !res.OK) — the state a late flush passes through (a missing
// file is CodeMissingArtifact, err == nil). An ERROR means "no contract for
// this phase" or an IO fault; neither resolves by waiting, so those return at
// once — uncontracted phases pay ZERO retries. Waiting can only UPGRADE toward
// the agent's real on-disk verdict; a never-settling deliverable still returns
// not-OK.
//
// CANCELLATION: the wait observes ctx, so a cancelled phase stops waiting
// instead of sleeping out the ladder. The first probe always runs (a
// deliverable already on disk is still caught); cancellation then stops both
// the sleeping and the re-probing, and the last result stands. The check
// straddles the sleep (before and after) rather than racing a timer against
// it: the sleep is an injected, uninterruptible seam, so post-cancel cost is
// bounded at ONE interval with no further probe. The teardown call site passes
// context.WithoutCancel (see reconcileTeardown): there a cancelled ctx is
// frequently the CAUSE of the teardown; on the clean-exit path the agent
// already exited 0, so cancellation genuinely means nothing more is coming.
```

### `go/internal/phases/runner/verdict/settle.go:70` — above `func rootsFor(d Dispatch) phasecontract.Roots {`

```text
// rootsFor is the ONE translation from a dispatch to the contract roots the
// engine verifies against — shared by the clean-exit check and the teardown
// reconcile so they can never resolve different paths, or a different cycle,
// for the same phase. EvolveDir completes the roots (orchestrator-target
// deliverables, the declared-effects lifecycle state) AND locates the merged
// catalog for the catalog-aware default; Cycle names the processing/cycle-N/
// a declared effect is judged under (ADR-0100).
```

### `go/internal/phases/runner/verdict/settle_test.go:3` — above `import (`

```text
// settle_test.go — the bounded settle ladder (ADR-0103 unit 11 §6 tests 16, 17,
// 31): the bound, the stop conditions, the re-probe count, both ctx checks,
// and the ONE translation from a dispatch to the contract roots.
```

### `go/internal/phases/runner/verdict/verdict.go:1` — above `package verdict`

```text
// Package verdict is unit 11 of the component breakdown (ADR-0103): the phase
// runner's verdict engine — the fifth and sixth step of BaseRunner.Run's
// template. One Engine turns (what the bridge did, what preparation
// snapshotted, the deliverable probe) into the core.PhaseResponse core's
// dispatch loops route on: the bounded settle ladder, the teardown reconcile
// arms (stale-leftover refusal → well-formed → optional degrade → ACS
// deterministic floor → forensic FAIL), the substantive-error FAIL, the
// verdict-source rule (the contracted file is the sole verdict source, the
// pane only for an uncontracted phase), the clean-stdout companion, the ship
// guard and the violation trail. Prompt preparation, routing, the dispatch
// chain and the worktree fence stay in the host (unit 11b). The Engine holds
// the probe, a sleep, the stdout filter, the optional flag and the Signal
// Center accessor; it persists nothing, writes no stderr, reads no env, and
// reports its five decisions as runner.warning under module runner. Design:
// docs/architecture/decomposition/11-phaserunner.md.
```

### `go/internal/phases/runner/verdict/verdict.go:46` — above `const (`

```text
// The settle ladder's bounds — a pure LIVENESS ceiling (≈ 3 s) for a
// contracted deliverable that has not finished flushing to disk, never the
// verdict-correctness mechanism (that is the file-authoritative rule): a
// clean-exit agent can return control before its `Write <phase>-report.md`
// lands (cycle-921, the ADR-0072 verdict incoherence). The host projects them
// under their old names for the settle tests it keeps.
```

### `go/internal/phases/runner/verdict/verdict_test.go:3` — above `import (`

```text
// verdict_test.go — construction, the closed set, the shapes and the Judge
// contract (ADR-0103 unit 11 §6 tests 11, 14, 15, 18-20, 32-34). Every export
// is named and exercised in this package's own tests (apicover, cover-strict
// count package-local tests only). Every test injects WithSleep: no real sleep.
```
