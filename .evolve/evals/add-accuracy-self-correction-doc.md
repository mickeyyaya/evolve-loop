# Eval: Add Accuracy Self-Correction Documentation

## Code Graders (bash commands that must exit 0)

```bash
# docs/accuracy-self-correction.md exists and is non-empty
test -s docs/accuracy-self-correction.md
```

```bash
# Document covers at least 3 distinct accuracy techniques
grep -c "^##" docs/accuracy-self-correction.md | awk '{exit ($1 < 3)}'
```

```bash
# Document references chain-of-thought or step-by-step reasoning
grep -iE "chain.of.thought|step-by-step|CoT" docs/accuracy-self-correction.md
```

## Regression Evals (full test suite)

```bash
# No broken links introduced
python3 -c "
import os, re
broken = []
for root, dirs, files in os.walk('.'):
    dirs[:] = [d for d in dirs if d not in ['.evolve', '.git']]
    for fname in files:
        if not fname.endswith('.md'):
            continue
        fpath = os.path.join(root, fname)
        with open(fpath, 'r', errors='ignore') as f:
            content = f.read()
        links = re.findall(r'\[.*?\]\(([^)]+\.md[^)]*)\)', content)
        for link in links:
            link_path = link.split('#')[0].strip()
            if link_path.startswith('http'):
                continue
            source_dir = os.path.dirname(fpath)
            target = os.path.normpath(os.path.join(source_dir, link_path))
            if not os.path.exists(target):
                broken.append(f'{fpath} -> {link}')
if broken:
    for b in broken:
        print('BROKEN:', b)
    exit(1)
print('All links valid')
"
```

```bash
# File stays under 200 lines (focused doc)
wc -l < docs/accuracy-self-correction.md | awk '{exit ($1 > 200)}'
```

## Acceptance Checks (verification commands)

```bash
# docs/ directory listing includes accuracy-self-correction.md
test -f docs/accuracy-self-correction.md
```

```bash
# Document mentions evolve-loop relevance (not purely abstract)
grep -iE "evolve.loop|pipeline|cycle|scout|builder|auditor" docs/accuracy-self-correction.md
```

## Thresholds
- All checks: pass@1 = 1.0
