# 2026-08-25 — Submit-verify false-wedge reds the v22.20.0 release (RealTmux family)

## Symptom

v22.20.0's release commit went red on `go` and `release` workflows: the whole
`TestRealTmux_*` integration family failed `exit = 81, want ExitOK` —
deterministically, reproduced locally. The release published propagated but
ASSETLESS (the release workflow's test stage failed before goreleaser).

## Root cause (two halves)

1. The wave-landed submit-verify feature (the retro-stall delivery-failure
   fast-fail, cycles 1505/1510/1517) classifies "prompt still parked" from
   pane echo heuristics and short-circuits `submit_wedged → instant exit 81`
   at the prompt site. A REPL that consumes its input SILENTLY and never
   redraws its input line is indistinguishable from parked by that heuristic —
   which is exactly what the RealTmux fake REPLs do (write the artifact as a
   side effect, no output), and what any real side-effect-answering agent
   would look like.
2. The landing lanes' local bars never ran the RealTmux family (quiet-host
   skip while wave tmux sessions lingered — the `requiretmux-tier-quiet-host-
   backstop` class), and nobody watched the post-push CI mid-wave — main was
   red from the wave-4 ship onward, discovered only by the release post-watch.

## Fix

- **Ground-truth belt at the wedged short-circuit**: before the instant 81,
  one read-only probe — an artifact present that is NOT the pre-dispatch
  baseline (#498's key) proves the submission landed; fall through to the
  normal wait (stability window still gates completion). A genuinely parked
  pane with no deliverable — or with only the prior attempt's stale leftover —
  keeps the fast-fail. The nudge-site wedged branch already reaches ground
  truth via the timeout epilogue's final poll.
- **Harness fidelity**: the RealTmux fakes re-print their input-line marker
  after every consumed line (real-REPL discipline), with a no-resend pin so
  the clean submit-verify path stays exercised rather than belt-shadowed.

## Fix-forward

Per the publish skill: a red released commit is never rolled back — v22.20.0
stands assetless (the v22.13.0 precedent) and v22.20.1 folds it forward with
this fix.

## Regression pins

`driver_tmux_delivery_failure_test.go`: parked+delivered completes OK (loud
override), parked+stale-only still fast-fails (real CaptureBaseline).
`tmux_repl_integration_test.go`: happy path asserts ZERO resends.
Mutation-tested: 4 mutants killed (belt removed / baseline check dropped /
belt inverted / fake fidelity reverted).
