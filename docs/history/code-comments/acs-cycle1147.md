# Comment history: `acs/cycle1147`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1147/predicates_test.go:3` — above `package cycle1147`

```text
// Package cycle1147 materialises the cycle-1147 acceptance criteria for the two
// tasks triage committed to THIS cycle:
//
//   - artifact-name-ssot-remaining-callsites → add phasecontract.ArtifactFilename
//     and route the three remaining hand-rolled `<phase>+"-report.md"` call
//     sites (core/cyclerun_remediate.go:81, core/phase_bindings.go:254,
//     cycleclassify/classify.go:446) through it, fixing the retro-phase path
//     mismatch (registry says "retrospective-report.md", the literal says
//     "retro-report.md").
//   - docs-floor-architecture-change-gate → PRE-EXISTING GREEN, see 006.
//
// The third fleet-scoped id (required-roles-ssot) was DROPPED by triage
// (already implemented; contract_registry.go:250-271 + required_ssot_test.go)
// and therefore carries ZERO predicates here — R9.3: predicates bind only to
// triage-committed work, and a predicate gating dropped/deferred work starves
// the committed task (the cycle-280 failure mode).
//
// Predicate strategy. The defect is a vocabulary duplication whose ONLY
// observable divergence is the retro phase, so the predicates attack it on the
// two axes that can actually see it:
//
//   - 001/002 are BEHAVIORAL over the new SSOT helper itself: they call
//     phasecontract.ArtifactFilename and assert its return value. 001 pins the
//     divergent phase (retro), 002 pins the fallback + NoArtifact edges. Both
//     red-fail today at COMPILE time — the helper does not exist.
//   - 003 is BEHAVIORAL end-to-end through the exported cycleclassify.Classify
//     over a synthetic workspace: the prompt-echo veto must find the retro
//     deliverable at its REGISTRY name. It red-fails today because
//     classify.go:446 reads "retro-report.md", which never exists.
//   - 004 is 003's negative twin and the anti-gaming half: with the deliverable
//     present ONLY at the legacy "retro-report.md" name, the veto must NOT
//     fire. Without it, a builder could green 003 by reading both names (or by
//     failing open), which would preserve the very duplication the task
//     removes.
//   - 005 is the duplication-ABSENCE check over the three named call sites.
//     Duplication is inherently a source-level property — no runtime
//     observation can distinguish three copies of an equal string from one
//     shared call — so this is the sanctioned absence-check form
//     (go/acs/README.md), and it is load-bearing only in company with 001,
//     which fails if the SSOT declaration is deleted or renamed to green it.
//   - 006 is the docs-floor task's REGRESSION predicate. That task's
//     implementation is already in-tree (see the AC-Materialization table in
//     test-report.md); 006 is behavioral and pins the gate's decision table and
//     its policy-injected default so the pre-existing GREEN cannot silently rot.
```

### `go/acs/cycle1147/predicates_test.go:126` — above `func TestC1147_003_classify_prompt_echo_veto_finds_retro_deliverable(t *testing.T) {`

```text
// TestC1147_003_classify_prompt_echo_veto_finds_retro_deliverable drives the
// REAL production path end-to-end through the exported cycleclassify.Classify.
//
// The cycle-641/642 prompt-echo veto (classify.go:435 isPromptEchoSelfReport)
// suppresses a bogus infra_failure when an agent merely quoted its own prompt
// on a phase that PASSed and exited 0. Its condition (2) reads the phase's
// deliverable — at `phase+"-report.md"`. For retro that path never exists, so
// the veto can never fire and a retro prompt-echo is permanently misclassified
// as an infrastructure failure.
//
// This fixture satisfies all three veto conditions with the deliverable at its
// REGISTRY name. RED today (veto misses ⇒ ClassInfrastructure); GREEN once
// classify.go:446 resolves through the SSOT.
```

### `go/acs/cycle1147/predicates_test.go:205` — above `func TestC1147_006_docsfloor_gate_regression(t *testing.T) {`

```text
// TestC1147_006_docsfloor_gate_regression is the docs-floor task's regression
// predicate. Unlike 001-005 this is PRE-EXISTING GREEN: the gate, its policy
// dial, its consumers and ADR-0077 all landed already (see test-report.md's
// AC-Materialization table for the evidence). It is authored anyway so the
// committed task carries a binding predicate that fails loudly if the gate
// regresses inside this cycle.
//
// Behavioral: exercises docsfloor.Evaluate's real decision table plus the
// policy-injected compiled default.
```
