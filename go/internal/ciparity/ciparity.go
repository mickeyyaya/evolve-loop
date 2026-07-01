// Package ciparity holds the pure helpers for the cycle audit's "CI-parity"
// gate — the deterministic checks that run the EXACT whole-repo CI command set
// (go vet ./..., -tags acs acs-durable, apicover -enforce) against a cycle's
// worktree BEFORE ship, so a cycle can never ship green-locally / red-in-CI.
//
// This package is a LEAF: pure functions only, no subprocess, no I/O — the
// audit phase (internal/phases/audit/ciparity.go) supplies the shell-out hooks
// and calls these to decide scope and extract offenders. Keeping the logic here
// makes it unit-testable without forking the go toolchain.
package ciparity

import (
	"bufio"
	"bytes"
	"sort"
	"strings"
)

// IntersectEnforced returns the enforced apicover package patterns that a cycle
// actually touched — the intersection of the changed-package set with the
// go/.apicover-enforce list. Only these need a scoped `apicover -enforce` run
// (running it over the whole enforce list every cycle would be O(repo), not
// O(change)).
//
//   - changed: package patterns from changedpkgs.ChangedPackages, of the form
//     "./internal/foo/..." (trailing "/..." wildcard).
//   - enforceBytes: the raw go/.apicover-enforce file — one "./internal/foo"
//     pattern per line, plus "#" comments and blank lines.
//
// The trailing "/..." on changed entries is stripped before matching, since the
// enforce list carries the bare package pattern ("./internal/foo"). The result
// is in enforce-list form (feed straight to `go list`), deduped and sorted for
// determinism. A nil/empty changed set (best-effort locator returned nothing)
// yields nil — the caller then skips the scoped apicover run, fail-open.
func IntersectEnforced(changed []string, enforceBytes []byte) []string {
	if len(changed) == 0 {
		return nil
	}
	changedSet := make(map[string]bool, len(changed))
	for _, c := range changed {
		if c = normalizePattern(c); c != "" {
			changedSet[c] = true
		}
	}

	seen := make(map[string]bool)
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(enforceBytes))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if changedSet[line] && !seen[line] {
			seen[line] = true
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

// normalizePattern strips whitespace and a single trailing "/..." so a changed
// entry ("./internal/foo/...") matches its enforce-list form ("./internal/foo").
func normalizePattern(p string) string {
	return strings.TrimSuffix(strings.TrimSpace(p), "/...")
}
