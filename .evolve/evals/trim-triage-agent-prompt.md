# Eval: trim-triage-agent-prompt

Task: Remove redundant/verbose instructional text from `agents/evolve-triage.md` while
preserving every behavioral rule, required section, and template marker. No new
policy/config/option/flag surface. Agent markdown only.

## Behavior under test
The reduction must be *real* (fewer lines in the committed file) and *behavior-preserving*
(all Core Principles 1-6, all Process steps, both output templates with their markers, and the
Reflection section survive). The cheapest gaming fake -- relocating content, or deleting a
load-bearing rule to shrink the file -- must fail.

## Graders

### [code] AC1 reduction-is-real
The file must have strictly fewer lines than the 256-line baseline, but stay >=200 (deletion of
real rules, not a gut).
```bash
lines=$(wc -l < agents/evolve-triage.md)
test "$lines" -lt 256 && test "$lines" -ge 200 && echo PASS || { echo "FAIL lines=$lines"; exit 1; }
```

### [code] AC2 required-sections-preserved
Every load-bearing header/marker must still be present (behavior preservation).
```bash
set -e
while IFS= read -r needle; do
  [ -z "$needle" ] && continue
  grep -qF "$needle" agents/evolve-triage.md || { echo "FAIL missing: $needle"; exit 1; }
done <<'NEEDLES'
# Evolve Triage
## Inputs
## Core Principles
### 5. Blockers ride alone
### 6. Research cache field passthrough
## Process (single-pass)
### 0a. Idempotency skip-list
### 3a. PSMAS phase_skip
### 3b. Predicate-graph reachability risk floor
### 4. Write the decision
### 5. Final checks before exit
## Reflection Authoring
ANCHOR:triage_decision
challenge-token
triage-decision.json
committed_floors
NEEDLES
echo PASS
```

### [code] AC3 no-new-config-surface
The diff must not introduce a new policy/flag/option key (cycle-382 inert-config trap). The
reduction only removes prose; it must not add EVOLVE_* flags or new workflow.* keys absent from
the baseline file.
```bash
base=$(git show HEAD:agents/evolve-triage.md 2>/dev/null | grep -oE 'EVOLVE_[A-Z_]+|workflow\.[a-z_]+' | sort -u)
new=$(grep -oE 'EVOLVE_[A-Z_]+|workflow\.[a-z_]+' agents/evolve-triage.md | sort -u)
added=$(comm -13 <(printf '%s\n' "$base") <(printf '%s\n' "$new"))
test -z "$added" && echo PASS || { echo "FAIL added config tokens: $added"; exit 1; }
```

### [code] AC4 scope-single-file (negative)
Exactly the agent prompt changed; no control-plane surface touched.
```bash
files=$(git diff --name-only HEAD -- 2>/dev/null | grep -vE '^(\.evolve/runs/|\.evolve/evals/)' || true)
bad=$(printf '%s\n' "$files" | grep -E '^(go/internal/acssuite|go/internal/guards|\.evolve/profiles|\.github|.*flagregistry)' || true)
test -z "$bad" && echo PASS || { echo "FAIL control-plane touched: $bad"; exit 1; }
```

### [code] AC5 build-still-green (edge)
Go build must remain green -- the trim must not touch any Go-consumed string.
```bash
cd go && go build ./... && echo PASS || { echo FAIL; exit 1; }
```

## Negative / edge cases
- Negative: deleting any Core Principle (1-6) or Process step header -> AC2 FAILs.
- Negative: adding an EVOLVE_* flag or new workflow.* key -> AC3 FAILs.
- Edge: a no-op or content-relocation keeping line count >=256 -> AC1 FAILs.
- Diversity: verbs wc / grep -qF / comm / git diff / go build -- no collapse.
