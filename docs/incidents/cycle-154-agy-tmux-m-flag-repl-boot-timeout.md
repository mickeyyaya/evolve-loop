# Incident: cycle-154 — agy-tmux `-m` flag → REPL boot timeout (exit=80) → batch abort

**Date:** 2026-05-31
**Severity:** HIGH (whole 5-cycle batch aborted on the first cycle's build phase)
**Run:** multi-CLI + dynamic-advisor + swarm validation (cycles 154–158), builder=`agy-tmux`, auditor=`codex-tmux`, rest=`claude-tmux`
**Status:** FIXED (proximate) + RESILIENCE ADDED (architectural) + DESIGN REFACTOR QUEUED

---

## Symptom

```
[runner] phase=build agent=builder cli=agy-tmux (source=profile.builder.cli)
evolve loop: cycle 154: phase build: build: bridge: bridge: launch exit=80
{ "stop_reason": "error", "cycles": [{ "Cycle": 154, "FinalVerdict": "SKIPPED",
  "PhasesRun": ["scout","tdd","build-planner"] }], "total_cost_usd": 0 }
```

The build phase started 08:36:20, observer reported `context canceled` at 08:37:23 (~63s).
No `build-stdout.log` / `build-stderr.log` was written — the agy process never reached
the REPL.

## Root cause (two compounding bugs)

### P2 — Proximate: unsupported `-m` flag breaks every agy launch

`go/internal/bridge/manifests/agy-tmux.json` declared:

```json
"model_tier": { "channel": "flag", "flag": "-m", "from": "model_tier_map" }
```

so the realizer produced launch flags `[-m gemini-3.5-flash --dangerously-skip-permissions]`.
But **agy 1.0.3 has no `-m`/`--model` flag** (verified live):

```
$ agy -m gemini-3.5-flash --print "say OK"
flags provided but not defined: -m
Usage of agy: ...                       # usage dump, no REPL
$ agy --print "say OK"
OK
```

agy printed a usage error and exited the interactive launch immediately, so the
`agy-tmux` driver's boot-wait loop never saw the `"? for shortcuts"` prompt marker
and timed out after 60s → `ExitREPLBootTimeout` (80) (`go/internal/bridge/exitcodes.go:30`,
`driver_tmux_repl.go:146-169`).

A 2026-05 manifest edit (note: *"agy DOES accept a -m/--model flag (corrected 2026-05)"*)
introduced this. It was wrong — agy never supported `-m`. The driver source even
documents the correct contract (`driver_agytmux.go:24`: *"model tier is a no-op (agy
has no -m flag)"*), so the manifest and driver **contradicted each other**, and a test
(`realizer_realmanifest_test.go:40`) enshrined the buggy expectation.

### P1 — Architectural: the CLI fallback chain is dead by default

`exit=80` is a registered fallback trigger (`cli_chain.go:127`,
`defaultFallbackOnExit = {80,81,124,127}`), so the cycle *should* have degraded to a
secondary CLI instead of aborting. It did not, because:

- `resolveCLIChain` (`cli_chain.go:153`) builds the candidate list from
  `primary + prof.CLIFallback` (the `cli_fallback` field) and **never reads
  `prof.AllowedCLIs`**.
- **No shipped profile sets `cli_fallback`** (`grep -l cli_fallback .evolve/profiles/*.json`
  → none). Profiles only set `allowed_clis` (e.g. builder `["claude","agy"]` with the
  note *"claude permitted as fallback"*).

Net: every phase resolves to a single-candidate chain, so any CLI-level launch failure
(80/81/124/127) kills the whole batch — the exact "one phase failure aborts everything"
mode this run's goal targets. The well-built fallback machinery in `cli_chain.go` is
**unreachable in practice**.

## Fix applied

| Layer | Change | File |
|---|---|---|
| P2 manifest | `model_tier.channel: "flag"` → `"noop"` (agy has no `-m`); note rewritten with live verification | `go/internal/bridge/manifests/agy-tmux.json` |
| P2 test | flip the contract to `[--dangerously-skip-permissions]` only — encodes agy's *real* CLI surface, not the buggy manifest | `go/internal/bridge/realizer_realmanifest_test.go` |
| P1 resilience | add `"cli_fallback": ["claude-tmux"]` — realizes the documented "claude permitted as fallback" intent; protects the codex auditor against its quota wall | `.evolve/profiles/builder.json`, `.evolve/profiles/auditor.json` |
| Binary | rebuilt `go/evolve` + `go/bin/evolve` (manifest is `go:embed` — disk edit is inert until recompiled) | — |

Verification: `go test ./internal/bridge/ -run TestRealizeFor -count=1` → PASS. Full
bridge suite passes except a pre-existing real-tmux concurrency flake
(`TestRealTmux_Interactive_StuckAutoRespond_TripsLoopGuard`, passes in isolation —
unrelated code path).

## Recommended improvements (queued)

1. **Derive the fallback chain from `allowed_clis` when `cli_fallback` is unset**
   (DRY / least-surprise) so operators get resilience from the field they already
   set, and the trap closes for every future profile — not just builder/auditor.
   Needs: base-name↔driver-name normalization (`claude` → `claude-tmux`/`claude-p`)
   and a cross-family guard for the auditor (must not fall back into the builder's
   family). **Queued as a carryover todo for the loop** (it is the loop's own
   self-heal goal; deserves a proper TDD cycle, not a hasty hand-roll).
2. **Emit a startup WARN** when a profile lists ≥2 `allowed_clis` but resolves to a
   single-candidate chain — would have surfaced this dead-config silently-degraded
   state immediately.
3. **TDD handoff-artifact gap**: cycle-154 logged `WARN spine not satisfied for
   next=build (a mandatory predecessor's handoff artifact is missing); proceeding
   fail-open` — TDD wrote `test-report.md` but not `tdd-report.md`. Same missing-
   artifact class as the run's "backfill" finding; investigate whether fail-open is
   correct here or should backfill the handoff.
4. **Capability probe should be a live boot check, not just binary+auth.**
   `evolve setup detect` rated agy "ready" (binary present, OAuth ok) yet its REPL
   could not boot. A one-shot `agy --print` smoke test in the probe would have
   caught the `-m` breakage before a 5-cycle run consumed it.

## Cross-references

- `docs/incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md` (sibling: codex exit=80/81 fallback gap)
- `docs/architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md`
- `docs/architecture/cli-capability-matrix.md` (agy `-m` claim to correct here too)
