# Build Explanation — Cycle 1676

## Build Binding
- Cycle: 1676
- Base SHA: 83a019aa0fa7fe6b50d91b2bb84070805a3cab45

## Summary
A cycle now records a deterministic advisory aggregate of four weak cross-artifact
verifiers — verdict agreement, test-count agreement, referenced-path existence and
provenance/phase order — at `<workspace>/crossartifact-invariants.json`, evaluated from the
lane worktree at the real cycle-close seam and changing nothing about whether the cycle blocks.

## Rationale
Weaver (arXiv:2506.18203) is the evidence behind the shape: a stack of weak deterministic
verifiers approaches strong-verifier power at near-zero cost and, unlike an LLM judge, cannot
be talked out of a finding. Each invariant is drawn from a real incident rather than invented —
cycle 1673 sealed an explanation document claiming "zero red predicates" while the
`acs-verdict.json` beside it recorded `red_count=3` (test-count agreement); PR #612 had to
re-bind a cross-artifact check to the lane worktree after a project-root snapshot stood in for
what a lane had actually changed (referenced-path lane binding); cycles 862→899 are why verdict
agreement is checked at all.

Three statuses rather than two is the load-bearing decision. `indeterminate` keeps an absent or
malformed artifact from ever reading as a verified match — which is how a presence-only check
gets gamed — and equally from ever being counted a violation, which is how an advisory earns a
false-positive rate and gets switched off. Every invariant fails safe into it.

Everything ships advisory, per the inbox record's own rule and the 1054/1060 breaker lesson.
The report is written on every cycle, all-ok included, because a false-positive rate that is
never recorded can never be evidenced — and that evidence is the only door to graduating any
invariant to blocking, which must be a separate and separately-evidenced decision.

Alternatives rejected: a new gate or policy key (it would give these checks teeth before their
false-positive rate exists); importing `internal/acssuite` to decode `acs-verdict.json` (it
would drag config/profiles/policy/gitexec into a deliberately lean leaf — a local struct matches
the `ReadCycleVerdicts` precedent in the same package); and a bespoke regex for the verdict
sentinel (it would re-open the cycle-603 placeholder-echo bug on the very signal the check
depends on, so parsing goes through `phasecontract` alone).

## Changed Areas
- `go/internal/coherence/crossartifact.go` — new. Adds the pure aggregate
  `CheckCrossArtifactInvariants(workspace, worktree)` and its report types beside the existing
  ADR-0072 verdict-coherence leaf, which is untouched. It reads the audit sentinel through
  `phasecontract.ParseVerdictSentinelFull`, the timing chain through `phasetiming.Read`, and
  `acs-verdict.json` through a local struct, then reports exactly four invariants in a fixed
  order so two evaluations of one unchanged workspace are byte-identical.
- `go/internal/core/crossartifact_invariants.go` — new. The advisory recorder: it evaluates the
  aggregate, writes `crossartifact-invariants.json` atomically (temp file + rename), warns
  loudly on stderr for each violation, and returns nothing a caller could branch on — an
  advisory observer must not be able to fail a cycle.
- `go/internal/core/cyclerun.go` — one call added to `finalizeCycle`, the terminal segment
  `RunCycle` always reaches, bound to `cs.WorkspacePath` and `cs.ActiveWorktree`. It is placed
  before the ADR-0072 floor deliberately, so a finding can be read next to the floor's decision
  without ever being an input to it.
- `docs/explain/builds/cycle-1676-01m2ffk2tswkga3btqsgps7cxv.md` — this document.

## Design Decisions
`CheckCrossArtifactInvariants` is the single public entry point; the four checks are unexported
functions, so the package's surface grows by one behaviour rather than four. The lane worktree
is a parameter rather than something the aggregate discovers, which is what lets the unit tests
pin lane resolution without a repository and lets the seam pin that the production caller passes
`cs.ActiveWorktree` rather than the `projectRoot` argument. No map is used anywhere in the
report path, because map iteration order would defeat the determinism the advisory depends on.
`Violations()` is the projection every consumer reads through, and it returns violated entries
only — indeterminates never reach it.

## Verification
`go test -count=1 ./internal/coherence` — 18/18 PASS, covering the all-ok pole, each invariant's
disagreement with the values named in its evidence, a prose report stuffed with the word PASS
that must not satisfy the sentinel check, six malformed/out-of-distribution artifacts that must
each land indeterminate, and byte-identical re-evaluation. `go test -count=1 -run
TestFinalizeCycle_... ./internal/core` — 4/4 PASS at the real seam: the artifact is recorded,
four violations still close the cycle PASS with no system failure, a path that exists only under
the project root stays violated while the same path in the lane worktree reads ok, and the
ADR-0072 forgery halt is unchanged. The cycle's ACS suite is 7/7 GREEN, and the repo-wide
public-API gate reports 16 exported / 16 covered / 0 false-green on the enrolled package.

## Compatibility
Purely additive. No existing signature, artifact or gate changes; `CheckVerdictCoherence` and
`ReadCycleVerdicts` keep their exact behaviour, and no verdict, system-failure or ship decision
reads the new report. A consumer that does not know `crossartifact-invariants.json` is
unaffected by its presence.

## Limitations
The false-positive baseline measured at authoring time covers the test-count invariant only,
against the real `acs-verdict.json` artifacts of cycles 1659, 1666 and 1673 (0/3). The
referenced-path and phase-order invariants have no real-corpus baseline yet, which is precisely
why every finding is advisory and why the record is written on every cycle. The phase-order
check reads only entries carrying both timestamps and reports indeterminate when fewer than two
remain, so a workspace whose timing log is largely untimed yields no claim rather than a weak one.
