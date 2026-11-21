package core

import (
	"context"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type scriptedPass struct {
	out    string
	status int
	write  bool
}

func scriptCoverPass(t *testing.T, script []scriptedPass) {
	t.Helper()
	var mu sync.Mutex
	next := 0
	orig := coverPass
	coverPass = func(_ context.Context, _ int, _ string, profile string, _, _ []string) (string, int) {
		mu.Lock()
		step := script[next]
		next++
		mu.Unlock()
		if step.write {
			if err := os.WriteFile(profile, []byte("mode: atomic\n"), 0o644); err != nil {
				return err.Error(), coverStatusPlumbingError
			}
		}
		return step.out, step.status
	}
	t.Cleanup(func() { coverPass = orig })
}

func TestScopedCoverProfile_ARetakePlumbingErrorKeepsTheFirstRedTags(t *testing.T) {
	scriptCoverPass(t, []scriptedPass{
		{status: coverStatusOK, write: true},
		{out: retakeRedOutput, status: coverStatusTestsFailed},
		{status: coverStatusOK, write: true},
		{status: coverStatusOK, write: false},
	})
	in := ReviewInput{ProjectRoot: t.TempDir(), Signals: signalcenter.New(), Stderr: &strings.Builder{}}

	profile, out, status, tags := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./internal/bridge", "./cmd/evolve"}, []string{"integration"})
	removeProfileDir(profile)

	if status != coverStatusTestsFailed || !strings.Contains(out, "--- FAIL: TestFlaky") {
		t.Fatalf("(%q, %d), want the first red kept", out, status)
	}
	if want := []string{"integration", "notmux"}; !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v, the tags of the first red", tags, want)
	}
}

func TestScopedCoverProfile_ARetakeRedOnAnotherRunNamesThatRunsTags(t *testing.T) {
	scriptCoverPass(t, []scriptedPass{
		{status: coverStatusOK, write: true},
		{out: retakeRedOutput, status: coverStatusTestsFailed},
		{out: "--- FAIL: TestOther (0.01s)\nFAIL\texample.com/m/cmd\t0.2s\nFAIL\n", status: coverStatusTestsFailed},
	})
	in := ReviewInput{ProjectRoot: t.TempDir(), Signals: signalcenter.New(), Stderr: &strings.Builder{}}

	profile, out, status, tags := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./internal/bridge", "./cmd/evolve"}, []string{"integration"})
	removeProfileDir(profile)

	if status != coverStatusTestsFailed || !strings.Contains(out, "TestOther") {
		t.Fatalf("(%q, %d), want the red of the retake", out, status)
	}
	if want := []string{"integration"}; !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v, the tags of the retake run", tags, want)
	}
}
