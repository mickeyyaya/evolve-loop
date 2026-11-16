package ciparitygate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func pValue(t *testing.T, args []string) string {
	t.Helper()
	at := slices.Index(args, "-p")
	if at < 0 || at+1 >= len(args) {
		t.Fatalf("argv %v has no -p flag", args)
	}
	return args[at+1]
}

func TestRetake_RebuildsTheShareAfterTheLock(t *testing.T) {
	cpus := runtime.NumCPU()
	if cpus < 2 {
		t.Skip("one CPU gives every live width the same share")
	}
	root, _ := goWorktree(t)
	ws := t.TempDir()
	liveFile := filepath.Join(t.TempDir(), lanebudget.LiveWidthFile)
	t.Setenv(ipcenv.FleetWidthKey, strconv.Itoa(cpus))
	t.Setenv(ipcenv.FleetLiveWidthKey, liveFile)
	if err := lanebudget.WriteLiveWidth(liveFile, os.Getpid(), cpus); err != nil {
		t.Fatal(err)
	}
	rel, held, err := flock.TryLock(ciparity.TierLockPath(root))
	if err != nil || held {
		t.Fatalf("test could not take the tier lock: held=%v err=%v", held, err)
	}
	var argvs [][]string
	run := func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		argvs = append(argvs, slices.Clone(args))
		if len(argvs) == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n")
			return 1, nil
		}
		return 0, nil
	}
	released := false
	sleep := func(time.Duration) {
		if err := lanebudget.WriteLiveWidth(liveFile, os.Getpid(), 1); err != nil {
			t.Error(err)
		}
		rel()
		released = true
	}
	g, _ := observed(t, run, fixedSet("./internal/widget/..."), WithClock(time.Now, sleep))

	if _, err := g.IntegrationTier(tierRequest(root, ws)); err == nil {
		t.Fatal("a green retake after a red first attempt must report the absorbed flake")
	}

	if !released || len(argvs) != 2 {
		t.Fatalf("released=%v attempts=%d, want the retake to wait on the lock and then run", released, len(argvs))
	}
	if first, retake := pValue(t, argvs[0]), pValue(t, argvs[1]); first != "1" || retake != strconv.Itoa(cpus) {
		t.Errorf("-p of attempt 1 = %s and of the retake = %s, want 1 then %d (the share at the time of each run)", first, retake, cpus)
	}
	logBody, err := os.ReadFile(filepath.Join(ws, "integration-tier.log"))
	if err != nil {
		t.Fatal(err)
	}
	for i, argv := range argvs {
		if header := "# go " + strings.Join(argv, " ") + "\n"; !strings.Contains(string(logBody), header) {
			t.Errorf("the log has no header %q for attempt %d:\n%s", header, i+1, logBody)
		}
	}
}
