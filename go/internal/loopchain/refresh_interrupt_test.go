package loopchain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRefresh_AnInterruptBeforeTheExecSkipsTheReExec(t *testing.T) {
	cases := []struct {
		name      string
		interrupt func(f *refreshFixture, cancel context.CancelFunc)
		order     string
	}{
		{"pending at the boundary", func(_ *refreshFixture, cancel context.CancelFunc) { cancel() }, "ahead,lane"},
		{"arriving during the rebuild", func(f *refreshFixture, cancel context.CancelFunc) {
			f.deps.Rebuild = func(string) error { f.order = append(f.order, "rebuild"); cancel(); return nil }
		}, "ahead,lane,rebuild"},
		{"arriving during the re-pin", func(f *refreshFixture, cancel context.CancelFunc) {
			f.deps.Provenance = func(string) (string, phaseintegrity.ProvenanceVerified) {
				f.order = append(f.order, "provenance")
				cancel()
				return commit, func(c string) bool { return c == commit }
			}
		}, "ahead,lane,rebuild,provenance"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRefreshFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.interrupt(f, cancel)

			if f.refresher().Refresh(ctx, 7) {
				t.Fatal("an interrupted refresh reports refreshed=false")
			}
			if got := strings.Join(f.order, ","); got != c.order {
				t.Errorf("steps run: %s, want %s (no flush, no re-exec)", got, c.order)
			}
			ev := f.sig.only(t, CodeBoundaryRefreshSkipped)
			if ev.Fields["step"] != "interrupted" || ev.Fields["batch"] != "7" || ev.Fields["error"] != context.Canceled.Error() || ev.Severity != signalcenter.SeverityWarn {
				t.Errorf("one named skip: %+v", ev)
			}
			if _, err := os.Stat(filepath.Join(f.evolveDir, AttemptFile)); !os.IsNotExist(err) {
				t.Errorf("the breaker is not armed for an exec that never happens: %v", err)
			}
		})
	}
}

func TestRefresh_ALiveContextStillRebuildsAndReExecs(t *testing.T) {
	f := newRefreshFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !f.refresher().Refresh(ctx, 7) {
		t.Fatalf("a live context refreshes: %s", f.sig.console.String())
	}
	if got := strings.Join(f.order, ","); got != "ahead,lane,rebuild,provenance,argv,flush,reexec" {
		t.Errorf("order: %s", got)
	}
}
