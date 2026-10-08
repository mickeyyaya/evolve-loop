package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

func TestBootTmuxREPL_AnAdmissionOrRegistryFailureWarnsAndTheBootGoesOn(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", blocker)
	ws := t.TempDir()
	if err := os.Mkdir(sessionrecord.PathIn(ws), 0o755); err != nil {
		t.Fatal(err)
	}
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps, stderr := bootSmokeDeps(tmux)
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_CLI_MAX_CONCURRENT_CLAUDE-TMUX": "1"})

	rc, _ := BootSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, deps)

	if rc != ExitOK {
		t.Fatalf("rc = %d, want ExitOK: neither warning stops the boot\n%s", rc, stderr.String())
	}
	for _, want := range []string{"WARN cliadmit: cliadmit: mkdir:", "(proceeding uncapped)", "WARN session registry append failed:", "will not be registry-reapable"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
}
