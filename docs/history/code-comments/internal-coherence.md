# Comment history: `internal/coherence`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/coherence/coherence.go:1` — above `package coherence`

```text
// Package coherence computes the verdict-coherence signal (ADR-0072 S2): does a
// cycle's recorded verdict agree with the on-disk artifacts the phases actually
// wrote? A recorded FAIL/WARN that contradicts a green audit report AND a green
// ACS verdict means the pipeline forged the verdict — "verdict-incoherence",
// the clean-exit signature (cycles 862→899). This is the one deterministic
// input that lets the Go floor and the orchestrator catch a broken pipeline
// lying about whose fault a failure is.
//
// The pure comparison (CheckVerdictCoherence) is I/O-free and table-tested; the
// artifact reader (ReadCycleVerdicts) is the thin I/O adapter that feeds it.
```

### `go/internal/coherence/coherence.go:30` — above `SubstantiveError bool`

```text
// SubstantiveError: the negative verdict is EXPLAINED by real recorded
// evidence — a substantive (non-infra-teardown) bridge error, or a diagnosed
// runner-side gate downgrade (audit's CI-parity gates overrode a narrative
// PASS; persisted as audit-fail-reason.json error-severity reasons). An
// explained negative is a coherent task-level outcome, never forgery — the
// cycles-930/931/932 false-HALT was this field left unpopulated at the sole
// call site while the explanation sat in the response diagnostics.
```

### `go/internal/coherence/coherence.go:38` — above `FailReasons []string`

```text
// FailReasons carries the override explanations themselves (untruncated) —
// cycle-1022: SubstantiveError=true proved a reason EXISTED while every
// operator surface stayed silent about WHAT it was.
```

### `go/internal/coherence/coherence.go:42` — above `DeliverableValid bool`

```text
// DeliverableValid: the on-disk audit-report passed the FULL deliverable.Verify
// chain (challenge-token + required sections + ADR-0039 failure-context), NOT
// the cheap ParseVerdictSentinel read. It DOWNGRADES the forgery signature to a
// benign clean-exit-late-write race: when the recorded-negative is contradicted
// by green artifacts AND the deliverable fully verifies, the bridge merely
// declared the phase's clean exit before Claude Code finished its post-turn
// async writes (the runner's ~3s settle window < the observed 60-90s dribble,
// cycles 930/931/932/cycle-3) — self-heal, do not halt. A PASS-sentinel-tagged
// but malformed report yields DeliverableValid=false → still forgery → halt
// (the anti-laundering boundary). It NEVER manufactures a reconcile out of a
// case that is coherent without it — see CheckVerdictCoherence.
```

### `go/internal/coherence/coherence.go:118` — above `func ReadCycleVerdicts(workspace string) (audit, acs string, auditRan bool) {`

```text
// ReadCycleVerdicts extracts the audit evolve-verdict and the acs-verdict from a
// cycle workspace directory. The audit verdict is read via the canonical
// phasecontract.ParseVerdictSentinel (anchored to the <!-- evolve-verdict -->
// sentinel, with the placeholder-echo guard) — never a bespoke regex, which
// would re-open the cycle-603 echo bug on the very signal this gate depends on.
// Missing/malformed artifacts yield empty strings and auditRan=false — never an
// error and never a fabricated verdict (a reader that guessed would defeat the
// whole coherence check).
```

### `go/internal/coherence/coherence_test.go:37` — above `func TestCheckVerdictCoherence(t *testing.T) {`

```text
// ADR-0072 S2: the verdict-coherence signal. A recorded FAIL/WARN is only
// trustworthy if the phases' own on-disk artifacts agree. When the audit report
// says PASS and ACS says PASS but the cycle recorded FAIL/WARN, the pipeline
// forged the verdict — that is verdict-incoherence, the clean-exit signature
// that must halt (not retry). This is the exact fingerprint of cycles 862→899.
```

### `go/internal/coherence/crossartifact.go:3` — above `import (`

