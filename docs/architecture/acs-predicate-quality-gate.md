# ACS Predicate Quality Gate — Four-Layer Defense

> Design document for the predicate-quality four-layer defense, activated in cycle-86.
> See [CHANGELOG.md](../../CHANGELOG.md) for version history.

## Problem: Tautological Predicates

ACS predicates (`acs/cycle-N/*.sh`) can be trivially-tautological. A grep-only predicate like:

```bash
grep -q "Predicate quality review" agents/evolve-auditor.md ; exit $?
```

always passes if the string is present, regardless of whether the implementation is correct. The predicate tests string presence, not behavioral correctness. Mutation testing cannot kill it — any mutation that preserves the string passes regardless of actual system behavior.

## Four-Layer Defense (activated cycle-86)

### Layer 1 — Author Separation (cycle-85)

| Role | Responsibility |
|---|---|
| TDD-Engineer | Writes behavioral predicates BEFORE Builder implements |
| Builder | Implements to pass predicates — cannot author predicates |

**Mechanism:** `EVOLVE_TEST_PHASE_ENABLED=1` (default-on) forces TDD-Engineer phase between Triage and Build. Builder profile denies predicate authorship.

**Reference:** `.evolve/profiles/tdd-engineer.json`, `.evolve/profiles/builder.json`

### Layer 2 — Static Linter (`predicate-quality-c2-linter`)

**Script:** `legacy/scripts/verification/lint-acs-predicates.sh`

**Classification rules:**

| Classification | Criteria |
|---|---|
| `BEHAVIORAL` | Uses subprocess invocations (`$(...)`, backtick) or arithmetic/jq/awk/wc |
| `GREP_ONLY` | `grep -q` calls with no subprocess invocations (count > 0, subprocess_count == 0) |

**Gate hook:** `gate_build_to_audit` in `legacy/scripts/lifecycle/phase-gate.sh` runs the linter on every predicate in `acs/cycle-N/`. Any GREP_ONLY predicate **blocks the gate** (exit 1). Opt-out not available — this is a hard gate.

**Usage:**
```bash
# Classify predicates with explanation
bash legacy/scripts/verification/lint-acs-predicates.sh --predicates-dir acs/cycle-N --explain

# Exit codes: 0 = all behavioral, 1 = grep-only detected
```

**Test suite:** `tests/verification/test-lint-acs-predicates.sh` — 7 FAIL fixtures (grep-only patterns), 2 PASS fixtures (behavioral). Run: `bash tests/verification/test-lint-acs-predicates.sh`.

### Layer 3 — Auditor Review (`predicate-quality-c3-auditor-review`)

**Agent:** `agents/evolve-auditor.md` — **Predicate quality review** section

The Auditor classifies every predicate as `behavioral` / `grep-only` / `mixed` and:
- Raises **CRITICAL** defect for each un-waived grep-only predicate
- Raises **HIGH** warning for each mixed predicate (needs human verification)

**acs-verdict.json schema extension:**

```json
{
  "verdict": "PASS",
  "red_count": 0,
  "predicate_quality": {
    "per_predicate": [
      {
        "path": "acs/cycle-86/pred-lint-acs-exists.sh",
        "classification": "behavioral",
        "has_subprocess_invocation": true,
        "waived": false
      }
    ],
    "summary": {
      "behavioral_count": 5,
      "grep_only_count": 0,
      "mixed_count": 0,
      "blocking_count": 0
    }
  }
}
```

`blocking_count > 0` forces `verdict = "FAIL"` regardless of predicate exit codes.

**Inspect post-cycle:**
```bash
jq '.predicate_quality.summary' .evolve/runs/cycle-N/acs-verdict.json
```

### Layer 4 — Activation and Promotion (`predicate-quality-c4-activate-and-promote`)

| Change | Before (cycle-85) | After (cycle-86) |
|---|---|---|
| `EVOLVE_TEST_PHASE_ENABLED` | `0` (opt-in) | `1` (default-on) |
| Mutation gate `gate_discover_to_build` | WARN-only at kill_rate < 0.7 | **FAIL-gate** at kill_rate < 0.7 |
| Orchestrator phase flow | Scout → Triage → Builder | Scout → Triage → **TDD-Engineer** → Builder |
| Lint gate `gate_build_to_audit` | Not present | **FAIL-gate** on grep-only predicates |

