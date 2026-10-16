# Comment history: `internal/ciwatch`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d11

### `go/internal/ciwatch/ciwatch.go:1` — above `package ciwatch`

```text
// Package ciwatch closes the feedback edge from GitHub Actions back into the
// loop (cycle-748, push-ci-watch-remote-parity). The go.yml matrix runs
// ubuntu+macos while every local gate runs macOS-only, so ubuntu-only
// failures are structurally invisible to local gates: main stayed red from
// 2026-07-06 11:50 through 20:16+ across 8 pushes with the loop shipping on
// top. Watch polls the CI run for a pushed SHA until it completes (or times
// out), records the verdict as a workspace artifact the dossier ingests
// (dossier.CIWatchVerdictFile), and on a red conclusion auto-files a critical
// fix-forward inbox item naming the failing job/test with a bounded log
// excerpt. A green run files nothing.
//
// All knobs (enabled, timeout, poll interval) come from the policy.json
// ci_watch block (policy.CIWatchConfig) — zero env flags. Tests inject a fake
// Fetcher; production uses NewGHFetcher (gh CLI, existing gh auth).
```
