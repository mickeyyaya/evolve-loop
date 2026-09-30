//go:build acs

package cycle424

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestC424_001_BootTimeoutPatternBenchable(t *testing.T) {
	pat := clihealth.BootTimeoutPattern
	if pat == "" {
		t.Fatal("BootTimeoutPattern must be a non-empty string constant")
	}
	if !clihealth.Benchable(pat) {
		t.Errorf("Benchable(%q) = false, want true — boot-timeout pattern must be benchable", pat)
	}
	if !clihealth.Benchable("rate_limit") {
		t.Error("Benchable(\"rate_limit\") regressed to false")
	}
}

func TestC424_002_IsBootTimeoutExitCodeMapping(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{80, true},
		{81, false},
		{85, false},
		{0, false},
		{1, false},
		{127, false},
	}
	for _, c := range cases {
		got := clihealth.IsBootTimeoutExitCode(c.code)
		if got != c.want {
			t.Errorf("IsBootTimeoutExitCode(%d) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestC424_003_RepeatedStrikeBenchesDriver(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "codex-tmux"
	thresh := clihealth.DefaultBootBenchThreshold
	if thresh < 2 {
		t.Fatalf("DefaultBootBenchThreshold must be ≥ 2, got %d", thresh)
	}

	for i := 1; i < thresh; i++ {
		benched, err := store.RecordBootStrike(driver)
		if err != nil {
			t.Fatalf("RecordBootStrike call %d: unexpected error: %v", i, err)
		}
		if benched {
			t.Errorf("RecordBootStrike call %d/%d: benched=true before threshold (want false)", i, thresh)
		}
	}

	benched, err := store.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike (threshold call): unexpected error: %v", err)
	}
	if !benched {
		t.Errorf("RecordBootStrike at threshold (%d): benched=false, want true — repeated boot-timeout must bench the driver", thresh)
	}

	active := store.Active()
	if _, ok := active[driver]; !ok {
		t.Errorf("after threshold strikes, Active() does not contain %q — bench not recorded in store", driver)
	}
}

func TestC424_004_SingleStrikeDoesNotBench(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "claude-tmux"

	benched, err := store.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike: unexpected error: %v", err)
	}
	if benched {
		t.Errorf("RecordBootStrike (1 of %d): benched=true after single strike — single transient boot failure must NOT bench", clihealth.DefaultBootBenchThreshold)
	}

	active := store.Active()
	if len(active) != 0 {
		t.Errorf("Active() = %v after single strike, want empty — bench must not be recorded until threshold", active)
	}
}

func TestC424_005_BootBenchIsDriverScoped(t *testing.T) {
	dir := t.TempDir()
	store := clihealth.NewStore(dir, func() time.Time { return time.Now() })
	driver := "codex-tmux"
	thresh := clihealth.DefaultBootBenchThreshold

	for i := 0; i < thresh; i++ {
		if _, err := store.RecordBootStrike(driver); err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i+1, err)
		}
	}

	active := store.Active()
	if _, ok := active[driver]; !ok {
		t.Fatalf("expected %q in Active() after %d strikes, got %v", driver, thresh, active)
	}

	if _, ok := active["codex"]; ok {
		t.Errorf("Active()[\"codex\"] is set, but bench was for %q — boot bench must be driver-scoped, not family-scoped", driver)
	}

	if _, ok := active["claude-tmux"]; ok {
		t.Errorf("Active()[\"claude-tmux\"] is set — bench for %q must not propagate to other drivers", driver)
	}
}

func TestC424_006_OllamaDetectorThinkingConverging(t *testing.T) {
	p := panestream.Profiles["ollama"]
	base := panestream.NewDefaultDetector(3)
	det := panestream.NewOllamaDetector(3)

	thinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\nThinking...\n"

	base.Assess(thinkingFrame, p)
	det.Assess(thinkingFrame, p)

	baseState, baseConf := base.Assess(thinkingFrame, p)
	ollamaState, ollamaConf := det.Assess(thinkingFrame, p)

	if ollamaState != panestream.LivenessConverging {
		t.Errorf("OllamaDetector on Thinking... frame: got %v, want LivenessConverging", ollamaState)
	}

	if ollamaConf <= baseConf {
		t.Errorf("OllamaDetector confidence %v not > DefaultDetector %v (state=%v) on Thinking... frame; uplift required",
			ollamaConf, baseConf, baseState)
	}
}