```text
// crossartifact.go — the cross-artifact metamorphic invariant stack (cycle-1676,
// inbox item `crossartifact-invariant-stack`). Four WEAK deterministic verifiers
// run over one cycle's own artifacts and each reports independently:
//
//	verdict-agreement       the embedded <!-- evolve-verdict --> sentinel agrees
//	                        with the standalone acs-verdict.json verdict
//	test-count-agreement    the claimed suite counts survive an independent
//	                        recount of the artifact's own parsed runner results
//	                        (cycle-1673 M1: a document claiming "zero red
//	                        predicates" sat beside an artifact recording red_count=3)
//	referenced-paths-exist  every evidence_path the audit sentinel cites resolves
//	                        under the workspace or the LANE worktree (#612: a
//	                        project-root snapshot is not evidence of what a lane wrote)
//	provenance-phase-order  phase-timing.json's chain runs forward and no audit
//	                        precedes the build it audits
//
// Weaver (arXiv:2506.18203): a stack of weak deterministic verifiers approaches
// strong-verifier power at near-zero cost and is immune to LLM-judge bias. The
// stack COMPLEMENTS the adversarial audit; it never replaces it.
//
// THREE statuses, not two. `indeterminate` is load-bearing in both directions:
// an absent or malformed artifact can never read as a verified match (that is
// how a presence-only check gets gamed), and is equally never a violation (that
// is how an advisory earns a false-positive rate and gets switched off). Every
// invariant fails SAFE into it.
//
// ADVISORY. Per the inbox record's own rule and the 1054/1060 breaker lesson,
// every invariant ships advisory: InvariantReport.Advisory is true and nothing
// here blocks a cycle. Graduating any invariant to blocking is a separate,
// separately-evidenced decision that needs a measured ~0 false-positive rate —
// which is exactly why the caller records the report on EVERY cycle.
```

### `go/internal/coherence/crossartifact.go:164` — above `func checkVerdictAgreement(s phasecontract.VerdictSentinel, sentinelOK bool, a acsArtifact, acsOK bool) Invariant {`

```text
// checkVerdictAgreement compares the embedded sentinel verdict against the
// standalone acs-verdict.json verdict (cycles 862→899).
```

### `go/internal/coherence/crossartifact.go:183` — above `func checkTestCounts(a acsArtifact, acsOK bool) Invariant {`

```text
// checkTestCounts recounts the artifact's own parsed runner results and holds
// the claimed summary against them (the cycle-1673 M1 shape).
```

### `go/internal/coherence/crossartifact.go:224` — above `func checkReferencedPaths(s phasecontract.VerdictSentinel, sentinelOK bool, workspace, worktree string) Invariant {`

```text
// checkReferencedPaths resolves every evidence path the audit sentinel cites
// against the cycle workspace and then the LANE worktree. Only the tree the
// lane actually wrote counts as evidence the lane produced (#612).
```

### `go/internal/coherence/crossartifact.go:254` — above `func resolvesUnder(root, rel string) bool {`

```text
// resolvesUnder reports whether a cited relative path exists beneath root.
//
// Containment is checked BEFORE the stat. filepath.Join cleans "../" segments
// away, so a citation that climbs out of root would otherwise land on a real
// file outside both roots and read as resolved — and a path the lane did not
// write is not evidence the lane produced, however real it is (#612, and the
// cycle-1676 audit's L1).
```

### `go/internal/coherence/crossartifact_test.go:3` — above `import (`

