# Comment history: `internal/phases/ship`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/phases/ship/apicover_named_test.go:3` — above `package ship`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - const ExitMissingBin (native.go)
//   - func NewWithDefaultRunnerStage (ship.go)
//   - method Class.IsValid (native.go)
//   - func PluginVersion (statefile.go)
//
// Each test asserts a real contract (Rule 9), not a no-op reference.
```

### `go/internal/phases/ship/audit.go:37` — above `RunID           string 'json:"run_id,omitempty"'`

```text
// ADR-0049 S4 / G5: run-scope the binding lookup
```

### `go/internal/phases/ship/audit.go:92` — above `if entry.WorktreeTreeSHA != "" {`

```text
// Extract audit_bound_tree_sha for the gitops pre/post-merge tree-drift check.
// Source priority: the orchestrator's ledger binding entry (WorktreeTreeSHA =
// the worktree CHANGES tree it will commit) WINS over the auditor's report
// comment, because the auditor persona binds HEAD^{tree} = the unchanged base
// (the cycle's changes are uncommitted in the worktree at audit time), which
// can never equal the changes-commit tree → INTEGRITY_TREE_DRIFT every cycle
// (cycle-152). The report comment is the fallback for the non-worktree flow.
```

### `go/internal/phases/ship/audit.go:122` — above `carried, cfErr := tryTrivialRebaseCarryForward(ctx, opts, res, ledgerPath, entry, currentHEAD)`

```text
// Trivial-rebase carry-forward (merge ladder RUNG 0, cycle-786): a
// valid composition-verdict entry — unchanged patch-id, green
// composed-tree gates — lets the audit verdict follow the change
// across a clean rebase. Any rejected condition falls back here.
// The composition entry binds the COMPOSED tree (tree_state_sha
// verified inside), so the base tree check below is superseded.
```

### `go/internal/phases/ship/audit.go:179` — above `func findLatestAudit(ledgerPath, runID string) (*auditEntry, error) {`

```text
// findLatestAudit returns the auditor ledger entry ship binds to, walking
// ledger.jsonl backwards for the most recent agent_subprocess entry with
// role=auditor. When runID is set, ONLY an entry stamped with THIS run may
// bind (ADR-0049 S4 / gap G5); a miss is a hard integrity stop, never a
// fallback. The pre-2026-08-26 cross-run fallback was cycle-1571's H3
// fail-open hole: a FAILed cycle (which then emitted no auditor entry) bound
// a sibling lane's audit — surfacing as AUDIT_BINDING_HEAD_MOVED instead of
// this run's FAIL, and, when the sibling shares HEAD, capable of SHIPPING a
// FAILed cycle on the sibling's PASS. Same failure shape as the 2026-05-29
// "ancient bash-era auditor entry" incident, closed at the consumer this time.
// Standalone runID=="" keeps binding the latest auditor entry overall.
//
// Missing/empty ledger → IntegrityError. Found-but-no-auditor →
// IntegrityError. Any unmarshal error on a candidate line is treated as
// "not an auditor entry" (forward-compat: alien lines should not crash
// ship-gate).
```

### `go/internal/phases/ship/audit.go:249` — above `if s, ok := phasecontract.ParseVerdictSentinelFull(body); ok && s.Phase == string(core.PhaseAudit) {`

```text
// ADR-0050 §3.10 Slice 6: sentinel-first at enforce. The machine-readable
// verdict is authoritative and single-valued, so the prose regex below
// (which can match multiple verdict words and trip the dual-verdict guard at
// audit.go) is gated off. No usable sentinel → all false →
// CodeAuditBindingMalformed, i.e. the sentinel becomes mandatory.
//
// The sentinel MUST be the audit phase's own: this is a ship gate, so a
// foreign-phase sentinel (e.g. a build-report sentinel quoted into the
// audit artifact) must not be allowed to satisfy it. ParseVerdictSentinelFull
// surfaces the phase field; only an exact "audit" phase is trusted. SKIPPED
// and any out-of-vocab verdict also fall through to all-false (malformed).
```

### `go/internal/phases/ship/audit.go:292` — above `var headingVerdictRe = map[string]*regexp.Regexp{`

```text
// headingVerdictRe matches the `## Verdict` heading followed, within 5
// lines, by either `**X**` (bash awk window parity) or a BARE verdict line
// (exactly `X` — the cycle-249 shape; a sentence containing the word must
// not match).
```

### `go/internal/phases/ship/audit_bound_witness_integration_test.go:3` — above `package ship`

```text
// audit_bound_witness_integration_test.go — cycle-585 RED contract for
// preventiveAction #2 of the cycle-583 audit finding: "any post-audit tree
// mutation MUST be covered by a test that exercises the REAL push-integrity
// guard." Complements the static scan in audit_bound_witness_test.go, which
// pins WHO may write the field; this test pins WHAT HAPPENS when the value
// it holds doesn't match what was actually pushed to main — the exact
// failure mode a rebind (like the one cycle-583 rejected) would produce.
```

### `go/internal/phases/ship/audit_bound_witness_test.go:1` — above `package ship`

```text
// audit_bound_witness_test.go — cycle-585 regression guard for the
// cycle-583 audit finding: a "helpful" rebind of opts.internalAuditBoundTreeSHA
// anywhere outside audit.go disarms the post-push integrity guard
// (gitops.go:493-497) and the ship-binding.json sidecar (gitops.go:528).
//
// Two independent layers, matching preventiveAction #1/#2 of
// cycle-583-audit-bound-sha-rebind-disarms-integrity-guard.yaml:
//
//   - TestInternalAuditBoundTreeSHA_OnlyAssignedInAuditGo: a static source
//     scan (no build tag — no git needed) that fails if the field is EVER
//     assigned outside audit.go. This is the "can't quietly regress"
//     guardrail the cycle-583 audit asked for.
//   - TestShipFromWorktree_PostPushGuard_FiresOnRebind (integration-tagged,
//     below in a sibling file) exercises the REAL post-push guard end to
//     end and asserts it still fires CodeIntegrityTreeDrift when the field
//     holds a value that doesn't match what was actually pushed.
```

### `go/internal/phases/ship/audit_bound_witness_test.go:35` — above `func TestInternalAuditBoundTreeSHA_OnlyAssignedInAuditGo(t *testing.T) {`

```text
// TestInternalAuditBoundTreeSHA_OnlyAssignedInAuditGo is the cycle-583
// regression guard: it source-scans every non-test .go file in this package
// EXCEPT audit.go for an assignment to internalAuditBoundTreeSHA. The
// cycle-583 incident introduced exactly this shape (a rebind in the ship
// dispatch path to the post-merge tree) paired with a sound push-race fix;
// both the adversarial review and the Auditor caught it, but nothing
// mechanical did. This test is that missing mechanical check: it fails RED
// the moment a second assignment site appears anywhere in the package.
```

### `go/internal/phases/ship/audit_gaps_test.go:34` — above `func TestVerifyAuditBinding_DualVerdict_PASS_and_FAIL_IntegrityError(t *testing.T) {`

```text
// TestVerifyAuditBinding_DualVerdict_PASS_and_FAIL_IntegrityError: an
// audit report that declares BOTH "Verdict: PASS" and "Verdict: FAIL"
// is an inconsistent artifact. Ship must refuse it with IntegrityError
// (v8.30.0 dual-verdict detection).
```

### `go/internal/phases/ship/audit_gaps_test.go:153` — above `func TestVerifyAuditBinding_LegacyEntryNoGitHead_IntegrityError(t *testing.T) {`

```text
// TestVerifyAuditBinding_LegacyEntryNoGitHead_IntegrityError: an auditor
// ledger entry without git_head/tree_state_sha (pre-v8.13.0) must block ship
// with a "predates v8.13.0 cycle-binding" message.
```

### `go/internal/phases/ship/audit_gaps_test.go:265` — above `func TestParseVerdicts_BareHeadingLine(t *testing.T) {`

```text
// TestParseVerdicts_BareHeadingLine: the heading form must also accept a BARE
// verdict line (`## Verdict` + `PASS` without bold) — the cycle-249 shape that
// blocked the v16.8.0 release preflight and would equally have produced a
// false AUDIT_BINDING_MALFORMED_VERDICT here. The bare line must be exactly
// the verdict word (a sentence containing PASS must NOT match).
```

### `go/internal/phases/ship/binary_staging_guard.go:1` — above `package ship`

```text
// binary_staging_guard.go — a staging-time backstop against accidental
// compiled-binary commits (tracked-binary-in-acs-dir).
//
// Root cause (ship 0405658a): an ACS predicate under go/acs/cycle536/ ran
// `go build` without `-o os.DevNull`, dropping an ~18MB `evolve` binary into
// the worktree that `git add -A` then swept into history. `.gitignore` closes
// the known instance; this guard closes the CLASS by refusing to commit any
// staged oversized executable outside the two legitimate committed-binary
// locations (go/bin/** and go/evolve).
```

### `go/internal/phases/ship/binary_staging_guard_test.go:43` — above `func TestBinaryStagingGuard_RejectsLargeExecutableOutsideAllowlist(t *testing.T) {`

```text
// TestBinaryStagingGuard_RejectsLargeExecutableOutsideAllowlist is the RED
// anchor for tracked-binary-in-acs-dir (ship 0405658a: an 18MB
// go/acs/cycle536/evolve landed in git history via a `go build` ACS
// predicate lacking `-o os.DevNull`). A staged executable over 1MB outside
// the go/bin/ and go/evolve allowlist must fail the guard with an actionable
// message naming the offending path.
```

### `go/internal/phases/ship/closure_idempotency_test.go:3` — above `package ship`

```text
// closure_idempotency_test.go — cycle-234 task `ship-closure-idempotency` (RED).
//
// Three defects from the cycle-233 landing saga (inbox
// 2026-06-06T03-27-08Z-ship-closure.json) made a fully-successful push
// batch-fatal:
//
//	D1: a correction re-dispatch AFTER the push re-ran the FULL ship →
//	    AUDIT_BINDING_HEAD_MOVED dead-end (HEAD is the ship's own commit).
//	D2: `git -C <worktree> add -A` swept unaudited go/evolve binary churn
//	    into the cycle commit (build's recoverBuildLeak pattern not applied
//	    in ship) → audit AC5 silent-pass broken.
//	D3: expected_ship_sha pinned from a PRE-commit blob, not the binary as
//	    committed at HEAD → SELF_SHA_TAMPERED next cycle, hand-corrected
//	    twice via state.json delete.
//
// All tests use the real-git fixtures from native_test.go / worktree_test.go
// (makeRepo, addRemote, makeWorktree, seedAudit, runShip). They are
// BEHAVIORAL: they assert on git history, remote refs and state.json — not
// on log strings alone.
```

### `go/internal/phases/ship/commitgate_test.go:140` — above `func writePersonaFixture(t *testing.T, root, name string, personaTools, allowedTools []string) {`

```text
// --- persona lint (cycle-241, migration step 5: commitgate-persona-lint) ---
//
// runPersonaLint wires phasecoherence.Check + CheckArtifactNames into the
// ship gate for BOTH --class manual and --class cycle, so persona↔profile
// drift cannot silently enter the commit chain. Contract pinned here:
//
//   - layout: agents/ under opts.ProjectRoot; profiles under
//     <ProjectRoot>/.evolve/profiles (the phasesCheckCoherence default).
//   - Kind "disallowed" (persona declares a tool its profile forbids — a
//     contradiction) BLOCKS with *IntegrityError. A persona lying about its
//     capabilities is an integrity breach, never acceptable drift.
//   - Kind "undeclared"/artifact-name "mismatch" (profile allows more than
//     the persona declares) LOGS loudly but does NOT block: the real repo
//     carries ~40 such WARNs today (`evolve phases check-coherence`);
//     blocking on them would brick every ship including this cycle's own.
//   - missing agents/ or profiles dir → skip with a log (repos without
//     personas — including every other ship test fixture — are unaffected).
//   - BypassCommitGate=true → lint skipped (consistent with the attestation
//     bypass; routine use is a policy violation).
//
// NOTE for Builder: the real repo has 7 "disallowed" contradictions across
// doc-sync/intent/scout personas. Those persona/profile pairs MUST be
// reconciled in this cycle or the new gate blocks our own --class cycle ship.
```

### `go/internal/phases/ship/composition.go:1` — above `package ship`

```text
// composition.go — trivial-rebase audit carry-forward (merge ladder RUNG 0,
// cycle-786; knowledge-base/research/merge-concurrency-2026).
//
// Review verdicts follow the CHANGE (git patch-id), gates follow the TREE.
// When git HEAD moved after the audit only because the lane was rebased
// conflict-free onto a moved main (patch-id unchanged) and the full native
// gate set re-ran green on the composed tree, the audit verdict carries
// forward via a composition-verdict ledger entry instead of hard-failing
// CodeAuditBindingHeadMoved into a full re-audit — the Gerrit
// `copyCondition: TRIVIAL_REBASE` precedent. Every rejected condition falls
// back to the pre-existing full re-audit path; the fast path can only
// narrow, never widen, what ships.
```

### `go/internal/phases/ship/composition.go:39` — above `func tryTrivialRebaseCarryForward(ctx context.Context, opts *Options, res *RunResult, ledgerPath string, audit *auditEnt…`

```text
// tryTrivialRebaseCarryForward reports whether a valid composition-verdict
// entry lets the bound audit carry forward to currentHEAD:
//
//  1. entry chains the bound auditor entry (lane_audit_ref) to currentHEAD
//     and records the audited base the audit actually bound;
//  2. the full native gate set re-ran green on the composed tree
//     (ciparity.RequiredComposedGates — gates follow the tree, ADR-0069);
//  3. the composed tree ship sees NOW is the one the entry's gates ran on;
//  4. live kernel recompute: `git diff HEAD | git patch-id --stable` still
//     equals the audited patch_id — semantic drift, however textually
//     clean, falls back to full re-audit.
//
// false = fall back to the pre-existing CodeAuditBindingHeadMoved error.
// A non-nil error is reserved for I/O failures while recomputing the
// composed tree, which must surface as themselves rather than as HeadMoved.
```

### `go/internal/phases/ship/consume.go:35` — above `func consumeCommittedItems(ctx context.Context, opts *Options, res *RunResult, dir string) {`

```text
// consumeCommittedItems moves this cycle's committed inbox items into the
// tracked consumed/ dir INSIDE the ship tree and stages the moves, so they
// ride the ship commit. Gated on the verdict string PASS — the sole
// authority, shared with postship's landedPASS gate. Both writers derive it
// from their own counts (acssuite: red_count==0; acsrunner: red_count==0 AND
// incomplete_count==0), so red_count alone is weaker: acsrunner writes
// red_count:0 beside verdict FAIL for a suite that never finished. On the
// cycle path acssuite.ReadVerdict already refuses a non-PASS verdict with
// red_count:0 before this runs; the manual path has no such reader, so this
// gate is its only guard (warn-ship-consumption-gap, cycle-1691). Every step
// is per-item fail-open and LOUD: a consumption problem must never block a
// ship that already earned its verdict.
```

### `go/internal/phases/ship/consume.go:128` — above `bound, boundOK := readBindingForConsume(opts, res, id)`

```text
// Transactional retire, registry half (park-consume-releases-continuation-
// binding): consumption takes the item out of the batch loader's reach, so
// its scope-keyed continuation binding must go too — otherwise the next
// wave mints a lane straight off the immortal binding (cycles 1487, 1497).
// The pointer is PRESERVED into the consumed item here; the registry delete
// happens only after the move is staged, so a rollback below leaves both
// stores exactly as they were.
```

### `go/internal/phases/ship/consume.go:263` — above `args = append(args, rawPathRead("diff-tree", "-r", "--name-only", boundTree, actualTree)...)`

```text
// rawPathRead/unquoteGitPath (cycle-1108): without them a non-ASCII byte in
// an item filename comes back C-quoted, never matches the sanctioned set,
// and false-refuses a legitimate consumption ship (review M4).
```

### `go/internal/phases/ship/consume_binding_test.go:5` — above `import (`

```text
// consume_binding_test.go — RED contract for the cycle-1506 pipeline-blocker
// halt (batch-20260817b): #466's in-commit consumption mutates the staged tree
// AFTER the audit bound it, so the pre-commit tree-drift integrity check
// refused EVERY PASS ship of an inbox-claimed item — two individually-correct
// mechanisms, jointly contradictory. (#466's own tests passed because they set
// no audit binding, and the check self-skips.) The fix: a drift whose tree
// delta consists EXACTLY of the sanctioned consumption moves is accepted with
// a loud log; any other path in the delta still refuses. The integrity
// guarantee is not weakened — it is taught about the one mutation the ship
// itself performs by design.
```

### `go/internal/phases/ship/consume_binding_test.go:42` — above `func TestShipFromWorktree_ConsumptionDriftIsSanctioned(t *testing.T) {`

```text
// The cycle-1506 shape: audit binding set to the pre-consumption tree; the
// ship consumes its item; the resulting drift is exactly the consumption move
// and MUST be accepted.
```

### `go/internal/phases/ship/consume_gate_authority_test.go:11` — above `func TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption(t *testing.T) {`

```text
// TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption (cycle-1691
// audit H2) pins why the verdict-string consumption gate is sufficient on the
// cycle path: the WARN + red_count:0 quadrant from the 2026-08-16 inbox item
// cannot pass the EGPS reader that verifyClass → verifyPredicateReceipt runs
// before atomicShip, so it never reaches consumeCommittedItems. The PASS twin
// of the same record is the positive control — the refusal is about the
// verdict, not a malformed fixture. If ReadVerdict ever tolerates WARN again,
// this goes red and the consumption gate must be revisited with it.
```

### `go/internal/phases/ship/consume_integration_test.go:3` — above `package ship`

```text
// consume_integration_test.go — transactional inbox consumption (the re-pick
// killer, consumption-rides-landing-ship 0.92; three live burns: cycle-1448,
// cycle-1464, cycle-1471). The defect: PASS promotion moves items to the
// GITIGNORED processed/ on the runtime plane AFTER the commit, so main keeps
// the tracked item and every fresh lane worktree re-picks it. The contract:
// the PASS ship commit ITSELF carries the consumption — tracked root deletion
// plus a tracked consumed/ record — so main stops offering the item the
// moment the work lands.
```

### `go/internal/phases/ship/consume_integration_test.go:88` — above `func TestShipFromWorktree_WarnVerdictDoesNotConsume(t *testing.T) {`

```text
// Consumption is authorized by the verdict string PASS alone — never WARN.
// Neither verdict writer (acssuite, acsrunner) emits WARN, and on the cycle
// path acssuite.ReadVerdict refuses a WARN + red_count:0 artifact before
// consumption can run (pinned by
// TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption). A WARN file
// is therefore unknown evidence, and unknown evidence must leave the item
// pickable (warn-ship-consumption-gap, cycle-1691 audit H1/H2).
```

### `go/internal/phases/ship/consume_integration_test.go:249` — above `repo, wt, ws, itemRel := consumeScenarioWith(t, func(ws string) {`

```text
// soak-20260824a wave-2 burn: 1552's triage put the fleet-scope id in
// dropped[] with top_n:[], build shipped the item's implementation
// anyway (df322f6c), consumption resolved zero ids, and the stale item
// cost the next wave a full lane re-proving finished work. A dropped
// ASSIGNED id is an affirmative close and must retire in-commit.
```

### `go/internal/phases/ship/consume_integration_test.go:314` — above `func TestManualShip_NonShippableVerdictKeepsItemPickable(t *testing.T) {`

```text
// --- Which acs-verdict.json may retire an inbox item in the landing commit
// (warn-ship-consumption-gap, cycle-1691 audit H1/M1). The verdict string
// PASS is the only authority: both writers derive it from their own counts
// (acssuite: red_count==0; acsrunner: red_count==0 AND incomplete_count==0),
// so red_count alone is a weaker key — acsrunner writes red_count:0 beside
// verdict FAIL for a suite that never finished.
```

### `go/internal/phases/ship/consume_lanescope_union_test.go:3` — above `import (`

```text
// consume_lanescope_union_test.go — consumption-id-linkage-lane-scope-union
// (0.86). Two live burns of one class: triage's bookkeeping defeats the #466
// in-commit consumption on exactly the lanes that matter. Cycle-1515: triage
// DECOMPOSED the assigned id into three sub-ids, so top_n named none of the
// inbox files. Cycle-1552 (soak-20260824a wave 2's burn): triage DROPPED the
// assigned id as "already-shipped" with top_n:[], build shipped the item's
// implementation anyway (df322f6c), consumption resolved zero ids, and the
// stale item cost wave 2 a full lane re-proving finished work. The contract:
// a PASS lane ship retires its ASSIGNED scope ids regardless of triage's
// renaming/decomposition/drop — the one exception is an id triage EXPLICITLY
// deferred, which stays pickable (its remainder rides carryover).
```

### `go/internal/phases/ship/consume_lanescope_union_test.go:29` — above `func TestCommittedInboxIDs_UnionsLaneScopeWhenTriageDroppedTheScope(t *testing.T) {`

```text
// The cycle-1552 shape: scope id dropped by triage, top_n empty — the id must
// still resolve for consumption.
```

### `go/internal/phases/ship/consume_lanescope_union_test.go:42` — above `func TestCommittedInboxIDs_UnionsLaneScopeWithDecomposedTopN(t *testing.T) {`

```text
// The cycle-1515 shape: triage decomposed into sub-ids; top_n names things
// that are not inbox files. The scope id joins the set (the sub-ids stay too —
// FindFileByTaskID misses them harmlessly).
```

### `go/internal/phases/ship/consume_paths_witness_test.go:3` — above `import (`

```text
// consume_paths_witness_test.go — single-writer witness for
// internalConsumedPaths (review M2 on the cycle-1506 fix), mirroring the
// cycle-583 pattern in audit_bound_witness_test.go: the drift-tolerance's
// sanctioned set is a smuggling channel the moment any writer other than
// consumeCommittedItems appends to it, and nothing but a mechanical scan
// resists that drift.
```

### `go/internal/phases/ship/consume_registry_release_test.go:3` — above `import (`

```text
// consume_registry_release_test.go — the ship-side wiring proof for the
// transactional retire (park-consume-releases-continuation-binding). Consuming
// an item takes it out of the batch loader's reach, so its scope-keyed
// continuation binding must go in the SAME operation; otherwise the next wave
// mints a lane straight off the immortal binding (cycles 1487, 1497). The
// pointer must survive the release, and an unrelated live lane's binding must
// not be touched.
```

### `go/internal/phases/ship/coverage_final_test.go:423` — above `mustWrite(t, filepath.Join(root, ".evolve", "runs"), "i am a file\n")`

```text
// Make .evolve/runs a regular file so MkdirAll for cycle-77 fails.
```

### `go/internal/phases/ship/coverage_raise_test.go:172` — above `func TestVerifyAuditBinding_WorktreeTreeSHA_TakesPriority(t *testing.T) {`

```text
// TestVerifyAuditBinding_WorktreeTreeSHA_TakesPriority pins that when the
// ledger auditor entry carries worktree_tree_sha, it is the bound tree SHA
// (the changes-commit tree), overriding any report-comment value. This is
// the cycle-152 fix that prevents INTEGRITY_TREE_DRIFT on every worktree cycle.
```

### `go/internal/phases/ship/coverage_raise_test.go:311` — above `func TestPhaseRun_DefaultCommitMessage_WhenContextMissing(t *testing.T) {`

```text
// TestPhaseRun_DefaultCommitMessage_WhenContextMissing pins that an absent
// Context["commit_message"] is backfilled with the synthesized cycle
// message (cycle-150 fix) so the ship still proceeds end-to-end. We assert
// the synthesized message reached the commit by reading HEAD's subject.
```

### `go/internal/phases/ship/ebadf_retry_test.go:5` — above `import (`

```text
// RED-phase contract for cycle-249 task `macos-ebadf-test-hardening`
// (inbox: macos-ci-ebadf-flake-hardening).
//
// TestShipFromWorktree_GitAddFails_Errors flakes on macos-latest CI with
// `read |0: bad file descriptor` — a darwin pipe-teardown race in the
// test git-runner's CombinedOutput path. The mitigation is TEST-INFRA
// ONLY: a capture helper that retries exactly once when the error chain
// contains syscall.EBADF or io.ErrClosedPipe, used by runGit/runGitOut.
//
// Contract (to be implemented in a _test.go helper file — production
// ship/ files must NOT change):
//
//	func captureWithEBADFRetry(run func() ([]byte, error)) ([]byte, error)
//
// Fails at baseline: captureWithEBADFRetry is undefined (compile RED).
```

### `go/internal/phases/ship/error_paths_test.go:49` — above `if err == nil || !strings.Contains(err.Error(), "git add failed") {`

```text
// cycle-1067: the message lost its `-A` with the switch to explicit-path
// staging (stageExplicitPaths); the failure branch it pins is unchanged.
```

### `go/internal/phases/ship/final_gaps_test.go:3` — above `package ship`

```text
// final_gaps_test.go — last achievable coverage gaps after 92.5%:
//
//   - verifySelfSHA: sha256File error on unreadable binary (verify.go:65)
//   - verifySelfSHA: repin writeStateMap error (verify.go:81)
//   - verifyManualConfirm: diff --cached --quiet runner error (verify.go:161)
//   - verifyTrivial: diff --cached --name-only runner error (verify.go:232)
//   - verifyAuditBinding: sha256File error on unreadable artifact (audit.go:66)
//   - verifyAuditBinding: os.ReadFile error on unreadable artifact (audit.go:77)
//   - verifyAuditBinding: rev-parse HEAD runner error (audit.go:131)
//   - verifyAuditBinding: computeTreeStateSHA runner error (audit.go:141)
//   - verifyAuditBinding: os.Stat freshness error (audit.go:152)
//   - shipDirect: runCommitPrefixGate error (gitops.go:103)
//   - shipFromWorktree: runCommitPrefixGate error (gitops.go:186)
//   - shipFromWorktree: git commit runner failure (gitops.go:195)
//   - shipFromWorktree: write-tree error + empty-output fail-closed (ADR-0048 C1)
//   - repinPostCycle: readStateMap error (postship.go:167)
//   - repinPostCycle: writeStateMap error (postship.go:188)
```

### `go/internal/phases/ship/final_gaps_test.go:91` — above `var se *core.ShipError`

```text
// With the ADR-0049 S2 shared state.json lock, a read-only .evolve dir now
// fails at lock-acquire (the .lock file can't be created) BEFORE the write —
// both are the same fail-safe STATE_IO refusal on an unwritable state dir.
// Assert the contract (a STATE_IO refusal, not silent pass), not the site.
```

### `go/internal/phases/ship/final_gaps_test.go:314` — above `func TestShipFromWorktree_WriteTreeFails_Errors(t *testing.T) {`

```text
// --- shipFromWorktree: write-tree error in the pre-commit binding check -----
// ADR-0048 Slice C1 moved the audit-bound tree-SHA verification to a
// `git write-tree` on the staged index BEFORE the commit. A write-tree failure
// must propagate (the binding cannot be verified, so ship must not advance).
```

### `go/internal/phases/ship/final_gaps_test.go:346` — above `func TestShipFromWorktree_WriteTreeEmptyOutput_FailsClosed(t *testing.T) {`

```text
// TestShipFromWorktree_WriteTreeEmptyOutput_FailsClosed: ADR-0048 Slice C1
// fail-closed posture — if `write-tree` returns exit 0 but EMPTY stdout, the
// audit binding cannot be verified, so ship must abort rather than commit
// unverified work (a set binding is never silently skipped).
```

### `go/internal/phases/ship/gitops.go:32` — above `func (o *Options) acquireShipLock() (release func(), err error) {`

```text
// acquireShipLock acquires the ADR-0049 S5 integrator lock (gap G1): the
// BLOCKING flock serializing the shared-main integration critical section so
// two concurrent ships can't corrupt main's index/ref/origin. nil seam →
// flock.Lock on <ProjectRoot>/.evolve/ship.lock. No-op under the whole-cycle
// project lock (uncontended); load-bearing once that lock is scoped per-run.
```

### `go/internal/phases/ship/gitops.go:220` — above `func (o *Options) cycleStateFile() string {`

```text
// cycleStateFile returns the file ship reads run-defining inputs (active_worktree,
// cycle_id) from. It prefers the per-run run.json mirror under the run workspace
// (ADR-0049 S3 / gap G3) — a full cycle-state.json mirror (CB.4, only
// CycleState-modeled keys) — so a concurrent cycle's host-global cycle-state.json
// cannot make ship integrate the WRONG run's worktree/number. Falls back to the
// global file when WorkspacePath is unset (standalone `evolve ship`) or the
// mirror is absent. A no-op for the live loop: with one cycle running, run.json
// and the global file hold identical content. (cycle_size_estimate is NOT a
// CycleState field, so verifyTrivial — a standalone-only path — keeps reading the
// global file directly.)
```

### `go/internal/phases/ship/gitops.go:249` — above `if !opts.DryRun {`

```text
// ADR-0049 S5 / gap G1: the non-worktree ship path (manual ships, release
// ships, and any cycle ship without a live worktree) mutates
// opts.ProjectRoot's index directly via add -A → commit → push. Hold the
// integrator lock across that whole critical section so two concurrent
// ships that both land here serialize instead of racing main's
// index/ref/origin — the same guard shipFromWorktree already holds.
// BLOCKING flock; skipped on dry-run (mutates nothing). No-op under the
// whole-cycle project lock (uncontended).
```

### `go/internal/phases/ship/gitops.go:268` — above `if err := stageReleaseSet(ctx, opts); err != nil {`

```text
// Release staging is class-special (v18.3.0→v18.5.0 forensics):
// the pipeline's rebuild-binary step is the AUDITED producer of
// go/evolve, so the churn discard below would throw away the
// release's own product (→ SELF_SHA_TAMPERED on the next ship);
// and `add -A` swept untracked operator files (evolve.log,
// release-*.log) into release commits. Stage exactly the known
// release set instead.
```

### `go/internal/phases/ship/gitops.go:321` — above `if err := runCommitPrefixGate(ctx, opts, msg, opts.ProjectRoot); err != nil {`

```text
// Optional: commit-prefix-gate (Layer 1 of ADR-0012). Best-effort
// shellout to the bash gate when present; missing or non-executable
// is silently skipped to match bash behavior (`if [ -x ... ]`).
```

### `go/internal/phases/ship/gitops.go:330` — above `if !opts.DryRun && opts.internalAuditBoundTreeSHA != "" {`

```text
// Audit-binding check, pre-commit. This path verified NOTHING before, while
// still writing audit_bound_tree_sha into ship-binding.json — a sidecar
// asserting a verification that had not happened.
//
// The comparand is the STAGED tree, mirroring verifyStagedTree. Getting
// this wrong is easy and expensive, so the reasoning is recorded: the
// auditor persona computes `git rev-parse HEAD^{tree}` (evolve-auditor.md),
// which on an uncommitted cycle is the BASE tree — but that value cannot
// reach here. audit.go's report-comment fallback is taken only when the
// ledger entry has no worktree_tree_sha, and twenty lines later
// verifyAuditBinding refuses outright unless treefence.Take's tree equals
// that same empty value, which it never does. So the binding that reaches
// shipDirect is always entry.WorktreeTreeSHA — `git add -u` + `git
// write-tree`, the CHANGES tree (core/phase_bindings.go, the cycle-152
// fix). An intermediate version of this guard compared HEAD^{tree}
// instead, on the strength of a test that hand-set a binding no producer
// emits; it would have been inert where reachable and is the reason
// TestShipDirect_LegitimateBoundShipIsNotBlocked now drives the REAL
// producer rather than a literal.
```

### `go/internal/phases/ship/gitops.go:417` — above `func shipFromWorktree(ctx context.Context, opts *Options, res *RunResult, branch, worktree string) error {`

```text
// shipFromWorktree: the v8.43.0 worktree-aware path. Commit in the
// cycle's worktree (where Builder's edits live), pre-merge tree-SHA
// check, ff-merge cycle branch into main, push main, post-push
// integrity verification, ship-binding.json sidecar (the ff-merge, the push
// with its inline repair and the binding writer live in landing/ — ADR-0103
// unit 07; gitops_landing.go is the seam).
```

### `go/internal/phases/ship/gitops.go:478` — above `func runCommitPrefixGate(ctx context.Context, opts *Options, msg, repoDir string) error {`

```text
// runCommitPrefixGate calls the commitprefixgate Go library directly
// (v11.8.2+; prior versions shelled out to legacy/scripts/guards/
// commit-prefix-gate.sh). Missing manifest is silently passed through by
// the library (matches the bash "pass-through when not provisioned" rule).
```

### `go/internal/phases/ship/gitops.go:541` — above `func rawPathRead(args ...string) []string {`

```text
// rawPathRead prefixes a path-REPORTING git read with `-c core.quotePath=false`
// so git emits non-ASCII paths raw instead of octal-escaped — the zero-parsing
// half of the cycle-1108 fix, with unquoteGitPath (manifest.go) covering the
// residue that flag does not suppress (embedded quotes, backslashes, control
// chars, which stay C-quoted regardless). Config args are only legal BEFORE the
// subcommand, which is where this puts them (captureGitOutputAtDir's own -C is
// a global option too, so either order is accepted by git).
```

### `go/internal/phases/ship/gitops.go:552` — above `func stageExplicitPaths(ctx context.Context, opts *Options, res *RunResult, dir string) error {`

```text
// stageExplicitPaths stages a non-release ship (cycle/manual/trivial) as an
// explicit `git add -- <paths>` instead of the `git add -A` sweep both call
// sites used before cycle-1067 (`ship-stage-explicit-paths`). The pathspec is
// stagePathspec(declared manifest, porcelain changed set) — see its doc for the
// exact set and for why the fallbacks are the changed set and never `-A` nor
// nothing. dir is the tree to stage in: "" for opts.ProjectRoot (shipDirect),
// or the cycle worktree (shipFromWorktree).
//
// The `git add` call is issued even for an empty pathspec (git: "Nothing
// specified, nothing added.", rc=0) so the staging step stays observable and a
// clean tree keeps flowing into the staged-diff check that exits cleanly —
// staging is never silently skipped.
```

### `go/internal/phases/ship/gitops.go:610` — above `var errTail bytes.Buffer`

```text
// Tee stderr into a bounded buffer so the failure REASON travels in the
// ship error (failure digest, retro, escalation report) — cycle-1098's
// `fatal: Invalid path '/go'` was only visible in the lane log while the
// error said `git add failed (rc=128): <nil>`.
```

### `go/internal/phases/ship/gitops.go:621` — above `if offenders := ignoredPathsFromAddRefusal(errTail.String()); len(offenders) > 0 {`

```text
// Layer 4 of the staging onion (2026-08-14 halt, ship|unknown|99c38818):
// a pathspec the check-ignore probe is blind to can still be refused
// by add. The live blind shape (probed on the runtime repo, git
// 2.50.1): a directory whose rule sits under a NEGATED parent
// (`!.evolve/inbox/` re-include above `.evolve/inbox/processed/`) —
// check-ignore returns not-ignored for BOTH slash forms there, while
// a minimal `dir/` rule without the negation IS flagged (review
// probe). The fix is deliberately mechanism-independent: rather than
// re-implement ignore semantics a third time, trust git's own
// refusal — it NAMES the offending pathspecs in stderr. Drop exactly
// those, retry ONCE.
```

### `go/internal/phases/ship/gitops.go:643` — above `if len(retryPaths) > 0 && len(retryPaths) < len(paths) {`

```text
// Progress is mandatory, and so is having something left to stage:
// an offender list that filters nothing (form mismatch) falls
// through to the honest error below, and an ALL-ignored set stays
// on the two-strikes ladder (cycle-1365: refusal → strike →
// deterministic precondition → continuation/salvage) — a success
// that staged nothing would delete that routing.
```

### `go/internal/phases/ship/gitops.go:669` — above `class := core.ShipClassTransient`

```text
// Two-strikes-same-pathspec (deterministic-stage-refusal-router): the
// FIRST refusal keeps its retry (a genuinely flaky add must), but the
// SAME pathspec refused twice in a row cannot win in place — cycle-1365
// burned its whole retry budget re-adding one .evolve/evals path whose
// worktree base predated the .gitignore carve-out. Precondition routes
// it to continuation/salvage instead of another doomed attempt.
```

### `go/internal/phases/ship/gitops.go:694` — above `const stageRefusalMemoFile = "ship-stage-refusal.txt"`

```text
// dropIgnoredPaths removes pathspec entries git refuses to stage: a declared
// path matched by .gitignore makes `git add` exit 1 ("The following paths are
// ignored by one of your .gitignore files") even though it stages the other
// paths first. Cycle-1101: the eval-quality contract puts the cycle's
// `.evolve/evals/<slug>.md` in every test-report, .evolve/* is ignored BY
// DESIGN (runtime state, never committed), so every green cycle's declared
// manifest carried a refusal — a deterministic ship-killer. `check-ignore`
// rc 0 lists the ignored subset one-per-line; rc 1 means none (both are
// success for captureGitOutput). NOT `-z`: that flag is stdin-mode-only
// (`fatal: -z only makes sense with --stdin`, rc=128 — adversarial review
// caught the probe failing open on EVERY ship). Newline parsing stays safe
// (git never puts a bare newline in this output — it C-quotes such a path
// instead), but cycle-1108 falsified this comment's older premise that git
// never quotes here at all: the pathspec is the DECLARED manifest, whose
// entries are arbitrary repo paths, so a non-ASCII or quote-bearing one comes
// back C-quoted and matched nothing — the ignored path then rode into `git add`
// and reproduced the cycle-1101 rc=1 ship-killer. Hence the raw-path read plus
// unquoteGitPath on every probe line. A broken probe fails OPEN with the full
// set and a loud log: the probe must never block ship — if the refusal
// survives, the add's own stderr now travels in the ship error.
// stageRefusalMemoFile is where a lane remembers the pathspec its last `git
// add` refused. Workspace-scoped on purpose: fleet lanes run concurrently, so
// one lane's strike must never deterministically block a peer's first attempt.
```

### `go/internal/phases/ship/gitops.go:844` — above `var osExecutable = os.Executable`

```text
// discardBinaryChurn discards unaudited tracked-binary rebuild churn from the
// WORKTREE before `git add -A` stages the ship commit. It deliberately uses
// `git checkout -- <path>` (restore from INDEX), not `git checkout HEAD -- <path>`:
// after normalizeWorktreeToBase's `git reset --soft`, the index holds the full
// audited diff, so the index — not HEAD — is the audited reference. Restoring
// from the index discards exactly the post-audit worktree churn (e.g. an ACS
// rerun rebuilding go/evolve) while preserving an audited, intentionally staged
// binary update. Contrast with core.discardMainLeak, which runs mid-cycle on the
// MAIN tree where the cycle is not yet committed and HEAD is the audited
// reference — there `HEAD --` is correct. The two forms are not interchangeable.
// osExecutable is a test seam for the running-binary lookup (production =
// os.Executable). The churn discard must never delete the binary it is
// executing from — inbox ship-manual-deletes-running-binary, 2026-07-12
// incidents: `ship --class manual` resolved the untracked go/bin/evolve via
// this fallback and os.Remove'd it, degrading every kernel hook to the stale
// tracked fallback until rebuild.
```

### `go/internal/phases/ship/gitops.go:909` — above `if isRunningExecutable(absPath) {`

```text
// Untracked: remove the file — unless it is the currently-executing
// binary. An untracked go/bin/evolve is gitignored (go/.gitignore
// `/bin/`), so `git add -A` can never stage it; removing it has zero
// staging-hygiene value and kills the binary the kernel hooks and the
// rollback shellout are running (2026-07-12 incidents, cycle-243).
```

### `go/internal/phases/ship/gitops.go:927` — above `func ignoredPathsFromAddRefusal(stderr string) []string {`

```text
// ignoredPathsFromAddRefusal extracts the pathspecs git names in an add
// refusal ("The following paths are ignored by one of your .gitignore
// files:") — the lines between that header and the first hint:. Strict by
// design: no header means no offenders (a fuzzy parse of an unrelated failure
// would silently under-stage the ship). Quoted lines decode through
// unquoteGitPath (the cycle-1108 quotepath contract applies here too).
```

### `go/internal/phases/ship/gitops_binarychurn_test.go:1` — above `package ship`

```text
// gitops_binarychurn_test.go — RED contract for inbox defect
// ship-manual-deletes-running-binary (2026-07-13T08-45Z; live incidents
// 2026-07-12 19:5x and 23:53, cycle-243 precedent).
//
// Root cause: `evolve ship --class manual` routes through shipDirect, which
// calls discardBinaryChurn on the SUCCESS path before `git add -A`.
// cmd_ship.go never sets Options.ShipBinaryPath, so the churn discard falls
// back to os.Executable() — the running go/bin/evolve. That path is UNTRACKED
// (go/.gitignore `/bin/`), so the discard loop hits os.Remove and deletes the
// binary every PreToolUse kernel hook resolves first. `git add -A` can never
// stage a gitignored file, so the removal had zero staging-hygiene value —
// pure harm. Rollback shells `evolve ship --class manual`
// (rollback.defaultRevertAndShip), so the deletion fired there transitively;
// the same shipDirect guard covers that path.
//
// Contract pinned here:
//  1. discardBinaryChurn must NEVER remove the currently-executing binary
//     (skip + WARN), regardless of how its path was resolved.
//  2. Manual-class shipDirect success leaves an untracked go/bin/evolve on
//     disk while KEEPING the untracked-go/evolve removal (the discard's actual
//     purpose — see TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll).
```

### `go/internal/phases/ship/gitops_binarychurn_test.go:59` — above `ShipBinaryPath: "",`

```text
// cmd_ship.go:79-90 never sets it — the incident path
```

### `go/internal/phases/ship/gitops_binarychurn_test.go:73` — above `func TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent(t *testing.T) {`

```text
// TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent — the incident
// end-to-end at the shipDirect level (the exact function the rollback
// shellout's `evolve ship --class manual` re-enters): a successful manual
// ship keeps the running go/bin/evolve, while the untracked tracked-path
// go/evolve churn is still discarded (guard is narrow, not a behavior revert).
```

### `go/internal/phases/ship/gitops_collider_quotepath_test.go:1` — above `package ship`

```text
// gitops_collider_quotepath_test.go — RED contract for cycle-1469 top_n task
// `gitstage-collider-quotepath` (the collider-side hole left by cycle-1108).
//
// Cycle-1108 taught every path-classifying git reader in this package the
// quote-path contract — `-c core.quotePath=false` on the read (rawPathRead,
// gitops.go) plus unquoteGitPath (manifest.go) for the residue that flag does
// not suppress. detectColliders (gitops.go:77) was never enrolled. It reads
// BOTH `git diff --name-only` and `git status --porcelain` with a bare
// captureGitOutputAtDir, and its only decoding is a naive strip of the wrapping
// quotes:
//
//	if strings.HasPrefix(path, "\"") && strings.HasSuffix(path, "\"") {
//	    path = path[1 : len(path)-1]
//	}
//
// Verified against real git 2.50.1 (2026-08-15, `git status --porcelain` on a
// tree holding all three input classes):
//
//	?? "caf\303\251.txt"       # non-ASCII → octal-escaped, quoted
//	?? "we\"ird.txt"           # embedded quote → backslash-escaped, quoted
//	?? "we -> ird.txt"         # space → quoted, NOT escaped
//
// So the strip yields the 15-byte literal `caf\303\251.txt` and the 11-byte
// literal `we\"ird.txt` — paths that exist on no disk. detectColliders then
// os.Stat()s them, gets ENOENT, and `continue`s: the real untracked main-side
// collider is INVISIBLE to both the pre-flight and the repair ladder, and the
// ff-merge hits the file git refuses to overwrite. The `git diff --name-only`
// stream is worse — it gets no decoding at all, not even the strip.
//
// Contract pinned here:
//  1. A quoted porcelain entry (non-ASCII, embedded quote, backslash) is
//     compared as its LITERAL repo-relative path, so a real collider is found.
//  2. A quoted `git diff --name-only` entry decodes the same way.
//  3. Negative/anti-no-op: a path that is not a collider — absent on the main
//     side, or present but TRACKED there — is never reported, and a deleted
//     porcelain entry stays skipped.
//  4. Both classification reads carry `-c core.quotePath=false` BEFORE the
//     subcommand, the zero-parsing half of the same contract every other reader
//     in this package already honours.
```

### `go/internal/phases/ship/gitops_collider_test.go:3` — above `package ship`

```text
// gitops_collider_test.go — collider pre-flight false-positive guard.
//
// History: cycle-232 (retro I-10) added the collider PRE-FLIGHT that refuses
// before the worktree commit when untracked main-side files would be
// overwritten by the ff-merge. The repair ladder (ADR-0039 §8,
// operator-approved 2026-06-07) SUPERSEDED the refuse-and-stop contract:
// colliders are now self-healed in-ship — byte-identical copies removed,
// differing copies quarantine-moved — and the merge proceeds. Those behavior
// contracts live in repair_colliders_test.go.
//
// What remains here is the false-positive guard: untracked main-side files
// that do NOT collide with an incoming path must never trip the pre-flight
// (and must never be touched by the repair).
```

### `go/internal/phases/ship/gitops_landing.go:3` — above `import (`

```text
// gitops_landing.go — the ADR-0103 unit-07 seam between the ship phase and
// the landing leaf (internal/phases/ship/landing): the landing's ONE wired
// construction, the push step's projection of the host-owned once-guard, and
// the Strangler Fig facades (writeShipBinding, isAncestor, captureGitOutput)
// the ship paths, the resume repair and the by-name tests keep.
```

### `go/internal/phases/ship/gitops_landing_test.go:3` — above `import (`

```text
// gitops_landing_test.go — ADR-0103 unit 07: the seam between the ship phase
// and the landing leaf. One wired construction, the once-guard projected
// through pushWithRepair, the Center threaded from Config to Options, the
// happy path silent on the stream, the layout spelled once.
```

### `go/internal/phases/ship/gitops_landing_test.go:188` — above `func TestShipOptions_ThreadsSignals(t *testing.T) {`

```text
// Test 37 — shipOptions copies the Phase's Center into Options.Signals and
// (*Phase).signalsWired reports it (the cycle-1064 trap: a Config field the
// translation forgets is silently never wired).
```

### `go/internal/phases/ship/gitops_lock_test.go:3` — above `package ship`

```text
// gitops_lock_test.go — cycle 815 coverage for the shipDirect ship-lock gap
// (fleet-ship-git-index-lock-serialization). ADR-0049 S5's acquireShipLock
// (gap G1) was wired into shipFromWorktree only (see worktree_test.go's
// TestShipFromWorktree_AcquiresAndReleasesShipLock); shipDirect — the
// non-worktree path used by manual ships, release ships, and any cycle ship
// without a live worktree — ran git add -A / commit / push against
// opts.ProjectRoot's index with no lock acquisition anywhere. These tests
// pin AC1 (shipDirect acquires+releases opts.acquireShipLock() around its
// git-mutating section, non-dry-run) and AC2 (dry-run acquires zero locks),
// mirroring the injected shipLock seam pattern already used for
// shipFromWorktree.
```

### `go/internal/phases/ship/host_audit_roundtrip_integration_test.go:40` — above `wt := filepath.Join(t.TempDir(), "cycle-7")`

```text
// The cycle's worktree is what production provisions — a detached
// worktree of the repository — never the repository itself, which
// core refuses to stage or normalize (inPlaceWorktree, 2026-09-14).
```

### `go/internal/phases/ship/integrity.go:11` — above `func verifyNoControlPlaneEdits(ctx context.Context, opts *Options, res *RunResult) error {`

```text
// verifyNoControlPlaneEdits is the post-hoc backstop of the pipeline integrity
// boundary (ADR-0064). The real-time role-gate hook only intercepts Edit/Write;
// a phase could still mutate the control plane via Bash (sed -i, redirection) or
// any non-tool channel. This gate runs at the --class cycle ship chokepoint and
// rejects a commit whose diff touches the integrity surface (the deterministic
// gates, metric SSOT, guards, campaign contract, grading rubrics, hook wiring) —
// regardless of HOW the file was changed. A cycle may not edit the gate that
// grades it.
//
// It runs ONLY for ClassCycle (see verifyClass), so operator-driven control-plane
// changes shipped via `evolve ship --class manual` are exempt by construction —
// that is the sanctioned path for hardening a gate.
```

### `go/internal/phases/ship/integrity.go:46` — above `func cycleChangedPaths(ctx context.Context, opts *Options) ([]string, error) {`

```text
// cycleChangedPaths returns every path the cycle would commit: tracked changes vs
// HEAD (modified/deleted/renamed) unioned with untracked new files — so the
// protected-path check sees the file no matter how it was introduced. Rename
// detection is OFF: with it on, --name-only prints only a rename's NEW path, so
// a protected file moved to an unprotected name would pass (architecture review
// F37 M1; the build handoff floor judges the same way).
```

### `go/internal/phases/ship/integrity_test.go:16` — above `func TestVerifyNoControlPlaneEdits_RejectsGateEdit(t *testing.T) {`

```text
// TestVerifyNoControlPlaneEdits_RejectsGateEdit is the cycle-20 regression at the
// ship boundary: a --class cycle commit whose diff touches a gate file is
// rejected with CodeControlPlaneViolation — even though the file was changed by a
// non-tool channel (here a direct write, mimicking a Bash `sed -i` bypass of the
// real-time role-gate hook).
```

### `go/internal/phases/ship/integrity_test.go:23` — above `mustWrite(t, filepath.Join(repo, "go/acs/regression/flagreaders/readers_test.go"),`

```text
// The exact cycle-20 attack surface: the gate that grades the cycle.
```

### `go/internal/phases/ship/integrity_test.go:67` — above `func TestVerifyNoControlPlaneEdits_RejectsARenameOutOfTheSurface(t *testing.T) {`

```text
// TestVerifyNoControlPlaneEdits_RejectsARenameOutOfTheSurface (architecture
// review F37 M1): a staged `git mv` of a protected file to an unprotected name
// is judged by its OLD path — with rename detection on, `git diff --name-only`
// printed only the new one and the tripwire passed.
```

### `go/internal/phases/ship/integrity_test.go:90` — above `func TestVerifyNoControlPlaneEdits_RejectsTrackedGateModification(t *testing.T) {`

```text
// TestVerifyNoControlPlaneEdits_RejectsTrackedGateModification is the precise
// cycle-20 scenario: an EXISTING tracked gate file is MODIFIED (not newly
// created), exercising the `git diff --name-only HEAD` path rather than the
// untracked `ls-files --others` path.
```

### `go/internal/phases/ship/integrity_test.go:100` — above `mustWrite(t, gate, "package flagreaders\n// tampered by a cycle\n")`

```text
// Modify the now-tracked gate — the exact cycle-20 attack.
```

### `go/internal/phases/ship/landing_pins_test.go:3` — above `import (`

```text
// landing_pins_test.go — ADR-0103 unit 07 (the ship landing): the pre-move
// pins over the ff-merge, the three push sites with their inline push-race
// repair, and the ship-binding writer. The goldens under landing/testdata
// were captured on 8e8f080f BEFORE any code moved (a throwaway recorder,
// deleted after the capture) and are replayed here through the host and in
// the leaf's own tests, so a transcription slip in the move is a byte diff,
// not a review opinion. Every test is untagged and fake-only: the recorder
// scripts git by its FULL argv (the package's scriptedRunner keys on the
// subcommand and cannot tell `rev-parse origin/main` from `rev-parse HEAD`).
```

### `go/internal/phases/ship/landprefixes_test.go:10` — above `func TestLandPrefixes(t *testing.T) {`

```text
// TestLandPrefixes names + exercises the live composed main-push driver
// (apicover): both landing modes through the real PrefixQueue, pinning the
// intent that made cycle-975's composer non-inert — in prefix-queue mode the
// culprit lane is ejected and the innocent survivors land (verified as a
// set), while per-lane mode keeps the legacy independent stand-or-fall
// behavior, and an empty lane set is nil/nil in both modes.
```

### `go/internal/phases/ship/manifest.go:19` — above `var manifestReportFiles = []string{`

```text
// manifest.go — ship-bind tree-manifest reconciliation (shadow + enforce).
//
// Cycle-653 second seam: ship binds the whole `git diff HEAD` tree, so any
// path present in the worktree ships (or blocks) regardless of whether the
// cycle's build/TDD phases declared it. reconcileManifest reconciles the paths
// ship is about to bind against the cycle's DECLARED file manifest (paths named
// in build-report.md + test-report.md).
//
// Two modes (opts.ManifestGate, config-sourced; default shadow — the
// ReportSizeGate "new gates default shadow" precedent):
//   - shadow (default): log out-of-manifest paths, never block — behavior-preserving.
//   - enforce: FAIL CLOSED on any out-of-manifest path — the cross-lane
//     untracked-leak guard (inbox `ship-stage-explicit-paths`, cycle-645).
//
// BEFORE enabling enforce in production (the deferred policy.json→Options
// wiring), prerequisites (2026-07-14 review):
//   1. DONE (2026-07-14): pathToken (below) now also extracts bare root-level
//      filenames (CHANGELOG.md, go.mod) via an extension allow-list, so a legit
//      root-file change is no longer a FALSE-BLOCK under enforce.
//   2. DONE (cycle-1064): the enforce branch carries the dedicated
//      core.CodeManifestGate (mirroring CodeCommitPrefixGate) instead of reusing
//      CodeGitStageFailed, so the ledger/debugger can tell a manifest block from
//      a real `git add` failure; router.shipLocalCodes routes it to the debugger.
//   3. DONE (cycle-1064): policy.json `gates.manifest_gate` resolves through
//      policy.GatesConfig() into ship.Config.ManifestGate → Options.ManifestGate,
//      so enforce is operator-activatable without a code edit. Default stays
//      "shadow" — behavior-preserving.
```

### `go/internal/phases/ship/manifest.go:165` — above `func isRepoRelative(p string) bool {`

```text
// isRepoRelative reports whether p can be handed to git as a repo-relative
// pathspec. Absolute ("/go/evolve") and repo-escaping ("../x", including
// interior escapes like "a/../../etc/x" — git: "is outside repository",
// rc=128) entries are the cycle-1098 `git add` fatal class: git canonicalizes
// them against the FILESYSTEM, not the repo root. The check runs on the
// path.Clean'd form so interior ".." segments cannot smuggle an escape past a
// prefix test. SSOT for the extraction filter (extractReportPaths) and the
// staging seam (stagePathspec).
```

### `go/internal/phases/ship/manifest.go:207` — above `func unquoteGitPath(tok string) string {`

```text
// unquoteGitPath decodes one C-quoted path token from git's output into the
// literal on-disk path. SSOT for every reader that classifies paths out of git
// output (porcelainChangedPaths here, dropIgnoredPaths' ignored set in
// gitops.go) — the isRepoRelative/manifestCovers shared-helper pattern.
//
// git quotes a path (core.quotePath, default true) whenever it contains a
// non-ASCII byte, a quote, a backslash, or a control char, escaping the payload
// with the SAME grammar Go string literals use: `\\`, `\"`, `\t`/`\n`/`\r`/…,
// and per-BYTE octal `\NNN` (`café.txt` → `"caf\303\251.txt"`, two escapes for
// one rune). strconv.Unquote is therefore the exact decoder, not an
// approximation. Cycle-1108: leaving it undecoded yielded the 15-byte literal
// `caf\303\251.txt`, a path that exists on no disk — it matched no manifest
// entry and staged nothing.
//
// Decoding is CONDITIONAL on the token being wrapped in quotes on both ends:
// an unquoted token is a path git did not escape, so its backslashes are
// literal (`not\quoted.txt`) and touching it would corrupt the common case. A
// token that fails to decode is likewise returned verbatim — never dropped.
```

### `go/internal/phases/ship/manifest.go:330` — above `func stagePathspec(manifest, changed []string, isFile func(string) bool) []string {`

```text
// stagePathspec computes the explicit `git add -- <paths>` pathspec for a
// non-release ship (cycle-1067, `ship-stage-explicit-paths`): the DECLARED
// manifest, not `git add -A`, decides what a cycle/manual ship binds — so a
// sibling lane's untracked leak (cycle-645) can no longer ride into the commit.
//
// The set is:
//   - every declared entry that is a real file on disk (isFile) or that git
//     reports as changed (so a DELETED declared path still stages its deletion);
//   - plus every changed path the manifest covers by directory prefix (a new
//     file under a declared directory is part of the declared change).
//
// Fallbacks — staging must never silently become a no-op, which would produce a
// false clean exit / empty ship, and must never fall back to `-A`:
//   - no manifest (no workspace, or no readable phase reports) → the full
//     porcelain changed set;
//   - a manifest that covers nothing that changed → likewise the changed set.
```

### `go/internal/phases/ship/manifest_continuation_test.go:3` — above `import (`

```text
// manifest_continuation_test.go — ADR-0076 slice C amendment #2 (architect
// review): a continuation cycle re-exposes the PRIOR attempt's files at ship
// time (post-build soft-reset to the original base), but this cycle's build
// report only declares what the resuming builder re-touched. Under
// manifest_gate=enforce that fails closed on resumed-but-undeclared paths, so
// declaredManifest must UNION the prior attempt's declared manifest, located
// via the continuation manifest the adoption seam copies into this cycle's
// workspace.
```

### `go/internal/phases/ship/manifest_gate_code_test.go:14` — above `func manifestLeakOpts(t *testing.T) *Options {`

```text
// manifestLeakOpts builds a reconcileManifest fixture whose declared manifest
// covers docs/declared.md while `git status --porcelain -uall` reports an
// UNDECLARED untracked file — the cross-lane leak shape (cycle-645).
```

### `go/internal/phases/ship/manifest_gate_code_test.go:30` — above `func TestReconcileManifest_EnforceCarriesManifestGateCode(t *testing.T) {`

```text
// TestReconcileManifest_EnforceCarriesManifestGateCode is the cycle-1064 crux
// for dedicated-manifest-gate-error-code: the enforce-mode block must surface
// the DEDICATED core.CodeManifestGate, never the generic CodeGitStageFailed a
// real failing `git add` also emits. Ledger/debugger triage keys off Code, so
// the reuse makes an integrity block indistinguishable from a transient git
// failure (GIT_STAGE_FAILED is class TRANSIENT in the code table).
```

### `go/internal/phases/ship/manifest_gate_test.go:21` — above `func TestReconcileManifest_EnforceBlocksCrossLaneLeak(t *testing.T) {`

```text
// TestReconcileManifest_EnforceBlocksCrossLaneLeak is the ship-stage-explicit-
// paths guard (cycle-645): under enforce, a worktree carrying an untracked file
// that no phase report declared — typically a sibling fleet-lane's leaked
// artifact — must FAIL the ship closed, not silently commit it to main. Shadow
// (the default) must NOT block: behavior-preserving. The workspace build-report
// declares only docs/declared.md; the fake `git status --porcelain -uall`
// reports an undeclared untracked file.
```

### `go/internal/phases/ship/manifest_gate_wiring_test.go:19` — above `func TestShipOptions_ThreadsManifestGate(t *testing.T) {`

```text
// TestShipOptions_ThreadsManifestGate is the cycle-1064 wiring crux: the ship
// PhaseRunner's PhaseRequest→Options translation must carry the config-sourced
// manifest-gate mode. Today Options.ManifestGate is never assigned at the sole
// production construction site (ship.go runNative), so the dial is permanently
// "" (shadow) no matter what policy.json says — the gate is unreachable short of
// a code edit.
//
// The translation is asserted through the exported constructor + the
// shipOptions seam (the extracted Options literal runNative uses), so the test
// exercises the real production path rather than a source-grep.
```

### `go/internal/phases/ship/manifest_relative_test.go:9` — above `func TestExtractReportPaths_RelativeDotPrefix(t *testing.T) {`

```text
// manifest_relative_test.go — regression lock for the cycle-1098 ship fatal
// (2026-07-27, first green cycle of batch-12): extractReportPaths ran
// strings.Trim(token, ".") on the matched token, so the ./-prefixed prose the
// ADR-0076 slice-B mandate puts in EVERY build-report ("$ ./go/bin/evolve
// selfcheck build") became the ABSOLUTE-looking manifest entry
// "/go/bin/evolve". stagePathspec's isFile filter resolved it INSIDE the
// worktree (filepath.Join(root, "/go/bin/evolve")), so it survived into
// `git add -A -- /go/bin/evolve ...` → git canonicalized the absolute path →
// `fatal: Invalid path '/go': No such file or directory` (rc=128, reproduced
// verbatim) → 2 futile transient retries → cycle aborted, audited-PASS work
// stranded in the preserved worktree. Deterministic batch-killer: every green
// cycle documents the same pre-flight line.
```

### `go/internal/phases/ship/native.go:93` — above `WorkspacePath string`

```text
// WorkspacePath is this run's per-cycle workspace (<ProjectRoot>/.evolve/runs/
// cycle-<N>/). When set, ship reads run-defining inputs (active_worktree,
// cycle_id) from <WorkspacePath>/run.json — the per-run mirror of
// cycle-state.json (CB.4) — instead of the host-global cycle-state.json, so a
// concurrent cycle can't make ship integrate the WRONG run (ADR-0049 S3 / gap
// G3). Empty (standalone `evolve ship`) → the global file. Set by the ship
// PhaseRunner from PhaseRequest.Workspace.
```

### `go/internal/phases/ship/native.go:114` — above `ManifestGate string`

```text
// ManifestGate is the ship-hygiene gate mode for the worktree ship path.
// "" (default) = SHADOW: log out-of-manifest paths, never block — behavior-
// preserving. "enforce" = FAIL-CLOSED: refuse to commit any path no phase
// report (build-report/test-report) declared; under a fleet these are
// typically a sibling lane's untracked leak (cycle-645) whose commit reddens
// main. Config-sourced (policy.json), never a code literal — the
// policy→Options wiring is a follow-up; today only tests set enforce.
```

### `go/internal/phases/ship/native.go:123` — above `RunID string`

```text
// RunID is this run's event-sourced identity (CA.5). When set, the
// audit→ship binding lookup (findLatestAudit) prefers the auditor ledger
// entry stamped with THIS RunID over a concurrent run's later entry
// (ADR-0049 S4 / gap G5). Empty (standalone / legacy) → latest auditor
// entry, as before. Set by the ship PhaseRunner from PhaseRequest.RunID.
```

### `go/internal/phases/ship/native.go:146` — above `PhaseIO config.Stage`

```text
// PhaseIO threads the EVOLVE_PHASE_IO stage into the audit-binding verdict
// parse (ADR-0050 §3.10 Slice 6). At >= StageEnforce parseVerdicts is
// sentinel-first; the zero value (StageOff) keeps the prose parse — byte-identical.
```

### `go/internal/phases/ship/native.go:163` — above `internalAuditBoundTreeSHA string`

```text
// internalAuditBoundTreeSHA is a WRITE-ONCE audit witness: audit.go is
// the SOLE authorized writer (it sets the field after parsing
// audit-report.md / the ledger binding entry — see audit.go:124-127).
// gitops.go READS it to enforce both the pre-commit tree-SHA binding
// check (gitops.go:407-409) and the post-push integrity guard
// (gitops.go:493-497), and to stamp the ship-binding.json sidecar
// (gitops.go:528). Do NOT reassign it anywhere else — a post-audit
// rebind (e.g. to a post-merge tree) silently disarms both guards and
// corrupts the forensic sidecar (cycle-583, rejected). This invariant
// is mechanically pinned by TestInternalAuditBoundTreeSHA_OnlyAssignedInAuditGo
// (audit_bound_witness_test.go), which turns RED on any second assignment
// site in this package. Not part of the public API.
```

### `go/internal/phases/ship/native.go:177` — above `internalConsumedPaths []string`

```text
// internalConsumedPaths is the exact set of repo-relative paths the ship's
// OWN in-commit inbox consumption staged (consume.go — the one mutation
// the ship performs by design AFTER the audit bound the tree). The two
// tree-drift integrity checks accept a bound-vs-actual mismatch IFF the
// tree delta is a subset of these paths (cycle-1506: consumption made
// every PASS ship of an inbox-claimed item refuse at the pre-commit
// check). Written ONLY by consumeCommittedItems; any other writer
// re-opens a smuggling channel through the drift tolerance.
```

### `go/internal/phases/ship/native.go:192` — above `shipLock func(path string) (release func(), err error)`

```text
// shipLock is the test seam for the ADR-0049 S5 integrator lock
// (gap G1): the BLOCKING flock acquired around the shared-main
// integration critical section (collider scan → ff-merge → push →
// post-push verify) in shipFromWorktree. nil → flock.Lock on
// <ProjectRoot>/.evolve/ship.lock. Signature mirrors flock.Lock.
```

### `go/internal/phases/ship/native.go:228` — above `RepairAttempted string`

```text
// RepairAttempted/RepairOutcome surface the repair ladder (ADR-0039 §8):
// the ShipError code a typed repair was attempted for, and its outcome
// (e.g. "repinned-verified-rebuild", "resume-pushed", "push-retried",
// "colliders-healed:…", "needs-reaudit", "declined"). Empty when no
// repair fired. When multiple repairs fire in one Run (e.g. a healed
// stale pin followed by a push retry) this records the LAST attempt —
// the full per-attempt trail lives in Logs and the ShipError Debug map.
// Mirrored as ship.repair_* signals by the PhaseRunner.
```

### `go/internal/phases/ship/native.go:297` — above `if _, err := runStageWithRepair(ctx, &opts, &res, func() error {`

```text
// 1. Self-SHA TOFU verification. Writes state.json on first-run /
// version-bump / legacy migration. INTEGRITY-FAILs on same-version
// SHA mismatch. The repair ladder (ADR-0039 §8) may heal a stale pin
// (verified rebuild of committed source) and re-run the check once.
```

### `go/internal/phases/ship/native.go:327` — above `resumed, err := runStageWithRepair(ctx, &opts, &res, func() error {`

```text
// 2. Class-aware pre-flight (audit-binding, kernel checks, or interactive
// confirm). The repair ladder may COMPLETE the ship here: an
// AUDIT_BINDING_HEAD_MOVED whose HEAD already carries the audit-bound
// tree (the cycle-246 merged-but-unpushed death) closes with a push-only
// resume — in that case the mutate stage below is skipped.
```

### `go/internal/phases/ship/native.go:371` — above `if res.CommitSHA != "" && !opts.DryRun && !opts.PushOnly {`

```text
// Durable per-commit ship provenance (pushonly.go): every MINTED commit is
// journaled — on the success path AND on a failure after the commit (a
// rejected push). The journal records that a sanctioned ship minted the
// commit, not that its push succeeded: the GIT_PUSH_REJECTED strand is the
// very case `evolve ship --push-only` completes, and it refused lane 1678's
// commit by name (2026-09-14) because the journal was written only on
// success. Push-only itself is exempt (review MEDIUM): it mints nothing,
// and journaling its HEAD would record commits this plane never shipped
// (e.g. a console-merged commit after a nothing-to-push clean exit) into
// the very trust anchor future pushes consult.
```

### `go/internal/phases/ship/native_explanation_gate.go:58` — above `hostActive, err := explanationdocs.CrossCheckActivation(binding)`

```text
// The activation belt lives in explanationdocs (single home; audit runs
// the same check — architecture review 2026-09-01). Inactive means the
// host AGREES this is a legacy cycle; a ship that still demands the
// handoff keeps its refusal.
```

### `go/internal/phases/ship/native_explanation_gate.go:67` — above `return nil`

```text
// Genuine legacy — the host agrees. A Require=true refusal here would
// be dead code: ship.go derives RequireBuildExplanationHandoff from
// version != 0, and the belt already refused every inactive+version!=0
// identity (2026-09-01 re-review wiring proof).
```

### `go/internal/phases/ship/native_test.go:81` — above `writeStrictAuditPolicy(t, repo)`

```text
// Strict mode now comes from .evolve/policy.json (workflow.strict_audit), not
// the retired EVOLVE_STRICT_AUDIT env dial (flag-reduction, ADR-0064).
```

### `go/internal/phases/ship/native_test.go:96` — above `func writeStrictAuditPolicy(t *testing.T, root string) {`

```text
// writeStrictAuditPolicy drops a .evolve/policy.json into root that turns on the
// strict (legacy-blocking) audit posture — the policy.json replacement for the
// retired EVOLVE_STRICT_AUDIT env dial (flag-reduction, ADR-0064).
```

### `go/internal/phases/ship/native_test.go:384` — above `mustWrite(t, filepath.Join(repo, "q1.txt"), "first audited\n")`

```text
// First ship at v1.0.0
```

### `go/internal/phases/ship/planlanding_test.go:11` — above `func TestPlanLanding_RoutesOnLandingMode(t *testing.T) {`

```text
// planlanding_test.go — default-tag coverage for the PlanLanding wiring seam
// (ADR-0069: the acs-tagged go/acs/cycle981 gate-wiring predicate does NOT run
// under `go test ./internal/...`, so repo-wide apicover flags PlanLanding as
// uncovered). This unit test pins the same routing contract under default tags.
```

### `go/internal/phases/ship/postship.go:67` — above `if err := withStateLock(stPath, func() error {`

```text
// Same shared lock every other state.json RMW takes (ADR-0049 S2 / G2), so
// a concurrent allocator write can neither lose nor be lost to this one.
```

### `go/internal/phases/ship/postship.go:109` — above `func advanceLastCycleNumber(opts *Options, res *RunResult) error {`

```text
// advanceLastCycleNumber reads cycle-state.json:cycle_id and writes it
// into state.json:lastCycleNumber atomically. Only fires for class=cycle.
//
// This is the v8.34.0 fix for stuck-counter: pre-v8.34, only failure
// paths wrote lastCycleNumber, so successful ships left the counter at
// the previous cycle → dispatcher's next iteration computed
// ran_cycle = last_before + 1 = the SAME cycle just shipped → 5-repeat
// circuit-breaker fired prematurely on legitimate runs.
```

### `go/internal/phases/ship/postship.go:118` — above `csPath := opts.cycleStateFile()`

```text
// ADR-0049 S3 / G3: run-scoped (cycle_id)
```

### `go/internal/phases/ship/postship.go:129` — above `var readErr error`

```text
// ADR-0049 S2 / G2: serialize the state.json RMW under the shared lock so
// it can't lose (or be lost to) a concurrent allocator/UpdateState write.
// Preserve the pre-lock contract: a READ error propagates (fail ship);
// only a write/lock error is the non-fatal WARN.
```

### `go/internal/phases/ship/postship.go:154` — above `func promoteInbox(ctx context.Context, opts *Options, res *RunResult) error {`

```text
// promoteInbox calls the inboxmover Go library directly (v11.8.1+; prior
// versions shelled out to legacy/scripts/lifecycle/inbox-mover.sh). Moves
// shipped inbox tasks to processed/. Best-effort: failures log WARN and
// don't block ship (Layer 1 idempotency catches residual in next cycle's
// Triage).
```

### `go/internal/phases/ship/postship.go:160` — above `csPath := opts.cycleStateFile()`

```text
// ADR-0049 S3 / G3: run-scoped (cycle_id)
```

### `go/internal/phases/ship/postship.go:174` — above `cycleDir := filepath.Join(opts.ProjectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cid))`

```text
// Promote top_n[] + skip_shipped[] to processed/. The companion the agent is
// instructed to emit is in practice almost never written (cycles 308/316/
// 320-322 all missing it), so triageDecisionBytes DETERMINISTICALLY PROJECTS
// it from triage-report.md when absent — single source, guaranteed present
// (triage-decision-json-not-emitted; ADR-0047 single-source-with-projection).
```

### `go/internal/phases/ship/postship.go:191` — above `var laneFallbackIDs []string`

```text
// PASS half of the stable-failure-identity rule (PR #439 closed the FAIL
// half): continuation/lane cycles carry NO triage decision, so a PASS ship
// promoted nothing and a full bookkeeping cycle was later spent moving one
// JSON file. File-ABSENT only — a present decision that committed zero ids
// keeps the declined menu unpromoted.
```

### `go/internal/phases/ship/postship.go:202` — above `markerIDs := inboxmover.ClosesInboxIDs(readBuildReport(cycleDir))`

```text
// Consumption rides the landing (consumption-rides-landing-ship): the ids the
// Builder line-anchored as closed by THIS diff. Before this, consuming an item
// was a separate act from the ship that closed it, so forgetting was always
// possible — #453 landed schema-aligned-salvage-layer with its item left open
// and wave cycle-1448 re-picked already-shipped work as live scope. The marker
// is additive to the triage/lane sources and rides the SAME landing gate
// below; an absent build-report.md (build-skipped cycles) reads as no claim,
// never an error.
```

### `go/internal/phases/ship/postship.go:215` — above `if res.CommitSHA != "" && !isLanded(ctx, opts, res.CommitSHA) {`

```text
// Landing gate (cycle-598 regression, inbox-promotion-requires-landed-ship):
// promote to processed/ ONLY when the ship commit actually reached durable
// history (ancestor of HEAD or origin/<branch>). Cycle 598's push was
// rejected (origin diverged), the recovery reclassified to needs-reaudit,
// yet promoteInbox promoted the item anyway because its only gate was
// "triage-decision.json present". The landing check is the single source of
// truth, independent of any verdict/outcome label. An unlanded commit leaves
// items in processing/ — the residual drain below releases them for the next
// cycle's triage to re-scan, so nothing is silently lost.
//
// Fail-open when res.CommitSHA is empty: the gate catches a commit that
// EXISTS but failed to reach durable history (the cycle-598 shape). An
// absent SHA is a different, pre-existing state (no commit recorded) with
// no signal to gate on — promoting it preserves the cycle-308 residual-drain
// contract rather than newly stranding correctly-shipped work.
```

### `go/internal/phases/ship/postship.go:238` — above `if retired, rErr := inboxmover.ReconcileSuperseded(mvOpts, inboxmover.SupersededInboxIDs(body), "processed", inboxmover.…`

```text
// Reconcile superseded[] — inbox items whose work shipped under a
// DIFFERENT id (cycle 544 shipped as recover-ship-fleet-starvation-
// observer, stranding loop-self-prioritize-unmet-fleet-concurrency).
// extractIDs only walks top_n/skip_shipped, so these orphans were never
// retired; ReconcileSuperseded retires them by id alone. Best-effort.
// Gated by the same landing check — an unlanded commit must not retire a
// superseded id either (scout Beyond-the-Ask Hypothesis 2).
```

### `go/internal/phases/ship/postship.go:262` — above `releaseReason := ""`

```text
// ALWAYS drain residual claims: every item still in processing/cycle-<cid>/
// is released back to the inbox root so the next cycle's triage re-scans it
// (Step 0a reads only inbox/ root, maxdepth 1). This MUST run even when
// triage-decision.json is absent — the early-return that used to skip it
// stranded EVERY claimed item invisibly (inbox-promote-on-ship-missing;
// orphans across cycles 124/265/294/295/308).
//
// When the landing gate above refused promotion (unlanded ship commit),
// the drain is a delivery-failure retry, not an ordinary residual drain —
// the ledger reason carries "unlanded" so triage/operators can tell them
// apart without hand forensics (cycle-598, inbox-promotion-requires-
// landed-ship). A landed cycle's residuals keep the generic reason.
```

### `go/internal/phases/ship/postship.go:278` — above `if or, outcomeErr := inboxmover.ApplyCycleOutcome(mvOpts, inboxmover.CycleOutcome{`

```text
// ONE lifecycle seam (menu-pass-promotes-committed-ids): promoting the
// committed ids and draining the residual claims are the two halves of a
// single PASS transition, so they go through ApplyCycleOutcome together
// rather than as an ad-hoc promote loop plus a separate release call. A
// menu that ships N items in one commit now promotes exactly those N ids in
// code — cycle-1147 shipped 3 and promoted 0 because the promote was prose
// the agent never executed.
```

### `go/internal/phases/ship/postship.go:307` — above `func isLanded(ctx context.Context, opts *Options, sha string) bool {`

```text
// isLanded reports whether the ship commit sha actually reached durable
// history — an ancestor of local HEAD, or of origin/<branch>. Reuses the
// existing isAncestor helper (repair.go, git merge-base --is-ancestor) rather
// than duplicating an ancestry probe. An empty sha is never landed (nothing to
// verify). See promoteInbox's landing gate for the cycle-598 regression this
// guards against.
```

### `go/internal/phases/ship/postship.go:401` — above `func committedInboxIDs(cycleDir string, body []byte, landedPASS bool) []string {`

```text
// committedInboxIDs is the SINGLE resolver for "which inbox ids did this cycle
// close". Both consumption sites use it: the in-commit consumption
// (consume.go, so the retirement rides the landing) and the post-ship promotion
// below. They diverged for eight cycles — consume.go read triage top_n alone
// while this file already resolved three sources — and that asymmetry is why a
// carryover-driven lane could land a PASS ship that closed an item and leave it
// pickable, because its triage ids never match the inbox file's own id.
//
// Precedence is the PER-ID rule documented inside the body (lane-scope-union,
// cycle-1515/1552): triage's committed set always counts; the lane's ASSIGNED
// scope ids join it unless triage deferred them or declined the whole menu;
// and the build-report Closes-Inbox marker always unions in — the id nobody
// named at dispatch is exactly the one that used to survive its own landing.
// The declined-menu contract (present decision, zero committed, id
// unmentioned -> stays open) is preserved verbatim and pinned at both sites.
```

### `go/internal/phases/ship/postship.go:417` — above `ids := inboxmover.CommittedIDs(body)`

```text
// consumption-id-linkage-lane-scope-union (0.86; burns: cycle-1515 triage
// DECOMPOSED the assigned id into sub-ids top_n named instead, cycle-1552
// triage DROPPED it as already-shipped with top_n:[] while build shipped
// the implementation anyway): triage's bookkeeping must not defeat the
// in-commit consumption of a PASS landing. The assigned lane-scope ids
// join the committed set under a per-id rule that keeps every prior pin:
//   - triage DEFERRED the id           -> stays pickable (postponed;
//     the remainder rides carryover)
//   - triage DROPPED the id            -> consumes (an affirmative close;
//     the carryover twin is already retired on the same signal)
//   - the decision committed ANY work  -> scope ids consume (fleet lanes
//     scout only their scope, so committed work is scope-derived — the
//     cycle-1515 decomposition shape)
//   - decision present, zero committed, id unmentioned -> stays open (the
//     declined-menu contract: lane-scope must not override an explicit
//     empty commitment; see TestPromoteInbox_EmptyCommittedDeclinedMenuStaysOpen)
```

### `go/internal/phases/ship/postship.go:462` — above `engagedByName := false`

```text
// Named-engagement discriminator: lane scopes are multi-item MENUS, and
// triage may commit a subset and leave menu mates pending as dispatchable
// backlog (triagecap lane_menu contract). If triage engaged the scope BY
// NAME (scope ∩ committed non-empty), only the named scope ids consume —
// an unmentioned menu mate stays pickable. Only when the committed set
// names NO scope id (the cycle-1515 rename/decomposition shape) does the
// whole non-deferred scope ride the landing.
```

### `go/internal/phases/ship/postship.go:520` — above `return withStateLock(statePath, func() error {`

```text
// ADR-0049 S2 / G2: serialize the whole read→check→write under the shared
// state.json lock. Any error (lock/read/write) propagates, as before.
```

### `go/internal/phases/ship/postship_carryover_retire_test.go:3` — above `import (`

```text
// postship_carryover_retire_test.go — WIRING PROOF for cycle-1440 task
// `carryover-pass-retirement`.
//
// core.RetireCarryoverTodos passing its unit tests proves nothing on its own: a
// seam whose only caller is a test is dead code. These tests drive the PRODUCTION
// PASS-closeout caller (promoteInbox) end to end and assert the observable side
// effect on .evolve/state.json, so they stay RED until a real production path
// reaches the retirement seam.
//
// Deliberately asserts the STATE, not the call: any implementation that retires
// the committed ids at PASS closeout satisfies it.
```

### `go/internal/phases/ship/postship_carryover_retire_test.go:94` — above `func TestPromoteInbox_UnlandedPassKeepsCarryover(t *testing.T) {`

```text
// TestPromoteInbox_UnlandedPassKeepsCarryover is the negative twin: promotion is
// already gated on the ship commit reaching durable history (cycle-598), and
// retirement must obey the same gate. An unlanded commit that retired the todo
// would erase the only record of work that never shipped.
```

### `go/internal/phases/ship/postship_closesinbox_test.go:3` — above `import (`

```text
// postship_closesinbox_test.go — RED contract for the wiring half of
// consumption-rides-landing-ship (cycle 1452): a PASS ship whose diff closes an
// inbox item consumes that item in the SAME landing, even when triage never
// named the id.
//
// The live instance: schema-aligned-salvage-layer landed in #453, nothing
// consumed its item, and wave cycle-1448 re-picked already-shipped work as live
// scope. The defect is structural — consumption is a separate act from landing,
// so forgetting is always possible.
//
// What these tests freeze (doNotModifyTests):
//   - marker ids join the committed set through the ONE existing lifecycle seam;
//   - they ride the EXACT cycle-598 landing gate (`isLanded`) — no new, parallel,
//     or weaker gate: an unlanded ship consumes nothing;
//   - a marker works with NO triage decision present (the continuation/lane
//     shape that produced the live instance);
//   - absence of build-report.md is not an error and does not disturb the
//     triage-sourced promotion;
//   - a landed ship WITHOUT a marker consumes only what triage named (the
//     anti-over-consumption half — a diff-inference implementation fails here).
//
// Fixture helpers (writeDrainCycleState / writeDrainTriageDecision /
// writeDrainInboxItem) are shared with the sibling postship drain tests.
```

### `go/internal/phases/ship/postship_closesinbox_test.go:110` — above `func TestPromoteInbox_ClosesInboxMarkerSkippedOnUnlandedShip(t *testing.T) {`

```text
// TestPromoteInbox_ClosesInboxMarkerSkippedOnUnlandedShip — the cycle-598 gate
// must bind the marker path exactly as it binds the triage and lane-scope paths.
// A non-git TempDir with a real-looking SHA makes isLanded fail closed.
```

### `go/internal/phases/ship/postship_drain_claim_test.go:13` — above `func writeDrainCycleState(t *testing.T, root string, cycleID int) {`

```text
// postship_drain_claim_test.go pins the cycle-1156 D1 *aggravator*: promoteInbox
// used to append "[ship] OK: inbox lifecycle drain complete" unconditionally,
// after having already logged a WARN for the very failure that stopped the
// drain. An operator (and every log-grepping gate) read success from a cycle
// whose lifecycle transition demonstrably did not complete. The code defect and
// the false-success claim are separately regressible, so they are pinned
// separately — these are the two tests the cycle-1158 eval's third score_cap
// names as its evidence command.
```

### `go/internal/phases/ship/postship_drain_claim_test.go:68` — above `func blockDrainPath(t *testing.T, path string) {`

```text
// blockDrainPath writes a regular FILE where a directory is needed, so the
// destination MkdirAll inside Promote fails — the infrastructure non-delivery
// ADR-0079 decision 4 made loud.
```

### `go/internal/phases/ship/postship_drain_claim_test.go:110` — above `func TestPromoteInbox_PromoteError_StillDrainsResidualClaims(t *testing.T) {`

```text
// TestPromoteInbox_PromoteError_StillDrainsResidualClaims: the D1 defect proper.
// A failed committed-id promote must not skip the residual drain — the early
// return stranded every item parked in processing/cycle-N/ (the cross-cycle
// orphan shape of cycles 124/265/294/295/308). The residual item must be back
// at the inbox root even though the promote failed.
```

### `go/internal/phases/ship/postship_landing_note_test.go:1` — above `package ship`

```text
// postship_landing_note_test.go — RED tests for cycle-752 task
// inbox-promotion-requires-landed-ship (the residual acceptance gap).
//
// The landing gate itself (unlanded SHA never promotes; landed twin promotes;
// needs-reaudit terminal never promotes) landed in a prior cycle and is
// pinned by postship_landing_test.go — pre-existing GREEN. What the inbox
// item's fix text still demands and the code does NOT do is the "retry note":
//
//	"Otherwise the item RETURNS to inbox with a retry note (mirror the
//	 failed-cycle release path)."
//
// Today an unlanded ship leaves the item in processing/ and the residual
// drain (ReleaseCycleProcessing) returns it to the inbox root with the
// generic ledger reason "cycle-release" — byte-identical to an ordinary
// residual drain. Nothing durable records WHY the item came back, so an
// operator (or the next triage) cannot distinguish "ship never landed,
// retry this" from "claimed but never committed". The cycle-598 incident
// was only diagnosed by hand for exactly this reason.
//
// Fix under test (not yet implemented — the note test MUST fail RED until
// Builder threads an unlanded-release annotation through the drain): when
// promoteInbox skips promotion because the ship commit is unlanded, the
// ledger entry recording each released item must carry an "unlanded" note
// in place of (or in addition to) the generic reason. The negative twin
// pins that an ordinary landed-cycle residual drain does NOT gain the note.
```

### `go/internal/phases/ship/postship_landing_note_test.go:54` — above `func TestPromoteInbox_UnlandedReleaseCarriesRetryNote(t *testing.T) {`

```text
// TestPromoteInbox_UnlandedReleaseCarriesRetryNote is the cycle-752 RED
// anchor: an unlanded ship (merge-base --is-ancestor exit 1, the cycle-598
// needs-reaudit shape) must still release the item back to the inbox root
// (pre-existing residual-drain behavior) AND leave durable per-item evidence
// — a ledger entry for the released item whose reason notes the unlanded
// ship — so triage/operators can tell a delivery failure from an ordinary
// residual drain.
```

### `go/internal/phases/ship/postship_landing_test.go:1` — above `package ship`

```text
// postship_landing_test.go — RED tests for cycle-609 task
// fix-inbox-promotion-landing-gate.
//
// Cycle 598 defect (inbox-promotion-requires-landed-ship.json): a ship push
// was rejected (origin diverged), the recovery path reclassified to
// needs-reaudit, the cycle still reported FinalVerdict PASS, and
// promoteInbox promoted the inbox item to processed/ anyway — even though
// `git log --all -S` over the touched path showed nothing ever landed on
// any ref. Root cause: promoteInbox's only promotion criterion is
// "triage-decision.json is non-nil"; it never asks whether res.CommitSHA
// actually reached main/origin.
//
// Fix under test (not yet implemented — these tests MUST fail RED until
// Builder wires a landing check, reusing the existing isAncestor helper
// from repair.go, into promoteInbox's Promote/ReconcileSuperseded calls):
// an unlanded commit SHA must skip promotion for BOTH the primary
// top_n/skip_shipped loop and the superseded-reconcile path, logging a
// "[ship] WARN: promotion skipped: unlanded" line instead of "promoted:
// landed", and the item must NOT appear under processed/cycle-<cid>/.
```

### `go/internal/phases/ship/postship_landing_test.go:198` — above `func TestPromoteInbox_NeedsReauditOutcomeNeverPromotes(t *testing.T) {`

```text
// TestPromoteInbox_NeedsReauditOutcomeNeverPromotes is the cycle-598
// regression itself: RepairOutcome=="needs-reaudit" (origin diverged, the
// landing's push repair declined to push) paired with a CommitSHA that is only a
// local commit (not an ancestor of HEAD-on-origin — modeled here as
// merge-base --is-ancestor failing) must never promote, regardless of
// whether a caller upstream still considers the cycle a "PASS". The landing
// check is the single source of truth, independent of verdict labels.
```

### `go/internal/phases/ship/postship_lanescope_test.go:3` — above `import (`

```text
// postship_lanescope_test.go — RED contract for the PASS half of the
// stable-failure-identity asymmetry (inbox consumption-rides-landing-ship
// 0.88; PR #439 fixed the FAIL half). Continuation/lane cycles carry NO
// triage-decision.json, so a PASS ship promoted NOTHING: the landed item
// stayed open and a full bookkeeping cycle (~25-30 min) was later spent
// moving one JSON file ("the crosspoll PASS consumed its item in-ship while
// the egps PASS did not"). When the decision file is ABSENT, the lane-scope
// pin's todo_ids are the committed set — same file-absent-only rule as the
// FAIL side (a PRESENT decision that committed zero ids keeps the declined
// menu unpromoted).
```

### `go/internal/phases/ship/postship_lanescope_test.go:52` — above `func TestPromoteInbox_UnlandedShipSkipsLaneFallbackPromotion(t *testing.T) {`

```text
// The cycle-598 landing gate must bind the FALLBACK entry path exactly as it
// binds the triage path (diff-review MEDIUM): an unlanded ship with a
// lane-scope committed set promotes nothing and drains with the retry reason.
```

### `go/internal/phases/ship/postship_release_test.go:3` — above `import (`

```text
// postship_release_test.go — RED test for cycle-308 task
// `inbox-promote-on-ship-missing` (ship-success residual drain).
//
// promoteInbox currently promotes only triage-decision.json's top_n[] +
// skip_shipped[] to processed/. Items that were CLAIMED into
// processing/cycle-<N>/ but then dropped from top_n are left stranded in
// processing/ forever. The fix: after promoting top_n, drain the residual
// claims back to the inbox root (via inboxmover.ReleaseCycleProcessing) so the
// next cycle re-triages them. Helpers mustWriteState/writeStateMap/anyContains
// live in postship_unit_test.go (same package).
```

### `go/internal/phases/ship/postship_release_test.go:29` — above `mustWriteState(t, filepath.Join(evolve, "cycle-state.json"), map[string]any{"cycle_id": float64(8)})`

```text
// Active cycle 8.
```

### `go/internal/phases/ship/postship_release_test.go:42` — above `procDir := filepath.Join(inbox, "processing", "cycle-8")`

```text
// Both items were claimed into processing/cycle-8/.
```

### `go/internal/phases/ship/postship_release_test.go:63` — above `if _, err := os.Stat(filepath.Join(procDir, "dropped.json")); err == nil {`

```text
// And it must no longer linger in processing/cycle-8/.
```

### `go/internal/phases/ship/postship_release_test.go:77` — above `func TestInboxPromote_NoTriageDecision_StillDrainsClaims(t *testing.T) {`

```text
// TestInboxPromote_NoTriageDecision_StillDrainsClaims pins the PRODUCTION
// strand bug: triage emits triage-report.md but NOT the triage-decision.json
// companion (observed missing in cycles 308/316/320-322), so promoteInbox
// early-returned before draining — leaving EVERY claimed item stranded
// invisibly in processing/cycle-N/ (Step 0a reads only inbox/ root). The
// residual drain must run regardless of triage-decision.json's presence.
```

### `go/internal/phases/ship/pushonly.go:3` — above `import (`

```text
// pushonly.go — the sanctioned completion for attested-but-stranded commits
// (inbox ship-push-only-recovery; 3 live instances). The strand: a ship's
// push hits GIT_PUSH_REJECTED with true divergence (a console PR merged to
// origin mid-batch), `evolve sync-main` reconciles merge-only (never
// pushes), and re-running ship reports "nothing to ship" while the plane
// sits ahead>0 — with bare `git push` correctly guard-denied, the recovery
// the error prescribed could not complete through any sanctioned command
// (the 2026-08-02 dead end; #408 stranded 15 commits the same way).
//
// Two halves close it:
//   - every successful ship appends its minted commit to the durable
//     provenance journal (.evolve/ship-journal.jsonl) — finalize's success
//     path, all classes, never dry-run;
//   - `evolve ship --push-only` pushes the ahead set after verifying EVERY
//     ahead commit's provenance: recorded in the journal, or a merge commit
//     with an origin-ancestor parent (the sync-main reconcile shape — the
//     merge is minted by the sanctioned command, so its identity is
//     structural). Any other commit refuses BY NAME — push-only must never
//     become the guard bypass for hand-made commits; commits predating the
//     journal refuse the same way, with the remediation named.
//
// TRUST POSTURE (stated, review MEDIUM): the journal is plane-local and
// unguarded-writable — its trust is equivalent in KIND to
// .commit-gate/attestation.json (which --class manual already pushes on),
// though weaker in degree (no tree-SHA binding, consulted unboundedly
// later); a Write-capable actor who can forge either can already ship. The
// sync-main reconcile merge's TREE is likewise trusted, not verified — a
// conflicted reconcile concluded by hand can smuggle content, the same
// plane-local-mutation trust class. rev-list enumerates every ahead commit
// INDIVIDUALLY, so a merge can never wrap unprovenanced commits past the
// check — each wrapped commit is tested and refused by name.
```

### `go/internal/phases/ship/pushonly_test.go:3` — above `import (`

```text
// pushonly_test.go — pins for the sanctioned strand completion (inbox
// ship-push-only-recovery; the 2026-08-02 dead end: GIT_PUSH_REJECTED →
// sync-main → "nothing to ship" while ahead>0 with bare git push
// guard-denied). See pushonly.go for the provenance model.
```

### `go/internal/phases/ship/pushonly_test.go:213` — above `func TestFinalize_MintedCommitIsJournaledEvenWhenThePushFailed(t *testing.T) {`

```text
// TestFinalize_MintedCommitIsJournaledEvenWhenThePushFailed — the strand
// push-only exists for (GIT_PUSH_REJECTED after the commit was minted) must be
// journaled, or push-only refuses the very commit it was built to complete:
// lane 1678 (2026-09-14) passed its gate, committed, had its push rejected,
// and push-only then answered "1 ahead commit(s) lack ship provenance". The
// journal is the record of a MINTED commit, not of a successful push.
```

### `go/internal/phases/ship/realgit_testhelpers_test.go:75` — above `func tempRepoDir(t *testing.T) string {`

```text
// tempRepoDir returns a fresh temp directory for a git repo whose cleanup is
// BEST-EFFORT — unlike t.TempDir(), a RemoveAll failure does NOT fail the test.
//
// On macOS CI runners, os.RemoveAll of a git work tree intermittently fails
// with EBADF ("bad file descriptor") on .git internals (e.g. a hooks/*.sample
// file) under -race load. With t.TempDir() that cleanup error fails an
// otherwise-passing test and forces a full ~5-minute CI re-run (observed on
// TestShipFromWorktree_GitAddFails_Errors, 2026-06-02). The temp dir is
// ephemeral — CI reclaims it regardless — so best-effort removal is safe and
// keeps a cosmetic cleanup race from gating a green build. The chmod-walk makes
// git's 0444 pack/object files removable.
```

### `go/internal/phases/ship/register_test.go:10` — above `func TestShipSelfRegisters(t *testing.T) {`

```text
// TestShipSelfRegisters asserts the ship phase publishes its own factory to the
// phase registry in package init() — exactly like every other built-in phase
// (intent/scout/...). The flow/dispatcher must never know HOW to construct
// ship; it looks the factory up by name. This is the phase-agnostic invariant
// (ADR-0035/0038): adding/owning a phase lives in the phase's package + JSON,
// never in a dispatcher switch.
//
// Registration previously lived in the dispatcher (internal/cli/phasecmd);
// ship now self-registers in its own init(). This is the permanent regression
// guard for that invariant — the test does not import the dispatcher.
```

### `go/internal/phases/ship/release_staging_test.go:1` — above `package ship`

```text
// release_staging_test.go — RED contract for inbox defects
// release-rebuild-binary-not-committed (2026-06-10T10-50Z, recurred v18.3.0 →
// v18.5.0) and release-stage-sweeps-untracked-root-logs (2026-06-10T10-51Z).
//
// Root cause (v18.5.0 forensic, commit d93e9f02): shipDirect runs
// discardBinaryChurn before `git add -A` for EVERY class. For cycle/manual
// that is correct (unaudited binary churn must not ride along); for
// --class release it throws away the binary the pipeline's rebuild-binary
// step produced ONE STEP EARLIER, so every release commit ships without
// go/evolve and the freshly-pinned expected_ship_sha guarantees
// SELF_SHA_TAMPERED on the next ship. Meanwhile `git add -A` sweeps
// untracked operator files (evolve.log, release-*.log) into release commits.
//
// Contract pinned here:
//  1. ClassRelease staging is an EXPLICIT pathspec — the versionbump marker
//     set (SSOT: versionbump.DefaultPaths) + CHANGELOG.md + the tracked
//     binary — never `add -A`, and never a churn discard of go/evolve.
//  2. ClassCycle keeps today's behavior byte-for-byte: churn discard, then
//     `git add -A` (the discriminator that keeps the fix class-scoped).
```

### `go/internal/phases/ship/release_staging_test.go:71` — above `func initReleaseStagingTree(t *testing.T) string {`

```text
// initReleaseStagingTree lays out the release file set plus an untracked
// stray log (the v18.4.0 sweep victim) and the freshly rebuilt binary.
```

### `go/internal/phases/ship/release_staging_test.go:150`

```text
// TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll was the original
// class discriminator: cycle ships kept churn discard + `git add -A` while
// the release class went explicit. Cycle-1067 (`ship-stage-explicit-paths`)
// removed `git add -A` from the cycle/manual paths too, so the `AddAll` half
// of that contract no longer exists and the test name would be stale.
//
// The two halves now live in stage_explicit_paths_test.go:
//   - churn discard for cycle → TestShipDirect_CycleClass_KeepsChurnDiscard
//   - the class discriminator → the release set is versionbump+CHANGELOG+binary
//     (TestShipDirect_ReleaseClass_StagesExplicitSetKeepsBinary, above), while
//     cycle/manual stage the DECLARED manifest
//     (TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll).
```

### `go/internal/phases/ship/repair.go:1` — above `package ship`

```text
// repair.go — the self-healing ship repair ladder (ADR-0039 §8).
//
// Ship is a pure executor: after audit PASS its job is to find the LEGITIMATE
// way to land the audited tree. Historically every verification failure was
// binary — it killed the cycle even when audited-PASS work sat ready to push
// (cycles 230, 246-248). The repair ladder gives each ShipError code at most
// ONE typed, provably-safe repair attempt per Run, then escalates through the
// existing orchestrator/router recovery machinery.
//
// Invariants (operator-approved 2026-06-07):
//   - Bounded: a given code is repaired at most once per Run (opts.repairAttempted);
//     the orchestrator's maxRecoveryDepth bounds the outer loop independently.
//   - Provably safe: every repair re-verifies the original invariant afterwards
//     (the failed stage is re-run, or the closure re-checks the tree binding).
//   - Policy-compliant: never rebase, never force-push, never set bypass env
//     vars, never delete content (differing colliders are quarantine-moved).
//   - Observable: every attempt is logged, recorded on the RunResult
//     (RepairAttempted/RepairOutcome), and stamped into the ShipError Debug
//     map when declined — flowing into ship-error.json and the failure floor.
```

### `go/internal/phases/ship/repair.go:127` — above `func repairSelfSHAPin(ctx context.Context, opts *Options, res *RunResult, _ *core.ShipError) repairOutcome {`

```text
// repairSelfSHAPin heals the cycles-246-248 stale-pin signature: the running
// ship binary's SHA equals the SHA of the binary blob COMMITTED AT GIT HEAD —
// a legitimate rebuild/manual-ship of audited, committed source whose pin was
// never refreshed. Re-pin and let verifySelfSHA re-run. Any divergence from
// the committed blob keeps the integrity BLOCK (this is the same trust
// boundary repinPostCycle already uses: HEAD is the audited reference).
```

### `go/internal/phases/ship/repair.go:156` — above `if err := withStateLock(statePath, func() error {`

```text
// ADR-0049 S2 / G2: serialize the RMW under the shared state.json lock. Any
// error (lock/read/write) → repairNone, as before (conservative repair).
```

### `go/internal/phases/ship/repair.go:190` — above `func repairColliders(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome {`

```text
// repairColliders heals the cycle-230 signature: untracked main-side files
// blocking the worktree ff-merge. Byte-identical copies are removed (the same
// bytes arrive via the merge); differing copies are quarantine-moved to
// .evolve/quarantine/cycle-<N>/ with a manifest record — content is never
// deleted. The atomic-ship stage is then re-run, which re-detects colliders
// from scratch (the repair never bypasses the pre-flight).
```

### `go/internal/phases/ship/repair.go:217` — above `csMap, err := readStateMap(opts.cycleStateFile())`

```text
// ADR-0049 S3 / G3: run-scoped (cycle_id)
```

### `go/internal/phases/ship/repair.go:331` — above `func repairResumeUnpushed(ctx context.Context, opts *Options, res *RunResult, se *core.ShipError) repairOutcome {`

```text
// repairResumeUnpushed heals the cycle-246 signature: ship died after its own
// commit/merge moved HEAD but before the push. When (a) HEAD's tree equals
// the audit-bound tree, (b) the audited base is an ancestor of HEAD, and
// (c) origin/<branch> is strictly behind HEAD on the same history, the
// audited work is already committed and merely unpushed — complete with a
// push-only closure. Anything else declines (→ the re-audit route).
```

### `go/internal/phases/ship/repair.go:351` — above `return repairNone`

```text
// Recorded decision (cycle-1506 review M5): a consumed-item PASS ship's
// HEAD tree legitimately differs from the bound tree by the sanctioned
// consumption delta — but this rung runs in a FRESH process after a
// died-between-commit-and-push crash, where internalConsumedPaths no
// longer exists, so the explained-by-consumption test cannot run here.
// Interrupted consumed-item ships therefore decline this fast heal and
// route to the slower re-audit path — fail-safe, deliberately.
```

### `go/internal/phases/ship/repair_colliders_test.go:3` — above `package ship`

```text
// repair_colliders_test.go — RED contract for repair-ladder mode #3
// (ADR-0039 §8): GIT_FF_MERGE_DIVERGED from untracked main-side colliders.
//
// Cycle-230 incident (retro I-10): untracked files in the main working tree
// blocked the worktree ff-merge, and the recovery chain looped audit↔ship —
// 3 PASS audits, 0 ships. The collider PRE-FLIGHT (cycle-232) made the
// refusal loud and pre-commit, but the cycle still died.
//
// The repair (operator-approved 2026-06-07): heal in-ship instead of dying —
//   - byte-IDENTICAL collider → remove main's untracked copy (the same bytes
//     arrive via the merge);
//   - DIFFERING collider → quarantine-move to .evolve/quarantine/cycle-<N>/
//     with a manifest record. Content is never deleted.
//
// Then re-run the atomic-ship stage exactly once.
//
// This supersedes the cycle-232 refuse-and-stop contract previously pinned in
// gitops_collider_test.go (deliberate, operator-approved behavior change).
```

### `go/internal/phases/ship/repair_pushrace_test.go:3` — above `package ship`

```text
// repair_pushrace_test.go — RED contract for repair-ladder mode #4
// (ADR-0039 §8): GIT_PUSH_REJECTED with an in-place fetch + ff-retry.
//
// Policy boundary (operator-confirmed 2026-06-07): ship must NEVER rebase or
// re-merge onto a moved base — the audit binding is on tree CONTENT, and a
// rebase produces a new tree. The only legitimate self-heals are:
//   - fetch, then retry the push once when origin is still an ancestor of
//     HEAD (a transient race / stale ref);
//   - otherwise reclassify the rejection as a Precondition with
//     repair_outcome=needs-reaudit so the recovery chain re-audits on the
//     new base, with the local commit preserved.
```

### `go/internal/phases/ship/repair_resume_test.go:3` — above `package ship`

```text
// repair_resume_test.go — RED contract for repair-ladder mode #2
// (ADR-0039 §8): AUDIT_BINDING_HEAD_MOVED with a resume-unpushed signature.
//
// Cycle-246 incident: ship died AFTER the ff-merge moved main's HEAD but
// BEFORE the push — main left ahead-1 of origin. The re-dispatch then failed
// AUDIT_BINDING_HEAD_MOVED (HEAD is the ship's own merge commit), and the
// operator hand-salvaged. The existing post-push idempotency check
// (native.go step 1.5) only covers the post-PUSH tail, because
// ship-binding.json is written after the push.
//
// The repair: when HEAD_MOVED fires but
//
//	(a) HEAD^{tree} equals the audit-bound tree SHA,
//	(b) the audited base commit is an ancestor of HEAD, and
//	(c) origin/<branch> is strictly behind HEAD on the same line of history,
//
// the audited work is already committed and merely unpushed — complete the
// ship with a push-only closure (never a rebase, never a force-push).
// Any other shape declines and the original error stands (→ re-audit route).
```

### `go/internal/phases/ship/repair_resume_test.go:32` — above `func mergedUnpushedFixture(t *testing.T, cycleBranch string) string {`

```text
// mergedUnpushedFixture provisions the cycle-246 death state: worktree commit
// created, ff-merged into main, origin NOT yet updated. Returns the repo.
// The audit is seeded BEFORE the merge (git_head = pre-merge main HEAD) with
// audit_bound_tree_sha = the worktree commit's tree — exactly what a real
// cycle's binding looks like at the moment the push fails.
```

### `go/internal/phases/ship/repair_selfsha_test.go:3` — above `package ship`

```text
// repair_selfsha_test.go — RED contract for repair-ladder mode #1
// (ADR-0039 §8): SELF_SHA_TAMPERED from a STALE TOFU pin.
//
// Cycles 246-248 incident: an operator manual ship (or rebuild) committed a
// new ship binary at HEAD without re-pinning expected_ship_sha (repinPostCycle
// only fires for --class cycle). The next cycle's ship then hit
// SELF_SHA_TAMPERED even though the running binary matched the binary blob
// COMMITTED AT HEAD — i.e. provably built from audited, committed source.
// The operator hand-edited state.json twice.
//
// The repair: when the running binary's SHA equals the SHA of the binary blob
// at git HEAD, the pin is stale — re-pin and re-run verifySelfSHA. When they
// differ, the binary genuinely diverges from committed source: the integrity
// BLOCK stands untouched (same trust boundary as repinPostCycle, which already
// pins from HEAD:go/evolve).
```

### `go/internal/phases/ship/repair_selfsha_test.go:26` — above `func stalePinStateJSON(t *testing.T, repo string) {`

```text
// stalePinStateJSON writes state.json with a deliberately wrong pin under the
// CURRENT plugin version — the exact signature of the cycles-246-248 incident.
```

### `go/internal/phases/ship/repocontract.go:3` — above `import (`

```text
// repocontract.go — the ship-time repo-contract scanner pack (2026-08-05).
//
// Lane ships push directly to main (per-lane landing), so a landing that
// breaks a REPO-WIDE guard suite reds main until a console fix lands. Four
// live incidents in one week, each an operator CI-email storm: the router
// Digest injection (cycle-1250), the phase-catalog metadata stub (1262), the
// tracked profile stubs (v22.13.0 release red), and the incident-postmortem
// spec rework (1313). Per-cycle changed-scope testing structurally cannot
// catch them — the guard suites scan repo-wide state (on-disk catalogs,
// tracked profiles, rendering parity) that a config-only diff never selects.
//
// The pack runs the four guard packages in the tree the ship will land — the
// lane worktree for a cycle ship (repoContractGateRoot; until 2026-09-14 the
// gate ran in the PROJECT ROOT, i.e. main's pre-landing tree, and could not
// see a lane's changes at all) — BEFORE the ship binds/pushes. They are existing deterministic tests with FP≈0 by
// construction: if one fails here, main's next run fails identically. A RED
// pack fails the ship closed with the dedicated CodeRepoContractGate
// (mirroring CodeManifestGate, cycle-1064) so the lane FAILs honestly in
// place instead of redding main. Dial: policy.json gates.repo_contract_gate
// ("enforce" default — see policy.go for the shadow-first deviation
// rationale; "off" disables).
//
// cycle-1409 — exit-code classification + forensic persistence. The original
// pack ran a bare `cmd.Run()` and wrapped ANY non-nil error as a contract RED,
// with output teed nowhere. A build-cache contention / module-fetch flake /
// OOM kill was therefore indistinguishable from a genuinely red guard suite,
// and left no artifact to disprove it with: that false RED blocked three
// audit-green ships (cycles 1402/1403/1405; the preserved worktree e0638346
// re-ran 4/4 GREEN against the identical tree, as did baseline cba017c5).
// Now the pack runs `go test -json`, classifies the events, and:
//   - a genuine test/build failure stays CodeRepoContractGate, un-retried, and
//     NAMES the failing tests in the message so ship-error.json carries them;
//   - anything else is retried exactly once, and if still unclassifiable is
//     returned as CodeRepoContractInfra — a distinct, re-dispatchable code;
//   - every run, green or red, tees the scanner output to the run dir's
//     ship-repocontract-scan.log (the green baseline is half the diagnosis).
```

### `go/internal/phases/ship/repocontract.go:59` — above `var repoContractPackages = []string{`

```text
// repoContractPackages are the repo-wide guard suites whose breakage turned
// main red. Kept to the incident-proven set deliberately: every addition
// costs every ship wall-time and must carry the same FP≈0 property.
```

### `go/internal/phases/ship/repocontract.go:69` — above `const scanLogName = "ship-repocontract-scan.log"`

```text
// scanLogName is the run-dir artifact every scanner-pack run is teed to —
// green runs included. cycle-1403's RED was undiagnosable precisely because
// no artifact of either the red run OR the green baseline survived. Kept
// unexported: nothing outside this package consumes the name today.
```

### `go/internal/phases/ship/repocontract.go:101` — above `const repoContractTestTimeout = "20m"`

```text
// repoContractTestTimeout is the per-binary deadline the gate hands `go test`.
//
// Go's default is 10m, and that default is what red-lined cycle-1679's first
// ship: this lane's added wiring test enrolled ./internal/core into the
// added-test backstop for the first time, and that package measured 355.8s
// under fleet load in the run that did pass (106.8s standalone). A package at
// 59% of the deadline is a coin flip, and when the deadline wins the panic
// makes `go test -json` emit a fail event for the running test AND every
// t.Parallel() test still paused — 19 named "failures" the gate then classes a
// real contract RED. That is a false RED on green code: the exact
// cycle-1173/1175/1178 shape.
//
// 20m is ~3.4x the measured worst case, so slow-but-green survives fleet load,
// while a genuine deadlock is still bounded rather than left to the ship's own
// context. Raising a deadline can only turn a timeout into a real verdict; it
// can never turn a failing test green.
```

### `go/internal/phases/ship/repocontract.go:371` — above `func contractRed(packName string, o packOutcome) error {`

```text
// contractRed builds the genuine-violation ship error, naming the parsed
// failing tests so ship-error.json carries them directly instead of the bare
// "exit status 1" that made cycle-1402/1403 undiagnosable.
```

### `go/internal/phases/ship/repocontract_addedtest_defects_test.go:3` — above `import (`

```text
// repocontract_addedtest_defects_test.go — cycle-1566 RED contract for the
// added-test backstop's audited defects.
//
// The backstop (repocontract.go, addedTestPackages + the "added-test backstop"
// runClassifiedPack call) closes the red-first-deliverable-reds-main incident
// class: three landings pushed a genuinely failing newly-added `*_test.go`
// with no ship-time consumer and redded main. Cycle-1559's audit reproduced
// four defects in that backstop against the real gate; they are OPEN in the
// tree today (.evolve/runs/cycle-1566/defect-dispositions.json). Each test
// below drives the REAL gate (no seam swap except where the fixed pack itself
// is the subject) against a REAL temporary git repo, so none of them can be
// satisfied by a source-level string.
//
//	H1 — a build-tag-guarded added test that genuinely FAILS is compiled out of
//	     the untagged backstop run; its package reports green and the failure
//	     ships silently. This is pinned incident 25040cea's exact shape.
//	H2 — a LONE tag-guarded added test package (the `//go:build acs` predicate
//	     file every cycle mints, including this one) has no files under the
//	     default tag set, which classifyPackEvents grades a genuine RED. That
//	     hard-blocks the lane's own honest ship with a false CodeRepoContractGate.
//	M1 — a `git diff --cached` error disables the whole backstop and writes
//	     nothing anywhere: the ship proceeds believing it was scanned.
//	M2 — the shared red message dropped the four fixed-suite names and cannot
//	     tell an operator whether the fixed pack or an added test went red.
//
// The gate-level skip case is pinned here too: the runNative half already
// exists (TestRunNative_AddedSkippedTestDoesNotBlockShip), but scout's Task 1
// acceptance criterion is stated at the gate, and that half had no test.
```

### `go/internal/phases/ship/repocontract_addedtest_defects_test.go:102` — above `func TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen(t *testing.T) {`

```text
// TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen pins H1
// — incident 25040cea's shape. The added file carries `//go:build integration`
// and fails; its package also holds an untagged green test, so an untagged
// backstop run reports the package `ok` and the ship sails with a red test in
// its diff. The backstop must run each added candidate under the build tags
// that file actually declares, and fail closed naming the failing test.
```

### `go/internal/phases/ship/repocontract_addedtest_defects_test.go:174` — above `se, ok := shiperr.AsShipError(err)`

```text
// 2026-09-14: an undiscoverable diff is an infrastructure gap, not a
// contract violation — it is the distinct, re-dispatchable INFRA class
// (as a twice-ambiguous pack run already was), never a silent green: a
// skipped guard on a red-main gate reads as a healthy ship in the ledger.
```

### `go/internal/phases/ship/repocontract_addedtest_defects_test.go:230` — above `func TestRepoContractTestArgs_CarriesAnExplicitTimeout(t *testing.T) {`

```text
// TestRepoContractTestArgs_CarriesAnExplicitTimeout pins cycle-1679's ship
// defect: the gate handed `go test` no -timeout, so every pack ran on Go's 10m
// default. ./internal/core — which this lane's own wiring test enrolled into
// the added-test backstop for the first time — measured 355.8s under fleet
// load, and when the deadline wins, the timeout panic makes `go test -json`
// emit a fail event for the running test plus every paused t.Parallel() one.
// classifyPackEvents then sees named test failures and classes it a real
// contract RED: a false RED on green code, which is exactly what blocked this
// cycle's first ship (19 named internal/core "failures", 18 of them parallel).
//
// The assertion is on the argv rather than on an observed timeout because the
// defect IS the missing flag; a test that actually waited out a deadline would
// have to burn one.
```

### `go/internal/phases/ship/repocontract_env_test.go:15` — above `func TestRunRepoContractPackages_ScrubsTheLaneIPCEnvFromGoTest(t *testing.T) {`

```text
// TestRunRepoContractPackages_ScrubsTheLaneIPCEnvFromGoTest reproduces lane
// 1677's false RED (2026-09-14): a fleet lane exports EVOLVE_FLEET=1 and
// EVOLVE_CYCLE_STATE_FILE=<its own run dir> process-wide, and the repo-contract
// gate's `go test` inherited both — so cmd/evolve's cycle-reset lease tests,
// guards' "outside a cycle" tests and ship's fleet-off goldens failed in the
// lane worktree while passing in CI and in a clean shell. The gate must hand
// `go test` the environment CI has: the lane's IPC namespace scrubbed.
```

### `go/internal/phases/ship/repocontract_importers.go:3` — above `import (`

```text
// repocontract_importers.go — the importer backstop (2026-09-14).
//
// The fixed scanner pack catches repo-wide guard suites and the added-test
// backstop catches red-first reproducers, but neither covers the shape that
// redded main from 4db205a8 until #590 (cycles 1657/1659): a lane MODIFIES a
// package, its own package tests stay green, and an UNTOUCHED test in a
// package that imports it still asserts the old contract. Per-package scope
// cannot see that edge; only the import graph can.
//
// The seed is the tree the ship will land, measured against its base — the
// one derivation in internal/changedpkgs (working tree, never the index: a
// lane's build output is unstaged until the ship itself stages it). The
// closure is changedpkgs.ImporterClosureChecked — build deps and the direct
// imports of each package's tests — projected to what `go test` can run under
// the default build context, minus what the fixed pack already ran, through
// the same classified pack runner as the other two layers.
```

### `go/internal/phases/ship/repocontract_importers.go:37` — above `importerBackstopRetryMaxTargets = 25`

```text
// importerBackstopRetryMaxTargets caps the pack size the ambiguous-exit
// retry (cycle-1402/1403/1405 class) is still worth: above it the second
// full run is more expensive than a re-dispatch, so an ambiguous exit is
// classed infra straight away.
```

### `go/internal/phases/ship/repocontract_importers_test.go:3` — above `import (`

```text
// repocontract_importers_test.go — the importer backstop and the gate's seed.
// A lane's own package tests are green, but a package that IMPORTS what the
// lane changed asserts the old contract and goes red on main's CI (cycle
// 1657/1659: the lane renamed a stop in internal/core; internal/deliverable's
// e2e test, untouched, redded main from 4db205a8 until #590). The gate now
// runs the reverse-dependency closure of every changed package — read from
// the WORKING TREE of the tree that will land, never from an index the ship
// has not yet populated — before a ship may land.
```

### `go/internal/phases/ship/repocontract_importers_test.go:144` — above `func TestRunNative_GateTestsTheLaneWorktreeNotTheProjectRoot(t *testing.T) {`

```text
// The wiring proof: a cycle ship's changes live in the LANE WORKTREE, and
// the gate must test that tree against the worktree's base. The project root
// (main's pre-landing tree) is untouched here — a gate that ran there, as it
// did until 2026-09-14, passes green and lets the red importer land.
```

### `go/internal/phases/ship/repocontract_repro_test.go:9` — above `func TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded(t *testing.T) {`

```text
// TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded reproduces
// H3 from cycle 1559: a textual mention of a build constraint is not itself a
// build constraint. The current detector excludes this real failing test,
// allowing the ship to proceed with a red test in its staged diff.
```

### `go/internal/phases/ship/repocontract_test.go:3` — above `import (`

```text
// repocontract_test.go — the ship-time repo-contract scanner pack. The gate
// exists because four lane landings redded main in one week; these tests pin:
// off skips, enforce-green passes, enforce-RED fails with the DEDICATED code
// (never a git-failure alias), unknown stage fails toward enforce, and the
// module dir is the lane worktree's go/.
//
// cycle-1409 adds the classification + persistence contract: a genuine RED is
// never retried and names its failing tests, an unclassifiable failure is
// retried EXACTLY once, a twice-unclassifiable failure is the distinct infra
// code, and every run tees its output to the run-dir scan log.
```

### `go/internal/phases/ship/repocontract_test.go:78` — above `func ambiguousPack() packOutcome {`

```text
// ambiguousPack is the cycle-1402/1403/1405 shape: nonzero exit, but not one
// guard suite reported a failure. The toolchain died, the contract did not.
```

### `go/internal/phases/ship/repocontract_test.go:137` — above `func TestNew_ThreadsRepoContractGate(t *testing.T) {`

```text
// TestNew_ThreadsRepoContractGate is the cycle-1064 anti-trap: the production
// construction site must thread the dial or the gate is permanently off no
// matter what policy says.
```

### `go/internal/phases/ship/repocontract_test.go:148` — above `func TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry(t *testing.T) {`

```text
// TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry — AC2, the
// crux anti-regression of the cycle-1409 rework. A pack that NAMES a failing
// test is a genuine violation: it must still fail the ship closed with
// CodeRepoContractGate and must run EXACTLY ONCE. Retrying a real RED both
// doubles every red ship's wall-time and gives a flaky-but-real suite a second
// chance to pass by luck, laundering the violation onto main.
```

### `go/internal/phases/ship/repocontract_test.go:174` — above `func TestRepoContractGate_TransientFailureRetriesOnceThenShips(t *testing.T) {`

```text
// TestRepoContractGate_TransientFailureRetriesOnceThenShips — AC3, the defect
// this cycle exists to kill. Attempt 1 exits nonzero with no test-level
// failure (build-cache contention / OOM kill), attempt 2 is clean: the ship
// MUST proceed. This is exactly what would have unblocked the audit-green
// cycles 1402/1403/1405.
```

### `go/internal/phases/ship/repocontract_test.go:216` — above `func TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns(t *testing.T) {`

```text
// TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns — AC6. Red-only
// persistence is the exact gap that made cycle-1403 undiagnosable: proving a
// RED false needs the green baseline from the same artifact path.
```

### `go/internal/phases/ship/repocontract_test.go:250` — above `func TestRepoContractGate_RedErrorMessageNamesFailingTests(t *testing.T) {`

```text
// TestRepoContractGate_RedErrorMessageNamesFailingTests — AC7. The parsed
// failing test names must ride in the ship error so ship-error.json carries
// them directly, instead of the generic "(exit status 1)" that forced a manual
// worktree re-run to diagnose cycle-1402.
```

### `go/internal/phases/ship/repocontract_test.go:543` — above `func TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded(t *testing.T) {`

```text
// TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded — T1 AC3, the
// third pinned incident shape (cycle-1559 addition to the red-first lane). A
// newly added test file that is genuinely un-runnable on a quiet host (the
// `requires_tmux` build-constraint convention scout flagged) must NEVER be
// silently claimed green: with no `-tags requires_tmux`, its lone-file
// package has "build constraints exclude all Go files" — indistinguishable
// from a genuine compile break unless the gate special-cases it — while
// simply skipping the package with no record at all launders it as
// "nothing to see here". Both are dishonest. The gate must (1) not fail the
// ship over an env-exclusive candidate it correctly cannot run, and (2)
// leave a durable, explicit backstop record in the scan log naming the file
// and its exclusion reason so the coverage gap is auditable, not silent.
```

### `go/internal/phases/ship/runscope_test.go:12` — above `func TestReadActiveWorktree_PrefersRunJSON_OverGlobal(t *testing.T) {`

```text
// TestReadActiveWorktree_PrefersRunJSON_OverGlobal pins ADR-0049 S3 / gap G3:
// when a run workspace is set, ship reads active_worktree from the per-run
// run.json mirror, NOT the host-global cycle-state.json — so a concurrent
// cycle's global write can't make ship integrate the WRONG run's worktree.
// RED before readActiveWorktree consults cycleStateFile (returns the global
// value), GREEN after.
```

### `go/internal/phases/ship/runscope_test.go:54` — above `func TestFindLatestAudit_PrefersThisRunsEntry(t *testing.T) {`

```text
// TestFindLatestAudit_PrefersThisRunsEntry pins ADR-0049 S4 / gap G5: with a
// runID set, ship binds to THIS run's auditor entry, not a concurrent run's
// later one. Ledger: run B auditor (older) then run A auditor (newer/latest);
// findLatestAudit(ledger,"B") must return B, not the latest A. RED before the
// run-filter (returns A), GREEN after.
```

### `go/internal/phases/ship/runscope_test.go:89` — above `func TestFindLatestAudit_RunIDNoMatch_RefusesUnstampedBind(t *testing.T) {`

```text
// TestFindLatestAudit_RunIDNoMatch_RefusesUnstampedBind: runID set but every
// auditor entry is unstamped → hard integrity stop (NO_AUDITOR), never a bind.
// FLIPPED 2026-08-26 from _FallsBackToLatest: the old pin's "zero regression
// for pre-S4 ledgers" premise is dead (every current recorder stamps run_id),
// and cycle-1571 proved the fallback is the H3 fail-open hole — a FAILed
// cycle's ship bound cycle-1570's audit and returned AUDIT_BINDING_HEAD_MOVED
// instead of this run's FAIL, burning a re-audit slot; had the foreign entry's
// git_head matched HEAD, the FAILed cycle would have SHIPPED on a sibling's
// PASS. "This run produced no independent review" is an integrity stop, not a
// recoverable lookup miss.
```

### `go/internal/phases/ship/runscope_test.go:108` — above `func TestFindLatestAudit_ForeignRunOnly_RefusesBind(t *testing.T) {`

```text
// TestFindLatestAudit_ForeignRunOnly_RefusesBind pins cycle-1571's exact H3
// shape: the only auditor entries belong to a DIFFERENT run (a sibling lane in
// the same HEAD window). Binding them lets one cycle's ship gate be satisfied
// by another cycle's audit; the error must name the refused foreign entry so
// an operator can see what would have been bound.
```

### `go/internal/phases/ship/s5b_test.go:49` — above `func TestShipFromWorktree_FleetMode_DivergedFFMerge_SignalsRebase(t *testing.T) {`

```text
// TestShipFromWorktree_FleetMode_DivergedFFMerge_SignalsRebase pins ADR-0049 S5b:
// under fleet mode a ff-merge divergence (a peer cycle moved main) is the
// EXPECTED concurrency case, not a terminal failure — ship signals
// GIT_FLEET_REBASE_NEEDED (transient) so the orchestrator rebases + re-verifies
// the merged tree and re-ships. RED before the fleet branch (returns the
// terminal GIT_FF_MERGE_DIVERGED), GREEN after.
```

### `go/internal/phases/ship/ship.go:33` — above `PhaseIO config.Stage`

```text
// PhaseIO threads the EVOLVE_PHASE_IO stage into the audit-binding verdict
// parse (ADR-0050 §3.10 Slice 6). Zero value (StageOff) = byte-identical
// (prose parse). Set by the composition root (cmd_cycle.go) from cfg.PhaseIO.
```

### `go/internal/phases/ship/ship.go:37` — above `ManifestGate string`

```text
// ManifestGate threads the policy.json `gates.manifest_gate` rollout dial
// (policy.GatesConfig().ManifestGate) into Options.ManifestGate for the
// ship-bind manifest reconciliation (cycle-1064). Raw resolved stage string,
// not a config.Stage, because manifest.go's seam is value-compared against
// ManifestGateEnforce. Zero value "" = shadow (byte-identical).
```

### `go/internal/phases/ship/ship.go:43` — above `RepoContractGate string`

```text
// RepoContractGate threads policy.json gates.repo_contract_gate into the
// ship-time repo-contract scanner pack (repocontract.go). Zero value ""
// = off for construction-site safety; the cmd_cycle wiring test asserts
// the production site threads the resolved default ("enforce") so the
// cycle-1064 silently-unwired trap cannot recur.
```

### `go/internal/phases/ship/ship.go:49` — above `Signals *signalcenter.Center`

```text
// Signals is the root's Signal Center (ADR-0103 unit 07): the landing's
// ship.warning events reach it through Options.Signals. The orchestrator
// root (cmd_cycle.go) passes its Center; the `evolve phase ship` registry
// factory leaves it nil (the Null Object — a subprocess root has no
// Center yet, 07-F10).
```

### `go/internal/phases/ship/ship.go:77` — above `func (p *Phase) signalsWired() bool { return p.signals != nil }`

```text
// signalsWired reports whether the phase carries a Signal Center for the
// landing's warnings — the in-package wiring test's handle (ADR-0103 unit
// 07; unexported: its only consumer is TestShipOptions_ThreadsSignals, and the
// composition root pins its own site by source scan).
```

### `go/internal/phases/ship/ship.go:88` — above `if core.DocumentCycle(req.Workspace) {`

```text
// ADR-0099 slice 2: a document cycle lands under `solution(<slug>)` so the
// commit-prefix vocabulary names the deliverable it carries (the kernel's
// own digest of the triage header decides the kind; the triage decision
// names the slugs). Code cycles keep the legacy message byte-identical.
```

### `go/internal/phases/ship/ship.go:114` — above `msg := req.Context["commit_message"]`

```text
// An explicit Context["commit_message"] always wins. When absent, synthesize
// a deterministic message from the cycle identity rather than failing the
// ship (single seam, cycle-147 lesson): `evolve cycle run` populates this
// Context key but the autonomous-loop construction path (cmd_loop.go
// buildCycleContext) does not, so erroring here silently sank EVERY loop
// cycle at the ship step once the audit gate finally let cycles reach ship
// (cycle-150). Defaulting here covers all current and future callers; the
// manual `evolve ship` CLI always passes an explicit message and is
// unaffected.
// ADR-0050 §3.10 Slice 4: read commit_message from the typed envelope at
// enforce, the legacy Context map below it (byte-identical — Active() is false
// unless enforce). The empty→defaultCommitMessage fallback below covers both
// paths, so an active envelope with no commit message still synthesizes one.
```

### `go/internal/phases/ship/ship.go:144` — above `func NewWithDefaultRunnerStage(stage config.Stage) *Phase {`

```text
// NewWithDefaultRunnerStage is NewWithDefaultRunner plus the EVOLVE_PHASE_IO stage
// (ADR-0050 §3.10 Slice 6). The composition root (cmd_cycle.go) passes cfg.PhaseIO
// so the audit-binding verdict parse is sentinel-first at >= StageEnforce.
// NewWithDefaultRunner stays as the StageOff (byte-identical) convenience.
```

### `go/internal/phases/ship/ship.go:152` — above `func (p *Phase) shipOptions(req core.PhaseRequest, msg string) Options {`

```text
// shipOptions is the PhaseRequest → Options translation, extracted from
// runNative so the sole production construction site is directly testable
// (cycle-1064: Options.ManifestGate was silently never assigned here, leaving
// the manifest gate permanently shadow no matter what policy.json said).
```

### `go/internal/phases/ship/ship.go:161` — above `WorkspacePath:                   req.Workspace,`

```text
// ADR-0049 S3 / G3: run-scope ship's reads
```

### `go/internal/phases/ship/ship.go:162` — above `RunID:                           req.RunID,`

```text
// ADR-0049 S4 / G5: run-scope the audit binding
```

### `go/internal/phases/ship/ship.go:172` — above `PhaseIO:                         p.phaseIO,`

```text
// ADR-0050 §3.10 Slice 6: sentinel-first verdict parse at enforce
```

### `go/internal/phases/ship/ship.go:175` — above `Signals:                         p.signals,`

```text
// ADR-0103 unit 07: the landing's ship.warning events reach the root's Center
```

### `go/internal/phases/ship/ship.go:182` — above `opts := p.shipOptions(req, msg)`

```text
// Repo-contract scanner pack BEFORE bind/push: a repo-wide guard suite
// RED in this lane must fail the ship here, not on main (repocontract.go).
// req.Workspace is the run dir: the gate tees the scanner output there
// (ship-repocontract-scan.log) on green AND red runs. Threading it here is
// the load-bearing half — a log seam reachable only from a test is dead
// code (the cycle-1064 manifest-gate anti-trap, applied to this parameter).
```

### `go/internal/phases/ship/ship.go:252` — above `func addRepairSignals(signals map[string]any, res RunResult) {`

```text
// addRepairSignals mirrors the repair ladder's observability fields
// (ADR-0039 §8) onto the generic signal plane — present on both PASS
// (self-healed) and FAIL (repair declined) responses.
```

### `go/internal/phases/ship/ship.go:262` — above `func init() {`

```text
// init self-registers the ship phase factory with the phase registry, like
// every other built-in phase. The subprocess dispatcher (internal/cli/phasecmd)
// resolves phases by name and never constructs ship directly — keeping the flow
// phase-agnostic (ADR-0035/0038). The orchestrator's in-process path
// (cmd_cycle.go) builds ship with a stage-threaded constructor; this registry
// factory serves the `evolve phase ship` subprocess entrypoint with the
// default (StageOff) runner, matching the prior phasecmd wiring byte-for-byte.
```

### `go/internal/phases/ship/ship.go:275` — above `func repoContractGateRoot(opts *Options) (root, baseRef string, err error) {`

```text
// repoContractGateRoot is the gate's projection of landingTree — the tree the
// ship will land, tested against the base its changes are measured from: the
// worktree's base SHA when the ship lands from a worktree and knows it, else
// HEAD. Until 2026-09-14 the gate ran in req.ProjectRoot, main's pre-landing
// tree, so on every lane ship it tested a tree without the lane's changes.
// An unresolvable typed worktree fails closed here with the same
// CodeWorktreeResolve atomicShip raises: the gate never tests the project
// root in the lane's stead and reports a misleading green.
```

### `go/internal/phases/ship/ship_gate_phaseio_test.go:10` — above `func TestShipGate_EnforceSentinelFirst(t *testing.T) {`

```text
// ADR-0050 §3.10 Slice 6: at enforce the ship gate's verdict parse is sentinel-first
// — the single-valued evolve-verdict sentinel is authoritative and the prose regex
// (which can match multiple verdict words and trip the dual-verdict guard) is gated
// off. Below enforce parseVerdicts stays prose-only — byte-identical.
```

### `go/internal/phases/ship/ship_phaseio_test.go:14` — above `func shipOnce(t *testing.T, req core.PhaseRequest) string {`

```text
// ADR-0050 §3.10 Slice 4: ship reads commit_message from the typed envelope at
// enforce (req.Input.Active()) and the legacy Context["commit_message"] below it.
// The empty→defaultCommitMessage(req) fallback (cycle-150 lesson) is preserved on
// both paths. We assert the resolved message reached the commit via HEAD's subject,
// matching the existing TestPhaseRun_DefaultCommitMessage_WhenContextMissing seam.
```

### `go/internal/phases/ship/ship_phaseio_test.go:63` — above `t.Run("typed_path_enforce", func(t *testing.T) {`

```text
// Enforce path: the typed envelope is consulted even with NO Context. RED before
// the fix — map read returns "" → defaultCommitMessage → subject "evolve-cycle 7".
```

### `go/internal/phases/ship/ship_solution_prefix_test.go:11` — above `func TestDefaultCommitMessage_SolutionPrefix(t *testing.T) {`

```text
// TestDefaultCommitMessage_SolutionPrefix — ADR-0099 slice 2: a document cycle
// lands under the `solution(<slug>)` prefix so the commit vocabulary names the
// deliverable it carries; code cycles keep the legacy message byte-identical.
```

### `go/internal/phases/ship/shipgate_attestation_amplified_test.go:1` — above `package ship`

```text
// shipgate_attestation_amplified_test.go — Test Amplifier (cycle 987).
//
// Adversarial additions on top of the TDD-contracted binding tests
// (TestShipGate_{Stale,Fresh,Missing}Attestation{Blocked,Passes} in
// shipgate_attestation_binding_test.go). These probe malformed/empty input,
// the "untracked .commit-gate/ never perturbs the diff" assumption the TDD
// contract states as fact, a time-of-check-to-time-of-use staleness gap, and
// a large-diff scale case. Written black-box against tdd-contract.md /
// build-report.md prose only — verifyCommitGateAttestation's body and the
// Builder's new binding test file were deliberately not read, to avoid
// anchoring test design on the implementation.
```

### `go/internal/phases/ship/shipgate_attestation_binding_test.go:1` — above `package ship`

```text
// shipgate_attestation_binding_test.go — cycle-987 gate-wiring binding tests
// for verifyCommitGateAttestation (commitgate.go), UNGUARDED (default suite).
//
// WHY UNGUARDED (the point of the task): the pre-existing coverage for the
// commit-attestation triangle (TestCommitGate_Manual*Attestation_* in
// commitgate_test.go) sits behind //go:build integration, so a severed
// verifyCommitGateAttestation wire is invisible to plain `go test ./...`.
// These three tests drive the enforcer directly — stale → block, fresh →
// pass, missing → block — in the DEFAULT build suite, so the wire is caught
// by normal CI. They call the real enforcer against a real .commit-gate
// attestation fixture (writeAttestation) and a real git tree (makeRepo /
// treeStateSHA), reusing the untagged helpers in realgit_testhelpers_test.go.
//
// verifyCommitGateAttestation computes computeTreeStateSHA → treestate.SHA →
// sha256(`git diff HEAD`), which is byte-identical to treeStateSHA(t, repo);
// the .commit-gate/ fixture is untracked so it never perturbs that diff (no
// git add is performed by the enforcer). Options.runner() defaults to
// sysexec.DefaultRunner when unset, so a bare Options{ProjectRoot: repo} runs
// real git in the fixture repo.
```

### `go/internal/phases/ship/stage_classify_stderr_test.go:3` — above `import (`

```text
// stage_classify_stderr_test.go — RED contract for cycle-1473 task
// `gitstage-deterministic-classification`.
//
// Defect (cycle-1098 and cycle-1101, both live): stageExplicitPaths assigns
// core.ShipClassTransient to the FIRST `git add` failure unconditionally, so the
// recovery ladder re-dispatched a byte-identical add twice for failures git had
// already declared unwinnable — an absolute pathspec (`fatal: Invalid path
// '/go/bin/evolve'`, rc=128) and a gitignore refusal (`The following paths are
// ignored …`, rc=1). Pure retry burn, and the failure digest recorded a
// "transient" that was nothing of the sort.
//
// Contract under test (RED until Builder adds the classifier): the recovery
// class is derived from the CAPTURED git stderr plus the exit code BEFORE the
// two-strikes fallback runs.
//
//	rc=128 + `fatal: Invalid path …`                     → non-transient
//	rc=128 + `… is outside repository at …`              → non-transient
//	rc=128 + `fatal: pathspec … did not match any files` → non-transient
//	rc=1   + `The following paths are ignored …`         → non-transient
//	rc=128 + `Unable to create … index.lock: File exists` → TRANSIENT (contention)
//	any unrecognised stderr                               → TRANSIENT (degrade)
//
// Trust boundary (the load-bearing negative, from the scout's beyond-the-ask
// hypothesis): the classifier reads opts-captured `git_stderr` ONLY, never the
// message Go composes around it — so an error-wrapper edit can never move a
// failure between recovery routes. TestStageFailureClassification/
// go_error_text_alone_does_not_classify pins that: the deterministic phrase is
// present in the Go error (and therefore in ShipError.Message) while git's own
// stderr is empty, and the class must stay transient.
```

### `go/internal/phases/ship/stage_classify_stderr_test.go:206` — above `func TestStageFailureClassification_TwoStrikesStillApplies(t *testing.T) {`

```text
// TestStageFailureClassification_TwoStrikesStillApplies is the regression guard
// for the cycle-1440 router that already ships: an UNRECOGNISED refusal keeps
// its first retry and still escalates on the second consecutive attempt with the
// same pathspec. The new stderr classifier must sit in FRONT of that rule, not
// replace it.
```

### `go/internal/phases/ship/stage_deleted_test.go:55` — above `func TestStageExplicitPaths_AlreadyStagedRename(t *testing.T) {`

```text
// TestStageExplicitPaths_AlreadyStagedRename is the sibling of the deletion
// case above, and it is a DIFFERENT porcelain shape: git reports a staged
// rename as `R  old -> new`, never as `D  old`, so the "D " filter does not
// see the source path — yet the source is gone from the worktree AND from the
// index under its old name, which is exactly the rc=128 condition.
//
// Real trigger (2026-07-30 console): reconciling the inbox moves each consumed
// item from .evolve/inbox/<ts>-<id>.json to .evolve/inbox/consumed/<date>-<id>.json
// with one field appended, so git's similarity detection reports renames rather
// than delete+add, and every boundary ship carrying a consumed item died on
// `fatal: pathspec '.evolve/inbox/…-spine-failopen-telemetry.json' did not
// match any files`. This has been carried as a known operator gotcha
// ("ship-staging RENAME rc=128") instead of being fixed.
```

### `go/internal/phases/ship/stage_explicit_paths_integration_test.go:3` — above `package ship`

```text
// stage_explicit_paths_integration_test.go — real-git half of the cycle-1067
// `ship-stage-explicit-paths` contract. The unit half (stage_explicit_paths_test.go)
// pins the git ARGUMENTS via a capture runner; this pins the OBSERVABLE EFFECT
// against a genuine repository: the ship commit contains the declared path and
// does NOT contain an undeclared untracked stray that `git add -A` would sweep
// in (the cross-lane leak of cycle-645).
```

### `go/internal/phases/ship/stage_explicit_paths_test.go:1` — above `package ship`

```text
// stage_explicit_paths_test.go — RED contract for inbox item
// `ship-stage-explicit-paths` (cycle-1067).
//
// Defect: shipDirect (gitops.go:228) and shipFromWorktree (gitops.go:374)
// stage with `git add -A` for the non-release classes. That sweeps whatever
// happens to be dirty in the tree — under a fleet, typically a sibling lane's
// untracked leak (cycle-645) — into the ship commit, and it violates the
// standing repo convention `git_add_explicit_paths`. The release class already
// does the right thing (stageReleaseSet, gitops.go:707: `git add -- <paths>`);
// the cycle/manual paths were simply never migrated.
//
// Contract pinned here:
//  1. Neither shipDirect nor shipFromWorktree invokes `git add -A` for
//     ClassCycle / ClassManual — staging is an explicit `git add -- <paths>`.
//  2. The staged path list is the DECLARED manifest (build-report.md +
//     test-report.md, the set declaredManifest already computes for the
//     manifest gate) when the workspace has readable phase reports.
//  3. When no manifest is readable (no workspace, or no reports), staging
//     falls back to the porcelain-status changed set — it must NOT silently
//     skip staging (that would produce a false clean-exit / empty ship) and it
//     must NOT fall back to `add -A`.
```

### `go/internal/phases/ship/stage_explicit_paths_test.go:46` — above `refuseAddPaths []string`

```text
// refuseAddPaths: pathspecs that make an `add` call refuse rc=1 with the
// real refusal stderr naming them — the layer-4 class where the
// check-ignore probe is BLIND to a path git add still refuses
// (directory-form rules; 2026-08-14 halt). A retry without them succeeds.
```

### `go/internal/phases/ship/stage_ignored_dir_integration_test.go:3` — above `package ship`

```text
// stage_ignored_dir_integration_test.go — real-git half of the layer-4
// directory-form contract (2026-08-14 batch halt). The unit half pins the
// refusal parser; this pins the OBSERVABLE EFFECT: a declared DIRECTORY whose
// ignore rule is the `dir/` form (invisible to check-ignore, refused by add)
// must not kill the ship — git names it, the stager drops it and retries, the
// real change lands.
```

### `go/internal/phases/ship/stage_ignored_dir_test.go:3` — above `import (`

```text
// stage_ignored_dir_test.go — layer 4 of the staging onion (after cycle-1098
// rc=128 absolute-pathspec, cycle-1101 rc=1 ignored-path, cycle-1108
// quotepath): the 2026-08-14 batch halt, fingerprint ship|unknown|99c38818.
//
// Defect, proven against real git: `git check-ignore` reports NOTHING for a
// DIRECTORY path (either slash form) when the ignore rule is the `dir/` form
// (.gitignore `.evolve/inbox/processed/`), so dropIgnoredPaths keeps the
// declared directory and `git add -A -- <dir>` refuses rc=1 ("The following
// paths are ignored…"). Three lanes hit the identical refusal → identical
// fingerprint ×3 → pipeline-blocker halt.
//
// The fix refuses to re-implement ignore semantics a third time: git's own
// refusal stderr NAMES the offending pathspecs verbatim — parse them, drop
// them (loudly), retry the add ONCE. Git stays the single source of truth for
// what is ignored, for every rule form, forever.
```

### `go/internal/phases/ship/stage_ignored_dir_test.go:52` — above `func TestIgnoredPathsFromAddRefusal_DecodesQuotedPaths(t *testing.T) {`

```text
// Quoted (non-ASCII) offender lines decode through the same unquoteGitPath the
// rest of the staging onion uses (cycle-1108 contract holds here too).
```

### `go/internal/phases/ship/stage_ignored_paths_integration_test.go:3` — above `package ship`

```text
// stage_ignored_paths_integration_test.go — real-git half of the cycle-1101
// gitignored-declared-path contract. The unit half (stage_ignored_paths_test.go)
// pins the git ARGUMENTS via a capture runner; this pins the OBSERVABLE EFFECT
// against a genuine repository — required by the adversarial review of the
// first fix attempt, whose `check-ignore -z` (stdin-mode-only flag) made the
// probe rc=128 on every ship: the capture-runner unit tests stayed green while
// the production feature was dead on arrival. Only a real git can catch that
// class.
```

### `go/internal/phases/ship/stage_ignored_paths_test.go:1` — above `package ship`

```text
// stage_ignored_paths_test.go — regression lock for the cycle-1101 ship fatal
// (2026-07-27, batch-12 attempt-2): the eval-quality contract makes every
// test-report declare its eval file (`.evolve/evals/<slug>.md`), and .evolve/*
// is gitignored BY DESIGN (runtime state, never committed). The declared
// manifest therefore always contains an ignored path, and `git add -A --
// <paths>` REFUSES with rc=1 ("The following paths are ignored by one of your
// .gitignore files") — it stages the legit paths and STILL exits 1, so every
// green cycle aborted at ship after two futile "transient" retries. Layer 2 of
// the staging onion: cycle-1098's absolute-pathspec rc=128 fatal (fixed at
// d202aeb6) fired first and masked this one.
//
// Contract: stageExplicitPaths pre-filters the pathspec through
// `git check-ignore` — ignored entries are dropped and logged, never handed to
// `git add`. A broken check-ignore probe fails OPEN (full set, loud log): a
// probe failure must not block ship.
```

### `go/internal/phases/ship/stage_ignored_paths_test.go:31` — above `evalRel := ".evolve/evals/persona-budget-inlane-gate.md"`

```text
// The eval file exists in the tree (isFile passes — that is how it slipped
// into the pathspec on cycle-1101).
```

### `go/internal/phases/ship/stage_quotepath_test.go:1` — above `package ship`

```text
// stage_quotepath_test.go — RED contract for cycle-1108 top_n task
// `gitstage-quotepath-determinism` (layer 3 of the staging onion, after
// cycle-1098's absolute-pathspec rc=128 fix `d202aeb6` and cycle-1101's
// ignored-path rc=1 fix `e8990e53`).
//
// Defect: ship classifies "what to stage" from `git status --porcelain` and
// `git check-ignore` output, but neither reader accounts for git's C-quoting.
// Verified against real git (2026-07-27):
//
//	$ git status --porcelain -uall
//	?? "caf\303\251.txt"          # non-ASCII → octal-escaped, quoted
//	?? "we\"ird.txt"              # embedded quote → backslash-escaped, quoted
//	?? "with space.txt"           # space → quoted, but NOT escaped
//	$ git -c core.quotePath=false status --porcelain -uall
//	?? café.txt                   # raw UTF-8 — the one-line fix for the common case
//	?? "we\"ird.txt"              # quotePath=false does NOT cover this residue
//
// porcelainChangedPaths (manifest.go:198) only strips wrapping quotes, so
// `"caf\303\251.txt"` becomes the literal 15-byte string `caf\303\251.txt` —
// a path that exists on no disk. That corrupted token flows into stagePathspec
// (`git add -A -- <paths>`) and manifestCovers, so the file is silently
// misclassified. dropIgnoredPaths (gitops.go:801) has the mirror exposure: it
// trims whitespace only, so a quoted check-ignore line never matches the raw
// declared path it is meant to filter, and the ignored path survives into the
// add — reproducing the exact cycle-1101 rc=1 ship-killer for the non-ASCII
// input class.
//
// Contract pinned here:
//  1. porcelainChangedPaths decodes C-quoted entries (octal bytes, \" and \\,
//     control escapes) to the literal on-disk path, both sides of a rename.
//  2. ASCII entries — including the quoted-but-unescaped space case, which
//     today's Trim already handles — are byte-identical after the change.
//  3. The `status --porcelain` and `check-ignore` reads are issued with
//     `-c core.quotePath=false`, so the common non-ASCII case never reaches
//     the parser escaped at all.
//  4. dropIgnoredPaths matches a quoted probe line against the raw path it is
//     filtering (drops it), and never drops a path the probe did not name.
```

### `go/internal/phases/ship/stage_quotepath_test.go:215` — above `func TestDropIgnoredPaths_QuotePathMatchesQuotedProbeOutput(t *testing.T) {`

```text
// TestDropIgnoredPaths_QuotePathMatchesQuotedProbeOutput — AC4, positive half.
// An ignored non-ASCII (or quote-bearing) declared path must be dropped even
// when git reports it C-quoted; otherwise it rides into `git add`, which exits
// 1 on any ignored pathspec — the cycle-1101 ship-killer, narrowed to this
// input class.
```

### `go/internal/phases/ship/stage_quotepath_test.go:300` — above `func TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// cycle-1469 top_n task `gitstage-rename-arrow-parse` — RED contract.
//
// Cycle-1466 audit H2 left this open: both porcelain rename readers treat
// " -> " as an unconditional token separator over the WHOLE payload —
//
//	porcelainChangedPaths: strings.Split(line[3:], " -> ")   (manifest.go:246)
//	stagedGonePaths:       strings.Cut(line[3:], " -> ")     (manifest.go:291)
//
// — but ` -> ` is a legal byte sequence inside a filename, and git quotes such
// a name rather than escaping the spaces. Verified against real git 2.50.1
// (2026-08-15):
//
//	$ git status --porcelain
//	?? "we -> ird.txt"              # space-quoted, NOT backslash-escaped
//	$ git -c core.quotePath=false status --porcelain
//	?? "we -> ird.txt"              # quotePath=false does NOT suppress this
//
// So a staged rename of that file prints `R  "we -> ird.txt" -> renamed.txt`,
// and today's readers tear it apart:
//
//	Split → ["\"we", "ird.txt\"", "renamed.txt"]   3 fragments, none decodable
//	Cut   → gone["\"we"]                            a path on no disk
//
// The fragments are unbalanced-quote tokens, so unquoteGitPath returns them
// verbatim and they flow straight into the `git add -- <paths>` pathspec.
// `git add` exits 128 ("did not match any files") on the first such token and
// fails the ENTIRE add — the rc=128 ship-killer stagedGonePaths exists to
// prevent, reproduced by the very input class it was written for. Inbox
// reconciliation produces renames by construction, so this is a live boundary-
// ship hazard, not a hypothetical.
//
// Contract pinned here:
//  1. Only the STRUCTURAL rename delimiter splits — a ` -> ` inside quoted
//     path content is retained, and both endpoints decode to literal paths.
//  2. stagedGonePaths takes the same quote-aware source endpoint.
//  3. Malformed quoted input (unbalanced quote, empty side, bare arrow) is
//     safe: no panic, no fragments, degrade to the verbatim token.
//  4. Ordinary unquoted renames and non-rename lines are byte-identical to
//     pre-change behaviour.
// ---------------------------------------------------------------------------
```

### `go/internal/phases/ship/stage_refusal_deterministic_test.go:3` — above `import (`

```text
// stage_refusal_deterministic_test.go — RED contract for cycle-1440 task
// `deterministic-stage-refusal-router`.
//
// Defect (cycle-1365, live): stageExplicitPaths classifies EVERY `git add`
// refusal as core.ShipClassTransient, so the failure floor keeps re-dispatching
// a refusal that can never succeed in place. Cycle 1365 burned its whole retry
// budget on the SAME .evolve/evals pathspec refused twice — its worktree base
// predated the .gitignore carve-out, so no retry could ever win.
//
// Contract under test (not yet implemented — RED until Builder adds the
// two-strikes rule): the FIRST refusal of a given pathspec stays TRANSIENT (a
// genuinely flaky add must keep its retry), and a SECOND CONSECUTIVE refusal of
// the SAME pathspec is reclassified core.ShipClassPrecondition — deterministic,
// so the router stops burning attempts and routes to continuation/salvage. A
// refusal of a DIFFERENT pathspec is a different failure and resets to transient.
//
// The refusal memory is per-workspace (opts.WorkspacePath), which is what makes
// "consecutive" observable across the separate ship attempts of one cycle.
```

### `go/internal/phases/ship/stage_refusal_deterministic_test.go:30` — above `func stagingRefusalRunner(porcelain string) *scriptedRunner {`

```text
// stagingRefusalRunner scripts `git status --porcelain` to report the given
// changed paths and makes `git add` refuse with an UNRECOGNISED stderr shape.
//
// Cycle-1473 re-base: this file's fixture was git's rc=1 gitignore-advice
// refusal, which the `gitstage-deterministic-classification` contract now
// classifies non-transient on the FIRST failure from captured git_stderr (see
// stage_classify_stderr_test.go). Keeping that fixture here would assert two
// contradictory classes for one stderr. The two-strikes rule these tests exist
// to pin is orthogonal to the stderr shape, so they now run on a stderr the
// classifier cannot place — which is exactly where the strike memo is still the
// only signal, and their original intent (a first, possibly-flaky refusal keeps
// its retry; the same pathspec twice does not) is preserved unchanged.
```

### `go/internal/phases/ship/statefile.go:23` — above `func readStateMap(path string) (map[string]any, error) {`

```text
// readStateMap parses path as JSON into a map. Missing/empty → empty map.
// Thin projection of the single-source statemap.ReadStateMap (cycle-659); the
// ship package keeps the short name its many call sites already use.
```

### `go/internal/phases/ship/statefile.go:30` — above `func writeStateMap(path string, m map[string]any) error {`

```text
// writeStateMap atomically replaces path with the JSON of m via the
// single-source statemap.WriteStateMap (cycle-659). Callers hold withStateLock
// around read+write themselves, so this stays the UNLOCKED primitive.
```

### `go/internal/phases/ship/statefile.go:94` — above `func lockStateFile(statePath string) (release func(), err error) {`

```text
// lockStateFile acquires the advisory lock that serializes state.json
// read-modify-writes — the SAME <path>.lock storage.UpdateState holds
// (ADR-0049 S2 / gap G2). flock is BLOCKING and per-open-file-description, so
// ship's map-based RMW and the typed UpdateState/allocator writers never
// interleave (the lost-update / stale-pin class). Projects through
// flock.PathLock — the single home for the "<file>.lock" sidecar suffix.
// Callers that need a phase-specific ShipError on lock failure
// use this directly + `defer release()`; the rest use withStateLock. A no-op
// for the live loop (the whole-cycle project lock already serializes ship vs
// the allocator); this joins the CA.3 lock domain so it stays correct once the
// coarse lock is scoped per-run.
```

### `go/internal/phases/ship/statelock_test.go:13` — above `func TestWithStateLock_SerializesWithUpdateState(t *testing.T) {`

```text
// TestWithStateLock_SerializesWithUpdateState pins ADR-0049 S2 / gap G2: ship's
// map-based state.json read-modify-write must hold the SAME advisory lock
// storage.UpdateState holds (<path>.lock), so the two whole-file writers cannot
// clobber each other. Half the goroutines bump an UNMODELED key via
// withStateLock; the other half bump a MODELED key via UpdateState — all on one
// state.json. Without a shared lock the interleaved whole-file writes lose
// updates on BOTH counters (RED); with the shared flock every write serializes
// and both counters reach the full total (GREEN). Run with -race.
```

### `go/internal/phases/ship/trivialrebase_carryforward_test.go:3` — above `package ship`

```text
// trivialrebase_carryforward_test.go — cycle-786 TDD contract for merge
// ladder RUNG 0 (inbox merge-rung0-trivial-rebase-carryforward; research
// knowledge-base/research/merge-concurrency-2026: verdicts follow the CHANGE
// via git patch-id, gates follow the TREE).
//
// Contract encoded here, at the existing verifyAuditBinding seam:
//
//   - A "composition-verdict" ledger entry (kind=composition-verdict,
//     method=trivial-rebase) written after a conflict-free rebase whose
//     git patch-id is UNCHANGED vs the audited diff lets verifyAuditBinding
//     accept the audit+composition chain instead of hard-failing
//     CodeAuditBindingHeadMoved — no fresh Auditor dispatch.
//   - Ship must kernel-recompute the composed tree's patch-id live (git
//     diff HEAD | git patch-id --stable) and reject an entry whose recorded
//     patch_id does not match — drift falls through to full re-audit.
//   - The entry's gate_results (full native gate set on the composed tree,
//     repo-wide per ADR-0069, run via ciparity runners) must all be "pass";
//     any other value keeps the fast path closed.
//
// Entry schema (the Builder contract — fields the fast path reads):
//
//	{
//	  "ts": ..., "cycle": N, "kind": "composition-verdict",
//	  "method": "trivial-rebase",
//	  "lane_audit_ref":      <artifact_sha256 of the bound auditor entry>,
//	  "patch_id":            <git patch-id --stable of the audited lane diff>,
//	  "audited_base":        <git_head the audit bound>,
//	  "new_base":            <the moved main head the lane was rebased onto>,
//	  "git_head":            <composed HEAD ship verifies against>,
//	  "tree_state_sha":      <sha256(git diff HEAD) of the composed tree>,
//	  "audited_diff_path":   <persisted audited diff (kernel-recompute input)>,
//	  "composed_diff_path":  <persisted composed diff (kernel-recompute input)>,
//	  "gate_results":        {"compile":"pass","test":"pass","acs":"pass","apicover":"pass"}
//	}
//
// RED status at authoring (cycle 786): TestTrivialRebase_CarriesAuditForward
// FAILS (verifyAuditBinding returns CodeAuditBindingHeadMoved — the fast path
// does not exist). The two rejection tests are pre-existing GREEN guards: they
// pass today because HEAD-moved rejects everything, and once the fast path
// lands they pin that it never over-accepts (drifted patch-id, failed gates).
```

### `go/internal/phases/ship/trivialrebase_carryforward_test.go:57` — above `func trPassingGates() map[string]string {`

```text
// trPassingGates is the full native gate set the composed tree must pass
// (compile, go test, ACS suite, apicover — repo-wide per ADR-0069).
```

### `go/internal/phases/ship/verify.go:78` — above `release, lockErr := lockStateFile(statePath)`

```text
// ADR-0049 S2 / G2: hold the shared state.json lock across the whole TOFU
// read→decide→repin so a concurrent allocator/UpdateState write can't
// interleave (stale-pin / lost-update). No-op under the whole-cycle lock.
```

### `go/internal/phases/ship/verify.go:187` — above `if err := verifyNoControlPlaneEdits(ctx, opts, res); err != nil {`

```text
// Integrity boundary backstop (ADR-0064): a cycle may not commit changes
// to the pipeline control plane that grades it, by any channel.
```

### `go/internal/phases/ship/verify_unit_test.go:1` — above `package ship`

```text
// verify_unit_test.go — seam-injected unit tests for the ship-class
// verification paths the integration matrix skips:
//
//   - checkEGPSGate     (audit.go) — the trust-kernel RED gate
//   - verifyTrivial     (verify.go) — --class trivial audit-bypass guard
//   - verifyManualConfirm (verify.go) — --class manual auto-confirm path
//
// These pin EXISTING safety-critical contracts. The most load-bearing is
// the EGPS gate's "red_count != 0 ⇒ refuse ship" branch (the v10.0.0
// trust-kernel invariant): a silent regression there would let a build
// with RED predicates ship. See docs/architecture/egps-v10.md.
```

### `go/internal/phases/ship/worktree_errors_test.go:38` — above `if err == nil || !strings.Contains(err.Error(), "git add failed") {`

```text
// cycle-1067: explicit-path staging (stageExplicitPaths) dropped the `-A`
// from the message; the worktree stage-failure branch is unchanged.
```

### `go/internal/phases/ship/worktree_test.go:3` — above `package ship`

```text
// worktree_test.go — coverage for the v8.43.0 worktree-aware ship path
// (shipFromWorktree + writeShipBinding). The 23-case native_test.go matrix
// ships directly from ProjectRoot and never sets cycle-state.json's
// active_worktree, so this entire path — commit-in-worktree, ff-merge into
// main, post-push tree-SHA binding, and the ship-binding.json sidecar —
// was previously 0% covered. These are the most irreversible operations in
// the package, so they earn dedicated behavioral tests.
```

### `go/internal/phases/ship/worktree_test.go:71` — above `func TestShipFromWorktree_TreeSHAMismatch_VerifiesBeforeCommit(t *testing.T) {`

```text
// TestShipFromWorktree_TreeSHAMismatch_VerifiesBeforeCommit: ADR-0048 Slice C1.
// The audit-bound tree-SHA binding is verified against the STAGED INDEX (via
// `git write-tree`) BEFORE the worktree commit, so a mismatch refuses with NO
// commit object ever created — eliminating the commit-then-rollback window.
// The distinguishing signal from the old post-commit-rollback behavior: the
// "committed in worktree" log MUST be absent (verification preceded mutation).
```

### `go/internal/phases/ship/worktree_test.go:108` — above `func TestShipFromWorktree_PreCommitBindingMatch_CommitsAndShips(t *testing.T) {`

```text
// TestShipFromWorktree_PreCommitBindingMatch_CommitsAndShips: ADR-0048 Slice C1
// happy path — when the staged-index tree equals the audit-bound tree, the
// pre-commit verification passes, the commit is then made, and the cycle branch
// ff-merges into main. Proves verification precedes (and gates) the commit.
```

### `go/internal/phases/ship/worktree_test.go:164` — above `func TestShipFromWorktree_AcquiresAndReleasesShipLock(t *testing.T) {`

```text
// TestShipFromWorktree_AcquiresAndReleasesShipLock pins ADR-0049 S5 / gap G1:
// the worktree-aware ship must hold the integrator lock across the shared-main
// critical section and release it. Inject a recording seam and assert it is
// acquired exactly once on <root>/.evolve/ship.lock and released. RED before
// the acquire is wired into shipFromWorktree (acquired=0), GREEN after.
```
