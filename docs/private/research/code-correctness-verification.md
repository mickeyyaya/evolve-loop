# Code Correctness Verification

> Reference doc for verifying correctness of AI-generated code. Covers verification techniques from lightweight (unit tests) to heavyweight (formal verification), with mappings to evolve-loop phases and implementation patterns.

---

## Table of Contents

- [Verification Technique Taxonomy](#verification-technique-taxonomy)
- [Technique Selection Matrix](#technique-selection-matrix)
- [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
- [Implementation Patterns](#implementation-patterns)
- [Prior Art](#prior-art)
- [Anti-Patterns](#anti-patterns)

---

## Verification Technique Taxonomy

| Technique | Description | Strengths | Weaknesses | Tooling |
|-----------|-------------|-----------|------------|---------|
| Unit tests | Assert expected output for specific inputs | Fast, easy to write, deterministic | Only covers enumerated cases; misses edge cases | Jest, pytest, Vitest |
| Property-based testing | Generate random inputs and verify invariants hold | Discovers edge cases automatically; tests properties not examples | Requires identifying meaningful properties; slower than unit tests | Hypothesis (Python), fast-check (JS/TS), QuickCheck (Haskell) |
| Mutation testing | Inject small code changes (mutants) and verify tests catch them | Measures test suite quality; finds weak assertions | Computationally expensive; noisy with equivalent mutants | Stryker (JS/TS), mutmut (Python), pit (Java) |
| Formal verification | Prove correctness mathematically against a specification | Guarantees correctness for all inputs; eliminates entire bug classes | Requires formal spec; steep learning curve; limited language support | Lean, Coq, Dafny, TLA+ |
| Static analysis | Analyze code without execution for bugs, smells, vulnerabilities | Fast; catches common errors at scale; no test data needed | High false-positive rate; cannot reason about runtime behavior | ESLint, Semgrep, SonarQube, Clippy |
| Type checking | Enforce type constraints at compile time | Catches type errors early; serves as lightweight specification | Cannot verify runtime logic; type systems vary in expressiveness | TypeScript, mypy, Pyright |
| Fuzzing | Feed malformed/random data to find crashes and vulnerabilities | Finds security bugs and crashes humans miss; automated | Slow to find deep bugs; requires harness setup; non-deterministic | AFL, libFuzzer, jazzer |

---

## Technique Selection Matrix

Use this matrix to select verification techniques based on the task context.

| Context | Recommended Techniques | Rationale |
|---------|----------------------|-----------|
| Simple utility function | Unit tests + type checking | Low complexity; enumerable inputs |
| Data transformation pipeline | Property-based testing + unit tests | Invariants (idempotency, roundtrip) are more expressive than examples |
| Security-sensitive code | Fuzzing + static analysis + unit tests | Fuzzing finds input-handling bugs; static analysis catches known vulnerability patterns |
| Concurrent/distributed logic | Formal verification (TLA+) + property-based testing | Concurrency bugs are hard to reproduce; formal methods cover all interleavings |
| AI-generated code (general) | Property-based testing + mutation testing + static analysis | AI code often passes happy-path tests but violates invariants; mutation testing validates test quality |
| Algorithm implementation | Property-based testing + formal verification | Verify algorithmic invariants (sorting, search, graph properties) across all inputs |
| Configuration/infrastructure | Static analysis + type checking | Catch misconfigurations before deployment; no runtime to test |
| Eval grader scripts | Unit tests + property-based testing | Graders must be precise; property-based tests verify grader does not over-match or under-match |

---

## Mapping to Evolve-Loop

Map each verification technique to the evolve-loop phase and agent where it applies.

| Phase | Agent | Technique | Application |
|-------|-------|-----------|-------------|
| BUILD | Builder | Unit tests | Write tests before implementation (TDD); run as eval graders |
| BUILD | Builder | Type checking | Enforce type safety in generated code; catch errors before execution |
| BUILD | Builder | Static analysis | Run linters on generated code before committing |
| AUDIT | Auditor | Mutation testing | Inject mutants into Builder output; verify eval graders catch them |
| AUDIT | Auditor | Property-based testing | Generate adversarial inputs to test Builder output against invariants |
| AUDIT | Auditor | Static analysis | Scan for security vulnerabilities, code smells, and anti-patterns |
| SCOUT | Scout | Static analysis | Analyze existing codebase to identify high-risk areas for improvement |
| LEARN | Orchestrator | Mutation testing (meta-cycle) | Mutate eval graders themselves; verify meta-graders catch degradation |
| LEARN | Orchestrator | Property-based testing | Test that scoring functions maintain expected invariants across cycles |

### Auditor Integration Details

The Auditor currently uses eval graders (bash exit-code checks) as the primary verification mechanism. Extend this with:

1. **Property-based graders** — Replace example-based graders with property checks where invariants exist
2. **Mutation score threshold** — Require mutation score >= 80% for generated test suites
3. **Static analysis gate** — Fail audit if static analysis finds CRITICAL or HIGH severity issues

---

## Implementation Patterns

### Add Property-Based Testing to Eval Graders

Replace example-based eval graders with property-based checks using Hypothesis or fast-check.

**Pattern: Roundtrip property**

| Step | Action |
|------|--------|
| 1 | Identify a pair of inverse operations (serialize/deserialize, encode/decode) |
| 2 | Write a property: `for all x, deserialize(serialize(x)) == x` |
| 3 | Use Hypothesis/fast-check to generate random `x` values |
| 4 | Run as eval grader; exit non-zero on any counterexample |

**Pattern: Invariant property**

| Step | Action |
|------|--------|
| 1 | Identify an invariant the output must satisfy (sorted, unique, non-empty) |
| 2 | Write a property: `for all input, invariant(transform(input)) == true` |
| 3 | Generate random inputs with appropriate constraints |
| 4 | Run as eval grader; log the counterexample on failure |

**Pattern: Oracle property (differential testing)**

| Step | Action |
|------|--------|
| 1 | Identify a reference implementation (slower but known-correct) |
| 2 | Write a property: `for all x, fast_impl(x) == reference_impl(x)` |
| 3 | Generate random inputs; compare outputs |
| 4 | Run as eval grader; report divergences |

### Add Mutation Testing to Meta-Cycle

| Step | Action |
|------|--------|
| 1 | Select a completed cycle's generated code and its eval graders |
| 2 | Run Stryker/mutmut to generate mutants of the code |
| 3 | Execute eval graders against each mutant |
| 4 | Calculate mutation score: `killed_mutants / total_mutants` |
| 5 | Flag cycles where mutation score < 80% for grader improvement |

### Add Formal Verification for Critical Invariants

| Step | Action |
|------|--------|
| 1 | Identify a critical invariant (e.g., phase-gate ordering, cycle counter monotonicity) |
| 2 | Write a formal specification in TLA+ or Dafny |
| 3 | Model the system transitions that affect the invariant |
| 4 | Run the model checker / proof assistant to verify |
| 5 | Add as a CI check for changes touching the specified component |

---

## Prior Art

| Project | Contribution | Relevance |
|---------|-------------|-----------|
| QuickCheck (Claessen & Hughes, 2000) | Pioneered property-based testing for Haskell | Foundation for all property-based testing libraries |
| Hypothesis (MacIver, 2013–present) | Property-based testing for Python with stateful testing and database of examples | Direct integration path for Python eval graders |
| fast-check (Dubien, 2017–present) | Property-based testing for JavaScript/TypeScript | Direct integration path for JS/TS eval graders |
| Astrogator | Formal verification of spacecraft trajectory algorithms | Demonstrates formal methods on safety-critical AI-adjacent systems |
| PREFACE (Steenhoek et al.) | Evaluates LLM-generated code using property-based testing to detect subtle bugs | Directly validates property-based testing as superior to unit tests for AI code |
| Stryker Mutator | Mutation testing framework for JS/TS with comprehensive mutant operators | Ready-to-use mutation testing for evolve-loop meta-cycle |
| TLA+ (Lamport) | Formal specification language for concurrent and distributed systems | Use for verifying evolve-loop phase ordering and concurrency invariants |
| AFL / libFuzzer | Coverage-guided fuzzing tools | Use for security-sensitive generated code |

---

## Anti-Patterns

| Anti-Pattern | Problem | Mitigation |
|--------------|---------|------------|
| Over-reliance on unit tests | Unit tests only cover enumerated cases; AI code that passes examples may violate invariants on unseen inputs | Add property-based testing; use unit tests for regression, properties for correctness |
| Specification gaming | AI optimizes for eval grader signals rather than actual correctness (Goodhart's Law) | Use mutation testing to verify grader quality; rotate grader strategies; add human review checkpoints |
| Testing happy path only | Tests cover expected inputs but miss edge cases, boundary conditions, and error paths | Use property-based testing with shrinking to find minimal counterexamples; add fuzzing for input handling |
| Snapshot/golden-file testing as sole verification | Brittle; passes only when output is byte-identical to a previous run; blocks valid improvements | Use property-based assertions on output structure and invariants instead of exact output matching |
| Ignoring mutation score | Test suite appears comprehensive but fails to detect injected bugs | Require mutation score >= 80%; investigate surviving mutants as potential test gaps |
| Formal verification without specification | Attempting to prove correctness without a clear spec leads to verifying the wrong thing | Write the specification first; review it independently; verify the spec matches requirements |
| Testing generated code without testing the generator prompt | Code passes tests but the prompt that generated it is fragile and non-reproducible | Test prompt stability: run the same prompt N times and verify output consistency via properties |
| Skipping static analysis for "simple" changes | Small changes can introduce security vulnerabilities or type errors | Run static analysis on every change regardless of size; automate in CI |
