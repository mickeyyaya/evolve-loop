# Comment history: `internal/evalqualitycheck`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/evalqualitycheck/evalqualitycheck.go:103` — above `res.Overall = LevelWarn`

```text
// Zero parsed commands means this gate verified NOTHING. Returning
// PASS here is the vacuity that silently defeated the gate for every
// bullet-format eval until 2026-08-09 (ADR-0084 invariant 2) — a
// format the scanner cannot read and an eval with no graders must
// both surface, not slide through.
```

### `go/internal/evalqualitycheck/evalqualitycheck.go:117` — above `var codeBulletRE = regexp.MustCompile("^-\\s*'\\[code\\]'\\s*'(.+)'\\s*$")`

````text
// codeBulletRE matches the scout template's grader bullet form
// (agents/evolve-scout-reference.md, eval-format-template anchor):
//
//   - `[code]` `<command>`
//
// The command is the second backtick span (greedy: a command may itself
// contain backticks). [model]/[human] bullets are not bash and are not
// matched. 281 of the 625 live evals use this form — reading only ```bash
// fences left them all unscanned (the vacuous-gate class, ADR-0084).
````

### `go/internal/evalqualitycheck/evalqualitycheck.go:187` — above `commitPresenceRE = regexp.MustCompile('\bgit\s+(log|rev-list)\b[^|&;]*[a-zA-Z0-9_~^@]\.{2,3}([a-zA-Z0-9_~^@]|\s|$)')`

```text
// commitPresenceRE matches `git log`/`git rev-list` invocations whose
// arguments contain a revision range (`a..b` / `a...b` / trailing-open
// `a..`), anywhere in the line (the RE is unanchored, so it also fires
// inside $(...) and after && — the [^|&;]* guard only stops a range in
// a LATER segment being attributed to this invocation). Such predicates
// assert commit PRESENCE, which is structurally false after
// worktree-normalize soft-resets builder commits to base before audit
// (killed cycles 236/237). Boundaries require rev-name characters so
// pathspecs like `dir/../file` don't fire. Deliberate gaps: left-open
// `..HEAD` and `^main HEAD` exclusion syntax are not matched (rare in
// eval predicates); `git diff a..b` is NOT matched — content parity is
// the sanctioned pattern.
```

### `go/internal/evalqualitycheck/evalqualitycheck_test.go:104` — above `func TestCheck_NonBashFencedBlock_Ignored(t *testing.T) {`

```text
// TestCheck_NonBashFencedBlock_Ignored — only bash fences are parsed as
// commands. (Since the ADR-0084 vacuity fix, an eval with zero parsed
// commands additionally carries one explanatory WARN entry — that entry is a
// diagnostic, not a parsed command.)
```

### `go/internal/evalqualitycheck/evalqualitycheck_test.go:190` — above `func TestCheck_CommitPresenceRange_HALT(t *testing.T) {`

```text
// TestCheck_CommitPresenceRange_HALT — `git log`/`git rev-list` range
// assertions are structurally false after worktree-normalize (builder
// commits are soft-reset to base before audit), so predicates asserting
// commit PRESENCE must be rejected at pre-flight. This is the defect that
// killed cycles 236 and 237 (inbox: normalize-vs-commit-claims).
```

### `go/internal/evalqualitycheck/evalqualitycheck_test.go:219` — above `func TestCheck_ContentParityAndPlainGit_PASS(t *testing.T) {`

```text
// TestCheck_ContentParityAndPlainGit_PASS — the GOOD exemplar (content
// parity via git diff, cycle-236 001-rescue-parity-landed.sh) and
// range-free git inspection must NOT be flagged.
```

### `go/internal/evalqualitycheck/flakylint.go:1` — above `package evalqualitycheck`

