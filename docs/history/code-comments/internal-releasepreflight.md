# Comment history: `internal/releasepreflight`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/releasepreflight/ciwatch_gate_test.go:10` — above `func redCIOpts(t *testing.T) (Options, *bytes.Buffer) {`

```text
// redCIOpts wires a happy-path repo whose release-commit CI verdict is a
// faked red run — no network, no live gh (AC2 of push-ci-watch-remote-parity,
// cycle-748).
```

### `go/internal/releasepreflight/ciwatch_gate_test.go:24` — above `func TestRun_RefusesTagOnRedReleaseCommitCI(t *testing.T) {`

```text
// TestRun_RefusesTagOnRedReleaseCommitCI pins the hard-gate: when the release
// commit's go CI run conclusion is not success, preflight refuses (v22.0.0
// was cut on red CI; this makes that structurally impossible without an
// explicit override).
```

### `go/internal/releasepreflight/preflight_run.go:306` — above `func (p *preflightRun) gateReleaseCommitCI() error {`

```text
// gateReleaseCommitCI is the release-commit CI hard-gate (cycle-748,
// push-ci-watch-remote-parity): the remote go CI run for HEAD must be
// conclusion=success before tagging (v22.0.0 was cut on red CI). It does NOT
// count toward StepsPassed/StepsTotal (back-compat with the 5-step
// contract), which is why it is called after the table rather than listed in
// it. An UNAVAILABLE verdict (no repo, gh missing, no run visible) is
// advisory-skipped — same determinism rule as auditVerdictNone — but a
// present non-success verdict hard-fails unless AllowRedCI is explicitly set,
// and an override is always logged loudly and recorded in Result.CIOverridden.
```

### `go/internal/releasepreflight/preflight_run.go:342` — above `func (p *preflightRun) adviseSimulation() {`

```text
// adviseSimulation is the advisory auto-respond simulation suite (v12.1.5+).
// It does NOT count toward StepsPassed/StepsTotal and never returns
// ErrCheckFailed — a failure is logged as WARN and recorded in the tri-state
// Result.SimulationAdvisoryOK (nil = not run). Promotes to a required step in
// v12.2.0, at which point it becomes a preflightSteps entry.
```

### `go/internal/releasepreflight/recent_audit_scope_test.go:11` — above `func seedLedgerAndArtifact(t *testing.T, gitHead, verdict string, ts time.Time, worktreeTree string) string {`

