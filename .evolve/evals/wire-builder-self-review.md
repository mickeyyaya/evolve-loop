# Eval: wire-builder-self-review

## Graders

```bash
# G1: Builder references code-review-simplify
grep -q 'code-review-simplify' agents/evolve-builder.md
```

```bash
# G2: Reference is in context of self-review or post-implementation
grep -A5 'code-review-simplify' agents/evolve-builder.md | grep -qiE 'self.review|after.*implement|before.*report|lightweight'
```

```bash
# G3: Contains correct path reference to script or skill
grep -qE 'scripts/code-review-simplify\.sh|skills/code-review-simplify' agents/evolve-builder.md
```

```bash
# G4: Existing Step 5 self-verify section unchanged
grep -q 'Step 5: Self-Verify' agents/evolve-builder.md
```

```bash
# G5: All original steps 1-9 still present (additive only)
for step in 1 2 3 4 5 6 7 8 9; do grep -q "Step $step:" agents/evolve-builder.md || exit 1; done
```

```bash
# G6: Self-review is marked optional (does not block build)
grep -A10 'code-review-simplify' agents/evolve-builder.md | grep -qiE 'optional|if.*exist|skip|fallback|non.blocking'
```
