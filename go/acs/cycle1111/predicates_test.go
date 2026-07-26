//go:build acs

// Package cycle1111 materialises the cycle-1111 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	tdd-file-scope-binding-check → extend tddScopeGate with a second,
//	independent, ADVISORY file-scope overlap check.
//
// The gap. tddScopeGate today compares only PROSE labels: test-report.md's
// "## Task: <slug>" against triage-report.md's "## top_n" slugs. Both drift
// cases are advisory (cycles 916 + 1012 false rejections), and the gate's own
// comments (gate.go:73-75, 132-135) name the missing complement as "the queued
// construction-level check" — the committed item's DECLARED file scope
// (scout-report.md's "**targetFiles:**" line) vs what TDD actually authored.
// A deliverable can carry exactly the right label and touch nothing the
// committed item names; today that passes silently.
//
// Predicate strategy — every predicate drives the SHIPPING code path
// (topngate.NewReviewer(...).Review, the same constructor cmd_cycle.go wires)
// against a synthetic workspace, and asserts on the reviewer's real outputs:
// the ReviewResult and the structured logf seam it writes to os.Stderr. No
// predicate greps production source, so adding a magic string cannot green
// them (the cycle-85 degenerate-predicate ban).
//
//   - 001 is the crux: right label, disjoint file scope → the advisory MUST be
//     emitted and the review MUST still approve at enforce.
//   - 002 is the anti-false-positive: the normal shape (Scout names the
//     production file, TDD authors the sibling _test.go) must stay SILENT —
//     an advisory that fires every healthy cycle is noise, not a signal.
//   - 003 is the fail-open case: no scout-report.md → no declared scope → no
//     new signal, per this gate family's ambiguity-favours-pass convention.
//   - 004 is the anti-overcorrection guard: the one FATAL case (authoring
//     under an empty ## top_n) must still abort at enforce.
package cycle1111

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
)

// fileScopeMarker is the greppable token the new advisory must carry so
// operators can find it in the reviewer's structured logf seam.
const fileScopeMarker = "file scope"

// writeWorkspace lays down the three phase deliverables tddScopeGate reads.
// An empty topN/targetFiles/testFiles slice omits the corresponding entries; a
// nil scoutSlug omits scout-report.md entirely.
func writeWorkspace(t *testing.T, topN []string, scoutSlug string, targetFiles []string, claimed string, testFiles []string) string {
	t.Helper()
	ws := t.TempDir()

	var triage strings.Builder
	triage.WriteString("# Triage Decision — Cycle 1111\n\n## top_n (commit to THIS cycle)\n")
	for _, s := range topN {
		triage.WriteString("- " + s + ": placeholder — priority=H, evidence=x, source=scout\n")
	}
	triage.WriteString("\n## deferred (carry to NEXT cycle's carryoverTodos)\n(none)\n")
	writeFile(t, ws, "triage-report.md", triage.String())

	if scoutSlug != "" {
		var scout strings.Builder
		scout.WriteString("# Scout Report — Cycle 1111\n\n## Selected Tasks\n\n### Task 1: " + scoutSlug + "\n\nPlaceholder narrative.\n\n")
		if len(targetFiles) > 0 {
			scout.WriteString("- **targetFiles:** ")
			for i, f := range targetFiles {
				if i > 0 {
					scout.WriteString(", ")
				}
				scout.WriteString("`" + f + "` (prose annotation)")
			}
			scout.WriteString("\n")
		}
		scout.WriteString("- **complexity:** M\n\n## Acceptance Criteria Summary\n\n- placeholder\n")
		writeFile(t, ws, "scout-report.md", scout.String())
	}

	var tdd strings.Builder
	tdd.WriteString("# TDD Report — Cycle 1111\n\n## Task: " + claimed + "\n\n## RED Run Output\n\n```\nFAIL\n```\n\n## Handoff to Builder\n\n```json\n{\"testFiles\": [")
	for i, f := range testFiles {
		if i > 0 {
			tdd.WriteString(", ")
		}
		tdd.WriteString("\"" + f + "\"")
	}
	tdd.WriteString("], \"redRunConfirmed\": true}\n```\n")
	writeFile(t, ws, "test-report.md", tdd.String())

	return ws
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// reviewTDD runs the SHIPPING reviewer over the workspace's tdd deliverable at
// the strictest stage and returns both observable outputs: whether the cycle
// was approved, and everything the reviewer wrote to its structured logf seam.
func reviewTDD(t *testing.T, workspace string) (approve bool, reason, logged string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	res := topngate.NewReviewer(config.StageEnforce).Review(
		context.Background(),
		core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: workspace},
	)

	os.Stderr = orig
	_ = w.Close()
	logged = <-done
	_ = r.Close()
	return res.Approve, res.Reason, logged
}

