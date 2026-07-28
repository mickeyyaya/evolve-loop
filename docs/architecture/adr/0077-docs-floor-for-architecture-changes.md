# ADR-0077 — Documentation floor for architecture-labeled changes

- **Status:** Accepted (cycle-1141)
- **Supersedes / relates to:** ADR-0034 (phasecontract deliverable registry),
  ADR-0064 (no feature flags — config-injected policy), ADR-0072 (system-failure
  floor), the `SpineFloor` dial (`internal/config` `RolloutStages.SpineFloor`)
- **Enforcement surface:** `go/internal/docsfloor` (decision),
  `go/internal/policy` (`docs_floor` block), `go/internal/core`
  (`docsFloorWarn`, wired into `DefaultBuildFloorChecks`; `ChangedWorktreePaths`),
  `go/internal/cli/phasecmd` (`evolve phase verify build`)

> **Addendum (cycle-1144) — the blocking-grade half.** The WARN above answers
> the broad question with a broad label. Cycle-1144 adds the precise half at the
> ADR-0034 verify seam: `docsfloor.IsArchitectureClass` / `docsfloor.HasDocsDelta`
> (the classification SSOT, shared with the WARN) plus
> `deliverable.CodeMissingArchitectureDocs` and
> `deliverable.VerifyBuildWithChangedPaths`, which fold the floor into a
> `Verify("build", …)` `Result` — `!OK`, additive to the existing
> well-formedness violations, fail-open on any non-architecture diff. The strict
> classifier drops test-only diffs (the WARN's main false positive) and picks up
> new packages, phase specs and the guards/ship/fleet trust-kernel surfaces; the
> live build-handoff floor now labels with it, so precision improves at WARN
> today. Promoting the seam's `!OK` to a live handoff REJECT is deliberately a
> separate step — it needs a soak on real diffs first, exactly as this ADR's
> "WARN, never REJECT" boundary demands. Worked example:
> [ADR-0078](0078-fleet-landing-prefix-queue.md).

> **Addendum (cycle-1150) — the blocking-grade half gets a caller.** The
> cycle-1144 seam shipped **inert**: `deliverable.VerifyBuildWithChangedPaths`
> had zero production callers, so an architecture-class build with no docs delta
> still passed the agent's own `evolve phase verify build` self-check — the exact
> command every phase prompt's Deliverable Contract tells the agent to run before
> declaring done. Cycle-1150 wires it:
>
> - `core.ChangedWorktreePaths` — an **exported projection** of the existing
>   unexported `changedWorktreePaths` (tracked diff vs `HEAD` + untracked adds,
>   repo-relative). A projection, not a second derivation: the host-side reviewer
>   (`build_floor_reviewer.go`) and the CLI self-check now read the same diff, so
>   they cannot drift (the ADR-0034 no-drift invariant).
> - `deliverable.VerifyBuildWithChangedPathsStage` — the resolver- and
>   `EVOLVE_PHASE_IO`-stage-aware form. The CLI resolves through the merged phase
>   catalog at the configured stage; reaching the floor via the defaulted
>   `VerifyBuildWithChangedPaths` (built-in resolver, `StageOff`) would have
>   silently *weakened* the build contract the self-check already enforces. The
>   defaulted form is preserved as a thin pinning of the new one.
> - `phasecmd.verifyDeliverable` — applies the floor **iff** `phase == "build"`
>   **and** `--worktree` is set. No worktree means no diff to classify, so
>   behaviour is byte-identical to before; the floor stays build-scoped and never
>   leaks into another phase's deliverable; a non-architecture-class diff never
>   yields the violation.
>
> Boundary 1 is unchanged: the **host-side** `docsFloorWarn` stays WARN pending a
> soak. What changed is that the agent-callable self-check now returns exit 1
> with `missing_architecture_docs` — a correction the builder can act on in the
> same handoff, before any gate has to.

## Context

[operating-policy.md](../../operations/operating-policy.md) §3 rule 2 says
architecture changes get an adversarial architect review, and the standing
`always_full_documentation` / `doc_stewardship_policy` rules require that what
is learned lands in `docs/` or `kb/`. Both were **pure prose**: a grep across
`go/internal/` at cycle-1141 found no compiled check tying "this change touched
the trust kernel" to "this change touched documentation".

That is the same shape as every other rule the repo eventually had to
mechanize. Prose rules degrade silently — nothing fails, so nothing is noticed
until a later cycle reads the code and finds no record of why it looks the way
it does. The repo already has the exact pattern for closing that gap:
`SpineFloor` — a compiled default, dialed from `.evolve/policy.json`, evaluated
on a real path, with an escape hatch that needs no recompile.

## Decision

Add a **documentation floor**: a change that touches an architecture surface
and touches no file under `docs/` produces a **WARN** at the build handoff
floor.

Three deliberate boundaries:

1. **WARN, never REJECT.** "Is there a doc at all" is mechanical; "is this doc
   adequate" is editorial. The gate reports only the mechanical half and leaves
   the judgement to the auditor. A blocking verdict would make the gate the
   arbiter of documentation quality, which it cannot be.
2. **The label is derived from the diff, not self-reported.** The production
   label source is `docsfloor.IsArchitectureClass` — the classifier
   `core.docsFloorWarn` actually calls (`build_floor_reviewer.go`). It matches
   the changed paths against the trust-kernel surfaces
   (`go/internal/{core,policy,config,phasecontract,router}/`,
   `docs/architecture/phase-registry.json`) plus the guard/ship/fleet surfaces,
   new packages and phase specs, and it drops test-only diffs. The original
   broad predicate `LabelArchitecture` is retained in the package as the
   documented coarse form but has **no production caller** — cycle-1144 rewired
   the WARN to the strict classifier and nothing else calls it. A self-reported
   label is exactly what a rushed change omits, so asking for one would make the
   floor self-defeating.
3. **SKIP is not PASS.** Stage off, an unlabeled change, and an empty change set
   all yield `StatusSkip` with a reason, so "not judged" can never be read back
   as "judged clean".

### Shape

```go
// go/internal/docsfloor — stdlib only, zero intra-repo imports
type Config  struct{ Stage string }                                  // off | shadow | enforce
type Input   struct{ ArchitectureLabeled bool; ChangedFiles []string }
type Verdict struct{ Status, Reason string }                          // PASS | WARN | SKIP
func Evaluate(cfg Config, in Input) Verdict
func IsArchitectureClass(changedFiles []string) bool // strict — THE labeler production calls
func HasDocsDelta(changedFiles []string) bool        // the "≥1 docs/ file" half
func LabelArchitecture(changedFiles []string) bool   // broad/coarse — no production caller
```

Decision order: `stage==off` ⇒ SKIP · not labeled ⇒ SKIP · empty change set ⇒
SKIP · ≥1 `docs/` file ⇒ PASS · otherwise WARN (with a non-empty `Reason`
naming the missing surface and citing §3.2).

### Configuration

Compiled default `stage="enforce"`, overridable with no recompile:

```json
{ "docs_floor": { "stage": "off" } }
```

`enforce` still only WARNs (see boundary 1), so arming it by default is
verdict-neutral for every existing cycle. `shadow` is reserved for a future
promotion to a blocking verdict — the dial exists now so that promotion needs no
new surface.

### Call site (wiring proof — the I2 invariant)

`core.DefaultBuildFloorChecks` already derives the handoff's changed-path set
once (`changedFloorPaths`, cycle-base diff with a HEAD fallback). `docsFloorWarn`
rides **that same set** — one `git diff` per handoff stays the standing rule —
reads `docs_floor.stage` from `<ProjectRoot>/.evolve/policy.json`, and prints
`[docs-floor] WARN: …` to stderr, the channel every other fail-open floor signal
uses and the one the auditor reads in the phase log. It returns no failure
string, so it can never convert into a handoff REJECT.

Cycle-1141 ACS predicate `TestC1141_008_docsfloor_config_injected_and_wired`
asserts the policy round-trip *and* greps for a production (non-test) caller
outside the gate's own package — the anti-inert half.

## Consequences

- **Positive.** The documentation rule now has a mechanical floor on the live
  build path. An architecture change that ships with no doc is visible in the
  phase log at handoff instead of being discovered cycles later.
- **Cost.** A lane that legitimately churns architecture surfaces without a doc
  delta will see a recurring WARN. That is the intended pressure; `stage: "off"`
  is the documented relief valve.
- **Import-graph side effect.** Deriving the retention engine's run markers from
  the phasecontract registry (the sibling cycle-1141 task) exposed an inverted
  edge: `internal/policy` — the config SSOT — held `GC *gc.Policy`, i.e. config
  depended on an engine, closing
  `policy → gc → phasecontract → phasespec → policy`. The gc config structs were
  extracted to the zero-dependency `internal/gcpolicy` leaf (`gc` re-exports via
  type aliases, so no call site changed), the same move `cyclestate` and
  `shiperr` made in the decoupling campaign.

## Alternatives rejected

- **Block the handoff on a missing doc.** Rejected: turns a mechanical presence
  check into a quality verdict the gate cannot make, and a false block on the
  build path is far more expensive than a missed WARN.
- **Require an explicit `architecture:` label from the agent.** Rejected: the
  change most likely to skip its documentation is also the one most likely to
  skip its label. Deriving from the diff cannot be forgotten.
- **Gate at commit/ship instead of build handoff.** Rejected for now: the build
  floor already owns the derived change set, and warning at handoff gives the
  auditor the finding while the cycle can still act on it.
