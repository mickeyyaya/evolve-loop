# Comment history: `internal/phases/triage`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/triage/carryforward_bounds_test.go:3` — above `import (`

```text
// carryforward_bounds_test.go — regression cover for the bounded-context +
// branch-cap fix (cycle 1356, closing inherited defects d803ddb6/d538a3c0/
// d7bc70d7 from the cycle-1343 defect ledger). A cycle-1343 disposition
// claimed CarryforwardCandidatesSection was already bounded (a deadline, a
// branch cap, newest-first ordering, and a "(partial: N of M)" truncation
// line); that claim was false — context.Background() was still live and
// nothing capped the branch sweep. These tests exercise the REAL, now-fixed
// behavior directly (no source-grep degenerate predicates).
```

### `go/internal/phases/triage/carryforward_bounds_test.go:83` — above `func TestCarryforwardCandidatesSection_CapsBranchesAndSurfacesTruncation(t *testing.T) {`

```text
// TestCarryforwardCandidatesSection_CapsBranchesAndSurfacesTruncation proves
// the sweep stops after carryforwardCandidatesMaxBranches branches and the
// truncation is surfaced (not silent) via a "(partial: N of M)" line naming
// both the examined count and the total — this is the exact gap the false
// cycle-1343 disposition claimed was already closed.
```

### `go/internal/phases/triage/carryforward_bounds_test.go:133` — above `func TestCarryforwardCandidatesSection_RespectsExpiredContext(t *testing.T) {`

```text
// TestComposePrompt_CarryforwardSectionUsesBoundedContext proves ComposePrompt
// no longer hands CarryforwardCandidatesSection an unbounded
// context.Background() — a context whose deadline has ALREADY passed handed
// straight through to `git for-each-ref` must fail-open to "", proving the
// call site is now deadline-aware rather than backgrounded (the exact defect
// d803ddb6 named: no deadline, no cancellation, ever).
```

### `go/internal/phases/triage/carryforward_candidates_test.go:3` — above `import (`

```text
// carryforward_candidates_test.go — in-package apicover naming coverage for
// CarryforwardCandidatesSection (cycle 1325, mint-profile-driver-suffix's
// sibling task scout-carryforward-real-cherrypick-filter). The full
// behavioral suite (landability filtering, wiring-into-ComposePrompt proof)
// lives in go/acs/cycle1325/predicates_test.go — a separate package/directory
// the repo-wide apicover unnamed-export gate does not scan for coverage of
// THIS package's exports, so this file exists purely to name+exercise the
// symbol from within internal/phases/triage itself.
```

### `go/internal/phases/triage/console_routed_prompt_test.go:3` — above `import (`

```text
// console_routed_prompt_test.go — RED contract for ADR-0074 I1 at the
// visibility seam: a console-routed inbox item must not be OFFERED to lane
// triage at all (an LLM cannot mis-pick what it never sees), and the exclusion
// must be loud in the prompt so triage knows operator-owned work exists.
```

### `go/internal/phases/triage/console_routed_prompt_test.go:38` — above `func TestTriageComposePrompt_ProtectedFixSurfaceAutoExcluded(t *testing.T) {`

```text
// Derived exclusion uses the REAL guards predicate — pins that a protected
// fix surface (role.go was cycle-1036's burn) auto-routes without any route
// field. Guards-manifest membership is asserted first so a manifest change
// surfaces here as a loud pin move, not a silent pass.
```

### `go/internal/phases/triage/domain_default_test.go:10` — above `func TestComposePrompt_RendersDeliverableKindDefault(t *testing.T) {`

```text
// TestComposePrompt_RendersDeliverableKindDefault — ADR-0099 slice 2: triage
// (whose header is the authoritative kind) sees the project default the
// orchestrator seeds from .evolve/domain.json; absent ⇒ byte-identical prompt.
```

### `go/internal/phases/triage/inbox_batches_prompt_test.go:3` — above `import (`

```text
// inbox_batches_prompt_test.go — RED contract for the inbox batch classifier's
// triage wiring (operator directive 2026-07-16: one-item-per-cycle consumption
// pays the full pipeline per item; related items must batch). The
// DETERMINISTIC grouping lives in internal/inboxbatch (Core Rule 5); triage's
// LLM keeps only the JUDGMENT of which batch to pick. ComposePrompt renders
// the computed batches AFTER the existing stable lines with an explicit
// prefer-a-whole-batch instruction; an empty/missing inbox keeps the prompt
// byte-identical (the same pin recent_outcomes carries).
```

### `go/internal/phases/triage/protected_surface_admission_test.go:3` — above `import (`

