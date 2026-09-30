//go:build acs

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

const fileScopeMarker = "file scope"

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

func TestC1111_003_MissingScoutReportFailsOpen(t *testing.T) {
	ws := writeWorkspace(t,
		[]string{"committed-slug"},
		"",
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

func TestC1111_004_EmptyTopNStillBlocks(t *testing.T) {
	ws := writeWorkspace(t,
		nil,
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
