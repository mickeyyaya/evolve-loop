# ADR-0042: EGPS v11 — Go-native acceptance predicates (bash retired)

> Status: **Accepted** (2026-06-08). Completes the EGPS Go-native migration: acceptance
> predicates are now Go tests, not bash scripts. Supersedes the bash predicate format of
> [egps-v10.md](../egps-v10.md) and the bash suite-runner of [ADR-0025](0025-acs-suite-runner-and-red-team.md).
> Honors the operator's go-only hard rule. Builds on the Phase-A keystone (ADR-unified two-lane runner).

## Context

EGPS v10 (egps-v10.md) compiled every acceptance criterion to an executable bash predicate at
`acs/cycle-N/{NNN}-{slug}.sh`; ADR-0025 restored the host-side bash suite-runner (`evolve acs suite`)
that globs `acs/cycle-N/` + `acs/regression-suite/cycle-*/` + `acs/red-team/` and writes
`acs-verdict.json`. Two problems motivated this ADR:

1. **Every cycle minted new bash predicates** — thin shims, ~36% of which just wrapped an existing Go
   test (`assert_go_test_pass ./pkg -run TestX`), duplicating that test in shell. This violated the
   go-only rule and the single-source principle.
2. **A parallel Go predicate harness already existed** (`go/acs/cycleNN/predicates_test.go`, 54
   packages) but the gate didn't count it — `acssuite` globbed only `*.sh`.

## Decision

**Acceptance predicates are Go tests tagged `//go:build acs`.** The gate (`evolve acs suite`) runs a
single Go lane over three scopes, every cycle:

| Scope | Path | Cardinality |
|---|---|---|
| current cycle | `go/acs/cycle<N>/predicates_test.go` | this cycle's ACs (authored fresh) |
| regression | `go/acs/regression/cycle<N>/predicates_test.go` | curated durable predicates, every cycle |
| red-team | `go/acs/redteam/predicates_test.go` | standing anti-gaming, every cycle |

A predicate is `func TestC<N>_<NNN>_<slug>(t *testing.T)` using the `acsassert` DSL; it reports RED by
failing (`t.Errorf`/`t.Fatalf`) and SKIP via `t.Skip` (evidence absent). Each scope runs as a
**separate** `go test -json -tags acs <pkg>` so a per-package compile error is a HARD suite error
(never a silent PASS) — a recursive `./acs/...` could let one package's failure hide behind another's
events.

### What carries over from v10 (unchanged invariants)
- `acs-verdict.json` is the verdict; **`red_count == 0` ⟺ PASS ⟺ ship_eligible** (binary, no WARN).
- SKIP (evidence absent) counts neither red nor green — a fresh clone still ships.
- The anti-grep rule: a behavioral predicate invokes the system under test; an
  `acsassert.FileContains`-only predicate (the Go-native `grep -qF`) is the forbidden cycle-85 shape.

### What changed (BREAKING vs v10 / ADR-0025)
| v10 / ADR-0025 | v11 |
|---|---|
| predicate = `acs/cycle-N/{NNN}-{slug}.sh` (bash) | predicate = Go test func, `//go:build acs` |
| `acssuite` bash lane globs `*.sh` across 3 roots | Go lane runs `./acs/{cycle<N>,regression/<sub>,redteam}` |
| current-cycle + curated `acs/regression-suite/` + `acs/red-team/` | `go/acs/{cycle<N>,regression,redteam}` |
| `acs/lib/assert.sh` bash helpers | `go/pkg/acsassert` DSL + `go/internal/redteamcheck` |
| suite scope = whole-repo bash | Go lane scoped to current cycle + curated regression (never every historical cycle — bit-rotted predicates would block the gate) |

## Migration (how we got here, gate-green throughout)

- **Phase A** — unified `acssuite.Run` to count Go predicates alongside bash (one merged verdict via a
  shared `record()`); migrated 54 predicate packages to `//go:build acs`; current-cycle scope.
- **Phase B** — switched the authoring contract (TDD-engineer/Builder personas + profiles) to Go;
  zero new bash predicates thereafter.
- **Phase C** — retired ~408 dormant bash predicates; ported the 3 red-team predicates to
  `internal/redteamcheck` (adversarially unit-tested in CI) + thin `go/acs/redteam` predicates;
  relocated the durable regression set to `go/acs/regression/`.
- **Phase D** — retired the bash runtime: deleted every bash predicate + `acs/lib/` (the `acs/`
  tree is gone), recorded this ADR; the now-inert `acssuite` bash lane (`discover`/`runBash` — globs
  nothing once no `.sh` remain) is removed in the final cleanup commit.

## Consequences

- **Predicate compile errors are caught at author time** (`go vet -tags acs`) and at the gate (hard
  error), not discovered as runtime bash failures.
- **Detection logic is unit-tested in CI** (e.g. `internal/redteamcheck` adversarial tests) — stronger
  than the old on-demand bash meta-test.
- **The regression set is a superset** of the former bash curation (whole cycle packages were
  relocated rather than risk a fragile per-predicate remap); a few non-promoted-but-green funcs ride
  along as stricter coverage, prunable if any flakes.
- **`enforce`-mode gates unchanged**: `EVOLVE_EVAL_GATE`, `EVOLVE_CONTRACT_GATE`, EGPS `red_count==0`
  to ship all continue to read `acs-verdict.json`.
- The historical bash predicates remain in git history (recoverable); the dormant/bit-rotted Go ports
  were deleted.