func TestC424_007_OllamaDetectorNoThinkingFallsBack(t *testing.T) {
	p := panestream.Profiles["ollama"]
	base := panestream.NewDefaultDetector(3)
	det := panestream.NewOllamaDetector(3)

	noThinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\n*   tmux is a terminal multiplexer.\n*   It keeps sessions alive.\n>>> Send a message (/? for help)\n"

	for range 3 {
		base.Assess(noThinkingFrame, p)
		det.Assess(noThinkingFrame, p)
	}
	baseState, baseConf := base.Assess(noThinkingFrame, p)
	ollamaState, ollamaConf := det.Assess(noThinkingFrame, p)

	if ollamaState != baseState {
		t.Errorf("OllamaDetector (no Thinking...): state %v ≠ DefaultDetector %v — fallback must be byte-identical", ollamaState, baseState)
	}
	if ollamaConf != baseConf {
		t.Errorf("OllamaDetector (no Thinking...): conf %v ≠ DefaultDetector %v — fallback must be byte-identical", ollamaConf, baseConf)
	}
}

func TestC424_008_OllamaDetectorNoPanicEdgeCases(t *testing.T) {
	p := panestream.Profiles["ollama"]
	edgeCases := []struct {
		name  string
		frame string
	}{
		{"empty", ""},
		{"whitespace-only", "   \n  \n"},
		{"partial-thinking-header", "Thinking\n"},
		{"thinking-mid-word", "DeepThinking...\n"},
		{"pure-chrome", "⠋⠙⠹ processing...\n"},
		{"idle-placeholder-only", ">>> Send a message (/? for help)\n"},
	}
	validStates := map[panestream.LivenessState]bool{
		panestream.LivenessConverging:      true,
		panestream.LivenessBusyButStagnant: true,
		panestream.LivenessIdle:            true,
		panestream.LivenessHung:            true,
	}
	for _, tc := range edgeCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("OllamaDetector panicked on edge case %q: %v", tc.name, r)
				}
			}()
			det := panestream.NewOllamaDetector(3)
			det.Assess(tc.frame, p)
			state, conf := det.Assess(tc.frame, p)
			if !validStates[state] {
				t.Errorf("edge case %q: invalid state %v (not in {Converging,Busy,Idle,Hung})", tc.name, state)
			}
			if conf < 0 || conf > 1.0 {
				t.Errorf("edge case %q: confidence %v out of [0,1]", tc.name, conf)
			}
		})
	}
}

func TestC424_009_DetectorForOllamaRoutesOllama(t *testing.T) {
	p := panestream.Profiles["ollama"]
	probe := panestream.DetectorFor(p)
	if probe == nil {
		t.Fatal("DetectorFor(ollama) = nil")
	}
	base := panestream.NewDefaultDetector(0)

	thinkingFrame := "user@host /tmp % ollama run gemma4:latest\n>>> what is tmux?\nThinking...\n"

	probe.Assess(thinkingFrame, p)
	base.Assess(thinkingFrame, p)

	_, baseConf := base.Assess(thinkingFrame, p)
	_, probeConf := probe.Assess(thinkingFrame, p)

	if probeConf <= baseConf {
		t.Errorf("DetectorFor(ollama) confidence %v not > DefaultDetector %v on Thinking... frame; registry must route to OllamaDetector", probeConf, baseConf)
	}
}

func TestC424_010_NoCliNameInStopReview(t *testing.T) { // acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	for _, cli := range []string{"claude", "codex", "agy", "ollama"} {
		acsassert.FileNotContains(t, path, `"`+cli+`"`)
	}
}
