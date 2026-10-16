# Comment history: `internal/phases/build`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d6

### `go/internal/phases/build/audit_repair_prompt_test.go:3` — above `import (`

```text
// audit_repair_prompt_test.go — WIRING PROOF for the audit-repair loop
// (operating-policy §3.3). The repair branch is worthless if the audit's
// reasoning never reaches the agent asked to act on it: the cycle would rebuild
// blind and re-earn the same rejection, twice, at full cost.
//
// This is the check PR #503 shipped without, and the reason its hardening could
// have gone permanently dark unnoticed. It asserts the verbatim reason text
// survives prompt composition, and the paired negative pins that an absent key
// leaves the legacy prompt byte-identical.
//
// Measured motivation: cycle-1577 resumed cycle-1574's failed task and its
// scout, fault-localization and bug-reproduction prompts contained ZERO
// references to the failure; only build mentioned it, once.
```

### `go/internal/phases/build/audit_repair_prompt_test.go:24` — above `const auditRepairFixtureReason = "EGPS: red_count=1 [record_absent_from_inbox_root_exactly_once] (cycle ships only when …`

```text
// The exact EGPS reason that failed cycle-1574 — a real string from a real
// rejection, so the proof is not tautological over a synthetic token.
```

### `go/internal/phases/build/build.go:55` — above `if scope := runner.LaneScope(req); scope != "" {`

```text
// Cycle-776 (fleet-lane-provisioning-split residual): render the pinned
// lane scope so the builder binds ONLY to this lane's task ids.
```

### `go/internal/phases/build/build.go:64` — above `fmt.Fprintf(&b, "\n\n## Task Contract\n%s", contract)`

```text
// Harness-owned (ADR-0098): heading only — the preamble and the block
// are composed once in core so tdd, build and audit read identical words.
```

### `go/internal/phases/build/build.go:73` — above `if findings := ctx["continuation_findings"]; findings != "" {`

```text
// ADR-0076 slice C: an adopted continuation resumes a prior attempt's
// preserved work — hand the builder that attempt's failure findings so it
// finishes what remains instead of rediscovering it. Absent key ⇒
// byte-identical legacy prompt.
```

### `go/internal/phases/build/build.go:120` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```

### `go/internal/phases/build/build.go:126` — above `PhaseIO config.Stage`

```text
// PhaseIO threads the EVOLVE_PHASE_IO stage into the reconcile rung (ADR-0050
// §3.10 Slice 1). Zero value (StageOff) = byte-identical.
```

### `go/internal/phases/build/build_envelope_test.go:10` — above `func TestComposePrompt_BuildPlanEnvelope(t *testing.T) {`

```text
// TestComposePrompt_BuildPlanEnvelope pins ADR-0050 Phase 3.7: the build phase's
// upstream build-plan is served exclusively via the typed PhaseRequest.BuildPlan
// envelope (populated once at the dispatch seam when the planner is enabled at
// advisory+). When BuildPlan is empty, no section is injected — the disk fallback
// was removed in cycle-39 alongside the EVOLVE_BUILD_PLANNER env migration.
```

### `go/internal/phases/build/build_test.go:113` — above `func TestClassifyArtifact_HeadingVariants(t *testing.T) {`

```text
// TestClassifyArtifact_HeadingVariants — cycle-192: the builder template's
// changed-files heading drifted to "## Changes"; the classifier must accept it
// (and the legacy/template variants) or a valid build report false-FAILs and
// trips the auditor cross-check, blocking ship.
```

### `go/internal/phases/build/build_test.go:223` — above `func TestRun_MissingBridge_ReturnsError(t *testing.T) {`

```text
// Direct unit tests for the resolver moved to
// internal/phaseflags/phaseflags_test.go when the helper was extracted
// in v12.1.0 second slice. The phase-runner-level integration tests
// above (TestRun_*) still exercise the resolver end-to-end through Run.
```

### `go/internal/phases/build/lanescope_prompt_test.go:3` — above `import (`

```text
// Cycle-776 RED contract — fleet-lane-provisioning-split (residual slice):
// build's composed prompt must render the pinned lane scope so the builder
// binds to THIS lane's todo ids (cycle-766 put the scope into Context; only
// triage renders it — see scout/lanescope_prompt_test.go for the incident).
```

### `go/internal/phases/build/prior_findings_test.go:3` — above `import (`

```text
// prior_findings_test.go — ADR-0076 slice C (C7): an adopted continuation's
// prior-attempt findings render as a dedicated prompt block so the builder
// resumes INFORMED (what failed, what to finish) instead of rediscovering.
// Absent findings pin a byte-identical legacy prompt.
```

### `go/internal/phases/build/standing_findings_prompt_test.go:10` — above `func TestComposePrompt_StandingAuditFindingsReachTheAgent(t *testing.T) {`

```text
// Cycle 1679 (2026-09-15): audit round 4 passed WARN with three MEDIUM
// defects and the cycle went to ship; the ship hit GIT_FLEET_REBASE_NEEDED and
// recovered via a rebuild whose brief carried NO audit section (the repair
// brief seeds only after a rejection), so round 5 found the same defects
// "standing" and WARNed again. A recovery rebuild is re-audited by the same
// rubric: the last audit's actionable findings ride the brief as data.
```
