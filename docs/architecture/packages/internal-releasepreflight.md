# internal/releasepreflight

## Purpose

`internal/releasepreflight` is the read-only gate that `evolve release-preflight` (step 1 of `/evo:publish`) runs before any release step that changes state. It checks the tree, the branch, the version bump and the latest audit. It also checks the gate-test suites, the commit subjects since the last tag and the remote CI of the release commit. It never changes state. It is the Go port of the deleted `preflight.sh` release script.

## Design

- **Five counted steps, first failure fatal, in `preflightSteps` order:**
  1. the working tree is clean (`git diff --quiet HEAD`);
  2. the branch is attached (`git symbolic-ref --short HEAD`; a detached HEAD resolves to `""`);
  3. the target parses as semver and is strictly greater than `.claude-plugin/plugin.json`'s version;
  4. the auditor ledger's most recent usable entry carries a PASS verdict (or WARN, the fluent posture) under `MaxAuditAge` old;
  5. the trust-boundary gate-test packages pass, followed by the naming sub-check. The packages are the Go equivalents of the deleted `legacy/scripts/tests/*.sh` suites:
     - `./internal/guards/...` (ship, role and phase gates);
     - `./internal/phases/ship/...` (the native ship matrix).
- **Exit codes** (the cmd layer maps them): 0 when every check passes, 1 when a check fails (`ErrCheckFailed` wraps the cause), 10 for invalid arguments.
- **Strict PASS.** `Options.StrictPass` (the `--strict-pass` parameter) makes step 4 reject WARN. The error and the WARN hint name the parameter.
- **Options → resolve → preflightRun.** `resolve` fills the plugin.json and ledger paths from `RepoRoot` and all nine seams once, at the boundary. It assigns func values and never calls them, so a nil seam under DryRun or SkipTests is never invoked (`TestRun_DryRunWithNilSeams`). An empty `RepoRoot` is the only failure before any step runs.
- **The counted table.** `StepsTotal` is `len(preflightSteps)`, and `StepsPassed` advances once per step that returns nil. `Result` comes back populated on the failure path too, for diagnostics (`TestRun_StepsPassedOnFailure_CountsStepsReached`). The table order decides which failure an operator sees first (`TestRun_FirstFailingStepWins`).
- **Three uncounted stages run after the table, in this order:** the no-WIP gate, the release-commit CI gate and the advisory simulation. They do not count toward `StepsPassed` or `StepsTotal`, which keeps the five-step contract. A table entry for each of them needs a per-entry "counted" flag for three cases.
- **The no-WIP gate** lists the commits since the nearest tag (`git describe --tags --abbrev=0`, then `git log --format='%h %s' <tag>..HEAD`). With no tag it scans the whole history of `HEAD`. A subject that matches `^(WIP|fixup!|squash!|amend!)` fails the gate, and the error lists each match as `<short-sha> <subject>`. The anchor keeps a prefix word in the middle of a subject from a match (`TestWIPCommitLines_MatchesOnlyAnchoredWIPPrefixes`). A `git log` failure fails closed (`TestRun_WIPScanErrorFailsClosed`, `TestDefaultWIPCommits_UnbornHeadIsAnError`). DryRun skips the gate (`TestRun_DryRunDoesNotScanForWIPCommits`).
- **The no-WIP gate came from the `/evo:release` skill** in cycle 1887, so a test now pins it.
- **Each step keeps its log lines and its dry-run branch inline.** Step 3 has no dry-run branch. A dry run still reads plugin.json and still rejects an invalid bump. Thus an operator learns about a bad target first (`TestRun_DryRun_SemverBumpStillEnforced`).
- **Error strings are a contract.** The suite asserts them by substring, and the CLI prints them verbatim as `[preflight] FAIL: %v`; `detached HEAD` is pinned in `TestRun_FirstFailingStepWins`.
- **Step 5 counts only when both halves are clean**: every gate suite, then the naming sub-check. SkipTests and DryRun count it without running anything.
  - The naming sub-check shares the legacynames acs gate's scanner and manifest (`.evolve/naming.json`). Thus a release cannot ship a rename that left a dead slug or command behind.
  - It no-ops only when the manifest does not exist (`TestDefaultNameGuard_MissingManifestIsCleanPass`).
  - Any other stat error on the manifest, such as a permission error or a symlink loop, fails the step (`TestDefaultNameGuard_StatErrorsOtherThanNotExistFail`). A scanner error also fails the step (`TestRun_NamingGuardErrorFails`).
