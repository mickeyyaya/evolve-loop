# ADR-0084 — Gate integrity invariants: tracked-state scanning, single-sourced contracts, persisted evidence

- **Status:** Accepted (operator-directed, 2026-08-10)
- **Driving incident:** [2026-08-09 zero-ship batch](../../incidents/2026-08-09-zero-ship-batch.md) — 10 cycles / 0 ships, ADR-0072 pipeline-blocker halt on fingerprint `ship|gate-block|cd49274beab2`.
- **Related:** ADR-0072 (system-failure policy), ADR-0074 (disposition contract), #421–#426.

## Problem

Three structural defect classes let a healthy quality bar produce a dead pipeline:

1. **Environment-divergent scanning.** Repo-wide guard suites (profiles, phasespec,
   phasecoherence) bound EVERY on-disk file in `.evolve/profiles` / `.evolve/phases`.
   The runtime mints untracked stubs into those same dirs, so the ship-time scanner
   pack red'd on state that can never reach a CI checkout — blocking three
   audit-green ships in one batch (and the v22.13.0 release before it). The prior
   mitigation — per-name `.gitignore` / allowlist entries — re-arms on every new mint.
2. **Uncommunicated machine-graded contracts.** Artifacts an LLM agent must author
   and Go code then parses (defect-dispositions.json, disposition.json, eval grader
   commands, …) documented their schema as prose or placeholder pseudo-JSON. Agents
   guessed shapes; fail-hard gates rejected the guesses; one gate
   (`evolve eval quality-check`) was silently vacuous for 281/625 evals because the
   template format and the scanner had drifted apart with no test binding them.
3. **Swallowed gate evidence.** The repo-contract ship gate ran a full `go test`
   pack and persisted none of its output (`ship-error.json` `debug:""`) — a false
   RED was undiagnosable from run artifacts and burned three cycles before a
   console forensic dig found the one failing test line in a process stdout stream.

## Hypothesis

Each class is killable by a **structural invariant enforced by deterministic Go
tests** (zero LLM-token runtime cost), rather than by more per-instance patches:

- I1 — *a scanner binds only git-TRACKED state*: untracked files are runtime
  state; they cannot land on main, so no gate may red on them.
- I2 — *every LLM-authored, machine-graded artifact ships a literal example
  single-sourced against its production reader*: the example lives in the
  authoring agent's instructions, a Go const mirrors it, and a three-legged test
  (prompt ↔ const ↔ real parser/gate) fails CI on any drift.
- I3 — *no gate fails without persisted evidence*: a gate that runs a subprocess
  or computes a verdict must write its underlying output to the cycle's run dir
  and name the offending item in its error; fail-open paths must log loudly.

## Decision

Adopt all three invariants; land enforcement in phases:

- **I1 (landed, #425):** `internal/repostate.TrackedSet/TrackedFiles` is the one
  answer to "what does git track here" (direct children only — no nested
  basename aliasing; staged counts as tracked; errors carry git stderr; callers
  fall back loud-and-strict to bind-all, never silently to bind-none). All 13
  real-tree scanner call sites funnel through it; decoy regression tests per
  package. Deviation from plan: the ship-time persona-lint scan needed NO filter —
  its enumeration is persona-driven (`fs.ReadDir(agents/)`), so untracked profile
  mints never enter it; forcing one would be dead code.
- **I2 (landed for the incident contracts, #422/#426):** disposition.json and
  defect-dispositions.json carry legal literal examples single-sourced against
  `VerifyDisposition` / `readDispositions`; the eval grader format is
  single-sourced template ↔ scanner with fence-state tracking so illustration/decoy
  bullets never count, zero commands is a reasoned WARN, and score_cap-graded
  evals are recognized as ACS-jurisdiction. Remaining untied contracts are queued
  (routing-plan mint, triage-decision.json, dead-contract sweep) — the recipe is
  [contract-single-sourcing.md](../contract-single-sourcing.md).
- **I3 (in progress):** the repo-contract gate gains full-output persistence to
  `<runs>/cycle-N/ship-repocontract-scan.log`, failing-test names in the ship
  error, one retry before RED, and infra-vs-contract classification; the
  vet/acs-durable ≤12-line truncation gains a full-output log artifact; silent
  fail-opens (`manifest.go` git-capture, `binary_staging_guard.go`,
  `audit.go` malformed acs-verdict) gain loud log lines.
- **Volume-keyed halt (landed, #423):** 3 consecutive failed cycles — any
  fingerprints — halt the batch for a deep-dive (`consecutive-failures`
  blocker-breaker rule; compiled default 3, policy-overridable). The
  identity-keyed rule alone let this incident run 10 cycles.

## Consequences

- The per-name `.gitignore` ratchet entries remain for staging hygiene but are no
  longer load-bearing for any test — new runtime mints need zero hand-edits.
- Prompt-side literal examples cost ~200–400 tokens per affected dispatch
  (~1K/cycle) against the ~2M-token cost of one wasted cycle — ≥1000× ROI. All
  gate-side enforcement is deterministic Go: zero runtime LLM tokens added.
- Repo-contract pack wall-time is unchanged (no new pack members; additions live
  inside existing suites).
- Review lenses (diff-review skill + auditor persona) now name the three
  invariants, so a diff violating one is challengeable at review time, before any
  gate fires.
