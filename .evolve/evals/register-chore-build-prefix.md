# Eval: register-chore-build-prefix

Register `chore(build)` in `.evolve/commit-prefix-scope.json` so the commit-prefix-gate stops emitting INFO noise for binary-rebuild commits.

## Criterion 1: chore(build) key exists in manifest [code]

```bash
python3 -c "
import json
d = json.load(open('.evolve/commit-prefix-scope.json'))
assert 'chore(build)' in d['prefixes'], 'chore(build) key missing from prefixes'
print('PASS')
"
```

Expected: exits 0, prints `PASS`.

## Criterion 2: required_paths covers tracked binary artifacts [code]

```bash
python3 -c "
import json
d = json.load(open('.evolve/commit-prefix-scope.json'))
paths = d['prefixes']['chore(build)'].get('required_paths', [])
assert any('go/evolve' in p or 'go/bin' in p for p in paths), \
    f'required_paths {paths} must cover go/evolve or go/bin/**'
print('PASS')
"
```

Expected: exits 0, prints `PASS`.

## Criterion 3: schema_version preserved at 1 [code]

```bash
python3 -c "
import json
d = json.load(open('.evolve/commit-prefix-scope.json'))
assert d['schema_version'] == 1, f'schema_version changed to {d[\"schema_version\"]}'
print('PASS')
"
```

Expected: exits 0, prints `PASS`.

## Criterion 4: description field present [code]

```bash
python3 -c "
import json
d = json.load(open('.evolve/commit-prefix-scope.json'))
e = d['prefixes']['chore(build)']
assert 'description' in e and len(e['description']) > 0, 'description field missing or empty'
print('PASS')
"
```

Expected: exits 0, prints `PASS`.

## Criterion 5 (negative): entry is NOT any_path=true [code]

```bash
python3 -c "
import json
d = json.load(open('.evolve/commit-prefix-scope.json'))
e = d['prefixes']['chore(build)']
assert e.get('any_path') is not True, \
    'chore(build) must be scoped (required_paths), not any_path=true — that defeats the purpose'
print('PASS')
"
```

Expected: exits 0, prints `PASS`. Negative case: verifies the entry is more specific than the generic `chore` entry (guards against a trivially passing but valueless implementation).

## Criterion 6 (edge): JSON file remains valid after edit [code]

```bash
python3 -m json.tool .evolve/commit-prefix-scope.json > /dev/null && echo PASS
```

Expected: exits 0, prints `PASS`.
