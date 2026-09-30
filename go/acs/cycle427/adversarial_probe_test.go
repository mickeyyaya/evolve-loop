//go:build acs

package cycle427

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAmplify_ParallelEvaluate_AdvisoryStage(t *testing.T) {
	got := policy.Policy{
		ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "advisory"},
	}.ParallelEvaluateConfig()
	if got.Stage != "advisory" {
		t.Errorf("Stage=advisory: known stage must persist, got %q (parseGateStage vs parseRouterStage mismatch risk)", got.Stage)
	}
	fmt.Printf("PROBE advisory-stage → Stage=%q Concurrency=%d\n", got.Stage, got.Concurrency)
}

func TestAmplify_ParallelEvaluate_CaseSensitive(t *testing.T) {
	for _, typo := range []string{"Shadow", "SHADOW", "Enforce", "ENFORCE", "Advisory"} {
		got := policy.Policy{
			ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: typo},
		}.ParallelEvaluateConfig()
		fmt.Printf("PROBE stage=%q → Stage=%q\n", typo, got.Stage)
		if got.Stage != "off" {
			t.Errorf("wrong-case stage %q must fall to off (fail-safe), got %q", typo, got.Stage)
		}
	}
}

func TestAmplify_ParallelEvaluate_ConcurrencyMinimumOne(t *testing.T) {
	got := policy.Policy{
		ParallelEvaluate: &policy.ParallelEvaluatePolicy{Concurrency: 1},
	}.ParallelEvaluateConfig()
	fmt.Printf("PROBE concurrency=1 → %d\n", got.Concurrency)
	if got.Concurrency != 1 {
		t.Errorf("concurrency=1 must pass through as 1, got %d (guard must not clamp valid positives)", got.Concurrency)
	}
}

func TestAmplify_ParallelEvaluate_WhitespaceStageFallsOff(t *testing.T) {
	got := policy.Policy{
		ParallelEvaluate: &policy.ParallelEvaluatePolicy{Stage: "  shadow  "},
	}.ParallelEvaluateConfig()
	fmt.Printf("PROBE stage='  shadow  ' → %q\n", got.Stage)
	if got.Stage != "off" {
		t.Errorf("whitespace-padded stage '  shadow  ' must fall to off, got %q", got.Stage)
	}
}

func TestAmplify_CodexGenerating_IsBusy(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	tdDir := filepath.Join(dir, "..", "..", "internal", "bridge", "panestream", "testdata", "codex")

	p := panestream.Profiles["codex"]

	nonBusy := []string{"thinking.txt", "answer.txt", "final.txt"}
	for _, name := range nonBusy {
		b, err := os.ReadFile(filepath.Join(tdDir, name))
		if err != nil {
			t.Fatalf("testdata %s: %v", name, err)
		}
		busy := panestream.PaneBusy(string(b), p)
		fmt.Printf("PROBE PaneBusy codex/%-20s → %v (expected false)\n", name, busy)
		if busy {
			t.Errorf("codex/%s: expected PaneBusy=false, got true", name)
		}
	}

	b, err := os.ReadFile(filepath.Join(tdDir, "generating.txt"))
	if err != nil {
		t.Fatalf("testdata generating.txt: %v", err)
	}
	busy := panestream.PaneBusy(string(b), p)
	fmt.Printf("PROBE PaneBusy codex/generating.txt   → %v (expected true: contains busy indicator)\n", busy)
	if !busy {
		t.Errorf("codex/generating.txt: expected PaneBusy=true (live generating pane); if false, testdata has changed")
	}
}

func TestAmplify_CodexGenerating_AnswerToGeneratingConverging(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	tdDir := filepath.Join(dir, "..", "..", "internal", "bridge", "panestream", "testdata", "codex")

	answer, err1 := os.ReadFile(filepath.Join(tdDir, "answer.txt"))
	gen, err2 := os.ReadFile(filepath.Join(tdDir, "generating.txt"))
	if err1 != nil || err2 != nil {
		t.Fatalf("testdata read: %v / %v", err1, err2)
	}

	p := panestream.Profiles["codex"]
	det := panestream.NewDefaultDetector(3)
	det.Assess(string(answer), p)
	state, conf := det.Assess(string(gen), p)
	fmt.Printf("PROBE answer→generating: state=%v conf=%.3f\n", state, conf)
	if state != panestream.LivenessConverging {
		t.Errorf("answer→generating: content grew substantially, expected Converging, got %v", state)
	}
	if conf < 0 || conf > 1 {
		t.Errorf("answer→generating: confidence %v out of [0,1]", conf)
	}
}

func TestAmplify_CodexGenerating_StalledIsHung(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "..", "..", "internal", "bridge", "panestream", "testdata", "codex", "generating.txt"))
	if err != nil {
		t.Fatalf("testdata generating.txt: %v", err)
	}
	gen := string(b)
	p := panestream.Profiles["codex"]

	if !panestream.PaneBusy(gen, p) {
		t.Skip("generating.txt is no longer busy — TestAmplify_CodexGenerating_IsBusy tracks this; this test would be stale")
	}

	det := panestream.NewDefaultDetector(3)
	det.Assess(gen, p)
	var hitHung bool
	for i := 1; i <= 6; i++ {
		state, _ := det.Assess(gen, p)
		fmt.Printf("PROBE repeated generating.txt stall=%d state=%v\n", i, state)
		if state == panestream.LivenessHung {
			hitHung = true
		}
	}
	if !hitHung {
		t.Errorf("repeated busy generating.txt across 6 intervals never reached Hung — busy stalled pane should eventually fast-fail")
	}
}

func TestAmplify_CodexGenerating_ConfidenceInRange(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "..", "..", "internal", "bridge", "panestream", "testdata", "codex", "generating.txt"))
	if err != nil {
		t.Fatalf("testdata generating.txt: %v", err)
	}
	gen := string(b)
	p := panestream.Profiles["codex"]
	det := panestream.NewDefaultDetector(3)
	_, c1 := det.Assess(gen, p)
	_, c2 := det.Assess(gen, p)
	fmt.Printf("PROBE generating.txt confidence prime=%.3f repeated=%.3f\n", c1, c2)
	for _, c := range []float64{c1, c2} {
		if c < 0 || c > 1 {
			t.Errorf("generating.txt confidence %v out of [0,1]", c)
		}
	}
}