```text
// crossartifact_test.go — the RED contract for cycle-1676 inbox item
// `crossartifact-invariant-stack` (triage `## top_n`, weight 0.85).
//
// WHAT IS BEING BUILT. One deterministic, per-cycle aggregate over a cycle
// workspace that evaluates the four weak verifiers the inbox record names, and
// reports each one's evidence independently:
//
//	verdict-agreement       embedded <!-- evolve-verdict --> sentinel == the
//	                        standalone acs-verdict.json verdict
//	test-count-agreement    the claimed counts in acs-verdict.json == the
//	                        counts independently recounted from its parsed
//	                        runner results (cycle-1673 M1 shipped a document
//	                        claiming "zero red predicates" while the artifact
//	                        beside it recorded red_count=3)
//	referenced-paths-exist  every evidence_path the audit sentinel cites
//	                        resolves on disk under the workspace or the LANE
//	                        worktree
//	provenance-phase-order  phase-timing.json's recorded chain runs forward and
//	                        respects the scout→tdd→build→audit floor
//
// Weaver (arXiv:2506.18203): a stack of weak deterministic verifiers approaches
// strong-verifier power at near-zero cost and is immune to LLM-judge bias. The
// aggregate COMPLEMENTS the adversarial audit; it never replaces it.
//
// THREE STATUSES, NOT TWO. `indeterminate` is the load-bearing one: an absent
// or malformed artifact can never read as a verified match (that is how a
// presence-only implementation games this suite), and it is equally never a
// violation (that is how an advisory check earns a false-positive rate and gets
// switched off). Every invariant fails SAFE into it.
//
// ADVISORY. Per the inbox record's own rule ("each invariant ships ADVISORY
// until its false-positive rate is evidenced ~0 — the 1054/1060 breaker
// lesson"), InvariantReport.Advisory is true and NOTHING in this cycle may make
// a violation blocking. The no-blocking half is pinned at the real call site in
// internal/core (crossartifact_invariants_wiring_test.go).
//
// These tests are authored by the TDD engineer and are RED now — they do not
// compile until the aggregate exists, which is a valid RED per the
// compile-failure rule. The Builder makes them GREEN by adding production code
// ONLY and must NOT modify this file.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - NEGATIVE  : prose "PASS" without a sentinel must NOT satisfy
//     verdict-agreement (an arbitrary PASS string is not a verdict); a cited
//     path that exists nowhere under the two lane roots must be reported
//     missing BY NAME.
//   - EDGE/OOD  : empty workspace, truncated JSON, wrong-typed fields, an
//     empty results array, a one-entry timing log, timestamps that do not
//     parse — every one indeterminate, none a violation, none a panic.
//   - SEMANTIC  : four distinct behaviours with four distinct evidence
//     strings, plus determinism (same workspace ⇒ byte-identical report).
//
// Reachability probe (cycle-644 rule): the aggregate lives in THIS package, and
// `go list -deps ./internal/phasetiming ./internal/acssuite` contains no edge
// back to internal/coherence — so reading phase-timing.json through
// phasetiming.Read is buildable from here. No import is pinned by these tests;
// only behaviour is.
```

### `go/internal/coherence/crossartifact_test.go:292` — above `func TestCrossArtifactInvariants_TestCountDisagreementNamesClaimedAndCounted(t *testing.T) {`

```text
// TestCrossArtifactInvariants_TestCountDisagreementNamesClaimedAndCounted —
// invariant 2, the cycle-1673 M1 shape: the artifact claims zero reds while its
// own parsed runner results carry one. Violated, with both numbers in evidence.
```

### `go/internal/coherence/crossartifact_test.go:309` — above `func TestCrossArtifactInvariants_TestCountTotalDisagreementIsViolated(t *testing.T) {`

```text
// TestCrossArtifactInvariants_TestCountTotalDisagreementIsViolated — the second
// count shape: the suite's claimed total does not match the number of results
// it actually carries (a truncated or padded runner parse). Verified on three
// real artifacts (cycles 1659/1666/1673: total == len(results), and the three
// claimed counts equal the recount) before being pinned — the advisory's
// measured false-positive rate on real data is 0/3.
```

### `go/internal/coherence/crossartifact_test.go:365` — above `func TestCrossArtifactInvariants_EscapingReferencedPathIsViolatedNotResolved(t *testing.T) {`

```text
// TestCrossArtifactInvariants_EscapingReferencedPathIsViolatedNotResolved is the
// cycle-1676 audit's L1, as a test: a citation that climbs out of both roots with
// enough "../" segments lands, after filepath.Join's cleaning, on a file that
// really exists — /etc/hosts here — and a containment-free implementation reports
// the invariant OK. A path outside the tree the lane wrote is never evidence the
// lane produced, so the only sound status is violated.
```

### `go/internal/coherence/crossartifact_test.go:540` — above `func TestCrossArtifactInvariants_ExistingVerdictCoherenceIsUntouched(t *testing.T) {`

```text
// TestCrossArtifactInvariants_ExistingVerdictCoherenceIsUntouched — AC4's
// no-regression half at the unit level: the ADR-0072 leaf this package already
// owns keeps its exact behaviour while the aggregate is added beside it.
```
