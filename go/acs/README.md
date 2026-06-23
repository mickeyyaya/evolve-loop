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

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
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

### Absence checks — use `FileNotContains`, never inverted `FileContains`

To assert something is **absent** (e.g. a removed flag no longer appears in a
source file), use `acsassert.FileNotContains(t, path, "X")`. Do **NOT** write
`if acsassert.FileContains(t, path, "X") { t.Errorf(...) }`: `FileContains` is a
*positive* assertion that fires its own `t.Errorf("missing")` when `X` is absent,
so the inverted idiom red-fails on the **correct** (absent) state — the `if` body
is skipped but the internal `Errorf` still fails the test (the cycle-352
broken-predicate incident). `FileNotContains` returns true and logs nothing when
`X` is absent, and fails only when `X` is present.

### Generated-from-source docs — the dual-root pattern

A cycle that edits a doc generated from source (e.g.
`docs/architecture/control-flags.md` from the `flagregistry`, or
`skills/*/SKILL.md` from phase facts) is editing a **worktree** artifact: the
change reaches main only at *ship*, after audit. The ACS suite runs predicates
with two roots so both kinds of read resolve correctly (the dual-root pattern):

- **`EVOLVE_PROJECT_ROOT` → main** (the STATE root): `.evolve/` runtime data
  (history, baselines, the current build-report) lives on main, not the worktree
  (issue #12). `cmd_acs.go` follows `git --git-common-dir` back to the main
  checkout to compute it.
- **`EVOLVE_WORKTREE_ROOT` → the cycle's worktree** (the SOURCE root): generated
  SOURCE docs live in the worktree, so `evolve flags check` / `evolve skills
  check` (and any reader via `sourceRoot()`) validate the **committed worktree
  artifact**, not main's stale working copy.

Because of this, shelling `evolve flags check` from a predicate now validates the
**worktree** doc and greens on correct work — no manual regen of main, no
operator-prepared state. (Before the dual-root fix it read main's stale copy and
red-failed correct work; the cycle-355 incident. "Greening" it by hand-regenerating
main is gaming — non-durable state outside the cycle's commit, which the audit
catches.)

The most robust predicate, however, takes no subprocess and no binary dependency:
in a `go test`, regenerate the expected output from the worktree's source-of-truth
(import the SSOT package directly) and assert it matches the worktree's committed
doc, resolving paths under `acsassert.RepoRoot(t)` (the worktree). Prefer that
when authoring a new generated-doc predicate; the `flags check` / `skills check`
subprocess path is the safety net the dual-root pattern keeps correct.
