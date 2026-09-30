# Comment history: `acs/regression/flagreaders`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/flagreaders/denied_dir_test.go:11` — above `func TestScanTextTree_SkipsASandboxDeniedDirectory(t *testing.T) {`

```text
// The audit phase's sandbox denies docs/private; the repo-wide shell scan
// must skip a denied directory instead of failing the gate (cycles 1676/1679).
```

### `go/acs/regression/flagreaders/readers_test.go:3` — above `package flagreaders`

```text
// Package flagreaders is a durable ACS regression guard for the flag-reduction
// campaign: every well-formed EVOLVE_* reference on a production surface MUST
// have a flagregistry row. Surfaces scanned: production Go (non-_test.go, outside
// go/acs), CI workflows (.github/), skill/agent instructions (skills/, agents/),
// shell scripts (*.sh anywhere), the go/Makefile, and current-state instruction
// docs at the repo root (AGENTS.md, CLAUDE.md, README.md, …) — but NOT historical
// or policy prose (CHANGELOG.md, SECURITY.md, …), which legitimately names removed
// flags and flag families.
//
// It catches the silent-orphan class — removing a registry entry while a reader
// still exists, or adding a reader without documenting the flag — which the
// registry has no other guard for (the read path does not funnel through the
// registry). It replaces the blunt `len(All) >= 250` count-floor that used to
// (accidentally) block intentional reduction.
//
// Cross-surface scope is load-bearing: cycle-360 removed two "dead" flags that
// still had live readers in adapters/claude.sh because the scan (and the Scout
// grep, and the per-cycle guard) inspected Go ONLY. A Go-only "dead" verdict for
// a flag with a shell/skill/CI reader is a false-dead — the recurring FAIL class
// this guard now forecloses (knowledge-base/research/flag-reduction-campaign-2026-06-18.md).
//
// Discrimination: Go is inspected at the STRING-LITERAL AST level (not comments,
// not substrings); text surfaces are scanned line-by-line with a \b-anchored
// token regex. Both reject mid-sentence mentions ("EVOLVE_FOO is deprecated")
// and dynamic prefixes ("EVOLVE_E2E_MODEL_${cli}" ends in '_'). docs/ and
// control-flags.md are NOT scanned: they catalog every flag by design.
```

### `go/acs/regression/flagreaders/readers_test.go:58` — above `var skipDirs = map[string]bool{`

```text
// skipDirs are subtrees whose EVOLVE_* references are not production surfaces:
// vendor/testdata are fixtures; .git is VCS metadata; node_modules is deps;
// .evolve is gitignored runtime state (worktrees/runs), not committed source;
// dist is build output (goreleaser dist/, landing/dist/ — gitignored, generated,
// e.g. landing/dist/install.sh is a copy of the source install.sh).
// ipcenv is the SSOT for IPC protocol constants — its literals ARE the protocol
// values, not readers; it has no flagregistry row by design (cycle-14).
// Matched by basename (a future production dir named "acs" is pruned by PATH in
// the Go walk, not here). _test.go files are skipped per-file.
```

### `go/acs/regression/flagreaders/readers_test.go:80` — above `var shellExts = map[string]bool{".sh": true}`

```text
// shellExts drives the repo-wide *.sh scan. The script→Go migration removed all
// shell; this keeps the invariant if shell ever returns (the exact gap that made
// cycle-360's Go-only scan unsafe).
```

### `go/acs/regression/flagreaders/readers_test.go:106` — above `"agents/evolve-tester.md": {"EVOLVE_WORKTREE_PATH": true},`

```text
// agents/evolve-tester.md preserves the dual-var worktree shell pattern required
// by the cycle-50/C50_009 regression invariant. EVOLVE_WORKTREE_PATH was retired
// in cycle-10; the snippet documents the backward-compat form for reference.
```

### `go/acs/regression/flagreaders/readers_test.go:110` — above `"skills/adversarial-testing/SKILL.md": {"EVOLVE_WORKTREE_BASE": true},`

```text
// skills/adversarial-testing/SKILL.md uses EVOLVE_WORKTREE_BASE as the WORKED
// EXAMPLE of the cycle-20 split-const dodge (the dodge it teaches reviewers to
// catch). The dial was legitimately removed in 2026-06 (policy.json worktree.base
// + WithWorktreeBase DI, ADR-0064); the name survives only as documentation of
// the historical dodge, not a live reader.
```

### `go/acs/regression/flagreaders/readers_test.go:118` — above `func TestEveryProductionReaderHasRegistryRow(t *testing.T) {`

```text
// TestEveryProductionReaderHasRegistryRow fails if any standalone EVOLVE_*
// reference — in production Go, a CI workflow, a skill/agent instruction, or a
// shell script — lacks a flagregistry row. Broadened beyond Go after cycle-360.
```

### `go/acs/regression/flagreaders/readers_test.go:221` — above `if os.IsPermission(err) && info != nil && info.IsDir() {`

```text
// A directory the phase sandbox denies (docs/private under the
// audit profile) is outside this scan's surface, not a scan
// failure: skip it visibly rather than red the whole gate
// (cycles 1676/1679 — an instrument fault charged as a defect).
```

### `go/acs/regression/flagreaders/surface_scan_test.go:10` — above `func TestScanTextTree_DetectsNonGoOnlyReference(t *testing.T) {`

```text
// TestScanTextTree_DetectsNonGoOnlyReference is the cycle-360 regression proof.
//
// Cycle-360 removed flags classified "dead" by a Go-only scan while they still
// had live readers in a non-Go surface (adapters/claude.sh). This test pins that
// the broadened guard detects an EVOLVE_* reference living ONLY in a non-Go
// surface: scanTextTree over a fixture skill flags EVOLVE_FAKE_NONGO_ONLY_FLAG
// (no registry row) while ignoring a real, registered flag in the same file and
// a dynamic-prefix fragment.
```
