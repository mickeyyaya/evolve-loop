# Comment history: `internal/dossier`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/dossier/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — named coverage for every exported symbol in the
// dossier package (ADR-0050 Phase 5 requirement). One named test per export
// that is not already covered by build_test.go / render_test.go / write_test.go.
//
// These are minimal behavioural assertions (not mere presence checks) so they
// also serve as regression guards.
```

### `go/internal/dossier/build.go:12` — above `func auditArtifactName() string {`

```text
// auditArtifactName is the audit deliverable's filename, DERIVED from the
// phasecontract registry (the SSOT for report filenames) rather than re-typed.
// Cycle-1141: the registry exists precisely so this vocabulary is declared once
// — a synthesized defect that points at a stale filename sends the next cycle
// looking for a file that is no longer written.
```

### `go/internal/dossier/build.go:70` — above `LedgerPath string`

```text
// LedgerPath is the path to ledger.jsonl (for phase record extraction).
// NOTE: no production caller sets it — the phase records come from
// PhaseTimings (or the workspace log). Kept for the ledger-walk slice that
// ADR-0055 describes; until that lands, absence of a ledger is NOT what
// makes a record degrade.
```

### `go/internal/dossier/build.go:92` — above `VerdictsNotAdopted []cyclestate.VerdictNotAdopted`

```text
// VerdictsNotAdopted are non-floor phases that RAN whose non-PASS outcome was
// declined rather than allowed to clobber the floor-derived FinalVerdict
// (cycle-802). Surfaced verbatim so the dossier records the degrade, never
// dropping it — and never as a "skipped phase" (dossier-retro-skipped-mislabel).
```

### `go/internal/dossier/build.go:97` — above `SpineFailOpens []cyclestate.SpineFailOpen`

```text
// SpineFailOpens are the cycle's spine-gate fail-open events (cycle-1166),
// surfaced verbatim so the dossier is where the epidemic becomes visible.
```

### `go/internal/dossier/build.go:100` — above `PhaseTimings []phasetiming.Entry`

```text
// PhaseTimings is the per-phase evidence the caller already holds — for the
// orchestrator, the set core composed and flushed ONCE
// (cycleRun.flushPhaseTimings), so the dossier projects exactly the record
// the on-disk log receives. It is passed rather than re-read because that
// log is written by a DEFERRED call in RunCycle and lands AFTER the dossier
// is produced on the normal path (cycle-1623: twelve phases ran, one
// synthetic phase was recorded).
//
// Empty ⇒ fall back to reading the workspace log. That fallback is live,
// not vestigial: a dossier built for a workspace whose live timings were
// never threaded has the file as its only evidence.
```

### `go/internal/dossier/build_ciwatch_test.go:9` — above `func TestBuild_IngestsCIWatchVerdict(t *testing.T) {`

```text
// TestBuild_IngestsCIWatchVerdict pins AC3 of push-ci-watch-remote-parity
// (cycle-748): the CI verdict recorded by the post-push watch round-trips
// into the cycle dossier, and an absent verdict is never fabricated.
```

### `go/internal/dossier/build_finalverdict_test.go:5` — above `func TestBuild_FinalVerdictDefaultsToPass(t *testing.T) {`

```text
// These tests cover BuildOpts.FinalVerdict — the extension that lets the cycle
// producer (core.RunCycle, ADR-0055) record a cycle's REAL outcome instead of an
// always-PASS skeleton. A FAIL dossier must still satisfy Validate (>=1 defect +
// >=1 carryover), so Build synthesizes a minimal, truthful pair pointing at the
// audit artifacts rather than fabricating a PASS for a failed cycle.
```

### `go/internal/dossier/ciwatch.go:10` — above `const CIWatchVerdictFile = "ci-watch-verdict.json"`

```text
// CIWatchVerdictFile is the workspace artifact the post-push CI watch
// (internal/ciwatch) writes and Build ingests — the durable evidence that the
// remote GitHub CI verdict for the cycle's pushed SHA was actually read
// (cycle-748, push-ci-watch-remote-parity).
```

### `go/internal/dossier/cyclesdir.go:5` — above `func CyclesDir(projectRoot string) string {`

```text
// CyclesDir is the committed home of the per-cycle dossiers,
// <projectRoot>/knowledge-base/cycles — a protocol-committed corpus
// (ADR-0094: tracked, read back by the dossier-closeout gate and the recent
// outcomes chronicle). The producer (core.dossier_producer), the chronicle
// seed (core.cyclerun_chronicle), ReadCommitted, `evolve dossier verify` and
// the dashboard's ship-rate history resolve the directory here.
// internal/contextfillcorrelate still spells the join inline (it is a
// deliberate leaf package that does not import dossier) — an unmigrated
// reader to carry along on any move, not a second owner.
```

### `go/internal/dossier/cyclesdir_test.go:8` — above `func TestCyclesDir(t *testing.T) {`

```text
// TestCyclesDir pins the dossier corpus location: the producer commits here,
// the chronicle and the dashboard read here. Renaming it is a protocol change
// (ADR-0094), not a refactor.
```

### `go/internal/dossier/delivery.go:12` — above `const ShipBindingFile = "ship-binding.json"`

```text
// delivery.go — the dossier's readers for the two facts that make a cycle
// record answerable: WHAT it was for (the committed task set) and WHETHER it
// delivered (the shipped commit). Both follow the package's established
// evidence discipline (see ciWatchRecord): read the phase's OWN artifact,
// return not-ok on absence, and never fabricate.
//
// Before this, Dossier.CommitSHA existed in the schema but no producer ever
// set it, and the committed task set was not recorded at all — so cycle-1623's
// record could not distinguish "shipped 922 lines" from "shipped nothing", and
// a reader (human or ship-rate query) had to reconstruct it from git.
```

### `go/internal/dossier/delivery.go:66` — above `func committedTasks(workspace string) ([]string, bool) {`

```text
// committedTasks returns the task ids this cycle is bound to, delegating to
// internal/committedset — the ONE projection of "what did this cycle commit
// to" (lane pin, else triage's top_n, minus deferrals). The dossier had its
// own top_n-only parser for exactly one review round; on runtime cycle-1621
// that read one member where the lane pin named two, and 17 of the last 20
// cycles carried a pin. A record must not report a commitment it did not read.
```

### `go/internal/dossier/dossier.go:1` — above `package dossier`

```text
// Package dossier is the durable, structured, cross-loop history/experience
// record for an evolve-loop cycle (ADR-0055). Today every cycle's structured
// data (phase reports, ledger, lessons, carryover) lives in gitignored runtime
// (.evolve/) and is lost across sessions/branches/loops. The Dossier aggregates
// it into ONE committed artifact (knowledge-base/cycles/cycle-N.json) that the
// next cycle's Scout — and any other session or loop — reads as the source of
// truth, so the project learns from experience including FAILED verdicts.
//
// The Go struct + Validate() are the SSOT (deterministic; the project does not
// use a JSON-Schema-v2020-12 validator). schemas/cycle-dossier.schema.json is the
// committed human/cross-tool reference, kept in sync by a drift test.
```

### `go/internal/dossier/dossier.go:42` — above `Tasks   *[]string     'json:"tasks,omitempty"'`

```text
// Tasks is the task set triage COMMITTED for this cycle (top_n ids). It is
// a POINTER because the three states are genuinely distinct and the record
// must not conflate them:
//
//	nil         → no triage decision recorded (unknown); the field is omitted
//	&[]string{} → an explicit EMPTY commitment; serializes as []
//	&[...]      → the committed ids
//
// "committed to nothing" is a finding in its own right — cycle-1623 did
// exactly that and then ran twelve phases anyway — so it must stay
// distinguishable from "we never asked". A plain []string cannot express
// this: without omitempty a nil marshals to `null`, which the schema's
// "type": "array" rejects; with omitempty an explicit empty commitment
// silently vanishes.
```

### `go/internal/dossier/dossier.go:74` — above `PhasesRunVerdictNotAdopted []cyclestate.VerdictNotAdopted 'json:"phases_run_verdict_not_adopted,omitempty"'`

```text
// PhasesRunVerdictNotAdopted records non-floor phases that RAN and returned
// non-PASS after the floor verdict was set, so their verdict was declined
// rather than allowed to overwrite the floor-derived FinalVerdict (cycle-802).
// Present so a cycle that PASSed its floor but had a retro/memo fail under
// quota pressure still records that experience instead of dropping it.
//
// These records used to be written into skipped_phases, which made every FAIL
// dossier claim `{phase: retro, reason: FAIL}` was SKIPPED while retro's report
// sat in the run dir — a record contradicting its own artifacts, poisoning the
// consumers that read dossiers to learn which judgment phases executed
// (dossier-retro-skipped-mislabel). omitempty, mirroring skipped_phases.
```

### `go/internal/dossier/dossier.go:86` — above `SpineFailOpens []cyclestate.SpineFailOpen 'json:"spine_fail_opens,omitempty"'`

```text
// SpineFailOpens records every spine-gate fail-open this cycle took (the
// phase entered anyway + the missing predecessor artifact + the reason).
// omitempty, mirroring skipped_phases: an operator scanning dossiers sees
// the field only where there is something to see (cycle-1166).
```

### `go/internal/dossier/dossier.go:96` — above `CIWatch *CIWatchRecord 'json:"ci_watch,omitempty"'`

```text
// CIWatch is the remote GitHub CI verdict for the cycle's pushed commit,
// ingested from ci-watch-verdict.json (cycle-748). Nil when the cycle
// recorded no watch verdict — never fabricated.
```

### `go/internal/dossier/dossier.go:115` — above `ModelSource   string 'json:"model_source,omitempty"'`

```text
// ModelSource + ResolvedModel (T3, cycle-463) project the per-phase model
// provenance ingested from phase-timing.json — "profile"|"pin"|"advisor"
// plus the concrete resolved model/tier. Both absent (never fabricated) on
// a legacy timing log written before this field existed.
```

### `go/internal/dossier/dossier.go:206` — above `func (d *Dossier) CommitmentLine() string {`

```text
// CommitmentLine renders the committed task ids for the human-readable half of
// the record. An explicit EMPTY commitment renders as a stated fact rather than
// a blank, because a cycle that committed to nothing and then ran a full spine
// is exactly what the reader needs to see (cycle-1623).
```

### `go/internal/dossier/evidence_projection_test.go:14` — above `func mustBuild(t *testing.T, cycle int, opts BuildOpts) *Dossier {`

```text
// evidence_projection_test.go — cycle-1623 forensics: a cycle that ran 12
// phases and shipped 922 lines to origin/main recorded a dossier containing
// ONE synthetic phase ("cycle-recorded", PASS, zero tokens) and no commit.
//
// Root cause: phase-timing.json is written by a DEFERRED call in RunCycle, so
// it lands AFTER writeCycleDossier has already read it. On the normal path the
// dossier therefore always missed its own evidence; only a resumed cycle (which
// writes the log mid-run) recorded real phases. The healthier the cycle, the
// emptier its permanent record — and a synthesized PASS made the gap invisible.
//
// The contract these tests pin: a dossier is a PROJECTION OF EVIDENCE. Live
// evidence is authoritative, the on-disk log is the fallback, and absent
// evidence is recorded LOUDLY as a degraded record — never synthesized as a
// passing phase.
```

### `go/internal/dossier/evidence_projection_test.go:120` — above `func TestBuild_ProjectsTheCommittedTasks(t *testing.T) {`

```text
// TestBuild_ProjectsTheCommittedTasks: "what was this cycle for" belongs in the
// permanent record. triage-decision.json's top_n is the committed set; an
// EMPTY commitment is itself the finding cycle-1623 needed to surface, so it is
// recorded as an explicit empty list rather than an absent field.
```

### `go/internal/dossier/evidence_projection_test.go:140` — above `func TestBuild_Cycle1623Shape(t *testing.T) {`

```text
// TestBuild_Cycle1623Shape is the forensic regression: the exact evidence
// cycle-1623 left on disk must produce a record an operator (or a ship-rate
// query) can read correctly — twelve phases, the shipped commit, and an empty
// commitment, all visible at once.
```

### `go/internal/dossier/evidence_projection_test.go:203` — above `func TestCommittedTasks_AbsentEmptyAndPopulated(t *testing.T) {`

```text
// TestCommittedTasks_AbsentEmptyAndPopulated pins the three-way contract the
// dossier depends on, using the artifact constant the reader resolves
// (committedset.DecisionFile) so a filename change fails here rather than silently
// emptying every record's commitment. Absent ⇒ not-ok (unknown); present but
// empty ⇒ ok with an empty non-nil slice (the cycle committed to nothing —
// cycle-1623's finding); populated ⇒ the ids in order.
```

### `go/internal/dossier/rollback_test.go:13` — above `func stagedPaths(t *testing.T, dir string) []string {`

```text
// rollback_test.go — RED contract for cycle-573 Task 3
// (dossier-commit-rollback-on-failure, inbox weight 0.84 medium).
//
// commitPairGit stages cycle-<base>.{json,md} via `git add`, then commits. On a
// PERMANENT (non-lock) commit failure it returns the error but never unstages
// the pair — so the staged files survive into the next cycle's tree-diff guard
// as phantom staged changes. The fix: on a permanent failure, `git reset` the
// pair back out of the index before returning the (unchanged) error.
//
// The load-bearing invariant is the STAGED set, not the whole porcelain: newly
// created files legitimately remain on disk as untracked after an unstage; the
// pollution the guard trips on is a non-empty index. So the assertion is
// `git diff --cached --name-only` == empty.
//
// RED today: after the failed commit the pair is still staged, so the staged set
// is non-empty. GREEN once commitPairGit resets on permanent failure.
```

### `go/internal/dossier/schema_drift_test.go:3` — above `import (`

```text
// schema_drift_test.go — the TestSchema_NoDrift that ADR-0055 promised
// ("The Go struct is the SSOT; schemas/cycle-dossier.schema.json is a derived
// artifact. The TestSchema_NoDrift drift test guards this in CI.") and that
// did not exist. Its absence is why the schema silently rotted: three fields
// (skipped_phases, spine_fail_opens, timing) were added to the struct and
// never to the schema, and the schema declares additionalProperties:false —
// so every real dossier would have FAILED validation by any external tool
// that took the committed schema at its word.
//
// The check is BIDIRECTIONAL on names and structural on shape.
//
// Bidirectional, because a one-way "every Go field appears in the schema"
// test still lets a removed field linger in the schema forever, and a
// schema-side-only test lets a new Go field rot exactly the way these did.
//
// Structural, because a name-only inventory is green while the schema is
// unusable. Concretely: swapping the items.$ref of skipped_phases and
// phases_run_verdict_not_adopted keeps every property name identical, yet the
// two definitions carry disjoint required keys, so every real dossier
// carrying either array is rejected. Shape is checked for the three axes that
// can produce that class of false rejection — $ref target, JSON type, and
// required — rather than by pulling in a JSON-Schema validator dependency
// into a module that deliberately has two.
//
// The `required` check is deliberately ONE-WAY: every schema-required field
// must be a Go field that is always marshalled (no omitempty). The reverse is
// not asserted, because a non-omitempty field absent from `required` is merely
// permissive, while a required field the writer may omit rejects valid
// documents. Assert the direction that can cause a false rejection.
```

### `go/internal/dossier/schema_version_test.go:3` — above `import (`

```text
// schema_version_test.go — the forward-only discriminator contract
// (dossier-corpus-carries-retro-mislabel, cycle 1666).
//
// PR #389 stopped NEW dossiers from recording a declined verdict as a skip,
// but left every record shape-identical: a pre-fix dossier whose
// `skipped_phases:[{phase:retro,reason:FAIL}]` is a MISLABEL (retro ran) and a
// post-fix dossier whose identical entry is a genuine skip cannot be told
// apart. The remedy chosen here is option (b) of the inbox record — a
// `schema_version` discriminator stamped on every new record — and NOT the
// backfill; the two are mutually exclusive by the record's own text.
//
// Wire contract pinned here (the record's consumers read the wire, not Go):
//   - key `schema_version`, JSON integer, on every record Build produces;
//   - equal to CurrentSchemaVersion, which is >= 2 (1 is the implicit,
//     never-written version of the pre-discriminator corpus);
//   - ABSENT from a legacy record, and a legacy record re-rendered stays
//     unstamped — the discriminator is forward-only, never a silent backfill.
```

### `go/internal/dossier/schema_version_test.go:29` — above `const legacyRetroMislabelRecord = '{`

```text
// legacyRetroMislabelRecord is the exact shape of the 134 affected corpus
// records (knowledge-base/cycles/cycle-823.json … cycle-1217.json): no
// discriminator, a single `cycle-recorded` phase, retro in skipped_phases.
```

### `go/internal/dossier/skipevidence_test.go:3` — above `import (`

```text
// skipevidence_test.go — the consumer-side contract of
// dossier-corpus-carries-retro-mislabel (cycle 1666).
//
// No production reader interprets skipped_phases today (premise-challenge,
// cycle 1666: contextfillcorrelate decodes only cycle + final_verdict; the
// loop-outcome roll-up reads only spine_fail_opens). So the "disposition
// assembler" the inbox record names is any FUTURE consumer that asks the
// committed corpus which judgment phases executed — and the record type's own
// package is the one boundary every such consumer must cross. PhaseSkipEvidence
// is that boundary: it is the ONLY sanctioned way to read a skipped_phases
// entry, and it refuses to treat a pre-discriminator record's entry as evidence
// of anything. A legacy record is cross-checked against the run artifacts
// instead:
//
//	Contradicted — an execution receipt proves the phase RAN (the mislabel)
//	Unverified   — no receipt survives: unknown, NOT a skip
//	Trusted      — the record carries the discriminator: the phase did not run
//	None         — no skipped_phases entry for the phase at all
//
// Two receipt sources, because the historical run dirs are gone: the phase's
// report in <root>/.evolve/runs/cycle-N/ (retrospective-report.md, or the
// pre-rename retro-report.md that 201 of the ledger's receipts still name), or
// the hash-chained ledger's {cycle:N, role:retro, kind:agent_subprocess} entry
// (<root>/.evolve/ledger.jsonl) — the record of the subprocess that wrote it.
```

### `go/internal/dossier/skipevidence_test.go:73` — above `func retroReceipt(cycle int) string {`

```text
// retroReceipt is the ledger line the real corpus carries for every one of
// the 134 affected cycles (cycle-823's verbatim shape, entry_seq 57180).
```

### `go/internal/dossier/spine_failopen.go:3` — above `import "sort"`

```text
// spine_failopen.go — the batch-level roll-up of spine-gate fail-opens
// (cycle-1166, spine-failopen-telemetry).
//
// Per-cycle records make one cycle's fail-opens visible; they do NOT make an
// epidemic visible. A width-3 batch on 2026-07-13 took 76 fail-opens spread
// across its cycles and nothing summed them. RollupSpineFailOpens is that sum,
// plus the two breakdowns that turn a number into a diagnosis: which phase kept
// entering without its predecessor's handoff, and which individual cycles were
// noisy enough to escalate.
```

### `go/internal/dossier/spine_failopen_rollup_test.go:3` — above `import (`

```text
// spine_failopen_rollup_test.go — RED contract for cycle-1166 Task 3
// (spine-failopen-telemetry, inbox weight 0.85), dossier half. The core half
// (recording the events) lives in internal/core/spine_failopen_telemetry_test.go.
//
// The item names two RED tests verbatim:
//   - TestSpineFailOpen_CountedInDossierWithPhaseAndArtifact
//   - TestLoopSummary_RollsUpSpineFailOpensPerBatch
//
// …plus "WARN escalation when a single cycle exceeds a threshold (e.g. 3)".
//
// The wiring follows the EXISTING SkippedPhases precedent exactly
// (cyclestate.SkippedPhase → BuildOpts.SkippedPhases → Dossier.SkippedPhases,
// build.go:78/113) rather than inventing a new shape — the scout report flags
// that precedent, and single-source-with-projection is the standing rule.
//
// RED today: cyclestate.SpineFailOpen, BuildOpts.SpineFailOpens,
// Dossier.SpineFailOpens and RollupSpineFailOpens do not exist — this file does
// not compile.
//
// Contract Builder must satisfy:
//
//	type cyclestate.SpineFailOpen struct {
//	    Phase           string `json:"phase"`
//	    MissingArtifact string `json:"missing_artifact"`
//	    Reason          string `json:"reason,omitempty"`
//	}
//	BuildOpts.SpineFailOpens []cyclestate.SpineFailOpen
//	Dossier.SpineFailOpens   []cyclestate.SpineFailOpen `json:"spine_fail_opens,omitempty"`
//	type SpineFailOpenRollup struct {
//	    Total               int
//	    ByPhase             map[string]int
//	    OverThresholdCycles []int   // cycles whose OWN count exceeded threshold
//	}
//	func RollupSpineFailOpens(ds []*Dossier, threshold int) SpineFailOpenRollup
```

### `go/internal/dossier/sweep.go:3` — above `import (`

```text
// sweep.go — cycle-564 orphan recovery (task
// sweep-orphaned-dossier-pairs-and-harden-commit).
//
// commitPair is best-effort: when it failed (historically un-retried, on a
// transient index.lock), the freshly-written cycle-N.{json,md} pair stayed
// untracked in the main tree with no commit history. Nine recorded cycle
// failures and 35 live orphaned pairs (cycle-564 scout) are exactly this: a
// later phase's tree-diff guard picks the stray pair up as unexplained churn
// and hard-aborts the cycle. SweepOrphans mops those up — it re-detects every
// untracked, COMPLETE dossier pair and recommits it, so the main tree lands
// clean without the next phase ever tripping.
```

### `go/internal/dossier/sweep_test.go:77` — above `func TestSweepOrphans_RecommitsUntrackedPairs(t *testing.T) {`

```text
// TestSweepOrphans_RecommitsUntrackedPairs is the scout-mandated verification
// anchor (verifiableBy) for sweep-orphaned-dossier-pairs-and-harden-commit:
// untracked, COMPLETE cycle-N.{json,md} pairs sitting in the main tree (the 35
// live orphans confirmed by cycle-564 scout) must be detected and recommitted.
```

### `go/internal/dossier/write.go:59` — above `const commitMaxAttempts = 4`

```text
// commitMaxAttempts bounds the transient-lock retry: 1 initial + 3 retries. A
// genuinely stuck lock surfaces as an error in a small, fixed number of tries
// rather than hanging cycle finalization (cycle-564 write_retry_test bounds).
```

### `go/internal/dossier/write.go:69` — above `func commitPairGit(g gitexec.Git, base string) error {`

```text
// commitPairGit stages and commits cycle-<base>.{json,md} in g's repo, scoped by
// pathspec so no unrelated staged change is swept in. A re-write with identical
// content (nothing staged) is a no-op, never an empty commit. A transient git
// index.lock failure on commit — the cycle-564 root cause: concurrent fleet
// lanes sharing one repo contend on .git/index.lock, which the old un-retried
// commitPair swallowed, permanently orphaning 9 recorded cycles' dossiers — is
// retried with bounded linear backoff. A non-lock (permanent) error fails fast.
```

### `go/internal/dossier/write_retry_test.go:11` — above `type fakeGitRun struct {`

```text
// fakeGitRun is a scripted sysexec.RunFunc for commitPairGit's retry tests.
// commitPairGit must reach git through an injected gitexec.Git so bounded
// retry/backoff on transient lock contention is testable without racing real
// git index.lock files (cycle-564 scout: 9 recorded tree-diff-leak cycle
// failures traced to commitPair's un-retried, best-effort git commit).
```

### `go/internal/dossier/write_retry_test.go:115`

```text
// NOTE (dossier-commit-rollback-on-failure, this cycle's scout selection):
// the rollback regression is already covered by
// TestCommitPairGit_RollsBackStagedOnPermanentFailure in rollback_test.go
// (authored cycle-573, still green here) — a real-git integration test
// asserting `git diff --cached --name-only` is empty after a forced
// identity-less commit failure. No additional seam-level duplicate added
// here; see test-report.md's coverage map for the pre-existing-GREEN note.
```
