# Comment history: `internal/releasepipeline`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/releasepipeline/bridges_changelog_test.go:11` — above `func makeHermeticGitRepo(t *testing.T) string {`

```text
// makeHermeticGitRepo creates a minimal hermetic git repository in a TempDir
// with one commit tagged v0.0.1, suitable for testing git-dependent code paths
// without depending on the operator's real repo state.
//
// Returns the repo root path.
```

### `go/internal/releasepipeline/bridges_changelog_test.go:105` — above `err := runChangelogGenLib(dir, "v0.0.1", "nonexistent-branch", "2.0.0", false)`

```text
// fromRef=v0.0.1 is valid, but toRef is invalid.
```

### `go/internal/releasepipeline/ci_advisory_test.go:10` — above `func TestRun_EmitsCINotVerifiedAdvisory(t *testing.T) {`

```text
// TestRun_EmitsCINotVerifiedAdvisory asserts the success path prints an explicit
// advisory that this gh-free pipeline does NOT verify GitHub CI. Without it a
// direct `evolve release` caller can believe a green pipeline means green CI —
// the v20.1.0 false-success class (the pipeline exited 0 while the released
// commit's `go` workflow was red). CI-gating itself lives in /publish (Option A:
// the binary stays self-contained/headless-safe and only discloses the gap).
```

### `go/internal/releasepipeline/classify_test.go:12` — above `func initClassifyRepo(t *testing.T) string {`

```text
// initClassifyRepo builds an isolated repo with a known tag/commit chain for the
// git-backed helpers: v1.0.0 (touches go/), v1.0.1 (touches README only), v1.1.0
// (touches go/). Returns the repo dir. Skips if git is unavailable.
```

### `go/internal/releasepipeline/classify_test.go:55` — above `func TestGitPathsChanged_RealRepo(t *testing.T) {`

```text
// TestGitPathsChanged_RealRepo pins the two-dot diff scoping: go/ changed between
// v1.0.1 and v1.1.0 but NOT between v1.0.0 and v1.0.1 (docs-only).
```

### `go/internal/releasepipeline/classify_test.go:70` — above `func TestGitOlderTags_RealRepo(t *testing.T) {`

```text
// TestGitOlderTags_RealRepo pins the sort + strictly-older filter: older than
// v1.1.0 is [v1.0.1, v1.0.0] newest-first; older than v1.0.0 is empty.
```

### `go/internal/releasepipeline/classify_test.go:111` — above `changed:   map[string]bool{"v22.1.0..v22.2.0": true},`

```text
// v22.2.0 changed the binary
```

### `go/internal/releasepipeline/classify_test.go:117` — above `changed:   map[string]bool{"v22.1.0..v22.2.0": true},`

```text
// v22.2.0 introduced the fingerprint (changed vs v22.1.0); v22.2.1 and
// v22.2.2 are config releases → the fingerprint traces back to v22.2.0.
```

### `go/internal/releasepipeline/default_release_verify_test.go:139` — above `tags := runGit("tag", "-l", "v1.2.3")`

```text
// 7. Verify tag v1.2.3 was created
```

### `go/internal/releasepipeline/journal_write_test.go:66`

```text
// NOTE: the former TestDefaultFullDryRunPreflight_Script* tests were removed in
// ADR-0062/T1.3 — defaultFullDryRunPreflight no longer shells out to the deleted
// legacy/scripts/release/full-dry-run.sh. Its Go-native behavior is covered by
// TestDefaultFullDryRunPreflight_NoDeadScript (preflight_no_deadscript_test.go).
```

### `go/internal/releasepipeline/preflight_no_deadscript_test.go:8` — above `func TestDefaultFullDryRunPreflight_NoDeadScript(t *testing.T) {`

```text
// TestDefaultFullDryRunPreflight_NoDeadScript guards the T1.3 fix: the step-0
// "full dry-run preflight" (`evolve release --require-preflight`) must run the
// Go-native preflight, not shell out to legacy/scripts/release/full-dry-run.sh —
// which the 2026-06-18 script→Go migration deleted, making the flag a guaranteed
// hard-fail. Against a throwaway dir the Go preflight errors for a REAL reason
// (no git / no version files); it must never fail by referencing the dead script.
```

