# Eval: create-code-review-simplify-script

## Graders

```bash
# G1: Script exists and is executable
test -x scripts/code-review-simplify.sh
```

```bash
# G2: Script runs without error on no-change scenario (HEAD = HEAD means no diff)
bash scripts/code-review-simplify.sh HEAD 2>/dev/null; [ $? -eq 0 ]
```

```bash
# G3: Implements nesting depth check
grep -q 'nesting\|indent\|depth' scripts/code-review-simplify.sh
```

```bash
# G4: Implements secrets detection
grep -qE 'secret|password|api.key|token|HARDCODED|SECRET' scripts/code-review-simplify.sh
```

```bash
# G5: Structured output format uses colon-delimited findings
grep -qE '(printf|echo).*:.*:' scripts/code-review-simplify.sh
```

```bash
# G6: Under 200 lines
[ $(wc -l < scripts/code-review-simplify.sh) -le 200 ]
```

```bash
# G7: Accepts git ref argument (checks for $1 or argument handling)
grep -qE '\$1|\$\{1|ARG|REF|ref' scripts/code-review-simplify.sh
```

```bash
# G8: Implements file length check (800 line threshold from SKILL.md)
grep -qE '800|file.*(length|size|lines)' scripts/code-review-simplify.sh
```
