package retro

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestResolveCLI_AWalkRefusalIsReturnedAndNeverBecomesACandidate(t *testing.T) {
	orig := walkRetroDecision
	t.Cleanup(func() { walkRetroDecision = orig })
	boom := errors.New("nothing permitted at the tier")
	walkRetroDecision = func(cliroute.Decision) (llmroute.Plan, error) { return llmroute.Plan{}, boom }
	p := &Phase{}

	cli, err := p.resolveCLI(core.PhaseRequest{ProjectRoot: t.TempDir()}, &profiles.Profile{Name: retroAgent, CLI: "claude-tmux"}, "balanced")

	if !errors.Is(err, boom) || cli != "" {
		t.Fatalf("cli=%q err=%v", cli, err)
	}
}

func TestMatchesCycleLesson_OnlyTheExactCycleNumberMatches(t *testing.T) {
	cases := []struct {
		name, prefix string
		want         bool
	}{
		{"inst-L12", "inst-L12", true},
		{"inst-L12.yaml", "inst-L12", true},
		{"inst-L12-note.yaml", "inst-L12", true},
		{"inst-L123.yaml", "inst-L12", false},
		{"inst-L1.yaml", "inst-L12", false},
		{"other-L12.yaml", "inst-L12", false},
		{"", "inst-L12", false},
	}
	for _, tc := range cases {
		if got := matchesCycleLesson(tc.name, tc.prefix); got != tc.want {
			t.Errorf("matchesCycleLesson(%q,%q) = %v, want %v", tc.name, tc.prefix, got, tc.want)
		}
	}
}

func TestHasFailureLesson_DirectoriesAreNotLessonsAndTheExactCycleFileIs(t *testing.T) {
	root := t.TempDir()
	lessons := filepath.Join(root, lessonsDirRel)
	if err := os.MkdirAll(filepath.Join(lessons, "inst-L7-dir.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()

	if hasFailureLesson(root, ws, 7) {
		t.Fatal("a directory named like a lesson is not a lesson")
	}
	if err := os.WriteFile(filepath.Join(lessons, "inst-L70.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if hasFailureLesson(root, ws, 7) {
		t.Fatal("cycle 70's lesson is not cycle 7's")
	}
	if err := os.WriteFile(filepath.Join(lessons, "inst-L7-timeout.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasFailureLesson(root, ws, 7) {
		t.Fatal("cycle 7's lesson file must be found")
	}
	if hasFailureLesson("", ws, 7) || hasFailureLesson(root, ws, 0) {
		t.Fatal("without a project root or a positive cycle only the workspace is consulted")
	}
}
