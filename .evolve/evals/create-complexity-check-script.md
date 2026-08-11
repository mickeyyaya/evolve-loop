# Eval: create-complexity-check-script

## Graders

```bash
# G1: Script exists and is executable
test -x scripts/complexity-check.sh
```

```bash
# G2: Running with no arguments shows usage and exits non-zero
output=$(bash scripts/complexity-check.sh 2>&1); rc=$?; echo "$output" | grep -qi 'usage' && [ $rc -ne 0 ]
```

```bash
# G3: Supports --threshold flag
grep -q '\-\-threshold' scripts/complexity-check.sh
```

```bash
# G4: Counts control flow keywords for complexity scoring
grep -qE '(if|for|while|case|catch)' scripts/complexity-check.sh && grep -qE '(complexity|score|count)' scripts/complexity-check.sh
```

```bash
# G5: Under 120 lines
[ $(wc -l < scripts/complexity-check.sh) -le 120 ]
```

```bash
# G6: Outputs per-function results (colon-delimited format)
grep -qE '(printf|echo).*:.*:' scripts/complexity-check.sh
```
