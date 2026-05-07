# Scoring Dimensions — 6-Dimension Quality Rubric

> Detailed scoring criteria for the `/evaluator` skill's 6 quality dimensions. Each dimension scored 0.0-1.0 with 5-point granularity.

## Contents
- [Dimension Definitions](#dimension-definitions)
- [Relationship to Existing Benchmarks](#relationship-to-existing-benchmarks)
- [Weight Allocation Rationale](#weight-allocation-rationale)
- [Automated Signal Mapping](#automated-signal-mapping)

---

## Dimension Definitions

### 1. Correctness (weight: 0.25)

Does the code do what it's supposed to, correctly?

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Critical flaws | Logic errors in core paths; crashes on normal input; missing error handling |
| 0.3-0.4 | Significant bugs | Edge cases unhandled; off-by-one errors; race conditions possible |
| 0.5-0.6 | Mostly correct | Happy path works; some edge cases missed; basic error handling |
| 0.7-0.8 | Solid | Edge cases handled; good error propagation; tests exist for key paths |
| 0.9-1.0 | Robust | Comprehensive test coverage; property-based tests; boundary conditions verified |

### 2. Security (weight: 0.20)

Is the code safe from attack and data exposure?

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Exploitable | Injection vectors present; hardcoded secrets; no input validation |
| 0.3-0.4 | Weak | Some validation but bypassable; secrets in config files; error messages leak info |
| 0.5-0.6 | Basic | Input validated at boundaries; no obvious injection; secrets in env vars |
| 0.7-0.8 | Defended | Parameterized queries; content security headers; auth on all endpoints |
| 0.9-1.0 | Hardened | Defense in depth; rate limiting; audit logging; OWASP Top 10 addressed |

### 3. Maintainability (weight: 0.20)

Can a developer understand and change this code efficiently?

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Unmaintainable | Functions > 200 lines; nesting > 6 levels; no comments on complex logic |
| 0.3-0.4 | Difficult | Large files (> 500 lines); unclear naming; duplicated code blocks |
| 0.5-0.6 | Acceptable | Functions < 100 lines; some duplication; adequate naming |
| 0.7-0.8 | Clean | Functions < 50 lines; DRY; clear naming; complexity < 15 per function |
| 0.9-1.0 | Exemplary | Self-documenting; single responsibility; complexity < 10; no dead code |

### 4. Architecture (weight: 0.15)

Is the code well-structured at the module/system level?

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Tangled | Circular dependencies; no module boundaries; God objects |
| 0.3-0.4 | Coupled | Tight coupling between unrelated modules; mixed abstraction levels |
| 0.5-0.6 | Functional | Some modularity; dependencies mostly one-directional; some abstraction |
| 0.7-0.8 | Well-structured | Clear boundaries; dependency injection; interface-based coupling |
| 0.9-1.0 | Excellent | Clean architecture; single responsibility at module level; testable in isolation |

### 5. Completeness (weight: 0.10)

Does the code cover all requirements and operational concerns?

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Incomplete | Missing major features; no error handling; no documentation |
| 0.3-0.4 | Partial | Core functionality present; error paths missing; no operational docs |
| 0.5-0.6 | Functional | Requirements met; basic error handling; README exists |
| 0.7-0.8 | Thorough | All requirements; comprehensive error handling; API docs; logging |
| 0.9-1.0 | Production-ready | Full docs; monitoring; health checks; graceful degradation; runbooks |

### 6. Evolution (weight: 0.10)

How well positioned is the code for future changes?

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Brittle | Hardcoded values everywhere; no configuration; monolithic |
| 0.3-0.4 | Rigid | Some config but lots of assumptions; tightly coupled to current requirements |
| 0.5-0.6 | Adaptable | Config-driven for common changes; some extension points |
| 0.7-0.8 | Flexible | Plugin/extension architecture; feature flags; clear versioning |
| 0.9-1.0 | Future-proof | Strategy pattern for variants; backward-compatible APIs; migration paths |

---

## Relationship to Existing Benchmarks

How the 6 evaluator dimensions map to evolve-loop's 8 benchmark dimensions:

| Evaluator Dimension | Benchmark Dimension(s) |
|--------------------|----------------------|
| correctness | — (no direct equivalent; evals cover this at task level) |
| security | defensiveDesign |
| maintainability | modularity, conventionAdherence |
| architecture | modularity, schemaHygiene |
| completeness | documentationCompleteness, featureCoverage |
| evolution | specificationConsistency (partial) |

The evaluator's 6 dimensions are **universally applicable** (any codebase), while benchmark's 8 are **project-specific** (evolve-loop structure). They complement, not replace, each other.

---

## Weight Allocation Rationale

| Dimension | Weight | Why |
|-----------|--------|-----|
| correctness | 0.25 | Highest — incorrect code is worse than ugly code |
| security | 0.20 | Second — vulnerabilities have outsized impact |
| maintainability | 0.20 | Equal to security — unmaintainable code accumulates debt fast |
| architecture | 0.15 | Important but changes less frequently |
| completeness | 0.10 | Lower — partial solutions that work > complete solutions that don't |
| evolution | 0.10 | Lowest — future-proofing matters but present correctness matters more |

---

## Automated Signal Mapping

Which tools/scripts feed into each dimension:

| Dimension | Automated Signals | Script/Tool |
|-----------|------------------|-------------|
| correctness | Test pass rate, complexity score | `npm test`, `scripts/complexity-check.sh` |
| security | Secret scan, injection patterns | `scripts/code-review-simplify.sh` (check 4) |
| maintainability | File/function length, nesting, duplicates | `scripts/code-review-simplify.sh` (checks 1-3, 5-6) |
| architecture | Import graph depth, file count per module | `grep -r import`, directory structure analysis |
| completeness | TODO/FIXME count, doc file existence, test count | `grep -r TODO`, `test -f README.md` |
| evolution | Hardcoded value count, config file usage, interface vs concrete | `grep -rn 'http://\|https://\|localhost'`, pattern analysis |