### `go/internal/releasepipeline/rebuild_binary_test.go:9` — above `func TestRun_RebuildBinaryStepInvokedBeforeShip(t *testing.T) {`

```text
// TestRun_RebuildBinaryStepInvokedBeforeShip is the regression for
// v12.2.1 bug #2: `evolve release X.Y.Z` previously shipped source
// only, leaving the marketplace binary frozen at the previous build.
// The pipeline now runs RebuildBinary between version-bump and
// release-sh-check, BEFORE ship's `git add -A` picks up the new bytes.
```

### `go/internal/releasepipeline/release_run.go:302` — above `r.logf("NOTE: GitHub CI is NOT verified by this pipeline — confirm the 'go' and 'CI' workflows are green on the release …`

```text
// GitHub CI is intentionally NOT checked here: this pipeline is self-contained
// and gh-free (headless/cron-safe). A green pipeline therefore does NOT imply a
// green `go`/`CI` workflow on the pushed commit (the v20.1.0 false-success: the
// release exited 0 while the released commit's apicover gate was red). Disclose
// the gap loudly; CI-gating lives in the /publish skill (pre-release CI-green
// check + post-release CI watch).
```

### `go/internal/releasepipeline/release_verify_test.go:1` — above `package releasepipeline`

```text
// release_verify_test.go — RED contract for the terminal release-verify step
// (inbox release-rebuild-binary-not-committed acceptance, v18.3.0→v18.5.0
// recurrence): after `evolve release X.Y.Z`, the release must be PROVEN
// self-consistent — tracked go/evolve on disk == the blob in the release
// commit == state.json:expected_ship_sha, `go/evolve --version` reports
// X.Y.Z, and the local tag vX.Y.Z exists at the release commit. A failing
// verify is a post-publish failure: auto-rollback unless --no-rollback.
```

### `go/internal/releasepipeline/releasepipeline.go:62` — above `RebuildBinary   func(repoRoot, target string, dryRun bool) error`

```text
// RebuildBinary runs `go build` with the Makefile-equivalent ldflags
// (pkg/version.version=<target> + commit + builtAt, from <RepoRoot>/go,
// output go/evolve) so the binary tracked at go/evolve is in sync with
// the version-bumped release AND self-reports the target version.
// Without this step, `evolve release X.Y.Z` ships source but leaves
// the marketplace binary frozen at the previous build. Source incident:
// v12.2.1 shipped source 2026-05-26 but marketplace binary stayed at
// v12.1.1 (2026-05-25). The Ship step (--class release) stages the
// rebuilt binary as part of the explicit release set.
```

### `go/internal/releasepipeline/releasepipeline.go:76` — above `ReleaseVerify func(repoRoot, target, commitSHA string) error`

```text
// ReleaseVerify is the terminal self-consistency proof (inbox
// release-rebuild-binary-not-committed, v18.3.0→v18.5.0 recurrence):
// tracked go/evolve on disk == the blob at <commitSHA>:go/evolve ==
// state.json:expected_ship_sha (re-pinned to the committed blob when
// stale — releases never went through repinPostCycle, which is
// cycle-class-only), `go/evolve --version` contains <target>, and the
// local tag v<target> exists at the release commit (created when the
// gh-side release left it remote-only). Failure → post-publish error
// (auto-rollback unless --no-rollback).
```

### `go/internal/releasepipeline/releasepipeline.go:406` — above `func defaultFullDryRunPreflight(repoRoot, target string) error {`

