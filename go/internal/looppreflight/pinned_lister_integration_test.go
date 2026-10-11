//go:build integration

package looppreflight

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"time"
)

func stubBrew(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "brew"), "#!/bin/sh\n"+script+"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/bin:/usr/bin")
}

func TestDefaultPinnedLister_ParsesOnePinPerNonBlankLine(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		want         []string
	}{
		{"pins", `[ "$1 $2" = "list --pinned" ] || exit 9; printf '  node\n\n\ttmux  \nclaude-code\n'`, []string{"node", "tmux", "claude-code"}},
		{"none pinned", `exit 0`, nil},
		{"only blank lines", `printf '\n  \n\n'`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubBrew(t, tc.script)

			got, err := defaultPinnedLister()

			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("defaultPinnedLister = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestDefaultPinnedLister_FailuresAreAmbiguityErrors(t *testing.T) {
	t.Run("brew exits non-zero", func(t *testing.T) {
		stubBrew(t, "echo partial; exit 1")
		got, err := defaultPinnedLister()
		var exitErr *exec.ExitError
		if got != nil || !errors.As(err, &exitErr) || !strings.HasPrefix(err.Error(), "brew list --pinned: ") {
			t.Fatalf("defaultPinnedLister = %q, %v; want nil and the wrapped exit", got, err)
		}
	})
	t.Run("brew missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		got, err := defaultPinnedLister()
		if got != nil || !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("defaultPinnedLister = %q, %v; want nil and exec.ErrNotFound", got, err)
		}
	})
	t.Run("brew hangs", func(t *testing.T) {
		stubBrew(t, "exec sleep 60")
		start := time.Now()
		got, err := defaultPinnedLister()
		if got != nil || err == nil || time.Since(start) > pinnedListerTimeout+3*time.Second {
			t.Fatalf("defaultPinnedLister = %q, %v after %v; want an error near the %v bound", got, err, time.Since(start), pinnedListerTimeout)
		}
	})
}