**Mutation gate opt-out:** Set `EVOLVE_MUTATION_GATE_STRICT=0` to revert to WARN-only. Not recommended for production.

### Layer 5 — Flaky-shape lint (`flaky-predicate-shape`, 2026-07-30)

Layers 1–4 ask *"does this predicate assert real behavior?"*. Layer 5 asks the orthogonal question *"will this predicate assert it **reliably**?"* — a behavioral predicate can still be a false-red generator. Cycles 1173/1175/1178 each FAILed on a predicate that shelled a whole-package `go test`; a reproducer's un-reaped `yes` load generators burned 8 cores for 9 hours across batches 18–21.

**Implementation:** `internal/evalqualitycheck.LintFlakyPredicates` — a deterministic `go/ast` lint over the cycle's predicate sources. Five patterns, each annotated with its [Luo et al. FSE'14](https://doi.org/10.1145/2635868.2635920) flakiness-taxonomy class:

| # | Pattern | Class | Sanctioned shape |
|---|---|---|---|
| 1 | suite-scope go-test shell (`./...`, any `/...`, multi-package, or the known 40s+ `internal/core` / `cmd/evolve`) | concurrency (20%) | one named package; or narrow with `-run` |
| 2 | `time.Now()`-derived deadline, **as a direct chain** (`time.Now().Add`/`.Before`/`.After`/`.Sub`) plus `time.Since`/`Until`. A pattern broken across statements (`now := time.Now(); now.Add(…)`) is NOT caught — no local dataflow | async-wait (45%) | poll on state, or bound from the test context |
| 3 | hardcoded PID < 100000 in a liveness check (`syscall.Kill`, `os.FindProcess`, `kill -0`, `/proc/<pid>`) | environment | discover at runtime (`os.Getpid`, pidfile, `pgrep`) |
| 4 | subprocess `git` without `-C` and without `cmd.Dir` | environment | `git -C <dir>`, `cmd.Dir`, or `cd` first in `sh -c` |
| 5 | un-reaped load generation (`yes`/`stress`/shell busy loop via `exec.Command`) | resource-leak | `exec.CommandContext` + `WaitDelay` |

**Gate hook — where it actually runs.** `internal/evalgate.flakyShapeGate` (Gate D), composed into `evalgate.NewReviewer` and dispatched by the orchestrator's per-phase `core.DeliverableReviewer` seam (`core.WithReviewer`) **at the end of the `tdd` phase** — the first moment `go/acs/cycle<N>/predicates_test.go` exists, and one phase *before* the build tokens a flaky predicate would later waste. This is the Go-native successor to the bash `gate_build_to_audit` hook Layer 2 describes.

> The discover-time `evolve eval quality-check` call in `skills/loop/phase2-discover.md` **cannot** carry this check: DISCOVER runs before any predicate source exists. `evolve eval quality-check -predicates <path>` is the operator/agent hand-check of the same rule, not the pipeline wire.

**Stage: ADVISORY, structurally.** Gate D returns `block=false` as a constant — a flaky *shape* is a strong smell, not proof the predicate is wrong. It never rejects a deliverable at any stage. In the CLI a finding raises `PASS→WARN` (exit 1) through a **monotonic severity join**, so it can never lower a Level-0 tautology HALT.

Gate D emits **exactly one line per `tdd` phase, on every path** — findings, `CLEAN — linted N file(s), 0 findings`, `no Go ACS predicate package`, or a loud stand-down. Reserving silence for "clean" would make a healthy cycle indistinguishable from a gate that silently no-ops, which is the dead-code failure mode this layer was created to close.

> **Known limit:** that line goes to the phase log (stderr) only. Nothing persists it to an artifact, cycle state, or the Auditor handoff, so findings are operator-visible but **not** agent-consumable — and the per-class audit the promotion path needs cannot be fed from live runs until they are. Wiring an advisory-finding channel is a reviewer-seam change, tracked separately.

**Promotion gate.** Measured over the live corpus (282 `go/acs/cycle*` dirs, 2026-07-30, shipped binary):

| | dirs flagged | findings | breakdown |
|---|---|---|---|
| blanket literal scan | 117 | 341 | recursive 144 · known-slow 197 |
| + argv-position awareness | 102 | 297 | recursive 118 · known-slow 166 · multi-package 11 · `git` no `-C` 2 |
| + one-level helper hop | **64** | **179** | recursive 120 · known-slow 46 · multi-package 11 · `git` no `-C` 2 |

Three cuts, each removing a class where the printed claim was **false about the code**:

1. A package pattern reaching a **non-`go test`** exec argv (`go vet` / `go build` / `rg`) is a compile or a search, not a 40s+ suite.
2. `-run`-narrowed known-slow invocations (a recursive pattern stays flagged — `-run` selects which *tests* run, not which *packages* are built and loaded).
3. Resolving one level into a same-package helper, binding the call's arguments to the helper's parameters.

(3) mattered most and (1) depended on it: the corpus's actual exec constructor is `acsassert.SubprocessOutput` (198 of 282 dirs, against 20 using `exec.Command`), and the canonical idiom puts the package pattern in a const the *test* function names while the `-run` lives in a shared helper taking the package as a parameter. Body-only indexing therefore saw no exec argv, fell into the "unresolvable, keep the note" branch, and advised "narrow the invocation with `-run`" at code that already did — 142 of the 297 (48%). The truly unresolvable cases (concatenated patterns, helpers two levels down) still keep their note.

The cuts also *added* true positives previously invisible: the `git`-without-`-C` and multi-package rows only appear once the house helper is recognized.

**Still required before promoting past advisory:**

- a per-class false-positive audit of the remaining 179 — a spot check reads as true-positive throughout, but "reads as" is not a measurement;
- patterns 2/3/5 (wall-clock, hardcoded PID, load-gen) have **zero** corpus hits, so 100% of the measured findings come from pattern 1 and the other rules' precision is untested against real predicates;
- `-run` is accepted by name, not by selectivity, so `-run .` silences a known-slow finding while running the whole suite. Relevant to the anti-gaming boundary if this ever becomes a blocking gate.

**Scope rule SSOT.** The package-pattern recognizer lives in `internal/gopkgpattern`, shared with `internal/acssuite`'s run-time scope-lint, so authoring-time and run-time scope rules cannot drift.

## Lifecycle

```
TDD-Engineer (writes behavioral predicates)
    ↓
Gate D (flaky-predicate-shape, end of tdd): flaky SHAPES surfaced — ADVISORY, never blocks
    ↓
Builder (implements; cannot author predicates)
    ↓
gate_build_to_audit: lint-acs-predicates.sh FAIL if grep-only
    ↓
Auditor: classifies, emits predicate_quality block, CRITICAL on grep-only
    ↓
acs-verdict.json: blocking_count > 0 → FAIL
    ↓
gate_discover_to_build (next cycle): mutation gate FAIL at kill_rate < 0.7
```

## References

- `legacy/scripts/verification/lint-acs-predicates.sh` — Layer 2 linter
- `tests/verification/test-lint-acs-predicates.sh` — Layer 2 test suite
- `legacy/scripts/verification/mutate-eval.sh` — mutation testing (grep_only_check pre-flight)
- `legacy/scripts/lifecycle/phase-gate.sh` — gate_build_to_audit, gate_discover_to_build
- `agents/evolve-auditor.md` — Predicate quality review section (Layer 3)
- `agents/evolve-orchestrator.md` — EGPS Tester Phase section (Layer 4)
- `.evolve/profiles/tdd-engineer.json` — Layer 1 author separation
- `.evolve/profiles/builder.json` — Builder predicate-authorship denial
- `go/internal/evalqualitycheck/flakylint.go` — Layer 5 lint (the five patterns + false-positive discipline)
- `go/internal/evalgate/flakyshape.go` — Layer 5 gate hook (Gate D, end of `tdd`)
- `go/internal/gopkgpattern/` — package-pattern scope rule shared with `internal/acssuite`'s run-time scope-lint
- `agents/evolve-tdd-engineer.md` — the authoring rules Layer 5 checks
