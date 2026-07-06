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

// NewUngraduatedPackages returns the changed package patterns that live under
// ./internal/ but are NOT yet listed in the go/.apicover-enforce file — exactly
// the set IntersectEnforced silently drops (a package new this cycle cannot yet
// be in the enforce list, so the touched∩enforced intersection excludes it and
// the unnamed-export gate never inspects it — the recurring
// warnship_apicover_ci_gap blind spot).
//
// Scope mirrors apicover's own: only ./internal/ packages are apicover-scoped,
// so ./cmd/... entrypoints are never flagged. Normalizes the same way
// IntersectEnforced does (strips a trailing "/..."), dedupes, and returns a
// sorted slice (nil when nothing is ungraduated). Pure; no I/O.
func NewUngraduatedPackages(changed []string, enforceBytes []byte) []string {
	if len(changed) == 0 {
		return nil
	}
	enforced := make(map[string]bool)
	sc := bufio.NewScanner(bytes.NewReader(enforceBytes))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		enforced[line] = true
	}

	seen := make(map[string]bool)
	var out []string
	for _, c := range changed {
		c = normalizePattern(c)
		if c == "" || !strings.HasPrefix(c, "./internal/") {
			continue // out of apicover's scope (e.g. ./cmd/...)
		}
		if enforced[c] || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// CoverageTags is the SSOT build-tag set covering BOTH tiers the repo gates real
// coverage behind: //go:build integration (real-FS/git/tmux) AND //go:build acs
// (in-package acs suites in internal/core, internal/acssuite,
// internal/phases/audit, internal/evalgate). Every scoped coverage-measuring
// call site MUST build its `go test` args through CoverageTestArgs so it reads
// the SAME (tagged) coverage number CI does. An untagged run under-reports a
// tag-gated package's real coverage by up to 43 points (R1:
// knowledge-base/research/test-coverage-audit-2026-07.md — internal/phases/ship
// measured 47.0% plain vs 90.6% tagged), which would let a gate ship on a wrong
// number; measuring the integration tier alone still under-counts the acs-gated
// in-package suites. This is the same defect class ADR-0069 fixed once for
// vet/apicover, now for the coverage dimension.
const CoverageTags = "integration acs"

// CoverageTestArgs returns tag-parity-correct `go test` args for a scoped
// coverage-profile run: ["test", "-tags", CoverageTags, "-coverprofile="+
// coverProfile, pkgs...]. The package list is preserved verbatim and in order
// as trailing args so `go test` measures exactly the scoped set (dropping it
// would silently fall back to the whole module). Pure; no I/O.
func CoverageTestArgs(coverProfile string, pkgs []string) []string {
	args := []string{"test", "-tags", CoverageTags, "-coverprofile=" + coverProfile}
	return append(args, pkgs...)
}
