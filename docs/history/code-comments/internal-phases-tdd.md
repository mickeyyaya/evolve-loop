# Comment history: `internal/phases/tdd`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d9

### `go/internal/phases/tdd/audit_repair_prompt_test.go:3` — above `import (`

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

### `go/internal/phases/tdd/audit_repair_prompt_test.go:24` — above `const auditRepairFixtureReason = "EGPS: red_count=1 [record_absent_from_inbox_root_exactly_once] (cycle ships only when …`

```text
// The exact EGPS reason that failed cycle-1574 — a real string from a real
// rejection, so the proof is not tautological over a synthetic token.
```

### `go/internal/phases/tdd/compose_parity_test.go:3` — above `import (`

```text
// RED-phase projection-equivalence contract for cycle-249 task
// `runner-base-cycle-context` (intent AC: "new tests prove projection
// equivalence — old hardcoded values == new config-derived values").
//
// tdd is the representative migrated caller: its ComposePrompt must equal
// runner.BaseCycleContext(body, req) + its single phase-specific extra
// (the worktree line). Byte-equality here is the behavioral guarantee that
// the refactor changed structure, not prompts.
//
// Fails at baseline: runner.BaseCycleContext is undefined (compile RED).
```

### `go/internal/phases/tdd/lanescope_prompt_test.go:3` — above `import (`

```text
// Cycle-776 RED contract — fleet-lane-provisioning-split (residual slice):
// tdd's composed prompt must render the pinned lane scope so predicates bind
// to THIS lane's todo ids (cycle-766 put the scope into Context; only triage
// renders it — see scout/lanescope_prompt_test.go for the incident).
```

### `go/internal/phases/tdd/standing_findings_prompt_test.go:10` — above `func TestComposePrompt_StandingAuditFindingsReachTheAgent(t *testing.T) {`

```text
// Cycle 1679 (2026-09-15): audit round 4 passed WARN with three MEDIUM
// defects and the cycle went to ship; the ship hit GIT_FLEET_REBASE_NEEDED and
// recovered via a rebuild whose brief carried NO audit section (the repair
// brief seeds only after a rejection), so round 5 found the same defects
// "standing" and WARNed again. A recovery rebuild is re-audited by the same
// rubric: the last audit's actionable findings ride the brief as data.
```

### `go/internal/phases/tdd/tdd.go:67` — above `if scope := runner.LaneScope(req); scope != "" {`

```text
// Cycle-776 (fleet-lane-provisioning-split residual): render the pinned
// lane scope so predicates bind ONLY to this lane's task ids.
```

### `go/internal/phases/tdd/tdd.go:81` — above `fmt.Fprintf(&b, "\n\n## Task Contract\n%s", contract)`

```text
// Harness-owned (ADR-0098): heading only — the preamble and the block
// are composed once in core so tdd, build and audit read identical words.
```

### `go/internal/phases/tdd/tdd.go:115` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```

### `go/internal/phases/tdd/tdd_test.go:201` — above `func TestClassify_HeadingVariants(t *testing.T) {`

```text
// TestClassify_HeadingVariants — cycle-192: the TDD report's acceptance and
// RED-tests headings drifted ("## RED Run Output"/"## Test Files Written",
// "## AC-Materialization"/"## Coverage Map"); the classifier must accept the
// current variants (and the legacy "## Acceptance"/"## RED Tests") or every
// current TDD report false-FAILs.
```
