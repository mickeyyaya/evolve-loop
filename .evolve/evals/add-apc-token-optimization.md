# Eval: Add APC Token Optimization Notes

## Code Graders (bash commands that must exit 0)

```bash
# docs/token-optimization.md contains APC section
grep -c "Agentic Plan Caching" docs/token-optimization.md
```

```bash
# APC section documents cost reduction figure
grep -c "50" docs/token-optimization.md
```

```bash
# Section documents dynamic turn limits
grep -iE "dynamic turn|turn limit" docs/token-optimization.md
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
# File stays under 250 lines
wc -l < docs/token-optimization.md | awk '{exit ($1 > 250)}'
```

## Acceptance Checks (verification commands)

```bash
# Summary table in token-optimization.md includes APC entry
grep -c "Plan Caching" docs/token-optimization.md
```

```bash
# New section references the NeurIPS 2025 paper or APC source
grep -iE "NeurIPS|2025|arxiv" docs/token-optimization.md
```

## Thresholds
- All checks: pass@1 = 1.0
