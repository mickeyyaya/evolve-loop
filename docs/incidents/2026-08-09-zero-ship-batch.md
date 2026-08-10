# Incident 2026-08-09: the zero-ship batch (10 cycles, 0 ships, pipeline-blocker halt)

**Severity:** P0 (whole-batch throughput loss) · **Duration:** cycles 1397–1406, one full batch
**Fixed by:** #421 (`7a42d30b`), #422 (`5f405e92`), #423 (`26690652`), plus lane ships in cycles 1409/1410 of the follow-up batch
**Fingerprint:** `ship|gate-block|cd49274beab2` (acked in `.evolve/resolved-fingerprints.json` after the fix)

## What happened

The first batch after the v22.15.0 pause ran 10 full-spine cycles and shipped
**nothing**. The loop's own halt fired only at cycle 10, when the ship-gate
fingerprint finally recurred 3× — the pre-existing ceiling counted *identical*
fingerprints, and this batch's failures were varied enough to evade it for 7
extra cycles. Two of the failure modes were pipeline defects, not task
defects; every audit-green attempt to ship work (including the fix for one of
the defects) was blocked by the other defect.

## Root causes (all verified against run artifacts, not retro narratives)

1. **False-RED ship gate** (`internal/phases/ship/repocontract.go`). The
   four-suite repo-contract pack ran against the PLANE source tree (via the
   test binary's `runtime.Caller` root), where the continuation defect-ledger
   machinery had minted an untracked, gitignored-by-design profile stub
   (`.evolve/profiles/defect-disposition-ledger.json`). Direction B of
   `TestRepoPersonaProfilePairing` bound every on-disk profile by name, saw a
   profile with no persona, and red'd the pack — blocking EVERY lane ship
   (cycles 1402, 1403, 1405, all audit-green). The gate also swallowed the
   scanner output (`ship-error.json` `debug:""`), making the false RED
   undiagnosable from run artifacts; the real `--- FAIL` line only survived in
   the loop's stdout stream.
2. **Unsatisfiable disposition contract.** Continuation cycles must author
   `defect-dispositions.json`, but the exact JSON schema was never shown to
   any agent — the instruction was prose. Agents skipped it (1397, 1400,
   1404) or guessed a plausible-but-wrong shape (1399: `evidence` as array vs
   the struct's `string`). The audit gate was CORRECT every time; the
   contract's communicability was the defect.
3. **Identity-keyed halt ceiling only.** With varied fingerprints, "the batch
   is failing" had no breaker: 10 consecutive failures never tripped anything
   until one fingerprint repeated 3×.
4. **(Chronic, contributing) retro completion cutoff.** The retro phase's
   completion detector tears the session down after the first artifact write,
   so `disposition.json` (ADR-0074 S2) has been absent on every retro cycle
   since ≤1382 — the S3 failure-learning router ran blind the whole time.
   Still open as inbox item `retro-disposition-completion-cutoff` (0.90).

## Fixes

| Fix | Where | Mechanism |
|---|---|---|
| Tracked-only Direction B | #421, `internal/phasecoherence/unpaired_test.go` | `trackedProfiles()` via `git ls-files`; untracked runtime-minted stubs are runtime state, not repo config; loud empty-set + stderr-surfaced fallbacks; top-level-only binding (no nested aliasing) |
| Satisfiable disposition contract | #422 (salvage of cycle-1403's audit-green work) | Literal schema example in `agents/evolve-auditor.md`, single-sourced against the Go reader by test; tolerant `evidence` unmarshal (string OR array, fail-closed otherwise); parse errors name the expected shape |
| Consecutive-failures breaker | #423, `internal/core/blocker_breaker.go` | 3 back-to-back failed cycles (ANY fingerprints) halt the batch through the ADR-0072 machinery; compiled default 3, `consecutive_failures_halt_ceiling` policy override |
| Gate diagnostics + infra classification | lane ship, cycle 1409 (residual halves continuing in-lane) | Scanner output persisted per-run; transient infra exit-1 distinguished from a genuine red suite |

## Regression coverage (all durable, in-repo)

| Failure mode | Pinned by |
|---|---|
| Untracked minted stub binds Direction B | `phasecoherence/unpaired_tracked_test.go` (incident name + arbitrary name) |
| Empty tracked set silently unbinds the gate | loud-fallback branch + `unpaired_tracked_edge_test.go` (empty-set shape) |
| Error hides git's reason | `unpaired_tracked_edge_test.go` (`ErrorCarriesGitStderr`) |
| Staged-vs-committed skew | `unpaired_tracked_edge_test.go` (`StagedButUncommittedCountsAsTracked`) |
| Nested tracked profile aliases a top-level stub | `unpaired_tracked_edge_test.go` (`NestedTrackedProfileDoesNotAliasTopLevelStub`) |
| Array-shaped evidence rejected as unparseable (cycle-1399) | `audit/defect_ledger_evidence_shape_test.go` (AC2) |
| Tolerance becomes a bypass (unresolvable/empty/object shapes) | `defect_ledger_evidence_shape_test.go` negatives |
| Mixed-type array, null, whitespace-only evidence | `audit/defect_ledger_evidence_edge_test.go` |
| Semicolon-join loosening acceptance | `defect_ledger_evidence_edge_test.go` (both directions) |
| Schema example drifts from the Go reader | `defect_ledger_schema_singlesource_test.go`, `_doc_example_test.go` |
| Varied-fingerprint failure streak never halts | `core/blocker_breaker_consecutive_test.go` (8 cases) |
| Streak rule edge cases (precedence, ack-splitting, duplicates, ceiling-1) | `core/blocker_breaker_consecutive_edge_test.go` |
| Streak default not 3 / not policy-overridable | `policy/policy_failure_consecutive_test.go` |

## Lessons (each is now enforced by a test, a rule, or both)

1. **A gate that scans live mutable state will eventually block on state it
   does not own.** Gates must scan what actually lands (tracked config), not
   what happens to be on disk. (Test-enforced.)
2. **A contract nobody can see is a defect in the contract, not the agent.**
   Any machine-graded artifact must ship its literal schema example in the
   authoring agent's instructions, single-sourced against the reader.
   (Test-enforced.)
3. **Per-name allowlists for a structural class re-arm on every new
   instance.** The cycle-~1326 firing of the same stub class was patched by
   name; the class fired again. Fix the classifier, delete the names.
4. **Identity-keyed breakers need a volume-keyed sibling.** 3 consecutive
   failures now halt regardless of fingerprint (operator directive
   2026-08-10, compiled default). (Breaker + operator standing rule.)
5. **Swallowed diagnostics turn a 10-minute diagnosis into a forensic dig.**
   Every gate must persist its subprocess output to the run dir. (Landed via
   cycle-1409/1411 lane ships.)
6. **Salvage before requeue works.** The stranded cycle-1403 fix was landed
   in ~30 minutes from its preserved snapshot; rebuilding it from scratch
   would have cost another full cycle.
7. **Verify halts against artifacts, not auto-hypotheses.** The halt's
   auto-filed P0 blamed the verdict-surface path; the artifacts proved a
   different mechanism. The P0's `fix` field was corrected before any lane
   spent tokens on the wrong theory.

## Follow-ups still open

- `retro-disposition-completion-cutoff` (0.90) — root cause #4.
- Reviewer note (#423): `ensureFailureDigest` is fail-soft; a digest-write
  failure on a genuinely failed cycle can hide a streak hole. Harden
  separately.
- Pre-existing ledger hash-chain break at line 78729 (2026-07-22, both
  planes) — unrelated to this incident; needs a re-anchor decision.