```text
// protected_surface_admission_test.go — RED contract for the second,
// commit-time admission route (F4, docs/operations/batch-integrity-review-
// 2026-08-04.md; inbox item triage-protected-surface-admission).
//
// console_routed_prompt_test.go already pins route #1: guards.IsProtectedSurface
// screens items sourced from .evolve/inbox before they are OFFERED in the
// prompt. That screen never runs for a top_n card the LLM writes from the
// fleet-todo/scout route — a card whose `files={...}` segment names a
// protected path sails through hooks.Classify today (Classify only checks
// non-empty artifact + "## top_n" heading + >=1 list item). Cycles
// 1257/1259/1263 burned on exactly this shape. These tests pin the second,
// independent commit-time check: Classify itself must FAIL a committed
// artifact whose top_n `files=` references guards.IsProtectedSurface.
```

### `go/internal/phases/triage/protected_surface_admission_test.go:47` — above `func TestTriageClassify_RejectsProtectedSurfaceTopNCard_BareSyntax(t *testing.T) {`

```text
// Same defect, the bare (unbraced) `files=a;b` encoding real cycle-1312
// output actually used (.evolve/runs/cycle-1312/triage-report.md) — the
// admission check must not silently no-op on the brace-less variant.
```

### `go/internal/phases/triage/recent_outcomes_prompt_test.go:3` — above `import (`

```text
// Chronicle S3 RED contract (cycle-784, chronicle-s3-digest-wiring, task
// inject-recent-outcomes-prompts) — triage half; see the scout twin for the
// full contract text. Injected Context["recent_outcomes"] renders AFTER the
// existing stable lines (carryover_summary/fleet_scope); absent or empty key
// keeps the composed prompt byte-identical (shadow-stage regression pin).
//
// Builder implements; must NOT modify these tests.
```

### `go/internal/phases/triage/refusal_codes_test.go:3` — above `import (`

```text
// refusal_codes_test.go — the three deterministic refusals Classify itself
// raises (not the agent's verdict) carry a stable Diagnostic.Code, so the C1
// record and the FAIL closeout can tell "the item is operator-owned" from
// "the agent had a bad day" without regexing prose
// (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).
```

### `go/internal/phases/triage/triage.go:71` — above `carryover := req.Context["carryover_summary"]`

```text
// ADR-0050 §3.10 Slice 2: typed envelope at enforce, legacy Context below it
// (byte-identical — Active() is false unless enforce).
```

### `go/internal/phases/triage/triage.go:80` — above `if scope := runner.LaneScope(req); scope != "" {`

```text
// ADR-0049 E: under `evolve fleet --plan` this cycle is one of several running
// concurrently, each assigned a DISJOINT set of tasks. Steer selection to ONLY
// the assigned ids so two cycles never pick work touching the same files.
// Resolution + control-char sanitization live in runner.LaneScope (shared
// with scout/build/tdd since cycle-776).
```

### `go/internal/phases/triage/triage.go:95` — above `if kind := req.Context[core.CtxKeyDeliverableKindDefault]; kind != "" {`

```text
// ADR-0099 slice 2: the project's default deliverable kind (.evolve/domain.json).
// Triage's `deliverable_kind:` header is the AUTHORITATIVE kind, so triage
// must see the default a task inherits when it declares none.
```

### `go/internal/phases/triage/triage.go:104` — above `if section := inboxBatchesSection(req.ProjectRoot); section != "" {`

```text
// Inbox batch classifier (2026-07-16): one-item-per-cycle consumption pays
// the full pipeline per item, so internal/inboxbatch DETERMINISTICALLY
// groups the backlog by campaign / package area / explicit links (Core
// Rule 5 — grouping is mechanical Go; only the PICK stays LLM judgment).
// Rendered last (the section varies as items land — keeps the stable
// prefix cache-friendly); an empty/missing/unreadable inbox keeps the
// prompt byte-identical (fail-open: a broken backlog must not block
// triage, which still reads the inbox directly).
```

### `go/internal/phases/triage/triage.go:115` — above `ctx, cancel := context.WithTimeout(context.Background(), carryforwardCandidatesTimeout)`

```text
// Carry-forward candidate filter (cycle 1325, inbox item
// scout-carryforward-real-cherrypick-filter): cycle-962 built a
// deterministic, zero-LLM landability screen
// (core.CarryforwardCandidateLandable) but never wired it into triage's
// own candidate-selection path, leaving it a second, uninvoked oracle
// while the LLM kept picking from raw `git merge-tree`'s 1-arg
// non-3-way form (cycle-826: mis-selected an already-superseded orphan).
// Rendered last for the same reason as inbox_batches; "main" matches
// the base every other production caller of the carryforward filter
// family uses (ClassifyFleetRebaseCandidate call sites).
//
// Bounded (cycle-1356, closing inherited defect d803ddb6 — a prior
// cycle-1343 disposition claimed this was already bounded; it was not):
// context.Background() would let one slow/hung `git merge-tree` per
// branch stall prompt composition on triage's critical path
// indefinitely, so the call is wrapped in a deadline sized for the
// worst case the section itself caps (carryforwardCandidatesMaxBranches
// branches probed at a few hundred ms of git subprocess work each).
```

