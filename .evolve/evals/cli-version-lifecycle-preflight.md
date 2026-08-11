# Eval: cli-version-lifecycle-preflight

## Code Graders (bash commands that must exit 0)

- `[code]` `cd go && go test -run 'TestCLIVersionInventory|TestVersionDriftDetection|TestVersionDrift_FrozenCLIChanged|TestInventory_LandsInPreflight' -v ./internal/looppreflight/... 2>&1 | grep -qE 'PASS'`
- `[code]` `cd go && go test -run 'TestVersionDrift_Fires_On_Synthetic_Transition' -v ./internal/looppreflight/... 2>&1 | grep -qE 'PASS'`

## Regression Evals

- `[code]` `cd go && go test ./internal/looppreflight/... 2>&1 | grep -qE '^ok'`

## Acceptance Checks

- `[code]` `grep -qn 'cli_versions\|CLIVersions\|cliVersions\|versionInventory\|VersionInventory' go/internal/looppreflight/result_json.go go/internal/looppreflight/looppreflight.go 2>/dev/null || grep -qrn 'CLIVersions\|versionInventory' go/internal/looppreflight/`
- `[code]` `grep -qrn 'drift\|Drift\|driftDetect\|checkVersionDrift\|CLIVersionDrift' go/internal/looppreflight/`

## Negative Cases (drift detection fires on version change, not on matching versions)

- `[code]` `cd go && go test -run 'TestVersionDrift_NoWarnWhenVersionUnchanged|TestVersionDrift_NoWarnWhenNoPriorRecord' -v ./internal/looppreflight/... 2>&1 | grep -qE 'PASS'`

## Thresholds

- All checks: pass@1 = 1.0
