# Design Patterns — selection, misuse, and when not to

> Load when: structuring a component, adding a dependency seam, or tempted to abstract. The 2025-26 finding is blunt: LLMs misapply patterns more often than they miss them — premature abstraction and interfaces-for-mocking are the dominant failures. This file is as much about restraint as application.

## The four load-bearing patterns (FLEXIBLE — strong defaults)

1. **Dependency Injection at the composition root.** Constructors take their collaborators (`NewOrchestrator(storage, ledger, runners, opts...)`); the main/cmd layer wires concretes. Never `init()`-time globals, never reaching into package singletons from deep code. Functional options for the optional tail.
2. **Strategy over branching.** When behavior varies by kind (driver, provider, source), one interface + one implementation per kind beats an if/else or switch ladder that every new kind must edit. The switch is fine until the SECOND site switches on the same kind — then extract the strategy.
3. **Specification for composable predicates.** Rules/filters/policies as predicate objects with one `Matches(x)` method, combined by union/intersection — no selector ladders. This is how config-driven behavior stays config-only (project standing rule: zero feature flags; policy blocks + resolvers instead).
4. **Ports & adapters (hexagonal).** Core logic defines the interface (port) it needs; adapters at the edge implement it for the real world (fs, git, network, clock, subprocess). Everything non-deterministic lives behind a port — that is what makes the core testable WITHOUT mocks of its own logic.

## Go interface discipline (RIGID for Go; the principle generalizes)

- **Consumer-side interfaces**: declared in the package that USES the dependency, not the one implementing it. The implementer exports a struct; `var _ Port = (*Impl)(nil)` pins conformance where needed.
- An interface earns existence with **≥2 real implementations** (a fake in tests counts only if it's a genuine behavioral fake, not an expectation mock — and even then, prefer waiting for the second real one).
- **Never extract an interface "for testability"** — that's the seam telling you the concrete type does too much or hides a process boundary; fix that instead.
- Keep interfaces 1-3 methods. A 7-method interface is a class hierarchy wearing a disguise.
- Accept interfaces, return structs.

## The misuse table (RIGID — stop when you catch yourself)

| Impulse | Why it's wrong | Instead |
|---|---|---|
| Abstract on the second occurrence | Two points define infinitely many lines; you'll abstract the wrong axis | Rule-of-three: wait for the third, then extract what actually varies (switch-on-kind is a different axis — Strategy triggers at the SECOND switch site, see below) |
| A `Manager`/`Service`/`Util` grab-bag type | Name admits it has no single responsibility | Split by the verbs it actually performs |
| Config knob for a hypothetical future need | YAGNI; every knob is test-matrix surface | Hardcode the current truth; add the knob when the second need is REAL |
| Wrapper that only forwards calls | Indirection without abstraction | Delete it; call the thing |
| Factory for a struct with one constructor call site | Ceremony | `New*()` function, or a literal |
| Singleton for shared state | Hidden coupling, untestable | Inject the instance from the composition root |
| Observer/event bus inside one package | Decouples things that were never coupled | Direct call; events earn their place at module boundaries |
| Generics/type-params to deduplicate two concrete funcs | Readability cost exceeds the duplication cost at n=2 | Rule-of-three again; generics for genuinely open sets |

## Choosing: a 30-second decision path

1. Is the variation **data**? → table/map, not code structure.
2. Is it **behavior varying by kind**, with ≥2 kinds today? → Strategy (interface at the consumer).
3. Is it a **rule set that composes**? → Specification objects + union/intersection.
4. Is it a **process boundary** (IO, time, subprocess)? → port + adapter, injected.
5. None of the above? → **write the plain code.** The absence of a pattern is not a design smell; the wrong pattern is.

## Refactoring toward patterns (FLEXIBLE)

Patterns are destinations reached by refactoring under green tests, not blueprints imposed up front. When the third duplication or second switch site appears: write the characterization tests if missing, extract, keep the diff scoped to the extraction (the behavior change that motivated you ships separately — one concern per change). If the extraction fights the existing conventions of the package, the package's conventions win until a dedicated refactor commit argues otherwise.
