//go:build acs

package cycle336

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/aggregator"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const aggregatorSrcRel = "go/internal/aggregator/aggregator.go"

const aggregatorBaselineLines = 466

func fixedNow() func() time.Time {
	return func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
}

func writeWorker(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write worker %s: %v", name, err)
	}
	return p
}

func runAggregate(t *testing.T, phase string, workers ...string) (int, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "agg.md")
	rc := aggregator.Aggregate(aggregator.Inputs{
		Phase:   phase,
		Output:  out,
		Workers: workers,
		Now:     fixedNow(),
	}, io.Discard)
	body, err := os.ReadFile(out)
	if err != nil {
		return rc, ""
	}
	return rc, string(body)
}

func TestC336_001_AggregateBehaviourPreserved(t *testing.T) {
	dir := t.TempDir()

	pass1 := writeWorker(t, dir, "p1.md", "Verdict: PASS\n# a\nok")
	pass2 := writeWorker(t, dir, "p2.md", "verdict: pass\n# b\nfine")
	fail := writeWorker(t, dir, "f.md", "Verdict: FAIL\nbroken")
	warn := writeWorker(t, dir, "w.md", "Verdict: WARN\nminor")
	noverdict := writeWorker(t, dir, "nv.md", "no verdict line, just prose")

	verdictCases := []struct {
		name     string
		workers  []string
		wantRC   int
		wantHead string
	}{
		{"all-pass", []string{pass1, pass2}, aggregator.ExitOK, "Verdict: PASS"},
		{"any-fail-vetoes", []string{pass1, fail}, aggregator.ExitVerdictBad, "Verdict: FAIL"},
		{"pass-plus-warn", []string{pass1, warn}, aggregator.ExitOK, "Verdict: WARN"},
		{"missing-verdict-is-warn", []string{noverdict}, aggregator.ExitOK, "Verdict: WARN"},
	}
	for _, tc := range verdictCases {
		rc, body := runAggregate(t, "audit", tc.workers...)
		if rc != tc.wantRC {
			t.Errorf("audit %s: rc=%d, want %d", tc.name, rc, tc.wantRC)
		}
		if !strings.HasPrefix(body, tc.wantHead) {
			t.Errorf("audit %s: body must start with %q, got:\n%s", tc.name, tc.wantHead, body)
		}
	}

	c1 := writeWorker(t, dir, "alpha.md", "findings-alpha")
	c2 := writeWorker(t, dir, "beta.md", "findings-beta")
	rc, body := runAggregate(t, "scout", c1, c2)
	if rc != aggregator.ExitOK {
		t.Fatalf("scout concat: rc=%d, want %d", rc, aggregator.ExitOK)
	}
	for _, want := range []string{"## Worker: alpha", "## Worker: beta", "findings-alpha", "findings-beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("scout concat: missing %q in:\n%s", want, body)
		}
	}

	v1 := writeWorker(t, dir, "claude.md", "Verdict: PASS\n")
	v2 := writeWorker(t, dir, "gemini.md", "Verdict: PASS\n")
	rc, body = runAggregate(t, "cross-cli-vote", v1, v2)
	if rc != aggregator.ExitOK {
		t.Fatalf("cross-cli-vote: rc=%d, want %d", rc, aggregator.ExitOK)
	}
	if !strings.Contains(body, "### Worker: claude") {
		t.Errorf("cross-cli-vote: missing '### Worker: claude' (heading level must stay ###) in:\n%s", body)
	}
	if strings.Contains(body, "## Worker: claude") && !strings.Contains(body, "### Worker: claude") {
		t.Errorf("cross-cli-vote: heading downgraded from ### to ##:\n%s", body)
	}
}

// acs-predicate: config-check — asserts the SOURCE-STRUCTURE outcome of the
func TestC336_002_DeadAnyWarnRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	if n := acsassert.CountOccurrencesAny(src, "anyWarn"); n != 0 {
		t.Errorf("RED: aggregator.go still references anyWarn %d time(s) — remove the dead write-only var (want 0)", n)
	}
}

// acs-predicate: config-check — asserts the dedup is real: the helper is defined
func TestC336_003_ScanFirstCaptureExtractedAndDelegated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	if !acsassert.FileContains(t, src, "func scanFirstCapture") {
		t.Errorf("RED: aggregator.go missing `func scanFirstCapture` helper")
	}
	if n := acsassert.CountOccurrencesAny(src, "scanFirstCapture"); n < 3 {
		t.Errorf("RED: scanFirstCapture appears %d× — want ≥3 (1 def + ≥2 call sites in extractVerdict/extractScore)", n)
	}
}

// acs-predicate: config-check — asserts the dedup is real: the helper is defined
func TestC336_004_AppendWorkerSectionsExtractedAndDelegated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	if !acsassert.FileContains(t, src, "func appendWorkerSections") {
		t.Errorf("RED: aggregator.go missing `func appendWorkerSections` helper")
	}
	if n := acsassert.CountOccurrencesAny(src, "appendWorkerSections"); n < 4 {
		t.Errorf("RED: appendWorkerSections appears %d× — want ≥4 (1 def + ≥3 call sites across write* funcs)", n)
	}
}

// acs-predicate: config-check — the code-reduction goal requires aggregator.go
func TestC336_005_NetLineReduction(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, aggregatorSrcRel)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("RED: cannot read %s: %v", aggregatorSrcRel, err)
	}
	lines := bytes.Count(data, []byte("\n"))
	if lines >= aggregatorBaselineLines {
		t.Errorf("RED: aggregator.go has %d lines — want < %d (net reduction not yet realized)", lines, aggregatorBaselineLines)
	}
}
