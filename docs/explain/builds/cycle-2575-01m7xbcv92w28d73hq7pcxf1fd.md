# Build Explanation — Cycle 2575

## Build Binding
- Cycle: 2575
- Base SHA: 63f36a9357898ca756cd09e7f945ceb628905b29

## Summary
The `claude-cloud` driver now confirms a launch only by a matching Resume row. The prompt that goes to the cloud holds no challenge token and no absolute path. The driver refuses a launch when the remote branch head differs from the local head.

## Rationale
A View row alone can be forged by text in the task. A Resume row that names the id of the View row is much harder to forge. The cloud prompt leaves the host, so it must not carry the challenge token or local paths. A cloud session clones the remote branch, so the remote head must equal the local head.

## Changed Areas
- `go/internal/bridge/manifest_cloud.go` — `resume_regex` joins the patterns, and `confirmation` pairs the last Resume row with a View row of the same id. A Created row must come before that View row.
- `go/internal/bridge/manifests/claude-cloud.json` — adds `resume_regex`.
- `go/internal/bridge/driver_claudecloud.go` — adds `cloudPromptText`, `cloudPromptReplacer`, `challengeCommentLine`, `absolutePathToken`, `baseOfAbsolutePath`, `verdict` and the second head check after the prompt is written. `challengeCommentLine` removes a challenge-token comment line. `absolutePathToken` finds an absolute path of two or more segments, and `baseOfAbsolutePath` cuts it to its base name.
- `go/internal/bridge/driver_common.go` — `promptChallengeToken` is split from `resolvePromptText` and shared. `workspaceChallengeToken` reads the stored token, and the cloud prompt removes it when no placeholder exists.
- `go/internal/bridge/engine.go` — adds the `Deps.CloudRemoteHead` seam.
- `go/internal/cloudlaunch/repo.go` — `Repo.Head`, `VerifyRemoteHead` and `remoteHead`.
- `go/internal/bridge/driver_claudecloud_test.go`, `go/internal/bridge/manifest_cloud_test.go`, `go/internal/cloudlaunch/repo_test.go` — tests of each change.
- `docs/architecture/packages/internal-bridge.md` — names each change.
- `docs/architecture/packages/internal-cloudlaunch.md` — names the head check.

## Design Decisions
The head check sits behind a `Deps` seam like `CloudRepo`, so tests inject it and production runs `git ls-remote`. The prompt scrub has three passes. First, `challengeCommentLine` removes the challenge-token comment. Second, an ordered replacer removes the token and the known roots, longest path first. Third, `absolutePathToken` cuts each remaining absolute path of two or more segments to its base name. A feature flag was rejected, because the rules always apply.

## Verification
The ACS predicates `TestC2575_001` to `TestC2575_019` pass. The cloud files of `internal/bridge` and `internal/cloudlaunch` have full function coverage.

## Compatibility
The manifest gains the required `resume_regex` key. The `Repo` type gains the `Head` field.

## Limitations
The scrub removes the challenge token and cuts each absolute path of two or more segments to its base name. A lone single-segment path, such as `/tmp`, and a path with a space stay. Relative paths and URLs stay.
