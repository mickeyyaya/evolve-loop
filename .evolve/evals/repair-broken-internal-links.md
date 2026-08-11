# Eval: Repair Broken Internal Links

## Code Graders (bash commands that must exit 0)

```bash
# SKILL.md: ../docs/configuration.md is corrected to ../../docs/configuration.md
grep -n '\.\./docs/configuration\.md' skills/evolve-loop/SKILL.md
```

```bash
# SKILL.md: ../docs/domain-adapters.md is corrected to ../../docs/domain-adapters.md
grep -n '\.\./docs/domain-adapters\.md' skills/evolve-loop/SKILL.md
```

```bash
# SKILL.md: docs/genes.md is corrected (no longer bare docs/genes.md relative to skills/evolve-loop/)
# After fix, should use ../../docs/genes.md
python3 -c "
import os, re
broken = []
for fpath in ['skills/evolve-loop/SKILL.md', 'skills/evolve-loop/phase5-learn.md']:
    with open(fpath) as f:
        content = f.read()
    links = re.findall(r'\[.*?\]\(([^)]+\.md[^)]*)\)', content)
    for link in links:
        link_path = link.split('#')[0].strip()
        if link_path.startswith('http'):
            continue
        source_dir = os.path.dirname(fpath)
        target = os.path.normpath(os.path.join(source_dir, link_path))
        if not os.path.exists(target):
            broken.append(f'{fpath}: {link}')
if broken:
    for b in broken:
        print('BROKEN:', b)
    exit(1)
print('All links valid')
"
```

## Regression Evals (full test suite)

```bash
# No broken links anywhere in the project after fix
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
print(f'All links valid ({len([f for root, d, fs in os.walk(\".\") for f in fs if f.endswith(\".md\")])} files checked)')
"
```

## Acceptance Checks (verification commands)

```bash
# SKILL.md contains correct path to docs/configuration.md
grep -c '../../docs/configuration\.md' skills/evolve-loop/SKILL.md
```

```bash
# SKILL.md contains correct path to docs/domain-adapters.md
grep -c '../../docs/domain-adapters\.md' skills/evolve-loop/SKILL.md
```

```bash
# phase5-learn.md contains correct path to docs/genes.md
grep -c '../../docs/genes\.md' skills/evolve-loop/phase5-learn.md
```

## Thresholds
- All checks: pass@1 = 1.0
