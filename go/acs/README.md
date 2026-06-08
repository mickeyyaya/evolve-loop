# ACS — Adversarial Cycle Suite

This directory contains per-cycle test packages (`cycleN/predicates_test.go`) that
encode the acceptance criteria (EGPS predicates) for each evolve-loop cycle. Each
package is a leaf: no cross-package imports, no shared fixtures.

## Build tag: `//go:build acs`

**Every `cycleN/predicates_test.go` carries `//go:build acs`.** EGPS predicates are
state/environment assertions, not unit tests — a predicate like cycle106's
"working tree has no uncommitted changes" is false mid-edit, so it must **not**
run in the normal `go test ./...` / CI suite. The `acs` tag is what excludes them
from the normal suite and selects them for the host-side predicate lane.

| Invocation | Runs predicates? | Used by |
|---|---|---|
| `go test ./...` / CI (`-tags integration`, `/acs/` excluded) | **No** (excluded by the `acs` tag) | every CI run |
| `evolve acs suite --cycle N` | **Yes** — bash lanes + the Go lane scoped to `./acs/cycleN` | the audit phase / EGPS ship gate |
| `go test -tags acs ./acs/cycleN` | **Yes** — that one cycle | local debugging |
| `go test -tags acs ./acs/...` | **Yes** — *all* historical cycles (see caveat below) | bulk replay only |

The one enforcement point is `internal/acssuite.TestAllACSPredicatesAreTagged`
(runs in the normal suite): it fails CI if any `predicates_test.go` is missing
`//go:build acs`.

> The `redteam/` package holds a **meta-test** of the bash `rt-*.sh` predicates
> (it shells out to them with fabricated-attack fixtures). It is *not* an EGPS
> predicate, so it is **not** `acs`-tagged and is named `redteam_test.go` (not
> `predicates_test.go`) — it runs in the normal suite like any other unit test.

## The gate runs only the current cycle

`evolve acs suite` scopes the Go lane to `./acs/cycleN/...` — the current cycle
only. This mirrors the bash lane, which auto-runs the current cycle + the curated
`acs/regression-suite/` (hand-promoted, durable predicates), and never replays
every historical `acs/cycle-N/`. A blind `go test -tags acs ./acs/...` would drag
in bit-rotted historical predicates and block the ship gate.

**Caveat — historical bit-rot.** Several older packages (authored against
point-in-time state, e.g. `phase-registry.json` schema_version, `state.json`
`carryoverTodos`, deleted `legacy/scripts/...` paths) **FAIL** under a full
`go test -tags acs ./acs/...` because the codebase has legitimately moved past
what they assert. They are kept for the historical record — read
`acs/cycle47/predicates_test.go` to see what *was* enforced at cycle-47 ship time
— but they are **never run by the gate** (current-cycle scope). Triaging /
retiring them is tracked under the EGPS Go-native migration Phase C.

## Authoring a new cycle's predicates (Phase B contract)

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

Place it at `go/acs/cycleN/predicates_test.go`. The `acssuite` Go lane picks it
up automatically when the cycle runs. If a bash predicate for the same
`(cycle, ac)` still exists, delete the `.sh` in the same change — the double-count
guard synthesizes a RED if both lanes assert the same pair.
