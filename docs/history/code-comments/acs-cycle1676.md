# Comment history: `acs/cycle1676`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1676/predicates_test.go:3` — above `package cycle1676`

```text
// Package cycle1676 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	crossartifact-invariant-stack  (medium, weight 0.85, feature)
//
// THE FEATURE. One deterministic per-cycle aggregate over a cycle's own
// artifacts, running the four weak verifiers the inbox record names — embedded
// sentinel == standalone verdict JSON, claimed test counts == independently
// recounted runner results, every cited evidence path resolves on disk, and the
// provenance/phase-order chain is intact. Weaver (arXiv:2506.18203): a stack of
// weak deterministic verifiers approaches strong-verifier power at near-zero
// cost and is immune to LLM-judge bias. Every invariant ships ADVISORY until
// its false-positive rate is evidenced ~0 (the 1054/1060 breaker lesson), which
// is the inbox record's own rule and the hardest thing for this suite to keep
// honest — hence 004.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 the suite reports sentinel/verdict disagreement, test-count
//	    disagreement, missing referenced paths and invalid phase order,
//	    each with concrete evidence                                     → 001
//	AC2 malformed/absent artifacts fail safe and stay distinguishable
//	    from a verified match; an arbitrary PASS string cannot satisfy
//	    the suite                                                       → 002
//	AC3 the aggregate is deterministic, keeps phasecontract as the
//	    canonical sentinel parser, and runs against the lane
//	    workspace/worktree rather than a project-root substitute        → 003
//	AC4 findings stay advisory pending a separate false-positive-rate
//	    graduation; existing verdict-coherence behaviour stays green    → 004, 005
//	AC5 the materialized eval exists with behavioral [code] checks      → 006
//	house rule: the enrolled package's public-API gate stays green      → 007
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002's prose-PASS
// and fail-safe bindings (a greppable implementation greens 001 and fails
// there), 003's lane-vs-project-root pair, and 004's "four violations still
// close the cycle PASS" — the one an over-eager implementation fails by
// blocking. EDGE/OOD: truncated JSON, wrong-typed counts, an empty results
// array, an object where an array belongs, unparseable timestamps, an empty
// audit report. SEMANTIC: four distinct invariants, determinism, advisory
// wiring, ADR-0072 no-regression, and the eval's own durability — five distinct
// behaviours, not one restated.
//
// Flaky-shape contract: every `go test` names ONE package and is -run narrowed
// (internal/core is a known-slow suite), every go invocation is `go -C` anchored
// to the lane's module root, no wall-clock bounds, no literal PIDs, every git
// call is -C anchored, and the coverage profile 007 needs is written to the
// test's own temp dir — never into the tree.
//
// Reachability probe (cycle-644 rule): this package imports only pkg/acsassert
// and the standard library — a leaf. The frozen bindings are in-package
// (internal/coherence, internal/core); internal/core ALREADY imports
// internal/coherence, so no new import edge is pinned. `go list -deps
// ./internal/phasetiming` carries no edge back to internal/coherence, and a
// compiler probe confirmed internal/coherence -> internal/phasetiming builds.
```

### `go/acs/cycle1676/predicates_test.go:142` — above `func TestC1676_003_AggregateIsDeterministicAndLaneBound(t *testing.T) {`

```text
// TestC1676_003_AggregateIsDeterministicAndLaneBound — AC3. Two evaluations of
// one unchanged workspace must be byte-identical over exactly the four named
// invariants in a stable order (an unstable report cannot be diffed across
// cycles, and that diff is the only way an advisory's false-positive rate ever
// becomes measurable). The lane half runs at BOTH levels: cited paths resolve
// under the workspace and the passed worktree in the unit, and the production
// caller hands it the LANE worktree rather than the project root at the seam
// (#612 — a project-root snapshot is not evidence of what a lane changed).
```

### `go/acs/cycle1676/predicates_test.go:160` — above `func TestC1676_004_FindingsAreRecordedAtTheRealSeamAndStayAdvisory(t *testing.T) {`

```text
// TestC1676_004_FindingsAreRecordedAtTheRealSeamAndStayAdvisory — AC4's
// advisory half, and the wiring proof. The bindings drive the REAL cycle-close
// path (finalizeCycle, the terminal segment RunCycle always reaches): it must
// leave a decodable crossartifact-invariants.json naming all four invariants,
// and four violations must still close the cycle PASS with no system failure. A
// checker nothing calls is the defect class the stack was written to catch
// (#373); an advisory that quietly gained teeth is the 1054/1060 breaker lesson
// the inbox record forbids repeating.
```

### `go/acs/cycle1676/predicates_test.go:175` — above `func TestC1676_005_ExistingVerdictCoherenceBehaviourIsUnchanged(t *testing.T) {`

```text
// TestC1676_005_ExistingVerdictCoherenceBehaviourIsUnchanged — AC4's
// no-regression half, at both levels: the ADR-0072 leaf keeps its forgery
// signature, its reconcile self-heal and its diagnosed-negative exemption, and
// the live floor still halts a recorded-FAIL-with-green-artifacts cycle with
// the advisory aggregate running beside it.
```

### `go/acs/cycle1676/predicates_test.go:197` — above `func TestC1676_006_MaterializedEvalIsDurableAndBehavioral(t *testing.T) {`

```text
// TestC1676_006_MaterializedEvalIsDurableAndBehavioral — AC5. The eval is the
// PERMANENT regression entry (ACS predicates are cycle-scoped and are never
// replayed), so it must exist, must not be gitignored — the cycle-93 shape,
// where a file present on disk was silently dropped at ship — and must carry
// score_cap evidence commands that RUN something. An eval whose evidence is a
// grep for a magic string caps nothing.
```
