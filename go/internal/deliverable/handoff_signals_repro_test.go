package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const amplificationReportWithSignals = "## Generated Tests\n- `go/internal/x/x_test.go`: TestEdge\n\n## Results\nok\n\n" +
	`<!-- evolve-signals: {"amplify.tests_added":3,"amplify.failures_found":0} -->` + "\n" +
	`<!-- evolve-verdict: {"phase":"test-amplification","verdict":"PASS","schema_version":1} -->` + "\n"

func TestNonSpinePhaseDeclaredSignalsReachTheRoutingDigest(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cat, _, _, err := phasespec.MergedCatalog(projectRoot)
	if err != nil {
		t.Fatalf("load the production phase catalog: %v", err)
	}
	spec, ok := cat.Get("test-amplification")
	if !ok || len(spec.Outputs.Signals) == 0 {
		t.Fatalf("precondition: test-amplification declares outputs.signals, got %+v (found=%v)", spec.Outputs.Signals, ok)
	}
	ws := t.TempDir()
	writeFile(t, ws, "test-amplification-report.md", amplificationReportWithSignals)
	in := core.ReviewInput{Phase: "test-amplification", Cycle: 7, Workspace: ws, ProjectRoot: t.TempDir()}
	if err := NewHostEffects(cat, recordingClaimer(&[]claimCall{})).Perform(context.Background(), in); err != nil {
		t.Fatalf("Perform: %v", err)
	}
	sig, err := router.Digest(ws, []string{"scout", "test-amplification"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	for _, field := range spec.Outputs.Signals {
		if _, ok := sig.GenericValue(field); !ok {
			t.Errorf("declared signal %q is absent from the routing digest after the phase completed; digest generic=%v degraded=%v", field, sig.Generic, sig.DigestDegraded)
		}
	}
	if v, _ := sig.GenericValue("amplify.tests_added"); v != float64(3) {
		t.Errorf("amplify.tests_added = %v, want 3 from the report", v)
	}
}

func TestDeclaredSignals_KeepsOnlyDeclaredKeysAndRejectsBadSentinels(t *testing.T) {
	for _, tc := range []struct {
		name   string
		report string
		want   map[string]any
	}{
		{"no sentinel", "plain report\n", nil},
		{"malformed json", "<!-- evolve-signals: {not json} -->\n", nil},
		{"undeclared key dropped", `<!-- evolve-signals: {"a":1,"b":2} -->` + "\n", map[string]any{"a": float64(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := declaredSignals([]byte(tc.report), []string{"a"}); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("declaredSignals = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWriteHandoffSignals_WritesNothingWhenThereIsNothingToWrite(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cat, _, _, err := phasespec.MergedCatalog(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	handoff := func(ws string) bool {
		_, err := os.Stat(filepath.Join(ws, "handoff-test-amplification.json"))
		return err == nil
	}
	for _, tc := range []struct {
		name   string
		report string
	}{
		{"missing report", ""},
		{"report without sentinel", "ok\n"},
		{"sentinel with only undeclared keys", `<!-- evolve-signals: {"zzz":1} -->` + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			if tc.report != "" {
				writeFile(t, ws, "test-amplification-report.md", tc.report)
			}
			in := core.ReviewInput{Phase: "test-amplification", Cycle: 7, Workspace: ws, ProjectRoot: t.TempDir()}
			if err := NewHostEffects(cat, recordingClaimer(&[]claimCall{})).Perform(context.Background(), in); err != nil {
				t.Fatalf("Perform: %v", err)
			}
			if handoff(ws) {
				t.Fatal("a handoff file was written")
			}
		})
	}
	t.Run("nil spec lookup", func(t *testing.T) {
		h := &HostEffects{}
		if err := h.writeHandoffSignals(phasecontract.Contract{Phase: "test-amplification"}, phasecontract.Roots{Workspace: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unknown phase", func(t *testing.T) {
		h := &HostEffects{spec: cat.Get}
		if err := h.writeHandoffSignals(phasecontract.Contract{Phase: "no-such-phase"}, phasecontract.Roots{Workspace: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestPerform_ReportsAHandoffSignalsWriteFailure(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cat, _, _, err := phasespec.MergedCatalog(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	writeFile(t, ws, "test-amplification-report.md", amplificationReportWithSignals)
	if err := os.Chmod(ws, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ws, 0o700) })
	in := core.ReviewInput{Phase: "test-amplification", Cycle: 7, Workspace: ws, ProjectRoot: t.TempDir()}
	err = NewHostEffects(cat, recordingClaimer(&[]claimCall{})).Perform(context.Background(), in)
	if err == nil || !strings.Contains(err.Error(), "handoff signals") {
		t.Fatalf("Perform error = %v, want a handoff signals failure", err)
	}
}
