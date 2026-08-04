# Channel e2e deflake: sleep-sync and the frozen clock
**Period:** 2026-08-03 · **Status:** shipped
**Primary artifacts:** PR #403 (merge `85b3d368`) · `go/internal/bridge/channel_e2e_test.go` · CI run 30807692874 (macOS)

## Problem

The macOS CI job hung on 2026-08-03 (run 30807692874):
`TestChannelE2E_RealFixtures_ClaudeSpan` ran 9m55s into the 10-minute
**package-timeout panic** — killing the entire `internal/bridge` package's
test output, yielding no diagnostics, and turning an intermittent scheduling
race into a red CI wall for whatever PR happened to be in flight. The blast
radius of a hung e2e is the whole package suite plus everyone waiting on CI,
which is why this got a same-day fix (PR #403 body).

## Context & evidence

The e2e drives the entire bidirectional channel against real captured claude
frames: `Supervisor.Ask` → inbox → tmux driver delivery → pane frames →
producer normalization → feed → answer-span recovery (test doc comment). Two
**layered** test defects produced the hang; neither alone explains it:

- **Defect 1 — sleep-as-synchronization (the flake).** The driver seeks the
  inbox cursor to EOF at boot (`driver_tmux_repl.go:383-386`); an ask appended
  *before* that seek is skipped forever. The test's only ordering guarantee
  between "driver booted" and "ask appended" was `time.Sleep(20ms)`. On a
  loaded CI runner the driver goroutine scheduled late → the ask was appended
  first → skipped → the test's self-paced fake tmux never saw
  `inject_applied` → the answer never arrived (PR body; the in-test comment
  now narrates this exactly).
- **Defect 2 — the frozen clock disabled the failsafe (the amplifier).**
  `Supervisor.Ask`'s deadline is computed *and checked* via the injected `Now`
  (`supervisor.go:128,144,164`); the test froze it at `time.Unix(0,0)`, so
  the configured 10s `Timeout` could **structurally never fire**. A lost ask
  therefore meant an infinite poll — the 10-minute package panic — instead of
  a crisp 10s `ErrResponseTimeout` with diagnostics (PR body; in-test
  comment).

The layering matters: Defect 1 loses the race occasionally; Defect 2 converts
each loss from a self-diagnosing 10s failure into a suite-killing hang.

## Approaches considered

- **A longer sleep.** The incumbent idiom, and the tempting one-liner.
  Rejected as a class, not a tuning problem: sleep-as-synchronization has no
  correct constant — any bound loses on a sufficiently loaded runner, and
  every increase taxes every clean run. The committed comment names the class
  so the next author doesn't re-tune it.
- **A real wall clock for the supervisor.** Would make the timeout fireable,
  but the producer's frozen clock exists so feed timestamps are deterministic;
  putting wall time back into the pipeline trades one flake class for another.
  Rejected in favor of splitting the clocks (PR body: "the producer keeps the
  frozen clock so feed timestamps are unchanged").
- **Chosen: positive synchronization + a deterministic advancing clock.**
  Wait for an observable effect that *happens-after* the event being
  synchronized on, and give the supervisor a clock that advances one fake
  millisecond per reading.

## Decision & reasoning

**The positive-sync principle:** never wait a guessed duration for an event —
wait, bounded and loudly, for an artifact the event provably precedes. The
driver creates `build-pane.live` with `O_CREATE`
(`driver_tmux_repl.go:407`) **strictly after the cursor seek in the same
goroutine**, so the file's existence is a happens-after proof that the seek
completed and an appended ask will be delivered. The test polls for that file
(bounded 30s, `t.Fatal` with a named reason on expiry) instead of sleeping
20ms (committed test code + comment).

**The fireable-failsafe principle:** a test that injects a clock owns the
consequence that timeout arithmetic runs on it. The supervisor gets
`supNow` — `Unix(0, supTick*ms)`, +1 fake millisecond per reading — keeping
the test free of wall-clock timestamps while making the 10s deadline real
(committed test code + comment). Trade-off named in the review of siblings:
frozen clocks are fine where no progress-gated wait exists (see below).

## Implementation

Test-only diff, merged 2026-08-03 (`85b3d368`). The distinctive part is the
**three-way proof matrix**: the CI scheduling accident was reproduced
deterministically by inducing a 50ms delay in the driver goroutine, then each
defect was toggled independently (PR body):

| Config | Result |
|---|---|
| old sync + frozen clock | **HANG** → package-timeout panic (exact CI signature) |
| old sync + advancing clock | crisp FAIL in 5s: `ErrResponseTimeout` |
| positive sync + advancing clock (shipped) | **PASS** |

Row 1 authenticates the reproduction against the live incident; row 2
isolates the amplifier (proves the clock alone converts hang→failure, and
that the sync defect alone still fails); row 3 proves the pair. The induced
delay was removed for the committed version.

Verification: e2e ×10 under `-race` in 2.0s; full `./internal/bridge/...`
suite `-race` green, 8/8 packages (PR body).

**Sibling audit (generalization, with a safety argument rather than a blanket
edit):** `channel/e2e_test.go` and `supervisor_test.go` share the frozen-clock
idiom but their feed appends are **unconditional** (timer-driven, not gated on
driver progress), so an answer always arrives and no hang hazard exists;
`TestSupervisor_Ask_Timeout` omits `Now` entirely → real clock → its timeout
genuinely fires (PR body). The audit is recorded so nobody "fixes" safe
siblings into churn.

## Results (measured)

- The proof matrix above is the measured result: hang → 5s honest failure →
  pass, under an identical induced fault.
- Committed suite timing: e2e ×10 `-race` in 2.0s (vs a 9m55s hang for a
  single run when the race was lost).
- No recurrence data beyond the merge is claimed; the deterministic
  reproduction (induced delay) rather than green-CI-so-far is the evidence
  the fix targets the mechanism.

## Retrospective — what we learned

- **Flakes layer.** This incident was a race *and* an amplifier. Fixing only
  the sync leaves a 10-minute panic waiting for the next lost message; fixing
  only the clock leaves a 10s flake. Diagnose to the point where each layer
  has its own named defect, and fix both.
- **A disabled failsafe is worse than no failsafe** — it advertises a bound
  (`Timeout: 10s`) the code cannot honor. Injected clocks must be audited
  against every deadline computed from them.
- **Positive sync generalizes:** the artifact to wait on must be created
  *after* the synchronized event **in the same goroutine** — that ordering,
  not the file itself, is the guarantee. The entry's one-line rule: wait for
  an observable effect that happens-after the event.
- **Prove test fixes the way we prove gate fixes:** induce the failure
  deterministically, toggle one variable at a time, and record the matrix.
  This is the test-suite analogue of the mutation-kill proof in
  [the graduation entry](2026-08-graduation-test-only.md).
- **Generalize by audit, not by sweep.** The sibling frozen clocks are safe
  for a stated structural reason (unconditional appends); recording the
  reason is what prevents both a future regression *and* a future
  cargo-cult "cleanup".

## Links

- PR #403 (merge `85b3d368`) — the body carries the full proof matrix
- `go/internal/bridge/channel_e2e_test.go` — both principles are written into
  the committed comments at the exact seams
- Sibling entries: [Worktree provisioning retry](2026-08-worktree-provisioning-retry.md)
  — the same week's companion lesson on time in tests (real backoff sleeps
  ballooning a package 52s→600s and masquerading as host contention);
  [The graduation test-only class](2026-08-graduation-test-only.md) — the
  matrix-proof discipline's counterpart for gates.
