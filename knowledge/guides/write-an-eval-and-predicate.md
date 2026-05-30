# How to write an eval and an ACS predicate

> An **EGPS predicate** is the executable form of an acceptance criterion: a bash
> script at `acs/cycle-N/{NNN}-{slug}.sh` that *runs the actual code path* and exits
> 0 (GREEN) or non-zero (RED). The auditor runs this cycle's predicates **plus every
> prior cycle's** (the regression suite); `red_count == 0` IS the ship gate. This
> guide is how to author one that holds. The *why* is
> [architecture/trust-kernel-and-egps.md](../architecture/trust-kernel-and-egps.md) §2.

## The predicate shape

Copy the convention from any `acs/cycle-104/*.sh`:

```bash
#!/usr/bin/env bash
# AC-ID: cycle-N-001-<slug>
# AC-source: scout-report.md AC <id>; intent.md acceptance_checks[k]
# Behavioral predicate: <one sentence — the behavior under test, not the code text>
# Mutation spec: <which mutants must FAIL — proves the predicate has teeth>
# Bash 3.2 compatible. No declare -A, no mapfile, no GNU-only flags.
# Exit codes: 0 = GREEN (satisfied), 1 = RED (violated)
set -uo pipefail
```

Rules the validator enforces (banned inside a predicate): grep-only "checks",
`echo PASS; exit 0`, network calls, `sleep ≥ 2s`, writes outside its scratch dir,
missing metadata headers. **Behavioral, not grep:** verify the *behavior* by running
the code path and asserting its exit code — not by `grep`-ing the source for a string
(that is the AC-by-grep gaming signal the kernel exists to stop).

## Use the shared assert lib — never re-grep `go test`

Source `acs/lib/assert.sh` and call its helpers instead of scraping tool output.
"Did a Go test pass?" is implemented correctly **once**, there, with its own tests
(`acs/lib/assert_test.sh`):

```bash
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
assert_go_test_pass ./internal/cyclehealth/... 'TestCountFieldDuplicates'
assert_go_build ./...
assert_go_coverage_ge ./internal/router/... 85
```

Real exports in the lib today:

| Helper | Contract |
|---|---|
| `assert_go_test_pass <pkg> [run-regex]` | runs `go test -race -count=1`; asserts **exit 0** (never scrapes `PASS`/`ok`). |
| `assert_go_build [pkg]` | runs `go build` (default `./...`); asserts exit 0. |
| `assert_go_coverage_ge <pkg> <min-pct>` | runs `go test -cover`; parses % via the tested pure `acs_coverage_pct` + `acs_pct_ge`. |
| `acs_go_module_dir` | echoes where `go test` must run — `<toplevel>/go` (the module root), since there is no `go.mod` at the repo root. |

> Why these exist: cycle-131's `^--- PASS:` anchor missed indented subtests, and
> cycle-137's `grep -q PASS` false-RED'd a passing package that only printed `ok`.
> The durable fix is one source of truth keyed off **exit codes**.

## RED-first

EGPS is TDD at the predicate layer. Author the predicate, run it against the current
tree, and **confirm it is RED** before the build makes it GREEN. A predicate that is
GREEN before the work exists is testing nothing.

## The committed-lib footgun (the `assert_go_build` incident)

Predicates `source` `assert.sh` from the **worktree** — a HEAD checkout. So if you add
a new helper to `acs/lib/assert.sh` in the same cycle that authors a predicate calling
it, the predicate fails `assert_go_build: command not found` because the lib change is
**not yet committed** and the worktree checkout doesn't have it. **Lib changes must be
committed to take effect.** Constrain authored predicates to helpers that already exist
in the committed lib (this is cross-CLI bug #3 in cli-matrix-and-drivers.md).

## Running the suite

`evolve acs suite --cycle N` runs cycle-N's predicates + the accumulated regression
suite + red-team, and writes `acs-verdict.json` (exit 2 on any RED, 0 when all green).
That verdict file's `red_count == 0` is what ship re-checks. See
[run-and-debug-locally.md](run-and-debug-locally.md) for where it lands.

## The Go-test alternative (trust-kernel tier)

Durable safety invariants are migrating off cycle-pegged bash predicates into
**black-box Go tests** under `go/test/trustkernel/` (e.g.
`TestShipGate_BlocksWhenRedCountNonZero`). These call only exported Go APIs, are
named for the **behavior** (never `TestC<N>_*`), and run in the unit gate. Prefer a
trust-kernel test for an invariant that should hold *forever*; use a cycle-N bash
predicate for a cycle-specific acceptance check. The tier model and naming rules are
in [`go/docs/testing.md`](../../go/docs/testing.md), and the
invariant→test→knowledge-doc map is its "G3" table.

## The tests that pin this

- `acs/lib/assert_test.sh` — the assert helpers' own tests (the pure
  `acs_coverage_pct` / `acs_pct_ge` field-extraction).
- `go/test/trustkernel/trustkernel_test.go` —
  `TestShipGate_ShipEligibleOnlyWhenRedCountZero`,
  `TestShipGate_BlocksWhenRedCountNonZero` (the `red_count == 0` ship gate the whole
  predicate system feeds).
