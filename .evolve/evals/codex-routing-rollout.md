# Eval: codex-routing-rollout

## Acceptance Criteria

### AC1: Router persona references codex-tmux as a routable CLI [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
grep -q "codex" agents/evolve-router.md \
  || { echo "FAIL: codex not mentioned in agents/evolve-router.md"; exit 1; }
echo "PASS: codex referenced in router persona"
```

### AC2: codex-tmux bridge manifest exists and is valid JSON [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
test -f go/internal/bridge/manifests/codex-tmux.json \
  || { echo "FAIL: codex-tmux.json bridge manifest missing"; exit 1; }
python3 -c "import json; json.load(open('go/internal/bridge/manifests/codex-tmux.json'))" \
  || { echo "FAIL: codex-tmux.json not valid JSON"; exit 1; }
echo "PASS: codex-tmux manifest present and valid"
```

### AC3: Config-only — no Go source files changed [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
changed_go=$(git diff HEAD --name-only | grep '\.go$' || true)
test -z "$changed_go" \
  || { echo "FAIL: Go source modified (must be config-only): $changed_go"; exit 1; }
echo "PASS: no .go changes"
```

### AC4 (NEGATIVE): codex-tmux not added to mandatory_phases [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
python3 -c "
import json
with open('.evolve/phase-registry.json') as f:
    r = json.load(f)
mandatory = r.get('config', {}).get('mandatory_phases', [])
assert 'codex-tmux' not in mandatory, f'FAIL: codex-tmux in mandatory_phases: {mandatory}'
print('PASS: codex-tmux not in mandatory_phases')
"
```

### AC5 (EDGE): Router persona section mentions codex-tmux tier or model-tier-map [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
# The router must show codex-tmux with its tier capabilities (fast/balanced/deep)
grep -i "codex" agents/evolve-router.md | grep -qi "tier\|model\|balanced\|fast\|deep\|gpt" \
  || { echo "FAIL: codex entry in router persona lacks tier/model guidance"; exit 1; }
echo "PASS: codex routing entry includes tier/model context"
```
