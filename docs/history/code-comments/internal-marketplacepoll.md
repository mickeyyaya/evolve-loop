# Comment history: `internal/marketplacepoll`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d9

### `go/internal/marketplacepoll/marketplacepoll.go:1` — above `package marketplacepoll`

```text
// Package marketplacepoll ports legacy/scripts/release/marketplace-poll.sh.
//
// Post-publish marketplace propagation verifier (v8.13.2). Polls the local
// marketplace checkout — the path Claude Code reads at session startup —
// until the plugin.json there matches the target version, OR until the
// deadline. On success, re-invokes release.sh to refresh installed_plugins.json
// (the **cache-refresh ordering bug fix**: poll first, then refresh, so the
// refresh check runs only when version-match is already true).
//
// Exit codes (mirrored by the cmd-layer wrapper):
//
//	0 — marketplace converged + release.sh refresh succeeded
//	1 — timeout: polled until MaxWait without matching version
//	2 — runtime error (missing dir, malformed plugin.json, release.sh failed)
//
// Testing seams: callers may inject Now/Sleep/Pull/ReleaseSh to bypass real
// time, real git operations, and real shell invocations. Defaults preserve
// the bash script's external behavior.
```

### `go/internal/marketplacepoll/marketplacepoll.go:122` — above `func DefaultReleaseSh(repoRoot, target string) error {`

```text
// DefaultReleaseSh is a Go-native no-op. It formerly shelled out to
// legacy/scripts/utility/release.sh for a cache-refresh side-effect (marketplace
// pull + installed_plugins.json registry update). That script was removed in the
// v12 script→Go migration; the cache-refresh side-effect is obsolete and release
// consistency is covered by go/internal/releaseconsistency. The injectable
// Options.ReleaseSh seam remains for callers that want a real check (ADR-0062/
// T1.7 — no bash shell-out remains; a stray legacy release.sh is NOT executed).
```

### `go/internal/marketplacepoll/marketplacepoll_test.go:427` — above `func TestDefaultReleaseSh_AlwaysNil(t *testing.T) {`

```text
// TestDefaultReleaseSh_AlwaysNil: DefaultReleaseSh is a Go-native no-op in v12+
// (ADR-0062/T1.7) — it returns nil regardless of repo contents and never shells
// out. The former Script{Absent,ExitsNonZero,ExitsOK} tests asserted the deleted
// bash path and were removed; the no-shell-out guarantee is additionally proven
// by TestDefaultReleaseSh_IgnoresLegacyScript (releasesh_noexec_test.go).
```
