# Eval: add-missing-phases-research-doc

## Purpose
Verify the builder produced a comprehensive gap-analysis research doc on missing development phases, with sourced external candidates, ADOPT/ALREADY-HAVE/REJECT classification, and one concrete recommendation.

## Criteria

### C1: Research doc exists [code]
```bash
test -f knowledge-base/research/missing-development-phases-extended-2026-06.md
echo "exit: $?"
```
Expected: exit 0 (file exists)

### C2: Baseline phase inventory documented [code]
```bash
grep -c "intent\|scout\|triage\|build\|audit\|ship" knowledge-base/research/missing-development-phases-extended-2026-06.md
```
Expected: count ≥ 5 (current phases listed as baseline)

### C3: ≥5 external candidate phases with named sources [code]
```bash
python3 -c "
import re, sys
content = open('knowledge-base/research/missing-development-phases-extended-2026-06.md').read()
# Count candidates with source citations (arxiv, github, URL patterns, or named frameworks)
source_patterns = [r'arXiv', r'github\.com', r'https?://', r'Source:', r'source:', r'citation:', r'\(Source\)', r'Framework:', r'Agentless', r'OpenHands', r'SWE-agent', r'Devin', r'LangGraph', r'CrewAI', r'AutoGen', r'GitLab', r'IBM DevOps']
found = sum(1 for p in source_patterns if re.search(p, content))
print(f'Source patterns found: {found}')
# Count section headers indicating candidates
candidates = len(re.findall(r'###.*[Cc]andidate|###.*[Pp]hase|##.*[Cc]andidate', content))
print(f'Candidate sections: {candidates}')
sys.exit(0 if found >= 3 else 1)
"
```
Expected: exit 0 (≥3 source patterns found; ≥5 candidates present)

### C4: Gap analysis with ADOPT/ALREADY-HAVE/REJECT classifications [code]
```bash
python3 -c "
content = open('knowledge-base/research/missing-development-phases-extended-2026-06.md').read()
adopt = content.count('ADOPT') + content.count('Adopt')
reject = content.count('REJECT') + content.count('Reject')
have = content.count('ALREADY-HAVE') + content.count('Already-Have') + content.count('already have')
print(f'ADOPT: {adopt}, REJECT: {reject}, ALREADY-HAVE: {have}')
import sys; sys.exit(0 if (adopt + reject + have) >= 5 else 1)
"
```
Expected: exit 0 (sum of classifications ≥ 5 across all candidates)

### C5: Exactly one concrete recommendation for follow-up cycle [code]
```bash
python3 -c "
import re
content = open('knowledge-base/research/missing-development-phases-extended-2026-06.md').read()
# Look for recommendation section
rec_section = re.search(r'##.*[Rr]ecommend', content)
print('Recommendation section found:', bool(rec_section))
# Check for single recommendation (not a list)
single_rec = re.search(r'[Rr]ecommend(?:ed|ation).*?([A-Z][a-z]+(?:-[a-z]+)*)\s+phase', content, re.DOTALL)
print('Single phase recommendation found:', bool(single_rec))
import sys; sys.exit(0 if rec_section else 1)
"
```
Expected: exit 0 (recommendation section present)

### C6: Negative case — doc does not contain vague non-recommendations [code]
```bash
# Verify the doc makes concrete phase-name proposals, not just meta-commentary
python3 -c "
content = open('knowledge-base/research/missing-development-phases-extended-2026-06.md').read()
# Should reference concrete implementation anchors, not just abstract ideas
has_phase_name = any(word in content for word in ['reproduce', 'benchmark', 'telemetry', 'mutation', 'governance', 'localization', 'spike', 'threat-model'])
print('Has concrete phase names:', has_phase_name)
import sys; sys.exit(0 if has_phase_name else 1)
"
```
Expected: exit 0 (at least one concrete phase-name recommendation present)

## Graders
- C1, C2, C3, C4, C5, C6: `[code]`
- Final human review: `[human]` — verify gap analysis is reasoned, not formulaic; sources are real; recommendation is novel vs existing phases.
