---
score_cap:
  - criterion: "internal/gittest provides FaultOn hook failing targeted git verb with descriptive output"
    max_if_missing: 8
    evidence: "cd go && go test -v -run 'TestFaultOn_VerbFails' ./internal/gittest/..."
  - criterion: "internal/gittest FaultOn forwards untargeted verbs to real git execution without error"
    max_if_missing: 8
    evidence: "cd go && go test -v -run 'TestFaultOn_NonTargetedVerbPasses' ./internal/gittest/..."
  - criterion: "gittest BranchExists verifies branch existence without raw exec of real git"
    max_if_missing: 7
    evidence: "cd go && go test -v -run 'TestBranchExists' ./internal/gittest/..."
  - criterion: "E3 worktree tests inject git faults and check branches through gittest, with no raw exec of the real git"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1893_008' ./acs/cycle1893"
---

# Eval: internal/gittest fault hook and clean branch existence helper

> Pins the behavior of the fault hook and branch existence helpers in `internal/gittest`.
> Provides `gittest.FaultOn` to install a PATH shim failing a specific git verb while forwarding others,
> and `gittest.BranchExists` to check branch existence without raw `exec.Command` in test call-sites.
> Replaces E3 manual `bfStub` git shims in worktree backfill tests.
> Source: inbox item gittest-fault-hook (2026-10-12), cycle 1893.

## Acceptance Criteria

### AC-1: FaultOn fails targeted git verb with exit code and error output [code]
```bash
cd go && go test -v -run 'TestFaultOn_VerbFails' ./internal/gittest/...
```

### AC-2: FaultOn allows non-targeted verbs to execute cleanly [code]
```bash
cd go && go test -v -run 'TestFaultOn_NonTargetedVerbPasses' ./internal/gittest/...
```

### AC-3: BranchExists inspects branches accurately via gittest helper [code]
```bash
cd go && go test -v -run 'TestBranchExists' ./internal/gittest/...
```

### AC-4: Edge case: FaultOn with skip count allows N successful invocations before failing [code]
```bash
cd go && go test -v -run 'TestFaultOn_SkipCount' ./internal/gittest/... # empty boundary
```

### AC-5: Negative case: E3 worktree cleanup failure handling verifies injected git failure [code]
```bash
cd go && go test -v -run 'TestWorktreeCleanupStale_FailuresToPlanExitOne' ./cmd/evolve/... || exit 1
```

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fault-injection | FaultOn fails targeted git verb | 8/10 | `TestFaultOn_VerbFails` |
| real-passthrough | Non-targeted verbs pass through to real git | 8/10 | `TestFaultOn_NonTargetedVerbPasses` |
| branch-helper | BranchExists helper identifies branches cleanly | 7/10 | `TestBranchExists` |
| edge-case | FaultOn skip count triggers failure only after threshold | 6/10 | `TestFaultOn_SkipCount` |
| no-raw-git | E3 worktree tests use the kit, no raw git exec | 7/10 | `TestC1893_008` |
| negative-case | Injected git failure triggers expected exit code 1 | 6/10 | `TestWorktreeCleanupStale_FailuresToPlanExitOne` |
