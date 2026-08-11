# Eval: dedup-scrollback-capture
<!-- cycle: 257 -->

Eliminate the double deep-scrollback `CapturePane` call per successful phase. The artifact-found branch (line 496) and `tmuxCleanup` (line 676) currently both issue `CapturePane(session, artifactScrollback)` independently. Pass the already-captured pane from the artifact-found branch to `tmuxCleanup` so the file write reuses the captured data, halving scrollback I/O.

## Acceptance Criteria

### 1. Successful phase issues exactly one final-depth CapturePane call [code]

```bash
cd go && go test ./internal/bridge/... -run TestSingleScrollbackCapturePerPhase -v 2>&1
# Expected: test exits 0; fakeTmux.captureScrollback contains exactly ONE entry
#           equal to tmuxArtifactScrollback (10000) for a happy-path phase;
#           the second deep capture is absent
```

### 2. Negative case: boot-phase captures (bootScrollback=0) are NOT counted [code]

```bash
cd go && go test ./internal/bridge/... -run TestSingleScrollbackCapturePerPhase/boot_captures_excluded -v 2>&1
# Expected: PASS — only the single final-depth capture is present;
#           zero-depth captures (boot polls) appear in the count but are
#           not mis-counted as final captures
```

### 3. tmux-final-scrollback.txt is still written with correct content [code]

```bash
cd go && go test ./internal/bridge/... -run TestTmuxPhase_WritesTokenUsage -v 2>&1 | tail -10
# Expected: PASS — the existing token-usage and scrollback sidecar tests still pass
#           (scrollback file write is not broken by the dedup refactor)
```

### 4. Full bridge suite regression-free [code]

```bash
cd go && go test ./internal/bridge/... 2>&1 | tail -5
# Expected: ok github.com/mickeyyaya/evolve-loop/go/internal/bridge
```
