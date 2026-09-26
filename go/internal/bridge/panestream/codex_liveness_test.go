package panestream

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func codexFrame(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "testdata", "codex", name))
	if err != nil {
		t.Fatalf("codexFrame(%q): %v", name, err)
	}
	return string(b)
}

func TestCodexLiveness_ThinkingToAnswerConverging(t *testing.T) {
	p := Profiles["codex"]
	det := NewDefaultDetector(3)
	think := codexFrame(t, "thinking.txt")
	answer := codexFrame(t, "answer.txt")
	det.Assess(think, p) // prime
	state, conf := det.Assess(answer, p)
	if state != LivenessConverging {
		t.Errorf("thinking→answer: got %v, want LivenessConverging", state)
	}
	if conf < 0 || conf > 1 {
		t.Errorf("thinking→answer: confidence %v out of [0,1]", conf)
	}
}

func TestCodexLiveness_PrimingNotHung(t *testing.T) {
	p := Profiles["codex"]
	answer := codexFrame(t, "answer.txt")
	// Use stallThreshold=1 so Hung fires as early as possible if the invariant breaks.
	det := NewDefaultDetector(1)
	state, conf := det.Assess(answer, p)
	if state == LivenessHung {
		t.Errorf("prime call must NOT return LivenessHung (got %v)", state)
	}
	if conf < 0 || conf > 1 {
		t.Errorf("prime confidence %v out of [0,1]", conf)
	}
}

func TestCodexLiveness_StalledIdleNotHung(t *testing.T) {
	p := Profiles["codex"]
	answer := codexFrame(t, "answer.txt")
	if PaneBusy(answer, p) {
		t.Fatal("precondition: codex/answer.txt must not be busy (no busy affordance)")
	}

	det := NewDefaultDetector(3)
	det.Assess(answer, p) // prime

	for i := 1; i <= 5; i++ {
		state, conf := det.Assess(answer, p)
		if state == LivenessHung {
			t.Errorf("stall interval %d: got LivenessHung — codex cannot reach Hung (no busy affordance)", i)
		}
		if state != LivenessIdle {
			t.Errorf("stall interval %d: got %v, want LivenessIdle (no new content, no busy signal)", i, state)
		}
		if conf < 0 || conf > 1 {
			t.Errorf("stall interval %d: confidence %v out of [0,1]", i, conf)
		}
	}
}

func TestCodexLiveness_ConfidenceInRange(t *testing.T) {
	p := Profiles["codex"]
	frames := []string{"thinking.txt", "answer.txt", "final.txt"}
	for _, f := range frames {
		t.Run(f, func(t *testing.T) {
			det := NewDefaultDetector(3)
			content := codexFrame(t, f)
			_, conf1 := det.Assess(content, p)
			if conf1 < 0 || conf1 > 1 {
				t.Errorf("prime confidence %v out of [0,1]", conf1)
			}
			_, conf2 := det.Assess(content, p)
			if conf2 < 0 || conf2 > 1 {
				t.Errorf("second call confidence %v out of [0,1]", conf2)
			}
		})
	}
}
