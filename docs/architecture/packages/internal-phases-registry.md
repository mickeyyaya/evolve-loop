# internal/phases/registry

## Purpose

`internal/phases/registry` is the Factory Method for phase runners. Each `internal/phases/<name>` package registers a `Factory` in `init()`. The dispatchers (`cmd_phase.go`, `cmd_compose.go`) look a factory up by name, so no switch statement needs an edit when a phase is added.

## Design

- **Pattern.** The product is `core.PhaseRunner`. The factory is a closure that builds a runner from the per-request envelope `core.PhaseRequest`. The closure captures its own dependencies (bridge, prompts). The registry knows only the `PhaseRunner` contract.
- **Open for extension.** A new phase is a new package plus one `init()` line, with no edit to dispatch. The build recipe lives next to the runner it builds.
- **Registration shape.** In each phase package: `func init() { registry.Register("build", func(req core.PhaseRequest) core.PhaseRunner { return New(Config{...}) }) }`.
- **Lookup.** `For` returns `(factory, true)` on a hit and `(nil, false)` on a miss. Callers exit with code 10 and print usage on a miss. `Names` returns a sorted snapshot, used for dispatcher usage output and the docs-contract test.
- **Test helpers.** `ResetForTesting` clears the registry. `SnapshotForTest` captures the registered set and returns a restore function, so a test that mutates the registry puts it back. Both are exported with an explicit suffix so their purpose is clear. They are the single home of the snapshot and restore idiom that the cmd test suites each re-implemented.

## Invariants

- **Registration happens at init time, lookups run concurrently.** An `RWMutex` guards both. Lookups take the read lock. Pinned under `-race` by `TestConcurrentLookupsAreSafe`.
- **A duplicate name panics.** An init-time conflict then surfaces at startup and does not become a runtime mystery (`TestRegister_DuplicatePanic`).
- **An empty name or a nil factory panics.** A silent registration later produces a nil dereference (`TestRegister_EmptyNamePanics`, `TestRegister_NilFactoryPanics`).
- **`Names` is sorted and never nil.** The sorted order is stable across runs, so docs-contract tests can compare against a golden list (`TestNames_Sorted`, `TestNames_Empty`).
- **Production code never calls `ResetForTesting` or `SnapshotForTest`.**
