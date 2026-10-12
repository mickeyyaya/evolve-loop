package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAutoResponderBackfillF1_BadTransientRegexWarnsAndDisablesDwell(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"cli":"f1-badregex","binary":"f1bin","transient_regex":"(unclosed"}`
	if err := os.WriteFile(filepath.Join(dir, "f1-badregex.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	useBridgeManifestDir(t, dir)
	var stderr bytes.Buffer
	ar := newAutoResponder("f1-badregex", t.TempDir(), Deps{Stderr: &stderr}, false, 0)
	if ar.transientPattern != nil {
		t.Fatalf("an invalid pattern compiled to %v", ar.transientPattern)
	}
	if ar.transientRegex != "(unclosed" {
		t.Fatalf("the raw pattern must still be recorded, got %q", ar.transientRegex)
	}
	if got := stderr.String(); !strings.Contains(got, "[f1-badregex] WARN: invalid manifest transient_regex:") {
		t.Fatalf("stderr = %q, want the invalid-regex warning", got)
	}
}

func TestNewAutoResponderBackfillF1_BadTransientRegexSilentWithoutStderr(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f1-badregex.json"), []byte(`{"cli":"f1-badregex","binary":"f1bin","transient_regex":"(unclosed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	useBridgeManifestDir(t, dir)
	ar := newAutoResponder("f1-badregex", t.TempDir(), Deps{}, false, 0)
	if ar.transientPattern != nil || ar.transientGate == nil || ar.exhaustGate == nil {
		t.Fatalf("construction must degrade to a disabled dwell with live gates: %+v", ar)
	}
}

func TestNewAutoResponderBackfillF1_AnUnknownCLIYieldsAnInertResponder(t *testing.T) {
	t.Parallel()
	ar := newAutoResponder("f1-no-such-cli", t.TempDir(), Deps{}, true, 77)
	if len(ar.prompts) != 0 || ar.transientRegex != "" || ar.exhaustedRegex != "" || ar.transientPattern != nil {
		t.Fatalf("no manifest means no rules and no patterns: %+v", ar)
	}
	if !ar.human || ar.scrollback != 77 || ar.counts == nil || ar.suppressLogged == nil || ar.shadowFired == nil {
		t.Fatalf("construction arguments and maps not wired: %+v", ar)
	}
}

const transientPaneF1 = "API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment.\n❯"

func TestAutoResponderTickPaneBackfillF1_DirectConstructionNeverPanicsOnMissingGates(t *testing.T) {
	t.Parallel()
	deps := Deps{Stderr: &bytes.Buffer{}}.withDefaults()
	ar := newAutoResponder("claude-tmux", t.TempDir(), deps, false, 0)
	if ar.transientPattern == nil || ar.exhaustedRegex == "" {
		t.Fatal("claude-tmux must declare both a transient and an exhaustion pattern for this fixture")
	}
	ar.transientDwellEnabled = true
	ar.transientGate = nil
	ar.exhaustGate = nil

	action, rc := ar.tickPane(context.Background(), "f1-session", transientPaneF1, true)

	if ar.transientGate == nil || ar.transientGate.threshold != transientDwellObservations {
		t.Fatalf("a nil transient gate must be rebuilt with the dwell threshold: %+v", ar.transientGate)
	}
	if ar.exhaustGate == nil {
		t.Fatal("a nil exhaustion gate must be rebuilt")
	}
	if rc == 3 || action == "transient_dwell" {
		t.Fatalf("a single observation must not satisfy the %d-observation dwell: (%q, %d)", transientDwellObservations, action, rc)
	}
}
