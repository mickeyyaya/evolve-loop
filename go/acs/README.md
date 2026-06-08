# ACS — Adversarial Cycle Suite (Go-native)

Go test packages that encode the acceptance criteria (EGPS predicates) the audit
phase / ship gate runs. Three scopes, all `//go:build acs`:

| Dir | Scope | When it runs |
|---|---|---|
| `go/acs/cycle<N>/` | the **current cycle**'s predicates (authored fresh each cycle) | that cycle's audit |
| `go/acs/regression/cycle<N>/` | the curated **durable** predicates, promoted from past cycles | **every** cycle |
| `go/acs/redteam/` | standing **anti-gaming** predicates (ledger/state integrity) | **every** cycle |

Each package is a leaf: no cross-package imports, no shared fixtures. The
red-team predicates are thin wrappers over `go/internal/redteamcheck` (where the
detection logic lives and is adversarially unit-tested in normal CI).

## Build tag: `//go:build acs`

**Every `predicates_test.go` carries `//go:build acs`.** EGPS predicates are
state/environment assertions, not unit tests — e.g. "the working tree has no
uncommitted changes" is false mid-edit — so they must **not** run in the normal
`go test ./...` / CI suite. The `acs` tag excludes them from the normal suite and
selects them for the host-side predicate lane.

| Invocation | Runs predicates? | Used by |
|---|---|---|
| `go test ./...` / CI (`-tags integration`, `/acs/` excluded) | **No** (excluded by the `acs` tag) | every CI run |
| `evolve acs suite --cycle N` | **Yes** — the Go lane: `./acs/cycle<N>` + each `./acs/regression/<sub>` + `./acs/redteam`, each a separate `go test` | the audit phase / EGPS ship gate |
| `go test -tags acs ./acs/regression/cycle<N>` | **Yes** — one regression package | local debugging |

Enforcement: `internal/acssuite.TestAllACSPredicatesAreTagged` (runs in the
normal suite) fails CI if any `predicates_test.go` is missing `//go:build acs`. A
predicate package that fails to compile is a HARD suite error (never a silent
PASS); each scope runs as a separate `go test` so one broken package can't hide
behind another's events.

## The gate scope mirrors the (retired) bash lane

`evolve acs suite` runs the current cycle + the curated regression set + red-team
every cycle — never every historical cycle (a blind `./acs/...` would drag in
bit-rotted point-in-time predicates and block the gate). This is the Go-native
successor to the bash `acs/cycle-N/` + `acs/regression-suite/cycle-*/` +
`acs/red-team/` lanes, retired in the EGPS Go-native migration (ADR-0025 → the
egps-v11 ADR).

> **Regression set is a SUPERSET of the former bash curation.** Phase C relocated
> the durable predicates by moving each origin cycle's whole Go port into
> `go/acs/regression/cycle<N>/`, so a few *non-promoted* (but green) funcs from
> those cycles ride along — stricter coverage, not weaker. All are green or SKIP
> (on absent runtime state) today. If a non-curated func ever flakes, prune that
> single func; the durable predicates are the load-bearing ones.

## Authoring a new cycle's predicates

A new acceptance criterion is a Go test, not a bash `.sh`:

```go
//go:build acs

// Package cycleN ports the cycle-N ACS predicates.
package cycleN

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestCN_001_Slug(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// Behavioral predicates invoke the system-under-test (no source-grep gaming).
	if !acsassert.FileContains(t, filepath.Join(root, "..."), "...") {
		t.Errorf("...")
	}
}
```

Place it at `go/acs/cycle<N>/predicates_test.go`; the `acssuite` Go lane picks it
up when the cycle runs. To promote a predicate to the permanent regression set,
move its package under `go/acs/regression/`.