```text
// recent_audit_scope_test.go — cycle-1571 H4, a live consequence of PR #503.
//
// checkRecentAudit takes the NEWEST auditor entry with an on-disk artifact —
// no run, cycle, or git_head filter — and a non-acceptable verdict aborts the
// release. Before #503 a FAILed cycle wrote no auditor entry at all, so that
// branch was unreachable; #503 made FAIL entries exist, so the newest entry is
// now routinely a FAILed LANE cycle that has nothing to do with the commit
// being released. Verified live: the runtime ledger's newest auditor entry was
// cycle-1574, exit 1, artifact on disk, marker verdict FAIL — i.e. `evolve
// release` was blocked by an unrelated lane failure.
//
// The gate was already inconsistent about this: NO audit is advisory-skipped
// because "CI-green on the release commit is the authoritative gate", while a
// FAILED audit of unrelated work hard-blocked. Scope the veto to the commit
// actually being released: an audit that bound a different HEAD gets no vote;
// an audit that bound THIS HEAD and rejected it still blocks.
```

### `go/internal/releasepreflight/recent_audit_scope_test.go:120` — above `func TestCheckRecentAudit_LaneAuditOnSameHeadIsAdvisory(t *testing.T) {`

```text
// TestCheckRecentAudit_LaneAuditOnSameHeadIsAdvisory is the REAL live shape, and
// it falsifies this fix's first premise. A cycle audit's git_head is the PROJECT
// ROOT's HEAD (phase_bindings.go resolves it with `rev-parse HEAD` against
// projectRoot), i.e. main's tip — NOT a lane worktree head. Verified in the
// runtime ledger: cycles 1572, 1573 and 1574 all carry git_head 31ae6518, each
// with a DISTINCT worktree_tree_sha.
//
// So head equality alone cannot separate "audited the release commit" from
// "audited a lane's uncommitted work that happened to be based on it", and
// scoping on head alone would still have vetoed the release that motivated this
// change. The second discriminator is worktree_tree_sha — a WRITER marker, not
// a delta marker: the orchestrator's recorder runs `git add -A; git write-tree`,
// which yields a tree even for a clean worktree, so every cycle audit records
// one, while the manual `evolve subagent run auditor` writer never emits it.
// That is the cut wanted: a manual audit of the released commit keeps its veto.
```

### `go/internal/releasepreflight/releasepreflight.go:112` — above `SimulationAdvisoryOK *bool`

```text
// SimulationAdvisoryOK records the outcome of the post-step-5 advisory
// auto-respond simulation suite run (v12.1.5+). nil = skipped (DryRun or
// SkipTests); &true = passed; &false = failed-but-advisory (logged as
// WARN but does not block release). Promotes to hard requirement in v12.2.0.
```

### `go/internal/releasepreflight/releasepreflight.go:270` — above `func defaultSimulationRunner(repoRoot string) error {`

```text
// defaultSimulationRunner runs the auto-respond regression coverage that
// guards against manifest/policy regressions. The bash bats simulation suite
// (tools/agent-bridge/tests/simulation) was removed in the v12 Go-bridge
// cutover; the equivalent coverage now lives in the Go bridge package's
// auto-respond + manifest tests, which this runs via `go test`.
//
// Returns an error if `go` is missing or any auto-respond test fails. The
// caller logs the error as WARN — advisory in v12.1.5, blocking in v12.2.0.
```

### `go/internal/releasepreflight/releasepreflight.go:426` — above `func markerVerdict(body string) (string, bool) {`

```text
// markerVerdict returns the report's authoritative machine-readable verdict
// (upper-cased PASS/WARN/FAIL) and whether any valid marker was present.
//
// Sentinel parsing is NOT done here: it delegates to
// phasecontract.ParseVerdictSentinelFull, the project's single source of truth
// for the `<!-- evolve-verdict: {…} -->` marker. That parser is tail-anchored
// (the report's own final verdict wins over a quoted prior-cycle one, cycle-1298)
// and rejects a Deliverable-Contract example echoed from scrollback (cycle-603) —
// guarantees this call site used to miss because it hand-rolled a second scanner.
// What stays local is releasepreflight's own POLICY: normalising the verdict and
// accepting only the three the release gate understands, so an unrecognised
// verdict falls through to the prose scan exactly as before.
```

### `go/internal/releasepreflight/releasepreflight.go:531` — above `auditedHead := entryHead(candidate)`

```text
// Cycle-1571 H4. A non-acceptable verdict vetoes the release only when
// the audit actually examined what is being released. Before PR #503 a
// FAILed cycle wrote no auditor entry, so this branch was effectively
// unreachable; #503 made FAIL entries exist, and the newest is routinely
// a FAILed lane cycle with no bearing on the release. Vetoing on that is
// false, and it contradicts this step's own determinism rule, which
// advisory-skips a MISSING audit precisely because CI-green on the
// release commit is authoritative.
//
// TWO discriminators are needed, and head alone is NOT enough — the
// first draft of this fix assumed a lane audit binds a lane worktree
// head, and that is false: recordAuditBinding resolves git_head with
// `rev-parse HEAD` against the PROJECT ROOT, so every concurrent lane
// records main's tip. Verified in the runtime ledger: cycles 1572, 1573
// and 1574 all carry git_head 31ae6518, each with a distinct
// worktree_tree_sha. Scoping on head alone would still have vetoed the
// very release that motivated this change.
//
//   1. a different git_head  ⇒ it audited another commit entirely;
//   2. a worktree_tree_sha   ⇒ a cycle/lane audit, not an audit of the
//      committed tree.
//
// On (2), precisely: this is NOT a delta marker. worktreeContentSHA runs
// `git add -A; git write-tree`, which yields a tree even for a clean
// worktree, so EVERY orchestrator cycle audit records one. It is absent
// because the other writer — subagent/run.go, the manual
// `evolve subagent run auditor` release audit — never emits it. So the
// field identifies which writer produced the entry, which is exactly the
// cut wanted here: a manual audit of the released commit keeps its veto.
// Anyone later tempted to stamp worktree_tree_sha on run.go's entry
// should know it would make release audits look like cycle audits and
// silently re-open this hole.
//
// An audit of the release commit itself has neither, so its rejection
// still blocks. An unresolvable releaseHead keeps the conservative
// block: we cannot prove the failing audit is unrelated, so we do not
// assume it — scoping is never a bypass.
```

### `go/internal/releasepreflight/releasepreflight.go:624` — above `lines := strings.Split(body, "\n")`

```text
// Heading form: scan for `## Verdict` line, then within 5 lines look
// for **PASS**/**WARN** (with optional trailing punctuation) or a BARE
// verdict line (exactly `PASS`/`WARN`, the cycle-249 shape — auditors
// legitimately omit the bold). A sentence merely containing the word must
// not match.
```

### `go/internal/releasepreflight/releasepreflight_test.go:308` — above `func TestRun_SimulationAdvisory(t *testing.T) {`

```text
// === Advisory simulation step (v12.1.5) =====================================
// Table-driven: covers skip, dry-run, pass, fail-but-advisory. Asserts that
// no path returns ErrCheckFailed (advisory) and that SimulationAdvisoryOK
// is nil/true/false as appropriate. Verifies StepsPassed stays 5.
```

### `go/internal/releasepreflight/releasepreflight_test.go:574` — above `{"heading bare PASS", "## Verdict\nPASS\n\n**Confidence:** 0.97\n", false, "PASS", true},`

```text
// Bare-line heading form — the cycle-249 release-blocker shape
// (auditor wrote `## Verdict\nPASS` without bold).
```

### `go/internal/releasepreflight/releasepreflight_test.go:582` — above `{"marker PASS with trailing-period prose", "## Verdict\n\n**PASS.** The change is correct.\n\n<!-- evolve-verdict: {\"ph…`

```text
// Machine-readable marker (the SSOT the auditor emits) — parsed first,
// immune to prose variation. This is the cycle-480 release-blocker shape:
// the prose is `**PASS.**` (trailing period in the bold), which the prose
// forms below miss, but the marker is unambiguous.
```