```text
// defaultFullDryRunPreflight runs the Go-native preflight gates in dry-run,
// strict mode as a pre-mutation rehearsal (step 0, opt-in via
// --require-preflight). It REPLACES the deleted
// legacy/scripts/release/full-dry-run.sh (script→Go migration, ADR-0062/T1.3):
// the dead script had made --require-preflight an unconditional hard-fail. The
// real test suite runs at the step-1 preflight, so the rehearsal sets
// skipTests=true to stay fast and avoid double-running it; dryRun=true mutates
// nothing; strictPass=true keeps the rehearsal at least as strict as step 1.
```

### `go/internal/releasepipeline/releasepipeline.go:473` — above `func defaultReleaseSh(repoRoot, target string) error {`

```text
// defaultReleaseSh calls the releaseconsistency Go library directly
// (v11.8.2+; prior versions shelled out to legacy/scripts/utility/release.sh).
// The cache-refresh half of the bash release.sh is intentionally not
// reproduced here — that's environment-specific and removed entirely in
// v12.0.0; the in-pipeline cache flow is handled by marketplace-poll.
```

### `go/internal/releasepipeline/releasepipeline.go:482` — above `func defaultShip(repoRoot, msg, releaseNotes string) (string, error) {`

```text
// defaultShip invokes the native evolve binary's ship subcommand
// (v11.8.3+; prior versions shelled out to legacy/scripts/lifecycle/ship.sh).
// Resolves the binary path via EVOLVE_GO_BIN, then <repoRoot>/go/bin/evolve,
// then <repoRoot>/go/evolve (what rebuild-binary produces), then `evolve` on PATH.
// Returns the new HEAD SHA after the commit lands.
```

### `go/internal/releasepipeline/releasepipeline.go:558` — above `func defaultReleaseVerify(repoRoot, target, commitSHA string) error {`

```text
// defaultReleaseVerify is the terminal release self-consistency proof
// (inbox release-rebuild-binary-not-committed acceptance):
//
//  1. sha256(disk go/evolve) == sha256(blob <commitSHA>:go/evolve) — the
//     binary the release built is the binary the release committed. This is
//     the structural check that failed silently in v18.3.0 and v18.5.0.
//  2. state.json:expected_ship_sha == that sha. Releases never pass through
//     repinPostCycle (cycle-class-only), so a stale pin here is expected on
//     every release — re-pin to the committed blob and log, don't fail.
//  3. `go/evolve --version` contains the target (the ldflags stamp).
//  4. Local tag v<target> exists; `gh release create` tags remote-only, so
//     create the local tag at the release commit when absent.
```

### `go/internal/releasepipeline/resolve_evolve_bin_test.go:123` — above `func TestResolveEvolveBin_TrackedGoEvolve(t *testing.T) {`

```text
// TestResolveEvolveBin_TrackedGoEvolve: the release's rebuild-binary step builds
// to <repoRoot>/go/evolve; resolveEvolveBin must find it when go/bin/evolve is
// absent. Regression guard for the v18.2.0 release failure, where ship reported
// "binary not found" one step after rebuild-binary produced the binary there.
```

### `go/internal/releasepipeline/resolve_evolve_bin_test.go:172`

```text
// NOTE: the former TestDefaultFullDryRunPreflight_Script{Missing,NotExecutable,
// Fails} tests were removed in ADR-0062/T1.3. They asserted the deleted bash
// shell-out (legacy/scripts/release/full-dry-run.sh); the Go-native replacement
// is covered by TestDefaultFullDryRunPreflight_NoDeadScript.
```

### `go/internal/releasepipeline/run_branches_test.go:71` — above `res, err := Run(Options{`

```text
// Hermetic repo with a v0.0.1 tag — deterministic across branches/CI
// (the live workspace's tag set drifts as releases are cut).
```

### `go/internal/releasepipeline/run_branches_test.go:76` — above `FromTag:     "",`

```text
// force auto-resolution → resolves v0.0.1
```

### `go/internal/releasepipeline/run_defaults_test.go:208` — above `deleteTagCmd := exec.Command("git", "-C", dir, "tag", "-d", "v0.0.1")`

```text
// Delete the v0.0.1 tag so resolvePrevTag fails; resolveInitCommit succeeds.
```