- **Every `go test` the preflight spawns is built by `goTestCommand`**: the `defaultGoBinFn` binary, run from `<repo>/go`. The child environment is projected through `ipcenv.Scrub`, which drops every `EVOLVE_` key.
  - The guard and ship suites are exactly the env-sensitive packages (the guards' "outside a cycle" tests, ship's fleet-off goldens). Thus a bypass variable in the operator's session can flip a DENY assertion. A fleet lane's `EVOLVE_FLEET` or cycle-state key can flip a golden.
  - Before, the gate runner stripped only `EVOLVE_BYPASS_*` and the simulation runner stripped nothing (`TestGoTestRunners_ScrubTheLaneIPCEnvironment`).
- **The gate runner** runs `go test -count=1 <suite>` through `goTestCommand`.
- **The advisory simulation** runs `go test ./internal/bridge/ -run AutoRespond|SendKeySequence|RealizeFor` through `goTestCommand`, the Go-bridge equivalent of the removed bats simulation suite. A failure is logged as WARN and never returns `ErrCheckFailed`. `Result.SimulationAdvisoryOK` is tri-state: nil when skipped (DryRun or SkipTests), true when it passed, false when it failed (`TestRun_SimulationAdvisory`). `defaultGoBinFn` is the seam tests swap to inject a fake `go`.
- **The ledger walk** takes the newest auditor entry whose artifact exists on disk. An entry with an empty or missing artifact path is a phantom: it is counted and skipped (`TestRun_PhantomEntries`).
- **Verdict extraction, in order** (`TestExtractVerdict`):
  1. the machine-readable `<!-- evolve-verdict: … -->` marker, parsed by `phasecontract.ParseVerdictSentinelFull`, the single parser for that marker. It is tail-anchored, so the report's own final verdict wins over a quoted earlier one. It also rejects a Deliverable-Contract example echoed from scrollback. Only PASS, WARN and FAIL are recognized; any other marker verdict falls through to the prose scan;
  2. an inline `Verdict: PASS`, case-insensitive, with optional bold wrapping on either side of the colon;
  3. a `Verdict` heading followed within five lines by a bold token or a bare `PASS` or `WARN` line. The bold token is `**PASS**`, with at most one trailing `.` or `!` inside the bold. Auditors write `**PASS.**`. A sentence that merely contains the word does not match.
- **Audit age** reads the entry's RFC3339 `ts`. A missing `ts` is an error; an unparseable one skips the age check, as the bash gate did (`TestCheckRecentAudit_MissingTS`, `TestCheckRecentAudit_UnparseableTS`).
- **Semver** is `X.Y.Z` with an optional `+` or `-` suffix that is parsed and ignored when comparing. `ExtractJSONVersion` reads the first `"version": "…"` field by regex, as the bash sed pipeline did.
- **The CI lookup** resolves HEAD and asks `gh run list --workflow <ciparity.RequiredWorkflow> --commit <sha> --limit 1`. A run that has not completed reads as `pending`.

## Invariants