```text
// flakylint.go — authoring-time flaky-shape lint over ACS predicate SOURCES
// (acs-metapredicate-suite-scope). TDD keeps authoring cycle predicates that
// flake under fleet load (cycles 1173/1175/1178: whole-package meta-predicates
// whose inner tests are environment-sensitive); acssuite's scope-lint demotes
// them at RUN time, but by then the cycle has already burned a suite run. This
// lint flags the same shapes — plus the other dominant flaky classes — while
// the predicate is being AUTHORED.
//
// PRODUCTION CALLERS (a lint with no caller is dead code — the review finding
// that held this back):
//
//   - internal/evalgate.flakyShapeGate — Gate D, mounted in NewReviewer's gate
//     slice and fired by the orchestrator's per-phase DeliverableReviewer at the
//     END of the tdd phase, i.e. the moment go/acs/cycle<N>/predicates_test.go
//     first exists. ADVISORY: it can surface, never block.
//   - internal/cli/guardcmd.flakyLintAdvisory — `evolve eval quality-check
//     -predicates <path>`, the operator/agent-facing hand check of the same rule.
//
// Five deterministic patterns, each annotated with its Luo et al. FSE'14
// flakiness-taxonomy class (async-wait 45% + concurrency 20% dominate):
//
//  1. suite-scope go-test shells (concurrency): `./...`, any `/...` expansion,
//     multi-package invocations, or the known 40s+ suites (internal/core,
//     cmd/evolve). A SINGLE named package (`go test ./internal/<pkg>`) is the
//     sanctioned shape.
//  2. time.Now()-derived deadlines/timeouts (async-wait): wall-clock bounds
//     stretch arbitrarily under host contention.
//  3. hardcoded PIDs below 100000 used for liveness (environment — house
//     extension of the taxonomy): PIDs are never stable across hosts/runs.
//  4. subprocess git without `-C <dir>` (environment): resolves the repo from
//     process cwd, which differs between main tree, worktree, and fleet lanes.
//     Setting cmd.Dir (the house idiom) or a leading `cd` in sh -c also counts
//     as anchored.
//  5. unreaped load-generation (resource-leak): spawning `yes`/`stress`/shell
//     busy loops through a constructor that binds NO context to the child's
//     lifetime (exec.Command, acsassert.SubprocessOutput) leaves children
//     pinning the host after the predicate exits.
//
// Stage: ADVISORY at both callers. In the CLI a finding raises PASS→WARN (exit
// 1) via a MONOTONIC severity join, so it can never lower a Level-0 tautology
// HALT; in the gate it is structurally non-blocking (block is a constant false).
// The gate policy machinery may promote later, mirroring the runtime-reference
// promotion path (WARN-only → fail-gate after soak).
//
// FALSE-POSITIVE DISCIPLINE. A lint that prints a claim which is false about the
// code teaches authors to ignore it. Suite-scope findings are therefore
// argv-position aware (execPatternIndex, flakylint_exec.go): a package pattern is
// flagged only when it reaches a `go test` argv, or reaches NO exec argv at all.
// A pattern handed to some OTHER subprocess (`go build`/`go vet`/`rg`) is a
// compile or a search, not a suite, and stays clean. Three cuts, each measured
// against the live 282-dir corpus (341 → 297 → 179 findings, 117 → 102 → 64 dirs):
//
//   - non-`go test` argvs are exempt;
//   - the known-slow finding is suppressed for a -run/-run= narrowed invocation
//     (a RECURSIVE pattern stays flagged — -run selects which tests run, not
//     which packages are built and loaded);
//   - the scan follows ONE level into a same-package helper with the call's args
//     bound to its params, because the canonical corpus idiom puts the pattern in
//     a const the test function names and the -run in a shared helper. Without
//     the hop, 142 of 297 findings (48%) advised "narrow with -run" at code that
//     already narrowed.
//
// "Reaches no exec argv" still keeps its finding: the pattern may be built by
// concatenation or handed to a helper two levels down, which is unresolvable
// here, and an advisory note is the safe direction. Known residuals: a
// package-pattern literal used as pure DATA in a function with no exec at all
// (strings.Contains(enforceFile, "./internal/core")); helper chains deeper than
// one hop; and -run accepted by NAME, not selectivity, so `-run .` silences a
// known-slow finding while running the whole suite.
//
// Other conservative limitations (same direction as acssuite's scope-lint):
// patterns assembled by string concatenation, fmt.Sprintf, or local variables are
// invisible, as is a time.Now() split across statements (`now := time.Now();
// now.Add(...)` — no local dataflow); a missed flaky shape merely goes unflagged,
// and the runtime scope-lint plus the bounded flake retry remain the backstops.
// Package-level string consts ARE resolved (the cycle-1117 bridgePkg shape).
// The package-pattern scope rule itself lives in internal/gopkgpattern, shared
// with acssuite's run-time scope-lint so the two can never drift apart.
```

### `go/internal/evalqualitycheck/flakylint.go:227` — above `func flakyStringConsts(files []*ast.File) map[string]string {`

```text
// flakyStringConsts collects package-level string const/var values by name so
// const-indirected patterns (cycle-1117's bridgePkg shape) resolve at use site.
```

### `go/internal/evalqualitycheck/flakylint.go:262` — above `index := indexExecPatterns(fn, consts, helpers)`

```text
// M4: one pass over every exec constructor reachable from this body records
// which argv position each package pattern reached. The literal scan below
// consults it so a pattern handed to `go build`/`go vet`/`rg` is not reported
// as a test suite (cycle-969 buildEvolve / cycle-941 module_builds shapes).
```

### `go/internal/evalqualitycheck/flakylint.go:425` — above `if !argsContainStringLit(call.Args, "-C", consts) {`

```text
// -C may be nested (append([]string{"-C", dir}, args...)... — the
// cycle-962/968 house idiom), so scan the raw arg ASTs, not just the
// top-level argv.
```

### `go/internal/evalqualitycheck/flakylint_test.go:10` — above `func writePredicateDir(t *testing.T, src string) string {`

```text
// flakylint_test.go — authoring-time flaky-shape lint over ACS predicate
// sources (acs-metapredicate-suite-scope). Each flagged fixture is the shape
// that burned cycles 1173/1175/1178 (whole-package meta-predicates whose inner
// tests are environment-sensitive under fleet load); each clean fixture is the
// sanctioned equivalent that must NOT be flagged.
```

### `go/internal/evalqualitycheck/flakylint_test.go:103` — above `func TestLintFlaky_SuiteScope_Flagged(t *testing.T) {`

```text
// TestLintFlaky_SuiteScope_Flagged — `./...`, `/...` expansion, the known
// 40s+ suites (internal/core, cmd/evolve — including via const indirection,
// the cycle-1117 bridgePkg shape), multi-package invocations, and sh -c
// wrapped forms are all suite-scope findings of the concurrency class.
```

### `go/internal/evalqualitycheck/flakylint_test.go:426` — above `func TestLintFlaky_GoBuildVet_NotSuiteScope(t *testing.T) {`

```text
// TestLintFlaky_GoBuildVet_NotSuiteScope — the suite-scope rule targets
// go-TEST shells: a direct `go build` / `go vet` over the same patterns is a
// compile, not a 40s+ test suite under contention (cycle-969 buildEvolve /
// cycle-941 module_builds corpus shapes must stay clean).
```

### `go/internal/evalqualitycheck/flakylint_test.go:779` — above `func TestLintFlaky_HelperHopStillFlagsWideInvocation(t *testing.T) {`

```text
// TestLintFlaky_HelperHopStillFlagsWideInvocation — the helper hop must resolve
// BOTH ways. A helper that shells the whole known-slow package with no -run is
// the real cycles-1173/1175/1178 shape and must still fire; a hop that only ever
// suppressed would have traded 48% false positives for false negatives.
```

### `go/internal/evalqualitycheck/vacuity_test.go:3` — above `import (`

````text
// vacuity_test.go — pins the fix for the vacuous quality-gate class found in
// the 2026-08-09 postmortem sweep (docs/incidents/2026-08-09-zero-ship-batch.md,
// ADR-0084 invariant 2): the scout template mandates `- [code] <cmd>` bullet
// graders (agents/evolve-scout-reference.md, eval-format-template anchor) but
// scanBashCommands read only ```bash fences — so 281 of 625 live evals
// (bullet-format) produced ZERO parsed commands and the anti-gaming gate
// returned LevelPass vacuously. Three pins: (1) the bullet format parses,
// (2) the template's own literal example round-trips through the production
// scanner (single-source: template drift breaks this test), (3) zero parsed
// commands is a WARN, never a silent PASS.
````
