# Eval: trim-evolve-tester-prompt

**Slug:** `trim-evolve-tester-prompt`
**Task:** Remove redundant / verbose / duplicated instructional text from `agents/evolve-tester.md` while preserving every behavioral instruction and required section. Baseline (cycle-389 HEAD `d84e1fa7`): **193 lines / 1320 words**.

**Intent under test:** the prompt is genuinely *smaller* (fewer tokens/lines, fully realized in the committed file — not a scaffold) **and** every load-bearing instruction the Tester depends on still survives. A reduction that drops a behavioral rule, a required section, or the banned-patterns contract is a regression, not a win.

---

## Acceptance Criteria

### AC1 — Real reduction landed (positive case) `[code]`
The committed `agents/evolve-tester.md` is strictly smaller than baseline by a non-trivial margin.
```bash
set -uo pipefail
L=$(wc -l < agents/evolve-tester.md)
W=$(wc -w < agents/evolve-tester.md)
# Baseline 193 lines / 1320 words. Require a real cut, not a 1-line nudge.
test "$L" -le 185 || { echo "FAIL: lines=$L not <=185 (no real reduction)"; exit 1; }
test "$W" -le 1280 || { echo "FAIL: words=$W not <=1280 (no real reduction)"; exit 1; }
echo "PASS: lines=$L words=$W (reduced from 193/1320)"
```

### AC2 — Every required section header preserved (behavior preserved) `[code]`
All structural sections the downstream phases/contract rely on must remain present.
```bash
set -uo pipefail
miss=0
for h in \
  '## Inputs' \
  '## What you produce' \
  '## Banned patterns' \
  '## How to translate an AC into a predicate' \
  '## When verification is impossible' \
  '## What you are NOT allowed to do' \
  '## Reference Index' \
  '## Output Artifact' \
  '## Reflection Authoring'; do
  grep -qF "$h" agents/evolve-tester.md || { echo "MISSING SECTION: $h"; miss=1; }
done
test "$miss" -eq 0 || { echo "FAIL: a required section header was removed"; exit 1; }
echo "PASS: all required section headers present"
```

### AC3 — Banned-patterns contract + predicate metadata intact (behavior preserved) `[code]`
The Tester's core safety contract — the predicate metadata header fields and the banned-grep principle — must not be trimmed away.
```bash
set -uo pipefail
for tok in 'AC-ID:' 'Acceptance-of:' 'validate-predicate.sh' 'presence ≠ execution' 'challenge-token'; do
  grep -qF "$tok" agents/evolve-tester.md || { echo "FAIL: required token removed: $tok"; exit 1; }
done
# All 6 banned-pattern list items (numbered 1..6) must still exist in the Banned patterns section.
n=$(awk '/^## Banned patterns/{f=1;next} /^## /{f=0} f && /^[0-9]+\./' agents/evolve-tester.md | wc -l | tr -d ' ')
test "$n" -ge 6 || { echo "FAIL: banned-patterns list has $n items, expected >=6"; exit 1; }
echo "PASS: metadata + banned-patterns contract intact ($n banned items)"
```

### AC4 — Diff scope: exactly one agent markdown file, no control-plane, no new config (negative case) `[code]`
Guards the cycle-382 (no inert config/flag surface) and cycle-383 (no control-plane) lessons, and the 1-3-file / agent-markdown-only constraint.
```bash
set -uo pipefail
files=$(git diff --name-only HEAD~1 HEAD)
echo "changed: $files"
# Exactly the one file, nothing else.
test "$files" = "agents/evolve-tester.md" || { echo "FAIL: diff touched more/other than agents/evolve-tester.md"; exit 1; }
# Negative: the diff must NOT introduce any new flag/option/config surface.
added=$(git diff HEAD~1 HEAD -- agents/evolve-tester.md | grep '^+' | grep -v '^+++')
echo "$added" | grep -Eq 'EVOLVE_[A-Z_]+|--[a-z-]+ flag|policy\.json.*gate|new flag' && {
  echo "FAIL: reduction added a flag/config/option surface (cycle-382 violation)"; exit 1; }
echo "PASS: single agent-md file, no control-plane, no new config surface"
```

### AC5 — Tail not truncated (edge case) `[code]`
A lazy "reduction" that simply chops the end of the file would lose the Reflection Authoring contract. The file must still terminate with that section, proving the cut was surgical, not a truncation.
```bash
set -uo pipefail
tail -25 agents/evolve-tester.md | grep -qF 'Reflection Authoring' || {
  echo "FAIL: file tail no longer contains Reflection Authoring - looks truncated, not trimmed"; exit 1; }
# Frontmatter (name) must survive at the top.
head -12 agents/evolve-tester.md | grep -qF 'name: evolve-tester' || {
  echo "FAIL: frontmatter name removed"; exit 1; }
echo "PASS: frontmatter + Reflection Authoring tail both present (surgical trim)"
```

---

## Grading

| AC | Grader | Gaming fake it defeats |
|----|--------|------------------------|
| AC1 | `[code]` | whitespace-only reduction / 1-line nudge -> blocked by `<=185 && <=1280` |
| AC2 | `[code]` | deleting a section to hit the line target -> blocked by header presence |
| AC3 | `[code]` | gutting the banned-patterns contract for line savings -> blocked by token + item-count check |
| AC4 | `[code]` | sneaking in a config/flag surface, or editing control-plane -> blocked by single-file + flag-grep |
| AC5 | `[code]` | `head -N` truncation masquerading as a trim -> blocked by tail + frontmatter check |

All five graders are `[code]` (deterministic). AC4 is the negative case (a disallowed change must make the suite RED); AC5 is the edge/OOD case (truncation boundary). Verbs are diverse: `wc`, `grep -F`, `awk`, `git diff`, `tail`/`head` - no diversity collapse.