- **An absent audit is advisory.** No ledger, a ledger with no auditor entry, or only phantom entries yields verdict `NONE` and no error. A release from a clean checkout, CI or a fresh worktree is never blocked by transient runtime state. CI-green on the release commit, enforced by `/publish`, is the authoritative gate. A present audit that failed still blocks. Pinned by `TestCheckRecentAudit_AbsentLedger`, `TestCheckRecentAudit_NoAuditorEntries`, `TestCheckRecentAudit_AllPhantom` and `TestRun_MissingLedger`.
- **A failed audit vetoes only when it examined the release commit's committed tree.** With the release head resolved, a non-acceptable verdict is `SCOPED_OUT` (advisory) when the entry's `git_head` differs from it. It is also `SCOPED_OUT` when the entry carries a `worktree_tree_sha`.
  - Head equality alone is not enough. A cycle audit's `git_head` is the project root's HEAD (main's tip), not the lane's. So every concurrent lane records the same head.
  - `worktree_tree_sha` is a writer marker, not a delta marker. The orchestrator's recorder runs `git add -A; git write-tree`, which yields a tree even for a clean worktree. So every cycle audit carries one. The manual `evolve subagent run auditor` release audit never does (`TestLedger_NeverCarriesWorktreeTreeSHA` in `internal/subagent/subagentrun`).
  - A stamp of the field on the manual writer's entry can make release audits look like cycle audits and reopen the gap.
  - Pinned by `TestCheckRecentAudit_ForeignCommitFail_IsAdvisory`, `TestCheckRecentAudit_LaneAuditOnSameHeadIsAdvisory`, `TestCheckRecentAudit_SameCommitFail_StillBlocks` and `TestCheckRecentAudit_ReleaseCommitAuditStillBlocks`.
- **Scoping is never a bypass.** An unresolvable release head (a `HeadSHA` error or an empty result) keeps the block, since the failing audit cannot be proved unrelated (`TestCheckRecentAudit_UnknownReleaseHeadKeepsBlocking`). A PASS satisfies the step whichever commit it bound: scoping narrows the veto, never the blessing (`TestCheckRecentAudit_PassIsUnaffectedByScoping`).
- **`SCOPED_OUT` stays distinct from `NONE`.** The log names the failed audit's artifact and both commits instead of claiming no audit exists. Thus an operator debugging a blocked release is not sent hunting for a missing artifact (`TestCheckRecentAudit_ForeignCommitFail_IsAdvisory`).
- **`RequireCI` makes an unavailable CI verdict blocking.** `--require-ci`, set by the `/evo:release` skill, changes an unavailable verdict into `ErrCheckFailed` (`TestRun_RequireCIFailsClosedOnlyOnAnUnavailableVerdict`).
- **The release-commit CI gate blocks only on a present non-green verdict without `RequireCI`.** An unavailable verdict (no repo, `gh` missing or unauthenticated, no run visible, unparsable output) is advisory-skipped. The lookup returns the unavailable verdict and never an error, so absent tooling cannot block a release (`TestDefaultCIConclusion_LookupFailuresAreUnavailableNotErrors`). A present non-success verdict, `pending` included, hard-fails unless `AllowRedCI` (`--allow-red-ci`) is set, and an override is logged loudly and recorded in `Result.CIOverridden`. Pinned by `TestRun_RefusesTagOnRedReleaseCommitCI` and `TestRun_CIOverrideAllowsRedCIAndLogsLoudly`. The lookup reads the required workflow, not the newest run (`TestDefaultCIConclusion_ReadsTheRequiredWorkflowNotTheNewestRun`).
- **A marker verdict is authoritative.** A FAIL marker (or WARN under strict) returns not-ok with no fall-through to the prose scan, so stray prose saying PASS cannot override it (`TestExtractVerdict`). The delegation to `phasecontract.ParseVerdictSentinelFull`, including its placeholder-echo guard, is pinned through `Run` by the `acs/cycle1303` predicates.
- **`MaxAuditAge` is exactly seven days**, and an audit at the bound is stale (`TestMaxAuditAge_Value`, `TestMaxAuditAge_IsTheStaleAuditBoundary`).
- **`git diff --quiet` exit 1 means dirty**; any other failure is an error (`TestDefaultGitClean_NonRepo`, `TestDefaultGitClean_OnRealRepo`).
- **The default gate suites are Go packages**, never the deleted shell suites (`TestDefaultGateTestSuites_AreGoPackages`).

## Findings

- Failed cycles write auditor entries. Thus the newest auditor entry is routinely a failed lane cycle that has nothing to do with the commit being released. The veto scoping above exists for that shape.
- `auditedUncommittedWork` reads as a delta test but checks the writer marker described above; the name is the only place the two meanings meet in code.
