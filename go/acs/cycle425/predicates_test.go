//go:build acs

package cycle425

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	adapterbridge "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func readTestdataFrame(t *testing.T, root, relPath string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "go", "internal", "bridge", "panestream", "testdata", relPath))
	if err != nil {
		t.Fatalf("readTestdataFrame(%q): %v", relPath, err)
	}
	return string(b)
}

func TestC425_001_BootTimeoutStoreWiredInProduction(t *testing.T) {
	root := acsassert.SetupTempProject(t)
	adapter := adapterbridge.NewDefault(root, nil)
	if !adapter.BootTimeoutStoreWired() {
		t.Errorf("adapters.NewDefault(%q).BootTimeoutStoreWired() = false; "+
			"production Adapter must set a non-nil BootTimeoutStore so engine.go:455 can record exit-80 strikes", root)
	}
}

func TestC425_002_SingleStrikeDoesNotBench(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "claude-tmux"

	benched, err := store.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike: unexpected error: %v", err)
	}
	if benched {
		t.Errorf("RecordBootStrike (1 of %d): benched=true — a single transient boot failure must NOT bench the driver",
			clihealth.DefaultBootBenchThreshold)
	}
	if active := store.Active(); len(active) != 0 {
		t.Errorf("Active() = %v after single strike, want empty — bench must not be recorded until threshold", active)
	}
}

func TestC425_003_WrongExitCodeNotBootTimeout(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{80, true},
		{81, false},
		{85, false},
		{0, false},
		{1, false},
	}
	for _, c := range cases {
		got := clihealth.IsBootTimeoutExitCode(c.code)
		if got != c.want {
			t.Errorf("IsBootTimeoutExitCode(%d) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestC425_004_AgyDetectorGeneratingFrameConverging(t *testing.T) {
	root := acsassert.RepoRoot(t)
	p := panestream.Profiles["agy"]
	det := panestream.NewAgyDetector(3)

	thinkingFrame := readTestdataFrame(t, root, "agy/thinking.txt")
	det.Assess(thinkingFrame, p)

	state, conf := det.Assess(thinkingFrame, p)
	if state != panestream.LivenessConverging {
		t.Errorf("AgyDetector on ⣯ Generating... frame: got %v, want LivenessConverging "+
			"(spinner affordance proves model is live; must not classify BusyButStagnant)", state)
	}
	if conf < 0.9 {
		t.Errorf("AgyDetector on generating frame: conf = %.2f, want ≥ 0.9 "+
			"(high-confidence signal from explicit spinner affordance)", conf)
	}
}

func TestC425_005_AgyDetectorAnswerFrameDefaultParity(t *testing.T) {
	root := acsassert.RepoRoot(t)
	p := panestream.Profiles["agy"]
	base := panestream.NewDefaultDetector(3)
	det := panestream.NewAgyDetector(3)

	answerFrame := readTestdataFrame(t, root, "agy/answer.txt")
	for range 3 {
		base.Assess(answerFrame, p)
		det.Assess(answerFrame, p)
	}
	baseState, baseConf := base.Assess(answerFrame, p)
	agyState, agyConf := det.Assess(answerFrame, p)

	if agyState != baseState {
		t.Errorf("AgyDetector (answer frame, no ⣯ Generating...): state %v ≠ DefaultDetector %v — "+
			"fallback must be byte-identical when spinner is absent", agyState, baseState)
	}
	if agyConf != baseConf {
		t.Errorf("AgyDetector (answer frame, no ⣯ Generating...): conf %.2f ≠ DefaultDetector %.2f — "+
			"fallback must be byte-identical when spinner is absent", agyConf, baseConf)
	}
}

func TestC425_006_DetectorForAgyRoutesToAgyDetector(t *testing.T) {
	root := acsassert.RepoRoot(t)
	p := panestream.Profiles["agy"]
	probe := panestream.DetectorFor(p)
	if probe == nil {
		t.Fatal("DetectorFor(agy) = nil")
	}

	if _, ok := probe.(*panestream.AgyDetector); !ok {
		t.Errorf("DetectorFor(agy) = %T, want *panestream.AgyDetector — "+
			"agy must be registered in DetectorFor, not fall through to default", probe)
	}

	base := panestream.NewDefaultDetector(0)
	thinkingFrame := readTestdataFrame(t, root, "agy/thinking.txt")
	probe.Assess(thinkingFrame, p)
	base.Assess(thinkingFrame, p)
	_, probeConf := probe.Assess(thinkingFrame, p)
	_, baseConf := base.Assess(thinkingFrame, p)
	if probeConf <= baseConf {
		t.Errorf("DetectorFor(agy) conf %.2f not > DefaultDetector %.2f on generating frame; "+
			"registry must route to AgyDetector (spinner layer must uplift confidence)", probeConf, baseConf)
	}
}

func TestC425_007_NoCliNameInStopReview(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	for _, cli := range []string{"claude", "codex", "agy", "ollama"} {
		acsassert.FileNotContains(t, path, `"`+cli+`"`)
	}
}
