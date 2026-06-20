# Flag → Parameter Conversion Standard (Definition of Done)

> **Status:** canonical standard for the flag-reduction campaign (2026-06-20).
> Applies to every cycle that converts an `EVOLVE_*` environment flag into a
> typed dynamic input parameter. Enforced by CI (apicover + the env-agnostic
> guard) and by the auditor phase. Reference implementation: `internal/policy`
> (the 4 config accessors) + `internal/quotareset` (`Options`/`Compute`).

## Why

Flag reduction exists to remove **global environment coupling**: `os.Getenv("EVOLVE_X")`
scattered across the code is implicit, untestable global state. The campaign
replaces each flag with a **typed input parameter** sourced from `.evolve/policy.json`
(config-as-code) or an explicit function argument. A conversion is only "done"
when the new parameter is (a) environment-agnostic and (b) provably correct
across its value space — controlled entirely through the component's public API.

## Definition of Done — every flag→parameter conversion MUST

1. **Be environment-agnostic.** The parameter-resolution package reads its value
   ONLY from the typed input (a `policy.json` block via `policy.Load` + an
   accessor, or an `Options`/`Config` struct field) — never `os.Getenv` /
   `os.LookupEnv` / `os.Environ`. Env reads, if any, stay at the application
   composition root (`cmd/`), never in the resolution package.

2. **Enroll in the env-agnostic guard.** Add the package to `paramPackages` in
   `go/internal/policy/param_env_agnostic_test.go`. CI then fails if that package
   ever reintroduces a system-env read.

3. **Ship an env-free, public-API, black-box test suite.** A new
   `*_param_test.go` in `package <pkg>_test` that:
   - drives behavior ONLY through exported APIs (the accessor / `Compute` /
     `Load(explicitPath)`) and explicit inputs — **no `t.Setenv`, no `os.Getenv`**;
   - is table-driven, one edge case per row (AAA; F.I.R.S.T.);
   - covers the full **field × edge-case matrix**: absent→built-in default, zero
     value, explicit `false`/`0` (omit-vs-explicit distinction for pointer
     fields), boundary values, malformed/invalid input, precedence/override
     ordering, and any default-clamp rule (e.g. "override only when ≥ 1");
   - asserts the resolved **values** (not just non-nil) and any never-nil pointer
     guarantee the `cmd/` call sites dereference.

4. **Reach 100% public-API coverage.** Every exported parameter function at
   `100.0%` statement coverage (`go tool cover -func`), and `apicover -enforce`
   exit 0 for the package (every exported symbol named in a same-package test,
   no false-greens). Provably-unreachable defensive branches in unexported
   helpers are documented, not chased (avoid the "100%-coverage-as-goal"
   anti-pattern — do not delete defensive code for a number).

5. **Cover the wiring.** Test the full env-free path `policy.json (temp file) →
   Load → accessor → resolved value`, and assert the accessor output maps
   correctly into the downstream `Config` it feeds (value-correspondence +
   deref-safety), without re-driving the downstream runtime behavior (that
   package owns those tests).

## Verification (run before claiming done)

```bash
cd go
go test ./internal/<pkg>/...                       # suite green, env-agnostic guard green
gofmt -l -s internal/<pkg> ; go vet ./internal/<pkg>/...
go test -coverprofile=cov.txt ./internal/<pkg>/... && go tool cover -func=cov.txt   # accessor/Compute = 100.0%
go build -o bin/apicover ./cmd/apicover
bin/apicover -enforce -cover <(go tool cover -func=cov.txt) "$(go list -f '{{.Dir}}' ./internal/<pkg>)"  # exit 0
```

## Anti-patterns (auto-reject)

- Reading `os.Getenv` inside the resolution package "for backward compat".
- Tests that set `t.Setenv` to exercise the parameter (couples the test to global
  env — the very thing being removed). Drive via the typed input instead.
- "Liar" tests asserting only `!= nil`; tests that touch lines without asserting
  resolved values; one giant test per accessor instead of per-edge-case rows.
- Deleting defensive error handling solely to hit 100% on an unexported helper.

## Reference template

`internal/quotareset/compute_param_test.go` and
`internal/policy/{quotareset,fanout,observer,bridge}_config_param_test.go` are the
worked examples (≈70 table cases, every public parameter API at 100%, apicover
exit 0, zero system-environment use).
