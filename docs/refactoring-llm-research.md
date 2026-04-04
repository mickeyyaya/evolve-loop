# Automated Code Refactoring Research Brief

> Compiled 2026-04-04 | Sources: arXiv, ACM DL, SonarSource, refactoring.com, refactoring.guru, Emergent Mind, Tanagram

---

## Table of Contents

1. [LLM-Based Automated Refactoring](#1-llm-based-automated-refactoring)
2. [AI Code Smell Detection](#2-ai-code-smell-detection)
3. [LLM Code Quality Improvement](#3-llm-code-quality-improvement)
4. [Fowler Refactoring Catalog](#4-fowler-refactoring-catalog)
5. [Code Smells Catalog](#5-code-smells-catalog)
6. [Automated Architecture Analysis](#6-automated-architecture-analysis)
7. [Dependency Analysis and Architecture Violation Detection](#7-dependency-analysis-and-architecture-violation-detection)
8. [Complexity Metrics](#8-complexity-metrics)
9. [Actionable Improvements for an Automated Refactoring Pipeline](#9-actionable-improvements)

---

## 1. LLM-Based Automated Refactoring

### Key Papers

| Paper | Year | Key Finding |
|-------|------|-------------|
| An Empirical Study on the Potential of LLMs in Automated Software Refactoring | 2024 | Prompt specificity raises identification from 15.6% to 86.7% |
| An Empirical Study on the Code Refactoring Capability of LLMs | 2024 | StarCoder2 reduces code smells 20.1% more than human developers |
| Code Refactoring with LLM: Comprehensive Evaluation With Few-Shot Settings | 2025 | Java achieves 99.99% correctness in 10-shot setting |
| LLM-Driven Code Refactoring: Opportunities and Limitations | 2025 | GPT-4o and DeepSeek-v3 achieve pass@5 above 90% on multi-file refactorings |

### RefactoringMirror Architecture (Critical Finding)

A three-stage hybrid approach that eliminates unsafe LLM refactorings:

1. **Detection**: Use LLM to generate refactored code, then run ReExtractor to identify what refactorings were applied between original and LLM output
2. **Extraction**: Custom algorithms extract detailed parameters for each detected refactoring
3. **Reapplication**: IntelliJ IDEA refactoring engine reapplies the identified refactorings using battle-tested, deterministic implementations

**Results**: 94.3% successful reapplication rate, 0% unsafe edits after reapplication (eliminated all 22 unsafe solutions from the study).

**Insight for pipeline**: Never apply LLM-generated code directly. Use the LLM as an advisor that identifies WHAT to refactor, then use deterministic refactoring engines to execute the transformation safely.

### Prompt Engineering for Refactoring

| Strategy | Effect |
|----------|--------|
| Generic prompt ("refactor this") | 15.6% identification rate |
| Specify refactoring type | 52.2% identification rate |
| Specify subcategory + narrow search space | 86.7% identification rate |
| Chain-of-thought prompting | +1.7% test pass rate, increased smell reduction |
| Few-shot (10-shot) | Up to 99.99% correctness (Java) |
| Multi-proposal generation (pass@5) | +28.8% functional correctness |
| Iterative re-prompting on compile/test errors | +40-65 percentage points correctness |

### Multi-Agent Refactoring Architecture

The state of the art uses specialized agent roles:

| Agent Role | Responsibility |
|------------|----------------|
| Planning Agent | Static analysis, compute software metrics, identify refactoring opportunities |
| Generator Agent | Synthesize transformations using few-shot examples |
| Validation Agent | Execute tests, verify behavioral preservation |
| Self-Reflection Agent | Iteratively repair failed transformations |

Strict iteration budgets (e.g., 20 rounds per class) prevent infinite loops.

### Quantitative Benchmarks

| Metric | Best Result | System |
|--------|-------------|--------|
| Smell reduction (test smells) | 89% | DSL-based rule sets |
| Smell reduction (general) | 52.5% median | Multi-agent systems |
| Test pass rate | 90% | RefAgent |
| Cyclomatic complexity reduction | -17.35% (Python), -47.06% (Haskell) | LLM-based |
| Developer agreement | 81.3% | Extract Method operations |
| Hallucination reduction | 76.3% to 23.7% | Embedding-based filtering |

### Open Challenges

- Cross-module and architectural refactorings remain limited
- Context window restrictions affect large codebases
- Non-determinism and API mismatches persist
- Domain-specific adaptation needs improvement
- Developers still outperform LLMs in complex, context-sensitive refactorings (e.g., attribute encapsulation)

---

## 2. AI Code Smell Detection

### Key Papers

| Paper | Year | Key Finding |
|-------|------|-------------|
| Machine Learning-Based Methods for Code Smell Detection: A Survey | 2024 | Comprehensive survey of 42 papers covering ML algorithms from 2005-2024 |
| iSMELL: Assembling LLMs with Expert Toolsets | 2024 (ASE) | LLMs + expert toolsets overcome token limits and repo-level knowledge gaps |
| Enhancing Software Quality with AI: Transformer-Based Approach | 2025 | RABERT achieves 90% accuracy, 91% precision |

### Detection Techniques Ranked by Maturity

| Technique | Strengths | Limitations |
|-----------|-----------|-------------|
| Rule-based (PMD, Checkstyle) | Deterministic, fast, zero false negatives for configured rules | Cannot detect context-dependent smells |
| Supervised ML (SVM, Random Forest) | Good on metric-based smells (Long Method, God Class) | Requires labeled training data |
| Transformer-based (RABERT) | Captures code dependencies via relational embeddings | Computationally expensive |
| LLM-based (iSMELL) | Handles natural-language smell descriptions, repo-level context | Token limits, hallucination risk |

### iSMELL Architecture

Addresses three LLM limitations for smell detection:
1. **Token restrictions** - assembles expert toolsets that pre-process code before LLM analysis
2. **Repository-level knowledge gaps** - tools provide cross-file dependency information
3. **Dynamic analysis needs** - integrates runtime analysis tools the LLM cannot perform alone

### Most Detectable Smells (by ML)

| Smell | Best Detection Accuracy | Preferred Technique |
|-------|------------------------|---------------------|
| God Class / Large Class | >95% | Metric thresholds + ML |
| Long Method | >90% | Lines + cyclomatic complexity |
| Feature Envy | ~85% | Coupling metrics + ML |
| Data Class | ~90% | Method-to-field ratio |
| Duplicate Code | >95% | AST/token similarity |

### Insight for Pipeline

Combine rule-based detection (fast, deterministic) with LLM-based detection (context-aware, flexible). Use rules for well-defined metric-based smells, LLMs for context-dependent smells like Feature Envy and Inappropriate Intimacy.

---

## 3. LLM Code Quality Improvement

### Systematic Literature Reviews

Two major SLRs published in 2025:
- **"Using LLMs to enhance code quality"** (ScienceDirect) - analyzed 49 studies through September 2024; refactoring is the most common LLM code quality task
- **"Software refactoring research with LLMs"** (ScienceDirect) - examined 50 primary studies on LLM-driven refactoring

### LLM Performance by Refactoring Type

| Refactoring Type | LLM Performance | Notes |
|-----------------|-----------------|-------|
| Extract Method | High (81.3% developer agreement) | Best when search space is narrowed |
| Inline Method | 87.5% acceptable solutions | Strong LLM performance |
| Rename (Method, Variable, Parameter) | 43.8% acceptable | LLMs struggle with naming |
| Extract Class | Near zero identification | Cannot detect cross-class responsibilities |
| Attribute Encapsulation | Below human level | Context-sensitive, requires domain understanding |

### Safety Statistics

| Model | Unsafe Solutions | Error Types |
|-------|-----------------|-------------|
| GPT | 7.4% (13/176) | 18 semantic bugs, 4 syntax errors |
| Gemini | 6.6% (9/137) | Same categories |

**Critical insight**: 7% unsafe rate means every LLM refactoring MUST be validated. The RefactoringMirror approach (detect-and-reapply through tested engines) eliminates this risk entirely.

### Model Comparison

| Model | Strength |
|-------|----------|
| LLaMA 3 | Highest overall code smell reduction (15.1% median) |
| DeepSeek-v3 | Greatest improvements in cohesion, coupling, complexity |
| GPT-4o | Best pass@5 unit test success (>90%) |
| StarCoder2 | 20.1% more smell reduction than human developers |

---

## 4. Fowler Refactoring Catalog

Source: [refactoring.com/catalog](https://refactoring.com/catalog/)

The second edition contains **72 refactorings**. Complete list organized by category:

### Encapsulation
| Refactoring | Purpose |
|------------|---------|
| Encapsulate Collection | Protect internal collection from external modification |
| Encapsulate Record | Replace raw data records with objects |
| Encapsulate Variable | Control access to widely used variables |

### Composing Methods
| Refactoring | Purpose |
|------------|---------|
| Extract Function | Turn code fragment into a named function |
| Inline Function | Replace function call with function body |
| Extract Variable | Name a complex expression |
| Inline Variable | Replace variable with expression |
| Replace Temp with Query | Replace temporary with a method call |
| Change Function Declaration | Rename or change parameters |

### Moving Features
| Refactoring | Purpose |
|------------|---------|
| Move Function | Move function to the class it most references |
| Move Field | Move field to the class that uses it most |
| Move Statements into Function | Merge repeated setup into the called function |
| Move Statements to Callers | Move varying behavior out of a function |
| Slide Statements | Group related code together |
| Split Loop | Separate loops that do different things |
| Split Phase | Separate code into sequential phases |

### Organizing Data
| Refactoring | Purpose |
|------------|---------|
| Change Reference to Value | Replace reference object with value object |
| Change Value to Reference | Replace copied value with shared reference |
| Replace Primitive with Object | Wrap primitive in a meaningful class |
| Replace Derived Variable with Query | Replace stored calculation with computed value |
| Replace Magic Literal | Replace magic numbers/strings with named constants |

### Simplifying Conditional Logic
| Refactoring | Purpose |
|------------|---------|
| Consolidate Conditional Expression | Combine conditions with same result |
| Decompose Conditional | Extract condition and branches into functions |
| Introduce Assertion | Make assumptions explicit |
| Introduce Special Case | Replace null/special-value checks with special case object |
| Replace Conditional with Polymorphism | Replace type-checking with polymorphic dispatch |
| Replace Control Flag with Break | Replace loop control flags with break/return |
| Replace Nested Conditional with Guard Clauses | Flatten nested ifs with early returns |

### Refactoring APIs
| Refactoring | Purpose |
|------------|---------|
| Introduce Parameter Object | Group related parameters into an object |
| Preserve Whole Object | Pass whole object instead of extracted values |
| Remove Flag Argument | Replace boolean flag with separate methods |
| Remove Setting Method | Make field immutable by removing setter |
| Replace Command with Function | Simplify command object to plain function |
| Replace Constructor with Factory Function | Use factory for flexible object creation |
| Replace Error Code with Exception | Use exceptions instead of error codes |
| Replace Function with Command | Wrap function in command object |
| Replace Parameter with Query | Remove parameter that can be computed |
| Replace Query with Parameter | Add parameter to remove dependency |
| Return Modified Value | Return value instead of modifying argument |
| Separate Query from Modifier | Split function that both reads and writes |

### Dealing with Inheritance
| Refactoring | Purpose |
|------------|---------|
| Collapse Hierarchy | Merge superclass and subclass |
| Extract Superclass | Pull shared behavior into new superclass |
| Pull Up Constructor Body | Move shared constructor code to superclass |
| Pull Up Field | Move shared field to superclass |
| Pull Up Method | Move shared method to superclass |
| Push Down Field | Move field to relevant subclass |
| Push Down Method | Move method to relevant subclass |
| Remove Subclass | Replace subclass with field on superclass |
| Replace Subclass with Delegate | Replace inheritance with composition |
| Replace Superclass with Delegate | Replace inheritance with delegation |
| Replace Type Code with Subclasses | Replace type field with subclass hierarchy |

### Other
| Refactoring | Purpose |
|------------|---------|
| Combine Functions into Class | Group functions operating on same data |
| Combine Functions into Transform | Gather derived data computations together |
| Extract Class | Split class with multiple responsibilities |
| Hide Delegate | Remove client knowledge of delegation chain |
| Inline Class | Merge class that does too little |
| Remove Dead Code | Delete unreachable code |
| Remove Middle Man | Remove unnecessary delegation |
| Replace Inline Code with Function Call | Replace duplicated expression with library call |
| Replace Loop with Pipeline | Replace loop with functional pipeline |
| Rename Field | Give field a clearer name |
| Rename Variable | Give variable a clearer name |
| Substitute Algorithm | Replace algorithm with clearer alternative |
| Split Variable | Split variable used for multiple purposes |

---

## 5. Code Smells Catalog

Source: [refactoring.guru/refactoring/smells](https://refactoring.guru/refactoring/smells)

### Complete Smell Taxonomy (23 smells, 5 categories)

#### Bloaters
| Smell | Description | Key Metric |
|-------|-------------|------------|
| Long Method | Method too long to understand easily | Lines of code, cyclomatic complexity |
| Large Class | Class with too many responsibilities | Number of methods/fields, LCOM |
| Primitive Obsession | Using primitives instead of small objects | Primitive parameter count |
| Long Parameter List | Methods with excessive parameters | Parameter count (>3-4) |
| Data Clumps | Groups of variables that travel together | Co-occurrence frequency |

#### Object-Orientation Abusers
| Smell | Description | Key Metric |
|-------|-------------|------------|
| Switch Statements | Excessive conditional logic replacing polymorphism | Switch/if-else chain count |
| Temporary Field | Fields only set/used in certain scenarios | Field usage coverage |
| Refused Bequest | Subclass ignores inherited interface | Override ratio |
| Alternative Classes with Different Interfaces | Similar classes with inconsistent APIs | Method signature similarity |

#### Change Preventers
| Smell | Description | Key Metric |
|-------|-------------|------------|
| Divergent Change | One class modified for unrelated reasons | Change frequency by reason |
| Shotgun Surgery | Single logical change touches many classes | Change set size |
| Parallel Inheritance Hierarchies | Adding subclass in one hierarchy requires another | Parallel subclass count |

#### Dispensables
| Smell | Description | Key Metric |
|-------|-------------|------------|
| Comments (excessive) | Comments compensating for unclear code | Comment-to-code ratio |
| Duplicate Code | Repeated code blocks | Clone detection (AST/token similarity) |
| Lazy Class | Class with too little behavior | Method count, usage frequency |
| Data Class | Class with only getters/setters, no behavior | Behavior-to-data ratio |
| Dead Code | Unreachable or unused code | Static reachability analysis |
| Speculative Generality | Unused abstractions created "just in case" | Usage count of abstract elements |

#### Couplers
| Smell | Description | Key Metric |
|-------|-------------|------------|
| Feature Envy | Method uses another class's data excessively | External reference ratio |
| Inappropriate Intimacy | Classes accessing each other's internals | Bidirectional coupling count |
| Message Chains | Long chains of method calls (a.b().c().d()) | Chain length |
| Middle Man | Class that only delegates | Delegation ratio |
| Incomplete Library Class | Library missing needed functionality | Extension frequency |

---

## 6. Automated Architecture Analysis

### AI Agent Architecture Patterns for Code Review

Source: [Tanagram](https://tanagram.ai/blog/ai-agent-architecture-patterns-for-code-review-automation-the-complete-guide)

#### Pattern 1: Deterministic Analysis with Selective LLM Integration

Build structural understanding through graph-based analysis (lexical, referential, dependency graphs), then apply LLMs only for complex reasoning.

| Component | Approach |
|-----------|----------|
| Structure analysis | Multiple codebase graphs (not LLM) |
| Rule enforcement | Deterministic queries (reproducible) |
| Complex reasoning | LLM integration (only when needed) |
| Policy generation | LLM translates natural language to rules |

**Target**: 85%+ accuracy on enforced policies; if accuracy drops below 80%, fix or remove the policy.

#### Pattern 2: Multi-Agent Collaboration

| Agent | Role |
|-------|------|
| Policy Enforcement | Deterministic rule compliance |
| Context Analysis | Structural codebase understanding |
| Security Scanning | Vulnerability identification |
| Quality Metrics | Code health tracking |

#### Pattern 3: Real-Time Feedback Integration

| Channel | Use Case |
|---------|----------|
| IDE extensions | In-editor suggestions |
| PR comments | Contextual review feedback |
| CI/CD gates | Blocking problematic merges |
| Chat notifications | Critical issue alerts |

### Performance Benchmarks (2025)

| Metric | AI-Powered Tools | Traditional Static Analysis |
|--------|------------------|-----------------------------|
| Bug detection rate | 42-48% | <20% |
| Time savings | 40% | N/A |
| PR merge rate improvement | 39% higher | N/A |
| Production bug reduction | 62% fewer | N/A |

---

## 7. Dependency Analysis and Architecture Violation Detection

### Tools Comparison

| Tool | Languages | Focus | Output |
|------|-----------|-------|--------|
| Depends | Java, C/C++, Ruby, Python | 13 dependency types (Call, Cast, Create, Extend, Implement, Import, etc.) | JSON, XML, DOT, PlantUML |
| ArchUnit | Java | Architecture compliance testing as unit tests | JUnit test results |
| NDepend | .NET | Code quality metrics + dependency graphs | Interactive explorer |
| OWASP Dependency-Check | Multi-language | Security vulnerability detection in dependencies | Reports |
| jQAssistant | Java | Architecture constraint queries | Neo4j graph DB |

### Architecture Fitness Functions

Fitness functions are automated tests that verify architectural rules in CI/CD:

| Rule Type | Example | Tool |
|-----------|---------|------|
| Layer dependency | "Controllers must not import Repository classes" | ArchUnit |
| Package cycles | "No circular dependencies between packages" | ArchUnit |
| Naming conventions | "Service classes must end with 'Service'" | ArchUnit |
| Module boundaries | "Module A must not depend on Module B internals" | ArchUnit, jQAssistant |

**Insight for pipeline**: Architecture fitness functions should run as part of the refactoring validation step. After any refactoring, verify that no architectural constraints were violated.

### Depends: 13 Dependency Types

| Type | Description |
|------|-------------|
| Call | Function/method invocations |
| Cast | Type conversions |
| Contain | Variable/field definitions |
| Create | Object instantiation |
| Extend | Inheritance |
| Implement | Interface implementation |
| Import/Include | File-level dependencies |
| Mixin | Mix-in relations |
| Parameter | Method parameter types |
| Return | Return types |
| Throw | Exception handling |
| Use | Variable access/modification |
| ImplLink | Call-to-implementation connections |

---

## 8. Complexity Metrics

### Cyclomatic Complexity (McCabe, 1976)

Counts linearly independent paths through source code.

| Score | Risk Level | Action |
|-------|------------|--------|
| 1-10 | Low | Acceptable |
| 11-20 | Moderate | Consider refactoring |
| 21-50 | High | Refactor required |
| 50+ | Very High | Untestable, must split |

**NIST recommendation**: Limit of 10 per function.

### Cognitive Complexity (SonarSource)

Measures how difficult code is for humans to understand.

#### Scoring Rules

| Construct | Increment | Notes |
|-----------|-----------|-------|
| if, else if, else | +1 | Each branch point |
| switch | +1 | Per switch statement |
| for, while, do-while | +1 | Each loop |
| catch | +1 | Each catch block |
| break/continue to label | +1 | Labeled jumps |
| Sequence of logical operators (&&, \|\|) | +1 | Per mixed sequence |
| **Nesting penalty** | +1 per level | Compounds with each nested control structure |
| Recursion | +1 | Self-referencing calls |

#### What Does NOT Increment

| Construct | Reason |
|-----------|--------|
| Ternary operator (simple) | Improves readability |
| Null-coalescing operator (??) | Clarity construct |
| Early return / guard clause | Reduces nesting |
| try (without catch) | Not a branch |

**Default threshold**: 15 per function (SonarQube).

### Comparison

| Dimension | Cyclomatic | Cognitive |
|-----------|-----------|-----------|
| Measures | Testability (paths) | Readability (mental effort) |
| Nesting | Not penalized | Heavily penalized |
| switch/case | Each case +1 | Entire switch +1 |
| Short-circuit logic | Each operator +1 | Sequence of same operator +1 |
| Best for | Determining test count | Identifying hard-to-read code |

### Additional Metrics for Pipeline

| Metric | What It Measures | Threshold |
|--------|-----------------|-----------|
| Halstead Volume | Code vocabulary richness | <1000 |
| Maintainability Index | Composite (volume + complexity + LOC) | >20 (easy), <10 (high risk) |
| LCOM (Lack of Cohesion) | Class cohesion | <0.5 (cohesive) |
| Coupling Between Objects | Inter-class dependencies | <5 per class |
| Depth of Inheritance | Inheritance chain length | <6 |
| Lines of Code per function | Function size | <50 |

---

## 9. Actionable Improvements

### For an Automated Refactoring Pipeline

#### A. Detection Phase

| Improvement | Source | Priority |
|-------------|--------|----------|
| Use multi-metric smell scoring (cyclomatic + cognitive + LCOM + coupling) instead of single thresholds | Complexity metrics research | HIGH |
| Implement the full 23-smell taxonomy from refactoring.guru as detection targets | refactoring.guru catalog | HIGH |
| Combine rule-based detection (fast) with LLM-based detection (context-aware) | iSMELL paper | HIGH |
| Add AST-based duplicate code detection alongside text-based | ML smell detection survey | MEDIUM |
| Compute Maintainability Index as composite health score | SonarSource | MEDIUM |
| Track "change preventers" (Divergent Change, Shotgun Surgery) via git history analysis | Smell taxonomy | MEDIUM |

#### B. Planning Phase

| Improvement | Source | Priority |
|-------------|--------|----------|
| Always specify refactoring subcategory in LLM prompts (86.7% vs 15.6% identification) | arXiv 2411.04444 | CRITICAL |
| Narrow search space to specific code regions in prompts | arXiv 2411.04444 | CRITICAL |
| Use few-shot examples (10-shot achieves 99.99% correctness in Java) | arXiv 2511.21788 | HIGH |
| Map each detected smell to specific Fowler catalog refactoring(s) | Fowler catalog | HIGH |
| Use chain-of-thought prompting for complex refactorings | Multiple papers | HIGH |
| Generate multiple proposals (pass@5) and select best | Few-shot evaluation | MEDIUM |

#### C. Execution Phase

| Improvement | Source | Priority |
|-------------|--------|----------|
| Implement RefactoringMirror: detect-and-reapply through tested engines | arXiv 2411.04444 | CRITICAL |
| Never apply LLM-generated code directly (7% unsafe rate) | Multiple papers | CRITICAL |
| Add iterative re-prompting on compile/test failures (+40-65pp correctness) | Emergent Mind | HIGH |
| Enforce strict iteration budgets (max 20 rounds per refactoring) | Multi-agent research | HIGH |
| Use embedding-based filtering to reduce hallucination (76.3% to 23.7%) | Emergent Mind | MEDIUM |

#### D. Validation Phase

| Improvement | Source | Priority |
|-------------|--------|----------|
| Run architecture fitness functions after every refactoring | ArchUnit research | HIGH |
| Verify no architectural constraint violations (layer deps, package cycles) | Fitness functions | HIGH |
| Compute before/after metrics: cyclomatic, cognitive, coupling, cohesion | Complexity metrics | HIGH |
| Validate behavioral preservation via test execution | RefactoringMirror | CRITICAL |
| Check dependency graph for introduced cycles or violations | Depends tool | MEDIUM |

#### E. Architecture-Level Improvements

| Improvement | Source | Priority |
|-------------|--------|----------|
| Build codebase graphs (lexical, referential, dependency) for structural understanding | Tanagram | HIGH |
| Use deterministic graph queries for consistent, reproducible analysis | Tanagram | HIGH |
| Reserve LLM for complex reasoning only, not structural analysis | Architecture patterns | HIGH |
| Implement hierarchical policy inheritance (org, team, repo levels) | Tanagram | MEDIUM |
| Target sub-2-second response for routine checks | Architecture patterns | MEDIUM |
| Maintain 85%+ accuracy on all enforced policies | Tanagram | MEDIUM |

---

## Sources

### LLM Refactoring
- [An Empirical Study on the Potential of LLMs in Automated Software Refactoring](https://arxiv.org/abs/2411.04444) - arXiv 2024
- [An Empirical Study on the Code Refactoring Capability of LLMs](https://arxiv.org/abs/2411.02320) - arXiv 2024
- [Code Refactoring with LLM: Comprehensive Evaluation With Few-Shot Settings](https://arxiv.org/abs/2511.21788) - arXiv 2025
- [LLM-Driven Code Refactoring: Opportunities and Limitations](https://seal-queensu.github.io/publications/pdf/IDE-Jonathan-2025.pdf) - SEAL Lab 2025
- [LLM-Based Code Refactoring Topic Summary](https://www.emergentmind.com/topics/llm-based-refactoring) - Emergent Mind
- [Software refactoring research with LLMs: A systematic literature review](https://www.sciencedirect.com/science/article/abs/pii/S0164121225004315) - ScienceDirect 2025
- [Using LLMs to enhance code quality: A systematic literature review](https://www.sciencedirect.com/science/article/abs/pii/S095058492500299X) - ScienceDirect 2025

### Code Smell Detection
- [Machine Learning-Based Methods for Code Smell Detection: A Survey](https://www.mdpi.com/2076-3417/14/14/6149) - Applied Sciences 2024
- [iSMELL: Assembling LLMs with Expert Toolsets for Code Smell Detection and Refactoring](https://dl.acm.org/doi/10.1145/3691620.3695508) - ASE 2024
- [Enhancing Software Quality with AI: Transformer-Based Approach](https://www.mdpi.com/2076-3417/15/8/4559) - Applied Sciences 2025

### Catalogs
- [Fowler Refactoring Catalog (72 refactorings)](https://refactoring.com/catalog/)
- [Refactoring.guru Code Smells (23 smells)](https://refactoring.guru/refactoring/smells)

### Architecture Analysis
- [AI Agent Architecture Patterns for Code Review Automation](https://tanagram.ai/blog/ai-agent-architecture-patterns-for-code-review-automation-the-complete-guide) - Tanagram 2025
- [Fitness Functions: Automating Architecture Decisions](https://lukasniessen.medium.com/fitness-functions-automating-your-architecture-decisions-08b2fe4e5f34) - 2026

### Dependency Analysis
- [Depends: Multi-language Code Dependency Analysis](https://github.com/multilang-depends/depends)
- [ArchUnit: Java Architecture Test Library](https://github.com/TNG/ArchUnit)

### Complexity Metrics
- [Cognitive Complexity Explained](https://axify.io/blog/cognitive-complexity) - Axify
- [5 Clean Code Tips for Reducing Cognitive Complexity](https://www.sonarsource.com/blog/5-clean-code-tips-for-reducing-cognitive-complexity) - SonarSource
- [Cyclomatic Complexity](https://en.wikipedia.org/wiki/Cyclomatic_complexity) - Wikipedia
