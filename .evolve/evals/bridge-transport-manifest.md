# Eval: bridge-transport-manifest

## Task
Add `Transport` field ("tmux"|"headless") to `Manifest` struct and all 7 manifests. Expose `Manifest.IsTmux() bool`. Fix 4 abstraction leaks where non-bridge packages branch on CLI name strings.

## Criteria

### C1 — Manifest struct has Transport field [code]
```bash
grep -n 'Transport.*string' /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/manifest.go
```
Expected: at least one match (the struct field declaration).

### C2 — IsTmux method exists on Manifest [code]
```bash
grep -n 'func.*Manifest.*IsTmux\|func.*IsTmux' /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/manifest.go
```
Expected: at least one match.

### C3 — All 7 manifests declare transport field [code]
```bash
python3 -c "
import os, json
d = '/Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/manifests'
missing = []
for f in os.listdir(d):
    if f.endswith('.json'):
        m = json.load(open(os.path.join(d, f)))
        if 'transport' not in m:
            missing.append(f)
if missing:
    print('MISSING transport in:', missing)
    exit(1)
print('OK: all manifests have transport field')
"
```
Expected: `OK: all manifests have transport field`

### C4 — No HasSuffix-tmux leaks in swarm/adapters/looppreflight [code]
```bash
count=$(grep -rn 'HasSuffix.*tmux' \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/swarm/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/adapters/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/looppreflight/ \
  2>/dev/null | grep -v '_test.go' | wc -l | tr -d ' ')
echo "leak count: $count"
[ "$count" -eq 0 ]
```
Expected: exit 0 (count = 0).

### C5 — Tests pass for all affected packages [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... ./internal/swarm/... ./internal/adapters/... ./internal/looppreflight/... -count=1 -short 2>&1 | tail -15
```
Expected: all `ok` lines, no `FAIL`.

### C6 — Negative: non-tmux manifests return IsTmux()=false [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestManifestIsTmux -v -count=1 2>&1 | grep -E "PASS|FAIL|headless|tmux"
```
Expected: PASS for all cases; headless manifests (claude-p, codex, agy) return false.
