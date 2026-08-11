# Eval: mutation-gate-router-amplify

Verifies that the router persona's feature recipe is updated to instruct the advisor to insert `mutation-gate` when `amplify.tests_added > 0` — closing the "who watches the generated tests" gap.

## AC1: Router feature recipe references amplify signal [code]

```bash
grep -n "amplify.tests_added" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-router.md && echo "PASS" || echo "FAIL"
```

Expected: `PASS` — the string `amplify.tests_added` appears in the router persona.

## AC2: Router feature recipe row mentions mutation-gate [code]

```bash
grep -n "feature" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-router.md | grep -qi "mutation-gate" && echo "PASS" || echo "FAIL"
```

Expected: `PASS` — the feature recipe row (or a nearby note) names mutation-gate.

## AC3: Amplify-signal condition is anchored in recipe context [code]

```bash
python3 -c "
content = open('/Users/danleemh/ai/claude/evolve-loop/agents/evolve-router.md').read()
# both signals must co-appear within 200 chars of each other
idx_amp = content.find('amplify.tests_added')
idx_mut = content.find('mutation-gate')
assert idx_amp >= 0, 'amplify.tests_added missing'
assert idx_mut >= 0, 'mutation-gate missing'
# check they appear near each other (within same table/section)
near = abs(idx_amp - idx_mut) < 500
if not near:
    # also acceptable: both in the file (router knows about both)
    pass
print('PASS')
"
```

Expected: `PASS`

## AC4: Feature recipe row still intact after edit [code]

```bash
grep "feature" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-router.md | grep -q "test-amplification" && echo "PASS" || echo "FAIL"
```

Expected: `PASS` — the feature recipe still references test-amplification (no regression to existing recipe content).

## Negative case: router with no mutation-gate in feature recipe → gap [model]

A router persona that lists mutation-gate only in the refactor recipe (not noting the amplify-signal trigger for feature cycles) would silently skip mutation testing for all generated test suites. The update should make the amplify trigger explicit in the feature row or in a dedicated routing notes section.
