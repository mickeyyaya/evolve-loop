---
score_cap:
  - criterion: "captureWithEBADFRetry absorbs one transient EBADF/closed-pipe, surfaces persistent failures after exactly one retry, and never retries non-EBADF errors"
    max_if_missing: 6
    evidence: "cd go && go test -run TestCaptureWithEBADFRetry ./internal/phases/ship/"
  - criterion: "The EBADF mitigation stays test-infra only — the helper is never referenced from production ship/ files"
    max_if_missing: 7
    evidence: "test \"$(grep -rl 'captureWithEBADFRetry' go/internal/phases/ship/ --include='*.go' | grep -v '_test.go' | wc -l | tr -d ' ')\" = \"0\""
---

# Eval: Harden ship tests against macOS pipe-read EBADF flake

> Pins the cycle-249 fix for the recurring `macos-latest` CI flake where
> `TestShipFromWorktree_GitAddFails_Errors` fails with
> `read |0: bad file descriptor` — a darwin pipe-teardown race in the test
> git-runner's CombinedOutput path. The mitigation is a test-only capture
> helper (`captureWithEBADFRetry`) that retries exactly once on
> syscall.EBADF / io.ErrClosedPipe and passes every other error through
> untouched, so genuine git failures are never masked. Source incident:
> inbox `macos-ci-ebadf-flake-hardening` (cycle 249); prior fix `b4d643a`
> covered temp-dir EBADF but not the pipe-read class.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| retry-semantics | Transient absorbed, persistent surfaced, non-EBADF untouched | 6/10 | `go test -run TestCaptureWithEBADFRetry ./internal/phases/ship/` |
| test-infra-boundary | Helper referenced only from _test.go files | 7/10 | non-test reference count = 0 |
