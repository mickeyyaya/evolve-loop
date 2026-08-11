# Eval: spinner-progress-disambiguation

## Metadata
- slug: spinner-progress-disambiguation
- cycle: 186
- task: Add `PaneHasSubstantiveChange(prev, cur string) bool` to bridge/stopreview.go (strip volatile spinner/elapsed-time/token-counter lines before comparing) and wire it into driver_tmux_repl.go in place of the raw `curPane != intervalBaselinePane` diff — closes ADR-0026 Stage 1 item #4.

> Pins the spinner-disambiguation fix (ADR-0026 Stage 1 #4). Before this, a pure
> spinner/clock animation read as `Progressed=true`, so a spinner-stuck agent
> rode the maxExtends backstop (~30 min) before being paused. PaneHasSubstantiveChange
> strips volatile chrome (braille spinner, "Deliberating… Xm Ys · ↓ X.Xk tokens")
> so spinner-only / elapsed-only / identical snapshots are NOT progress, while
> genuine new output still is. Source: cycle-186 (re-implements cycle-184/185's unshipped
> work; cycle-184 FAILed audit on mixed ACS predicate — not a code fault; cycle-185 reset);
> scout-report.md Task 2; incident cycle-109 (slow-but-productive Scout killed at 300s).

## Acceptance Criteria

### AC-1: PaneHasSubstantiveChange — spinner-only/elapsed-only/identical → false; real/changed/mixed → true [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/ -run '^TestPaneHasSubstantiveChange$' -count=1 -timeout 60s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestPaneHasSubstantiveChange` (no "no tests to run" line)

### AC-2: driver_tmux_repl.go computes progress via PaneHasSubstantiveChange (raw `!=` replaced) [code]
```bash
grep -c "PaneHasSubstantiveChange" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/driver_tmux_repl.go
```
Expected: at least 1

### AC-3: bridge package still passes in full (driver + reviewer suite, no regression) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/ -count=1 -timeout 120s 2>&1 | grep -E "^ok |^FAIL "
```
Expected: a line starting with `ok` (no FAIL)

### AC-4: ADR-0026 records Stage 1 item #4 shipped in cycle-186 [code]
```bash
grep -ci "cycle-186" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/adr/0026-self-healing-review-layer.md
```
Expected: at least 1

## Negative Cases

### NC-1: a bare full-pane `!=` (no stripping) would treat a spinner frame as progress — the implementation MUST strip it [code]
The "spinner frame advance only → false" subtest is the anti-no-op guard: an
implementation that just returned `prev != cur` would FAIL it. Verified by the
subtest being part of AC-1's table run.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/ -run '^TestPaneHasSubstantiveChange$/spinner_frame_advance_only' -count=1 -timeout 60s -v 2>&1 | grep -E "--- PASS|--- FAIL"
```
Expected: a `--- PASS:` line for the spinner-frame-advance-only subtest (no FAIL)
