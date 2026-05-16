---
name: security-review-scored
description: Security-focused code review that emits a numeric composite score (0.0–1.0) suitable for the evolve-loop Builder self-review convergence loop
argument-hint: "[--tier lightweight|standard|full]"
---

> Security review skill with scored output. Reads git diff autonomously, analyzes 5 security dimensions, emits `Composite Score: 0.XX` for loop integration. Formula: `1.0 - (critical×0.5 + high×0.2 + medium×0.05)`. Single-writer safe.

## Contents
- [Architecture](#architecture) — security-focused single-pass analysis
- [Single-Pass Flow](#single-pass-flow) — adaptive diff loading + 5-dimension scan
- [Scoring Formula](#scoring-formula) — severity-weighted composite
- [Output Schema](#output-schema) — structured security report
- [Integration Hooks](#integration-hooks) — evolve-loop builder wiring

## Architecture

Security specialist review. Reads the diff once, evaluates 5 security dimensions, produces a severity-weighted composite score that the Builder convergence loop can parse.

```
Input: git diff (changed files)
         │
         ▼
┌─────────────────────────┐
│  LOAD CONTEXT (once)    │  Adaptive HEAD / HEAD~1 detection
│  git diff HEAD --stat   │
└─────────┬───────────────┘
          │
          ▼
┌─────────────────────────┐
│  5-DIMENSION SCAN       │
│  1. Injection vectors   │  SQL, shell, command, template injection
│  2. Auth/authz gaps     │  Missing auth checks, privilege escalation
│  3. Sensitive exposure  │  Hardcoded secrets, credentials, PII logging
│  4. Crypto misuse       │  Weak algorithms, static IVs, predictable seeds
│  5. Input validation    │  Missing bounds, unsanitized input reaching sinks
└─────────┬───────────────┘
          │
          ▼
┌─────────────────────────┐
│  SEVERITY SCORING       │  CRITICAL / HIGH / MEDIUM / LOW counts
│  Composite formula:     │  1.0 - (crit×0.5 + high×0.2 + med×0.05)
│  Capped at 0.0 floor    │
└─────────────────────────┘
```

## Single-Pass Flow

### Step 1: LOAD (once)

```bash
# Adaptive context detection — same as code-review-simplify
DIFF_STAT=$(git diff HEAD --stat 2>/dev/null || echo "")
if [ -n "$DIFF_STAT" ]; then
  REF="HEAD"    # pre-commit: review uncommitted Builder changes
else
  REF="HEAD~1"  # standalone: review last committed change
fi
DIFF=$(git diff "$REF" --stat)
CHANGED_FILES=$(git diff "$REF" --name-only)
DIFF_FULL=$(git diff "$REF")
```

### Step 2: SCAN (5 dimensions)

Analyze each changed file for security issues across all 5 dimensions:

| Dimension | Patterns to Check | Severity Guide |
|-----------|-------------------|----------------|
| **Injection** | User input flowing to shell commands, SQL queries, template engines, `eval`-equivalent calls | CRITICAL if direct; HIGH if indirect |
| **Auth/Authz** | New endpoints or functions without authentication guards; privilege checks that can be bypassed; hardcoded role grants | CRITICAL if unauthenticated access to sensitive data; HIGH if partial bypass |
| **Sensitive Exposure** | Hardcoded passwords, API keys, tokens, private keys; logging of sensitive fields (passwords, tokens, PII); error messages leaking stack traces or internal paths | CRITICAL if secret committed; HIGH if PII logged |
| **Crypto Misuse** | Weak hash algorithms (MD5, SHA1 for passwords); static IVs or nonces; insecure random for security-sensitive values (`Math.random`, `random.random`); deprecated cipher modes (ECB) | HIGH if password hashing; MEDIUM if non-password crypto |
| **Input Validation** | Missing length/type/range checks on external input before reaching database, filesystem, or network sinks; path traversal risks (`../`); integer overflow potential | HIGH if reaching a sink; MEDIUM if internal only |

### Step 3: SCORE

Count findings by severity and compute composite:

```
critical = count of CRITICAL findings
high     = count of HIGH findings
medium   = count of MEDIUM findings
penalty  = critical×0.5 + high×0.2 + medium×0.05
composite = max(0.0, 1.0 - penalty)
```

**Verdict mapping:**

| Composite | Verdict | Builder action |
|-----------|---------|----------------|
| ≥ 0.85 | PASS | Converged — proceed to build-report |
| 0.6–0.84 | WARN | Ship with noted findings; address in next cycle |
| < 0.6 | FAIL | Block — fix required before shipping |

## Scoring Formula

`Composite Score: 1.0 - (critical×0.5 + high×0.2 + medium×0.05)`

Examples:
- 0 findings → `1.0 - 0 = 1.00` (PASS)
- 1 HIGH → `1.0 - 0.2 = 0.80` (WARN)
- 1 CRITICAL → `1.0 - 0.5 = 0.50` (FAIL)
- 2 MEDIUM → `1.0 - 0.10 = 0.90` (PASS)
- 1 HIGH + 2 MEDIUM → `1.0 - (0.2+0.10) = 0.70` (WARN)

## Output Schema

```markdown
## Security Review

### Composite Score: 0.XX
(formula: 1.0 - (critical×0.5 + high×0.2 + medium×0.05))

### Verdict: PASS | WARN | FAIL

### Findings
| Severity | Dimension | File:Line | Description |
|----------|-----------|-----------|-------------|
| CRITICAL | injection | ... | ... |
| HIGH | auth | ... | ... |
| MEDIUM | crypto | ... | ... |

### Summary
CRITICAL: N  HIGH: N  MEDIUM: N  LOW: N
```

**Builder self-review section format** (in build-report.md):

```
security-review-scored=0.XX
```

## Integration Hooks

### Builder Self-Review Loop

Invoke after implementation changes, before writing `build-report.md`:

```
Builder → Skill("security-review-scored") → parse "Composite Score: X.XX"
        → if score < EVOLVE_BUILDER_REVIEW_THRESHOLD: note findings in build-report
        → include "security-review-scored=X.XX" in ## Self-Review section
```

**Env-var configuration:**

```bash
export EVOLVE_BUILDER_REVIEW_SKILLS="code-review-simplify,security-review-scored"
export EVOLVE_BUILDER_SELF_REVIEW=1
```

### Standalone Invocation

```bash
/security-review-scored [--tier lightweight|standard|full]
```
