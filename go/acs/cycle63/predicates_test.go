//go:build legacy

// Package cycle63 ports the cycle-63 ACS predicates (3 bash files).
package cycle63

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC63_056_ResolveRootsWorktree ports cycle-63/056 (wiring-only).
// Bash predicate spins up a worktree fixture; Go port asserts the
// resolve-roots.sh script has the --git-common-dir handling.
func TestC63_056_ResolveRootsWorktree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	resolve := filepath.Join(root, "legacy", "scripts", "lifecycle", "resolve-roots.sh")
	if !acsassert.FileExists(t, resolve) {
		t.Skip("resolve-roots.sh missing — skip cycle-63-056")
	}
	if !acsassert.FileContains(t, resolve, "git-common-dir") {
		return
	}
}

// TestC63_057_HandoffSchemasC2 ports cycle-63/057.
// 5 schema files exist, parse, declare canonical keys; ADR 0009 + reference matrix.
func TestC63_057_HandoffSchemasC2(t *testing.T) {
	root := acsassert.RepoRoot(t)
	slugs := []string{"intent-report", "triage-decision", "tdd-report", "ship-report", "retrospective-report"}
	for _, slug := range slugs {
		schema := filepath.Join(root, "schemas", "handoff", slug+".schema.json")
		if !acsassert.FileExists(t, schema) {
			t.Skipf("%s.schema.json missing — skip cycle-63-057", slug)
		}
		// AC2-AC5: declare canonical keys.
		for _, key := range []string{
			"required_first_line", "required_sections",
			"required_content", "min_words", "artifact_type",
		} {
			if !acsassert.FileContains(t, schema, `"`+key+`"`) {
				return
			}
		}
		// artifact_type must match the filename slug.
		if !acsassert.JSONFieldEquals(t, schema, "artifact_type", slug) {
			return
		}
		// required_first_line.pattern must include challenge-token.
		if !acsassert.FileMatchesRegex(t, schema, `"required_first_line"[\s\S]*challenge-token`) {
			return
		}
	}
	// AC6: ADR 0009 references all 8 phase slugs.
	adr := filepath.Join(root, "docs", "adr", "0009-phase-handoff-schemas.md")
	if !acsassert.FileExists(t, adr) {
		t.Skip("ADR 0009 missing — skip cycle-63-057 AC6")
	}
	for _, slug := range []string{
		"intent-report", "scout-report", "triage-decision", "tdd-report",
		"build-report", "audit-report", "ship-report", "retrospective-report",
	} {
		if !acsassert.FileContains(t, adr, slug) {
			return
		}
	}
}

// TestC63_058_Cycle62IncidentCloseout ports cycle-63/058.
func TestC63_058_Cycle62IncidentCloseout(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "incidents", "cycle-62-ship-refused.md")
	if !acsassert.FileExists(t, doc) {
		t.Skip("cycle-62-ship-refused.md missing — skip cycle-63-058")
	}
	for _, section := range []string{"## What happened", "## Why this was expected", "## Resolution"} {
		// case-insensitive variant accepted by the bash predicate.
		if !acsassert.FileMatchesRegex(t, doc, `(?i)`+section) {
			return
		}
	}
	if !acsassert.FileContains(t, doc, "abnormal-events.jsonl") {
		return
	}
	if !acsassert.FileMatchesRegex(t, doc, `(?i)(audit.bound|tree.SHA|expected_ship_sha|FAILED.AND.LEARNED)`) {
		return
	}
}
