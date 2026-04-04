# Refactoring Tools & Methodologies Research

> Research conducted 2026-04-04. Sources: refactoring.guru, sourcemaking.com, SonarQube, ESLint, jscpd, knip, dependency-cruiser, madge.

## Table of Contents

- [Master Tool Comparison](#master-tool-comparison)
- [Refactoring Technique Catalog](#refactoring-technique-catalog)
- [Code Smell Catalog](#code-smell-catalog)
- [Tool Deep Dives](#tool-deep-dives)
- [Cognitive Complexity Scoring](#cognitive-complexity-scoring)
- [Skill Applicability Matrix](#skill-applicability-matrix)

---

## Master Tool Comparison

| Tool/Source | Category | Key Techniques | Applicable to Skill |
|---|---|---|---|
| **refactoring.guru** | Technique Catalog | 66 refactoring techniques across 6 categories (Composing Methods, Moving Features, Organizing Data, Simplifying Conditionals, Simplifying Method Calls, Dealing with Generalization) | Yes -- complete decision matrix for smell-to-fix mapping |
| **sourcemaking.com** | Technique Catalog | 66 refactoring techniques (identical to refactoring.guru, based on Fowler's catalog) + 22 code smells | Yes -- same catalog, useful for cross-referencing descriptions |
| **SonarQube** | Static Analysis | 6,000+ rules across 20+ languages; detects code smells, bugs, vulnerabilities, security hotspots; cognitive complexity scoring; duplicate detection | Yes -- rule categories map to detectable smells; cognitive complexity algorithm is encodable |
| **ESLint** | Static Analysis (JS/TS) | Rules in 4 categories: Possible Problems, Suggestions, Layout, Deprecated. Key quality rules: `complexity`, `max-depth`, `max-lines-per-function`, `max-params`, `no-eval`, `no-param-reassign`, `eqeqeq` | Yes -- complexity/depth/length thresholds directly usable |
| **jscpd** | Duplicate Detection | Rabin-Karp algorithm for copy-paste detection; 150+ language support; 3 detection modes (Strict, Mild, Weak); configurable min-tokens threshold | Yes -- duplicate detection with configurable sensitivity |
| **knip** | Dead Code Detection | Finds unused files, unused npm dependencies, unused exports, unused types, unused enum members, unused class members; 100+ framework plugins | Yes -- comprehensive dead code finder, plugin-aware |
| **dependency-cruiser** | Architecture Validation | Validates dependency rules (circular deps, orphans, missing deps, dev-dep leaks); enforces Clean Architecture boundaries; regex-based path rules | Yes -- architecture boundary enforcement, circular dep detection |
| **madge** | Circular Dependency Detection | DFS-based circular dependency detection; dependency graph visualization; JSON/DOT/text output; JS/TS support | Partial -- dependency-cruiser covers this plus more |
| **Cognitive Complexity** | Complexity Metric | +1 per control flow break (if/else/for/while/switch/catch); nesting multiplier compounds; excludes clarifying constructs; threshold recommended <15 | Yes -- scoring algorithm is fully encodable |

---

## Refactoring Technique Catalog

Complete list from refactoring.guru and sourcemaking.com (66 techniques):

### Composing Methods

| Technique | What It Fixes | Detection Signal |
|---|---|---|
| Extract Method | Long Method, Duplicate Code | Function >20 lines; repeated code blocks |
| Inline Method | Over-delegation | Method body is simpler than its name |
| Extract Variable | Complex expression | Expression used in multiple places or hard to read |
| Inline Temp | Unnecessary temp variable | Temp assigned once, used once |
| Replace Temp with Query | Temp holding computed value | Temp = expression that could be a method |
| Split Temporary Variable | Variable reused for multiple purposes | Same var assigned multiple times for different meanings |
| Remove Assignments to Parameters | Parameter mutation | Parameter reassigned inside method body |
| Replace Method with Method Object | Long method with many locals | Method too complex to extract from; many local vars |
| Substitute Algorithm | Overly complex algorithm | Simpler algorithm exists for same result |

### Moving Features Between Objects

| Technique | What It Fixes | Detection Signal |
|---|---|---|
| Move Method | Feature Envy | Method uses more data from another class |
| Move Field | Feature Envy | Field used more by another class |
| Extract Class | Large Class, Divergent Change | Class has multiple responsibilities |
| Inline Class | Lazy Class | Class does almost nothing |
| Hide Delegate | Message Chains | Client calls a.b().c().d() |
| Remove Middle Man | Too much delegation | Class just forwards calls |
| Introduce Foreign Method | Incomplete Library Class | Need utility method on library class |
| Introduce Local Extension | Incomplete Library Class | Need multiple methods on library class |

### Organizing Data

| Technique | What It Fixes | Detection Signal |
|---|---|---|
| Self Encapsulate Field | Direct field access coupling | Field accessed directly instead of via getter |
| Replace Data Value with Object | Primitive Obsession | Primitive holding structured data |
| Change Value to Reference | Duplicate objects for same entity | Multiple objects represent same real-world thing |
| Change Reference to Value | Unnecessary reference complexity | Simple value treated as reference |
| Replace Array with Object | Array used as struct | Array with elements meaning different things |
| Duplicate Observed Data | UI/domain coupling | Domain data mixed with UI code |
| Replace Magic Number with Symbolic Constant | Magic numbers | Literal numbers in logic |
| Encapsulate Field | Public field | Direct public field access |
| Encapsulate Collection | Exposed collection | Getter returns raw collection |
| Replace Type Code with Class | Primitive type codes | Numeric/string constants for types |
| Replace Type Code with Subclasses | Type code with behavior | Type code affects method behavior |
| Replace Type Code with State/Strategy | Type code changes at runtime | Type code changes during object lifetime |
| Replace Subclass with Fields | Subclasses differ only in data | Subclasses that only return different constants |

### Simplifying Conditional Expressions

| Technique | What It Fixes | Detection Signal |
|---|---|---|
| Decompose Conditional | Complex conditional | Long if/else with complex conditions |
| Consolidate Conditional Expression | Multiple conditions, same result | Several conditionals return same value |
| Consolidate Duplicate Conditional Fragments | Duplicate code in branches | Same code in all branches of conditional |
| Remove Control Flag | Control flag variable | Boolean flag controlling loop/flow |
| Replace Nested Conditional with Guard Clauses | Deep nesting | Deeply nested if/else chains |
| Replace Conditional with Polymorphism | Switch Statements smell | Switch/if-else on type code |
| Introduce Null Object | Null checks everywhere | Repeated null/undefined checks |
| Introduce Assertion | Assumed precondition | Code assumes state but doesn't verify |

### Simplifying Method Calls

| Technique | What It Fixes | Detection Signal |
|---|---|---|
| Rename Method | Unclear naming | Method name doesn't reveal intent |
| Add Parameter | Missing context | Method needs more data |
| Remove Parameter | Unused parameter | Parameter no longer used |
| Separate Query from Modifier | Side-effecting query | Method both returns value and changes state |
| Parameterize Method | Similar methods | Methods differ only by values used |
| Replace Parameter with Explicit Methods | Mode parameter | Method behavior changes based on parameter value |
| Preserve Whole Object | Long Parameter List | Passing multiple fields from same object |
| Replace Parameter with Method Call | Unnecessary parameter | Callee can obtain value itself |
| Introduce Parameter Object | Data Clumps | Same group of params in multiple methods |
| Remove Setting Method | Field shouldn't change after creation | Setter exists but field should be immutable |
| Hide Method | Overly public API | Method not used by other classes |
| Replace Constructor with Factory Method | Complex construction | Constructor logic is complex or needs polymorphism |
| Replace Error Code with Exception | Error codes | Methods return error codes instead of throwing |
| Replace Exception with Test | Exception for control flow | Exception used where condition check suffices |

### Dealing with Generalization

| Technique | What It Fixes | Detection Signal |
|---|---|---|
| Pull Up Field | Duplicate field in subclasses | Same field in multiple subclasses |
| Pull Up Method | Duplicate method in subclasses | Same method in multiple subclasses |
| Pull Up Constructor Body | Duplicate constructor logic | Similar constructors in subclasses |
| Push Down Field | Field used only in one subclass | Superclass field relevant to one subclass |
| Push Down Method | Method used only in one subclass | Superclass method relevant to one subclass |
| Extract Subclass | Class has features used only sometimes | Some features used only in some cases |
| Extract Superclass | Classes with common features | Two classes with similar code |
| Extract Interface | Common interface needed | Classes share a subset of methods |
| Collapse Hierarchy | Subclass adds nothing | Subclass nearly identical to parent |
| Form Template Method | Similar methods with different steps | Methods with same structure but different details |
| Replace Inheritance with Delegation | Refused Bequest | Subclass uses little of parent's interface |
| Replace Delegation with Inheritance | Too much delegation | Class delegates most calls to another class |

---

## Code Smell Catalog

22 smells from refactoring.guru, organized by category:

### Bloaters (things that grow too large)

| Smell | Detection Heuristic | Threshold |
|---|---|---|
| Long Method | Line count, cognitive complexity | >20 lines or complexity >15 |
| Large Class | Line count, field count, method count | >300 lines, >10 fields, >20 methods |
| Primitive Obsession | Primitives used where value objects belong | String/number for phone, money, date ranges |
| Long Parameter List | Parameter count | >3 parameters |
| Data Clumps | Same group of fields/params repeated | Same 3+ fields in multiple places |

### Object-Orientation Abusers

| Smell | Detection Heuristic | Threshold |
|---|---|---|
| Switch Statements | switch/if-else on type codes | >3 cases checking same discriminator |
| Temporary Field | Fields only set in some paths | Field null/undefined in most usage |
| Refused Bequest | Subclass ignores parent methods | Overrides to throw or no-op |
| Alternative Classes with Different Interfaces | Duplicate classes with different APIs | Two classes doing same thing differently |

### Change Preventers

| Smell | Detection Heuristic | Threshold |
|---|---|---|
| Divergent Change | One class changes for many reasons | Multiple unrelated change reasons |
| Shotgun Surgery | One change touches many classes | Single logical change spans >5 files |
| Parallel Inheritance Hierarchies | Adding subclass requires parallel subclass | Two hierarchies grow in lockstep |

### Dispensables (things that can be removed)

| Smell | Detection Heuristic | Threshold |
|---|---|---|
| Duplicate Code | Token-level similarity (Rabin-Karp) | >25 tokens duplicated |
| Dead Code | Unreachable or unused code | No references found |
| Lazy Class | Class does too little | <3 methods, <50 lines |
| Speculative Generality | Unused abstractions | Interface with one implementor, unused params |
| Data Class | Class with only getters/setters | No behavior methods |
| Comments (excessive) | Comments substituting for clarity | Comment explaining what, not why |

### Couplers (things too tightly linked)

| Smell | Detection Heuristic | Threshold |
|---|---|---|
| Feature Envy | Method uses another class more | >50% of references to external class |
| Inappropriate Intimacy | Classes access each other's internals | Bidirectional private access |
| Message Chains | Long delegation chains | a.b().c().d() with >3 links |
| Middle Man | Class only delegates | >50% methods are pure delegation |
| Incomplete Library Class | Missing utility on library type | Workaround code wrapping library |

---

## Tool Deep Dives

### jscpd -- Duplicate Code Detection

- **Algorithm**: Rabin-Karp string matching with probabilistic hashing
- **Detection modes**:
  - **Strict**: All tokens including whitespace, comments
  - **Mild**: Skip whitespace and empty lines
  - **Weak**: Skip whitespace, empty lines, and comments
- **Configuration**: `min-tokens` (default 50), `min-lines` (default 5), threshold percentage
- **Output**: JSON, XML, HTML, console; integrates with CI pipelines
- **Languages**: 150+ via tokenizer plugins
- **Key insight for skill**: Can be run as pre-commit check; configurable sensitivity allows project-specific tuning

### knip -- Dead Code Detection

- **Detects**:
  - Unused files (not imported anywhere)
  - Unused npm dependencies (in package.json but not imported)
  - Unused exports (exported but never imported)
  - Unused types (TypeScript types/interfaces with no references)
  - Unused enum members
  - Unused class members
  - Duplicate exports
- **Plugin system**: 100+ plugins for framework-aware analysis (Next.js, Jest, Storybook, ESLint, GitHub Actions, etc.)
- **Monorepo support**: Handles workspaces and cross-package references
- **Key insight for skill**: Framework plugin awareness prevents false positives (e.g., Next.js page exports)

### dependency-cruiser -- Architecture Validation

- **Rule types**:
  - `forbidden`: Dependencies that must not exist (e.g., feature A cannot import feature B)
  - `allowed`: Whitelist-only mode (everything not explicitly allowed is forbidden)
  - `required`: Dependencies that must exist
- **Built-in checks**:
  - Circular dependencies
  - Orphan modules (files not imported by anything)
  - Dependencies not in package.json
  - Production code importing devDependencies
- **Architecture enforcement**: Regex-based `from`/`to` path rules with severity levels
- **Visualization**: GraphViz DOT, Mermaid, JSON, HTML, CSV, text
- **Key insight for skill**: Rules can encode Clean Architecture / DDD boundaries (e.g., domain cannot depend on infrastructure)

### madge -- Circular Dependency Detection

- **Algorithm**: Modified depth-first search (DFS) on module graph
- **Detection**: Finds all cycles in the dependency graph
- **Output**: JSON, DOT (GraphViz), plain text, visual graph image
- **Languages**: JavaScript, TypeScript (via ts-config)
- **Key insight for skill**: Simpler than dependency-cruiser but focused; good for quick circular-dep checks

### ESLint -- Static Analysis Rules (Code Quality Subset)

| Rule | What It Detects | Default Threshold |
|---|---|---|
| `complexity` | Cyclomatic complexity per function | 20 |
| `max-depth` | Block nesting depth | 4 |
| `max-lines` | File length | 300 |
| `max-lines-per-function` | Function length | 50 |
| `max-params` | Parameter count | 3 |
| `max-nested-callbacks` | Callback nesting | 10 |
| `no-param-reassign` | Parameter mutation | -- |
| `no-eval` | eval() usage | -- |
| `eqeqeq` | Loose equality | -- |
| `no-duplicate-imports` | Duplicate imports | -- |
| `no-unreachable` | Dead code after return/throw | -- |
| `no-unused-vars` | Unused variables | -- |
| `no-shadow` | Variable shadowing | -- |
| `prefer-const` | Let where const suffices | -- |

### SonarQube -- Comprehensive Static Analysis

- **Rule count**: 6,000+ across 20+ languages
- **Rule categories**: Bugs, Code Smells, Vulnerabilities, Security Hotspots
- **Key metrics**:
  - Cognitive Complexity (see scoring below)
  - Cyclomatic Complexity
  - Duplicated Lines Density
  - Technical Debt (time-based estimate)
  - Maintainability Rating (A-E)
- **AI Code Assurance**: Specialized detection for AI-generated code issues
- **Key insight for skill**: Quality gate model (pass/fail on thresholds) is applicable to refactoring skill's audit phase

---

## Cognitive Complexity Scoring

SonarQube's Cognitive Complexity metric (G. Ann Campbell, SonarSource):

### Scoring Rules

| Construct | Increment | Nesting Penalty |
|---|---|---|
| `if`, `else if`, `else` | +1 each | +1 per nesting level |
| `switch` | +1 | +1 per nesting level |
| `for`, `while`, `do-while` | +1 each | +1 per nesting level |
| `catch` | +1 | +1 per nesting level |
| `break`/`continue` to label | +1 | none |
| Logical operator sequence (`a && b && c`) | +1 per mixed sequence | none |
| Ternary operator | +1 | +1 per nesting level |
| Recursion | +1 | none |

### Constructs That Do NOT Add Complexity

- `else` after `if` (counted as part of `if`)
- Null-coalescing operators (`??`)
- Early returns / guard clauses (reduce nesting, reduce score)
- Lambda/arrow function definitions
- Simple `try` block (only `catch` counts)

### Thresholds

| Rating | Score | Interpretation |
|---|---|---|
| Good | 0-10 | Easy to understand |
| Moderate | 11-15 | Acceptable, monitor |
| High | 16-25 | Should refactor |
| Critical | 26+ | Must refactor immediately |

### Key Principle

Cognitive Complexity rewards flattened code. Guard clauses, early returns, and extracted helper functions all reduce scores. Deeply nested code compounds geometrically.

---

## Skill Applicability Matrix

How each tool/technique maps to an automated refactoring skill:

| Capability | Detection Method | Automation Level | Priority |
|---|---|---|---|
| **Long Method detection** | Line count + cognitive complexity | Fully automatable via AST | P0 |
| **Duplicate Code detection** | jscpd Rabin-Karp + token hashing | Fully automatable | P0 |
| **Dead Code detection** | knip static analysis | Fully automatable | P0 |
| **Circular Dependencies** | dependency-cruiser / madge DFS | Fully automatable | P0 |
| **High Complexity functions** | Cognitive complexity scoring | Fully automatable | P0 |
| **Unused Dependencies** | knip package.json analysis | Fully automatable | P1 |
| **Architecture Violations** | dependency-cruiser path rules | Automatable with config | P1 |
| **Large Class detection** | Line/field/method counting | Fully automatable | P1 |
| **Long Parameter List** | Parameter counting | Fully automatable | P1 |
| **Deep Nesting** | AST depth analysis | Fully automatable | P1 |
| **Feature Envy detection** | Cross-class reference counting | Partially automatable | P2 |
| **Data Clumps** | Repeated parameter groups | Partially automatable | P2 |
| **Primitive Obsession** | Type usage pattern analysis | Partially automatable | P2 |
| **Switch Statements smell** | Switch/if-else chain counting | Automatable | P2 |
| **Magic Numbers** | Literal number detection | Fully automatable | P2 |
| **Suggested Fix selection** | Smell-to-technique mapping table | Rule-based lookup | P0 |

### Recommended Skill Pipeline

1. **Scan** -- Run jscpd, knip, dependency-cruiser, and complexity analysis in parallel
2. **Classify** -- Map findings to code smell categories using the catalog above
3. **Prioritize** -- Rank by severity (P0 first) and impact (most references/callers)
4. **Prescribe** -- Look up recommended refactoring technique from the smell-to-fix mapping
5. **Apply** -- Execute refactoring (Extract Method, Guard Clauses, Move Method, etc.)
6. **Verify** -- Re-run scan to confirm improvement, check no regressions

---

## Sources

- [Refactoring.Guru Catalog](https://refactoring.guru/refactoring/catalog)
- [Refactoring.Guru Code Smells](https://refactoring.guru/refactoring/smells)
- [SourceMaking Refactoring Techniques](https://sourcemaking.com/refactoring/refactorings)
- [SonarQube Code Smells](https://www.sonarsource.com/resources/library/code-smells/)
- [SonarQube Rules Documentation](https://docs.sonarsource.com/sonarqube-cloud/standards/managing-rules/rules)
- [Cognitive Complexity by SonarSource](https://www.sonarsource.com/resources/cognitive-complexity/)
- [5 Tips for Reducing Cognitive Complexity](https://www.sonarsource.com/blog/5-clean-code-tips-for-reducing-cognitive-complexity)
- [ESLint Rules Reference](https://eslint.org/docs/latest/rules/)
- [ESLint v10.0.0](https://eslint.org/blog/2026/02/eslint-v10.0.0-released/)
- [jscpd GitHub](https://github.com/kucherenko/jscpd)
- [jscpd npm](https://www.npmjs.com/package/jscpd)
- [Knip](https://knip.dev)
- [dependency-cruiser GitHub](https://github.com/sverweij/dependency-cruiser)
- [dependency-cruiser Rules Reference](https://github.com/sverweij/dependency-cruiser/blob/main/doc/rules-reference.md)
- [madge npm](https://www.npmjs.com/package/madge)
