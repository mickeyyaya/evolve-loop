# Eval: codequality runner seam coverage

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -race ./internal/codequality/ -run 'TestFormatGoFilesWithRunner_(MissingBinary|NonZeroExit)'`
- `[code]` `cd go && go test -count=1 -coverprofile=/tmp/codequality-runner-seam.cover ./internal/codequality/ >/tmp/codequality-runner-seam.out && awk '/total:/ {gsub("%","",$3); if ($3+0 < 95) exit 1}' <(go tool cover -func=/tmp/codequality-runner-seam.cover)`

## Regression Evals (full test suite)
- `[code]` `cd go && go test -count=1 ./internal/codequality/...`

## Acceptance Checks
- `[code]` `cd go && go test -count=1 ./internal/codequality/ -run TestFormatGoFiles_ReformatsDirtyFile`
- `[code]` `rg -q 'sysexec\\.RunFunc' go/internal/codequality/format.go && ! rg -q 'exec\\.Command\\(\"gofmt\"' go/internal/codequality/format.go`

## Adversarial Cases
- Negative: an injected runner returning an unrecoverable binary-not-found error must make `FormatGoFiles` return a wrapped error.
- Edge/OOD: an injected runner returning a non-zero process exit with no Go error must preserve the existing behavior of reporting parseable dirty files without treating the process exit as infrastructure failure.
- Cheapest gaming fake: adding unrelated tests to raise aggregate coverage while leaving the inline `exec.Command` coupling intact. The named branch tests plus seam assertion must fail that implementation.

## Thresholds
- All checks: pass@1 = 1.0
- Package statement coverage: >= 95.0%
