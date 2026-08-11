# Eval: bridge-dialog-auto-respond

**Slug:** bridge-dialog-auto-respond
**Phase target:** tdd + build

## Task
Add per-CLI dialog auto-respond profiles to the bridge (migration step 6):
1. agy rating-prompt auto-skip (dialog fired at session end)
2. idle-with-missing-artifact: one in-pane nudge before relaunch

## Acceptance Criteria

### AC1 — agy rating-prompt rule exists in agy-tmux manifest [code]
```bash
cat /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/manifests/agy-tmux.json | python3 -c "import json,sys; d=json.load(sys.stdin); names=[p['name'] for p in d['interactive_prompts']]; found=any('rating' in n or 'rate' in n for n in names); print('FOUND' if found else 'MISSING')"
```
Expected: `FOUND`

### AC2 — agy rating rule uses policy=auto_respond [code]
```bash
cat /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/manifests/agy-tmux.json | python3 -c "import json,sys; d=json.load(sys.stdin); rules=[p for p in d['interactive_prompts'] if 'rating' in p.get('name','') or 'rate' in p.get('name','')]; print(rules[0]['policy'] if rules else 'NOT FOUND')"
```
Expected: `auto_respond`

### AC3 — idle-with-missing-artifact nudge logic in driver_tmux_repl.go [code]
```bash
grep -cn "nudge\|Nudge\|nudgeSent\|nudgeFired\|missing.*artifact\|artifact.*idle" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/driver_tmux_repl.go 2>/dev/null
```
Expected: integer ≥ 1 (nudge logic present)

### AC4 — bridge tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -timeout 120s 2>&1 | tail -5
```
Expected: All packages `ok`; no FAIL lines.

### AC5 — negative: agy rating rule fires on rating pane, not on normal output [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run "TestRating\|TestAutoRespondRating\|TestAgyRating\|TestAutoRespond" -v 2>&1 | grep -E "^---\s+PASS|^---\s+FAIL|^ok|FAIL$" | tail -10
```
Expected: No FAIL lines.

### AC6 — nudge fires exactly once (guard variable present) [code]
```bash
grep -n "nudgeSent\|nudgeCount\|nudgeFired\|nudgeAttempt\|nudge.*bool\|once.*nudge" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/driver_tmux_repl.go
```
Expected: ≥1 line — confirms single-fire guard exists.

### AC7 — negative: nudge does NOT fire when artifact already present [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run "TestNudge\|TestMissingArtifact\|TestIdleNudge\|TestArtifactPresent" -v 2>&1 | grep -E "^---\s+PASS|^---\s+FAIL|^ok|FAIL$" | tail -5
```
Expected: No FAIL lines.
