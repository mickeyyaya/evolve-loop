# Eval: releasepipeline-default-verify-error-branches

## Task
Add unit tests for uncovered error branches in `go/internal/releasepipeline`:
1. `defaultReleaseVerify` with a relative `repoRoot` → returns "must be absolute" error
2. `defaultReleaseVerify` with an absolute `repoRoot` but no `go/evolve` binary on disk → returns "tracked binary missing on disk" error
3. `defaultShip` when no evolve binary is resolvable → returns "binary not found" error

## Acceptance Criteria

### AC1: `defaultReleaseVerify` rejects relative repoRoot
```bash
cd go && go test -v -run TestDefaultReleaseVerify_RelativeRepoRoot ./internal/releasepipeline/...
```
[code] must exit 0 and print `PASS`

### AC2: `defaultReleaseVerify` fails when binary missing on disk
```bash
cd go && go test -v -run TestDefaultReleaseVerify_MissingBinaryOnDisk ./internal/releasepipeline/...
```
[code] must exit 0 and print `PASS`

### AC3: `defaultShip` fails when no evolve binary is resolvable
```bash
cd go && go test -v -run TestDefaultShip_BinaryNotFound ./internal/releasepipeline/...
```
[code] must exit 0 and print `PASS`

### AC4: `defaultReleaseVerify` coverage improves from 0%
```bash
cd go && go test -coverprofile=/tmp/cover_rp_332.out ./internal/releasepipeline/... && go tool cover -func=/tmp/cover_rp_332.out | grep "defaultReleaseVerify"
```
[code] `defaultReleaseVerify` line must report > 0.0%

### AC5 (negative): `defaultReleaseVerify` with empty commitSHA still validates absolute path
```bash
cd go && go test -v -run TestDefaultReleaseVerify_RelativeRepoRoot_ErrorContainsPath ./internal/releasepipeline/...
```
[code] must exit 0; error message must contain the relative path value
