# The bash → Go Port Arc

> **Institutional memory.** This is the durable historical record of how evolve-loop
> moved from a pile of bash 3.2 scripts to a single Go binary. It **supersedes
> `docs/migration-from-bash.md`** (an operator-facing how-to written mid-flight, when
> bash was still the fallback). Read this for the *why* and the *lessons*; read the
> migration doc only if you need the v11.x rollback-hatch mechanics verbatim.
>
> Cross-links: [decision-digest.md](decision-digest.md) ·
> [rejected-approaches.md](rejected-approaches.md) ·
> [compound-improvement-arc.md](compound-improvement-arc.md) ·
> [../incidents/pattern-library.md](../incidents/pattern-library.md)

---

## TL;DR

The system was born as ~220 bash scripts (`legacy/scripts/`, ~3,000 of those lines in
the agent-bridge alone). Over six weeks it was ported to a single Go binary in a
**deliberately staged, reversible** sequence:

| Milestone | Version | What happened |
|---|---|---|
| **Go-primary cutover** | v11.0.0 | Go binary became the tier-1 entrypoint; bash kept fully working behind `EVOLVE_USE_LEGACY_BASH=1`. |
| **Physical relocation** | v11.1.0–v11.2.0 | `scripts/` → `legacy/scripts/`; backcompat symlink added then removed. |
| **Subsystem-by-subsystem native ports** | v11.3.0–v11.9.0 | ship, guards, EGPS predicates, cycle/loop dispatch, bridge — each ported behind its own rollback flag with a parity contract. |
| **Flag day** | v12.0.0 | ~220 bash scripts deleted. Go is the only runtime. |
| **Go-only steady state** | v13.0.0 | Native `evolve release`; the bash release pipeline gone; rollback hatches retired. |

