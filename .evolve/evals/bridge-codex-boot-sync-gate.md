# Eval: bridge-codex-boot-sync-gate

## Task
Fix the codex-tmux boot-sync gap (inbox HIGH: codex-update-menu-swallows-injection).
Three layers: (a) boot-sync gate — verify the per-CLI REPL prompt-marker is present BEFORE
injecting; handle codex's update-menu ('Update available!' + 'Press enter to continue') by
sending '2\n' (Skip) via autoresponder or by pre-writing dismissed_version into
~/.codex/version.json at preflight; (b) FatalPaneDetector signature for shell-spill ('zsh:
command not found', 'quote>', bare `user@host %` prompt post-launch); (c) observer stall
policy: a dead shell prompt is not CLI liveness.

## Criteria

### C1 — Boot-sync gate: injection withheld until REPL prompt confirmed [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestBootSyncGate -v -count=1 -timeout 30s 2>&1 | grep -E 'PASS|FAIL|boot.*sync|inject.*withheld|prompt.*marker'
```
Expected: `PASS` and at least one log line confirming the gate logic.

### C2 — Update-menu dismissed (skip path) in test [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestCodexUpdateMenuDismiss -v -count=1 -timeout 30s 2>&1 | grep -E 'PASS|FAIL|update.menu|dismiss|Skip'
```
Expected: `PASS`; test simulates the update-menu pane output and confirms it is dismissed
before prompt injection proceeds.

### C3 — Shell-spill pane classified as fatal [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestFatalPaneShellSpill -v -count=1 -timeout 30s 2>&1 | grep -E 'PASS|FAIL|shell.spill|fatal|zsh.*command.*not.*found|quote>'
```
Expected: `PASS`; the test verifies `fatalPaneVerdict` returns a non-empty fatal verdict for
a pane containing 'zsh: command not found' or `quote>` after a failed inject.

### C4 — Negative: normal REPL pane not classified as fatal [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestFatalPaneNoFalsePositive -v -count=1 -timeout 30s 2>&1 | grep -E 'PASS|FAIL'
```
Expected: `PASS`; normal codex `›` prompt pane is not classified as fatal.

### C5 — Bridge package still passes full suite [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -timeout 120s 2>&1 | tail -5
```
Expected: all `ok` lines; no `FAIL`.