### `go/internal/phases/triage/triage.go:141` — above `const carryforwardCandidatesTimeout = 8 * time.Second`

```text
// carryforwardCandidatesTimeout bounds the context ComposePrompt hands
// CarryforwardCandidatesSection — sized for carryforwardCandidatesMaxBranches
// branches at a few hundred ms of `git merge-tree` work each (cycle-1356,
// closing inherited defect d803ddb6: this advisory section must never stall
// triage's critical path on an unbounded git subprocess sweep).
```

### `go/internal/phases/triage/triage.go:148` — above `const carryforwardCandidatesMaxBranches = 40`

```text
// carryforwardCandidatesMaxBranches caps how many local cycle-* branches
// CarryforwardCandidatesSection probes with core.CarryforwardCandidateLandable
// (each probe is a real `git merge-tree` dry-run — not free). Branches are
// examined newest-committed-first so the cap drops the stalest candidates,
// never the freshest (sibling inboxBatchesSection caps via
// inboxbatch.DefaultMaxItems for the same reason: an uncapped local sweep
// grows linearly with the branch set and was measured at 150+ branches /
// 10s+ of blocking git work — cycle-1343 defect dba64c28 / d803ddb6).
```

### `go/internal/phases/triage/triage.go:158` — above `func CarryforwardCandidatesSection(ctx context.Context, dir, base string) string {`

```text
// CarryforwardCandidatesSection renders the deterministic carry-forward
// candidate list for the triage prompt: the local `cycle-*` branches in dir
// that core.CarryforwardCandidateLandable reports landable onto base (a real
// 3-way merge dry-run, not already superseded) — the exact cycle-962 filter
// left uninvoked. Fail-open like inboxBatchesSection: an unresolved dir/base,
// a git-infrastructure error, or zero landable candidates all render "",
// never blocking triage (which still runs its own selection independently).
//
// Bounded (cycle-1356): branches are listed newest-committed-first and
// probing stops after carryforwardCandidatesMaxBranches — truncation is
// surfaced via a trailing "(partial: N of M branches probed)" line, never
// silent, so an operator/auditor can tell the list is incomplete without
// re-deriving the branch count by hand.
```

### `go/internal/phases/triage/triage.go:232` — above `dispatchable, console, _ := inboxbatch.PartitionConsole(items, guards.IsProtectedScope)`

```text
// ADR-0074 I1: console-routed items (route:"console-*", a pipeline-* kind,
// or a protected fix surface) are operator-owned — lanes must never see
// them as selectable. The exclusion is loud (ids listed) so triage knows
// the work exists; inboxmover.Claim is the enforcement backstop if a pick
// slips through.
```

### `go/internal/phases/triage/triage.go:283` — above `if id, path, hit := protectedTopNViolation(body); hit {`

```text
// F4 (docs/operations/batch-integrity-review-2026-08-04.md): the prompt-side
// inboxbatch.PartitionConsole screen (inboxBatchesSection above) only ever
// sees items sourced from .evolve/inbox — it cannot see a top_n card the LLM
// wrote from the fleet-todo/scout route. This is the second, independent,
// commit-time admission check guards.IsProtectedSurface needs: any top_n
// card whose files={...} (or bare files=...) segment names a protected
// control-plane path is refused here, regardless of how the card originated.
```

### `go/internal/phases/triage/triage.go:380` — above `ContractVerifier func() runner.ContractVerifier`

```text
// ContractVerifier is the deliverables gate's verifier accessor for the
// verdict engine (runner.Options.ContractVerifier): one verifier for gate
// and engine (research F22). nil = the catalog-aware default.
```

### `go/internal/phases/triage/triage.go:385` — above `PhaseIO config.Stage`

```text
// PhaseIO threads the EVOLVE_PHASE_IO stage into the reconcile rung (ADR-0050
// §3.10 Slice 1). Zero value (StageOff) = byte-identical.
```

### `go/internal/phases/triage/triage_phaseio_test.go:11` — above `func TestTriage_ComposePrompt_TypedEqualsMap(t *testing.T) {`

```text
// ADR-0050 §3.10 Slice 2: triage reads carryover_summary + fleet_scope from the
// typed envelope at enforce (req.Input.Active()) and the legacy Context below it.
```

### `go/internal/phases/triage/triage_test.go:260` — above `func TestComposePrompt_InjectsFleetScope(t *testing.T) {`

```text
// TestComposePrompt_InjectsFleetScope: under `evolve fleet --plan` a cycle gets
// EVOLVE_FLEET_SCOPE → Context["fleet_scope"]; triage must steer selection to ONLY
// the assigned task IDs so concurrent cycles work disjoint files (ADR-0049 E).
// Absent when not scoped (legacy single-cycle behavior unchanged).
```