// TestC1111_001_FileScopeDriftEmitsAdvisoryWithoutBlocking is the crux: the
// claimed label matches the committed top_n exactly (so every pre-existing
// check passes silently) while the authored file lives in a tree the committed
// item never declares. That is the wrong-work shape the slug comparison
// provably cannot see.
func TestC1111_001_FileScopeDriftEmitsAdvisoryWithoutBlocking(t *testing.T) {
	ws := writeWorkspace(t,
		[]string{"tdd-file-scope-binding-check"},
		"tdd-file-scope-binding-check",
		[]string{"go/internal/topngate/gate.go", "go/internal/topngate/gate_test.go"},
		"tdd-file-scope-binding-check",
		[]string{"go/internal/tokenresolver/resolver_test.go"},
	)
	approve, reason, logged := reviewTDD(t, ws)
	if !approve {
		t.Fatalf("the file-scope signal is ADVISORY: it must never abort a cycle, even at enforce; got Approve=false reason=%q", reason)
	}
	if !strings.Contains(logged, fileScopeMarker) {
		t.Fatalf("declared scope {go/internal/topngate/...} vs authored {go/internal/tokenresolver/...} is zero-overlap and must emit a %q advisory through the reviewer's logf seam; seam output was %q", fileScopeMarker, logged)
	}
	if !strings.Contains(logged, "go/internal/tokenresolver/resolver_test.go") {
		t.Errorf("the advisory must name the AUTHORED file(s) so an operator can act on it; seam output was %q", logged)
	}
	if !strings.Contains(logged, "go/internal/topngate/gate.go") {
		t.Errorf("the advisory must name the committed item's DECLARED targetFiles; seam output was %q", logged)
	}
}

// TestC1111_002_MatchingScopeStaysSilent is the anti-false-positive predicate.
// The healthy shape is Scout naming the PRODUCTION file and TDD authoring the
// sibling _test.go — same directory, same scope. An advisory that fires here
// would fire on essentially every cycle and be worthless as a signal. This
// predicate fails a blanket "always warn" implementation of 001.
func TestC1111_002_MatchingScopeStaysSilent(t *testing.T) {
	ws := writeWorkspace(t,
		[]string{"committed-slug"},
		"committed-slug",
		[]string{"go/internal/topngate/gate.go"},
		"committed-slug",
		[]string{"go/internal/topngate/gate_test.go"},
	)
	approve, reason, logged := reviewTDD(t, ws)
	if !approve {
		t.Fatalf("an in-scope, in-lane deliverable must approve; got reason=%q", reason)
	}
	if strings.Contains(logged, fileScopeMarker) {
		t.Fatalf("a sibling file inside a declared target's directory IS in scope and must stay silent; seam wrongly emitted %q", logged)
	}
}

// TestC1111_003_MissingScoutReportFailsOpen pins the ambiguity convention: no
// scout-report.md means no declared scope, which is missing evidence rather
// than evidence of a violation. It must produce no new signal at all.
func TestC1111_003_MissingScoutReportFailsOpen(t *testing.T) {
	ws := writeWorkspace(t,
		[]string{"committed-slug"},
		"", // no scout-report.md
		nil,
		"committed-slug",
		[]string{"go/internal/tokenresolver/resolver_test.go"},
	)
	approve, reason, logged := reviewTDD(t, ws)
	if !approve {
		t.Fatalf("a missing scout-report.md must fail open; got Approve=false reason=%q", reason)
	}
	if strings.Contains(logged, fileScopeMarker) {
		t.Fatalf("with no declared scope there is nothing to compare — the check must stay quiet; seam emitted %q", logged)
	}
}

// TestC1111_004_EmptyTopNStillBlocks is the anti-overcorrection guard: the one
// unambiguous FATAL case (triage committed nothing, TDD authored anyway) must
// survive the new advisory path untouched.
func TestC1111_004_EmptyTopNStillBlocks(t *testing.T) {
	ws := writeWorkspace(t,
		nil, // triage committed nothing
		"orphan-task",
		[]string{"go/internal/topngate/gate.go"},
		"orphan-task",
		[]string{"go/internal/topngate/gate_test.go"},
	)
	approve, reason, _ := reviewTDD(t, ws)
	if approve {
		t.Fatalf("authoring under an EMPTY ## top_n must stay a hard block at enforce; got Approve=true")
	}
	if !strings.Contains(reason, "EMPTY") || !strings.Contains(reason, "orphan-task") {
		t.Errorf("the fatal reason must name the empty commitment and the claimed slug; got %q", reason)
	}
}