The governing principle the whole port obeyed: **never flip a default without a
parity contract proving byte-identical output, and never delete the old path until
the new path has survived at least one real cycle.** Where that discipline lapsed, a
whole subsystem vanished silently (see [Worktree provisioning](#the-cautionary-tale-a-subsystem-vanished-silently)).

---

## Why port at all

Bash 3.2 (macOS default) was the substrate because the system started life as a
Claude Code *plugin* — markdown agents + shell hooks, no compile step, instantly
editable. That bought fast iteration for the first ~100 cycles. The costs that
forced the port:

1. **No type safety across the dispatch seam.** `subagent-run.sh` mangled CLI names
   into function names (`drv_launch_${cli//-/_}`) and passed flags an adapter might
   silently ignore — the root of the ADR-0002 capability-matrix work.
2. **bash 3.2 portability tax.** No `declare -A`, no `mapfile`, no `${var^^}`, BSD-vs-GNU
   `sed`/`date` forks everywhere. Every script carried this burden; see
   [CLAUDE.md](../../CLAUDE.md) shell-conventions table for the banned-pattern list
   that survives as the bash-era scar tissue.
3. **Untestable orchestration.** The orchestrator was an LLM with raw `bash`, which
   the cycle 102–111 reward-hacking incident proved could simply *skip the whole loop*
   ([rejected-approaches.md § LLM-as-orchestrator](rejected-approaches.md#llm-as-deterministic-orchestrator)).
   A deterministic Go host that controls tool execution and reads exit codes
   independently of the LLM's opinion was the only durable fix.
4. **Structural perf.** Ledger append went from 10–20 ms (bash + jq) to 70 µs in Go
   (~150–280×); ledger verify from ~1–2 s to 656 µs. LLM-bound phases are unaffected
   (API latency dominates), but the *plumbing* between phases stopped being a tax.

The port was therefore never about speed of the agents — it was about making the
**harness** deterministic, typed, and testable, so the loop could be trusted to run
unattended for hours.

---

## What moved, subsystem by subsystem

Each subsystem ported behind its own flag with its own parity gate. The pattern was
uniform: **ship the Go path defaulted ON, keep the bash path reachable via one env
var, prove parity, then delete the bash in a later release.**

| Subsystem | Native-Go release | Rollback hatch | Parity contract | Retired |
|---|---|---|---|---|
| Doctor / guard / ledger / acs subcommands | v11.0.0 | (additive — bash still present) | same predicates, same exit codes | v12.0.0 |
| Orchestrator + 8 phases | v11.0.0 | `EVOLVE_USE_LEGACY_BASH=1` | byte-identical `scout/build/audit/ship-report.md`, `acs-verdict.json`, `cycle-state.json`; SHA chain reproduces | v12.0.0 |
| **Ship** | v11.3.0 | `EVOLVE_NATIVE_SHIP=0` | **23-test matrix** `native_test.go` ↔ `ship-integration-test.sh`, byte-for-byte on commit footers + exit codes + ledger semantics | v13.0.0 |
| Guards (PreToolUse hooks) | v11.4.0 | bash shim at `evolve-guard-dispatch.sh` | JSON-on-stdin contract, exit 0/2, `guards.log` format unchanged | v12.0.0 |
| EGPS predicates | v11.5.0 | bash predicates kept under `acs/regression-suite/` | Go `predicates_test.go` mirror; 272 predicates ported across cycles 44–141 | v13.0.0 (but see ADR-0025 caveat below) |
| Cycle / loop dispatch | v11.5.0 (M1–M6) | `EVOLVE_USE_LEGACY_BASH=1` exec's archived dispatcher | same dispatch semantics, failure-adapter, resume, cost accumulation | v12.0.0 |
| **Agent-bridge** (~3,000 lines bash) | v11.x | `EVOLVE_BRIDGE_GO=0` | shadow-parity on all 6 drivers; 100% covered in unit tests | v12.0.0 (ADR-0021) |
| Release pipeline | v13.0.0 | — | native `evolve release` replaces `release-pipeline.sh` | v13.0.0 |

### The bridge port (ADR-0021) — the largest single piece

The agent-bridge was the multi-CLI dispatch layer: `bin/bridge` + 6 drivers + 10
`lib/` modules + 6 JSON manifests, ~3,000 lines of bash. The Go reimplementation kept
the *exact same CLI surface and behavior* but rebuilt it on clean patterns:

- **Strategy + self-registering Registry** replaced bash's name-mangled file dispatch.
- **Template Method** (`LaunchArgs`, `runTmuxREPL`) gave all three tmux drivers one
  shared REPL engine.
- **Injectable seams** (`Runner`, `Tmux`, `Sleep`, `LookupEnv`, `LookPath`, `Now`,
  `NewChallengeToken`, `manifestSource` over `go:embed`) made the whole thing
  unit-testable with **no real subprocess/tmux/clock**.

This is the prototype for "a port should make the seam testable, not just translate
the lines." The bridge ports that came after it (ADR-0022 launch-intent realizer,
ADR-0023 live injection, ADR-0029 fallback chains) all built on these seams — which
would have been impossible in the bash original.

---

## The parity contract pattern (why the flips were safe)

A "parity contract" is the load-bearing artifact that made each default-flip safe.
It is not a smoke test — it is a **byte-level equivalence proof** between the bash
and Go paths on a fixed fixture. The canonical example is the **ship 23-test matrix**:

```
go/internal/phases/ship/native_test.go   ↔   legacy/scripts/tests/ship-integration-test.sh
```

Both run against ephemeral git repos using real `git`/`gh`, and both must produce
identical:
- commit-message footers (the `## Actual diff` trailer, class labels),
- exit codes per ship class (`cycle`/`manual`/`release`/`trivial`),
- ledger semantics (challenge-token, `prev_hash` + `entry_seq` chain).

The repo-wide parity harness was `legacy/scripts/parity-audit.sh` with three tiers:
`--dry-run` (report-only), `--simulate` (no-LLM smoke), `--full` (one real paid cycle
each side, ~$10–40). Each sub-release gate required: all four preflight gate suites
green, full Go `-race` regression, one real paid cycle, and a parity-audit pass.

**Lesson:** the parity contract is what let the team flip a default with confidence
and *then* delete bash a release later. The one place this discipline was skipped —
worktree provisioning — is exactly where a subsystem disappeared without a single red
test.

---

## The rollback hatches — what they protected, why they were safe to retire

The port shipped two primary rollback hatches. Both were **permanent compatibility
bridges by design**, not temporary scaffolding, and both were retired only once the
native path had accumulated real-cycle evidence.

### `EVOLVE_USE_LEGACY_BASH=1` — the dispatch hatch

When set, `evolve loop` exec'd `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh`
(the v10.x bash dispatcher, `git mv`'d to `archive/` in v11.5.0 M6 with full history
preserved). The native Go dispatch was not consulted at all.

- **Protected against:** a Go-side orchestrator bug biting mid-cycle. An operator
  could revert one invocation (or a whole session) to exactly v10.x behavior.
- **Safe to retire (v12.0.0) because:** by then the Go dispatch had run the entire
  meta-loop bring-up (cycles 109–116, see
  [compound-improvement-arc.md](compound-improvement-arc.md)), which exercised every
  integration seam the bash dispatcher had — and several it hadn't.

### `EVOLVE_NATIVE_SHIP=0` — the ship hatch

When set, the ship phase shelled out to the 961-line `legacy/scripts/lifecycle/ship.sh`
instead of the native `go/internal/phases/ship/`.

- **Protected against:** a divergence in the most dangerous phase — the one that
  commits, pushes, and creates releases. Ship is where audit-binding (SHA match,
  `red_count==0`) and the tamper-evident ledger meet `git push origin main`.
- **Safe to retire (v13.0.0) because:** the 23-test parity matrix passed on both
  paths through every v11.x release, and the native path carried the production
  default (`=1`) from v11.3.0 onward with no ship-class regression.

### `EVOLVE_BRIDGE_GO` — the bridge hatch

Defaulted off (bash) at first, flipped after shadow-parity on all six drivers. Retired
when the bash bridge was deleted in v12.0.0.

**General lesson on hatches:** a rollback hatch earns retirement by *accumulating
evidence*, not by elapsed time. The flag stays until the native path has survived the
exact failure class the hatch was insurance against. Retiring a hatch is itself a
shippable decision with its own gate.

---

## The cautionary tale: a subsystem vanished silently

The single most important lesson of the entire port. During the v11 port, bash
`run-cycle.sh` provisioned a per-cycle `git worktree` at cycle start. **The Go
orchestrator never ported it.** `cs.ActiveWorktree` was always `""`, and the
role-gate's only source-write allowance (`phase=="build" && ActiveWorktree!=""`) was
therefore *unsatisfiable* — **no phase could write source code.**

The Phase-1 unit suite was green the entire time, because the unit tests **mocked the
seam the worktree lived in**. The gap only surfaced in cycle 111/112 during the live
meta-loop, as `exit=81` / role-gate DENY, and took until cycle 116 to fully clear
(see [../incidents/pattern-library.md](../incidents/pattern-library.md),
Layer 3).

**Why it was invisible:** the port had unit-coverage of the *new* Go code but **no
behavioral-parity test** for the dropped subsystem. A green suite is not parity.

**The corrective principle (now doctrine):**

> A port needs **behavioral-parity tests, not just unit coverage of the new code.**
> Test the *contract* (the requirement / the agent doc), not the *code* — or a bug
> baked into both code and test becomes invisible (AGENTS.md Rule 9). The
> role-gate's "build-only" spec line and the tdd runner's `team-context.md` artifact
> name were *pinned by their own unit tests*, which were written from the buggy code
> rather than the contract.

This is why the meta-loop bring-up surfaced **seven** consecutive integration-layer
failures (timeout semantics, prompt-substitution, worktree provisioning,
worktree-relative tooling, auto-respond scanning, doc↔runner naming, per-CLI
rendering) — all the same shape: *a cycle-runtime behavior the port dropped or
changed, that the fakes-only unit suite never exercised.*

---

## Bash residue that intentionally survived

Not everything was ported. The team explicitly chose to keep a small bash surface
where porting cost exceeded the gain:

- **EGPS predicate scripts** remained bash under `acs/`. They are deliberately
  language-agnostic executable acceptance criteria; the Go side runs them via
  `evolve acs suite` (ADR-0025) in their own process groups with per-predicate
  timeouts. *Caveat:* v12.0.0's flag-day deleted `run-acs-suite.sh` (the runner)
  before its Go port existed, leaving the auditor doc pointing at a dangling script —
  the root of the [cycle 138–140 EGPS-verdict incident](../incidents/pattern-library.md).
  The lesson: **deleting the runner is not the same as deleting the work it ran.**
- **Hooks** can still invoke bash shims as a fallback when the binary is stale or
  missing.

---

## Don't-rediscover list (learned during v11.0.0 → v12.0.0)

1. **`BASH_SOURCE[0]/../..` path arithmetic is fragile under directory restructuring.**
   98 scripts hardcoded depth. The fix needed both a depth update *and* a `pwd -P`
   consideration — symlink invocation breaks `cd … && pwd` because `pwd` (without
   `-P`) returns the *logical* path the caller used. Prefer walk-up-to-`.claude-plugin/`
   over hardcoded depth (`resolve-roots.sh`).
2. **Blanket `perl` rewrites cause doc rot.** Self-referential prose ("scripts/ is a
   symlink to legacy/scripts/") becomes gibberish after the rewrite. Use negative
   lookbehind (`(?<!legacy/)scripts/`) to avoid `legacy/legacy/` double-prefixes, and
   review prose by hand.
3. **A green unit suite ≠ parity.** See the worktree story above.
4. **A port can change a contract silently** — artifact names, exit-code trigger
   lists (cycle-122: `exit=81` not in the fallback chain's default `[80,127]`),
   per-CLI rendering modes. Cross-seam contracts need *explicit* tests.
5. **The flag-day is a decision, not an event.** v12.0.0 (`rm -rf legacy/`) was
   sequenced as six sub-releases (v11.3 → v11.9), each gated. A single change-window
   would have been irrecoverable.

---

## Where to look in the code

- Native ship: `go/internal/phases/ship/{native,verify,audit,gitops,postship,statefile,dryrun}.go`
- Bridge: `go/internal/bridge/` (`driver.go` registry, `driver_tmux_repl.go` REPL engine, `manifests/*.json`)
- Orchestrator + worktree: `go/internal/core/{orchestrator,worktree}.go`
- Guards: `go/internal/guards/`
- ACS suite runner: `go/internal/acssuite/`

See [decision-digest.md](decision-digest.md) for the ADR rationale behind each of
these, and [compound-improvement-arc.md](compound-improvement-arc.md) for how the
loop drove its own port.
