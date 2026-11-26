package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
)

func TestRunCommitGate_ARegistryChangeThatDeletesAJourneyRowIsRefusedByName(t *testing.T) {
	repo := gittest.Fixture(t)
	registry := filepath.Join(repo.Dir, filepath.FromSlash(journeys.RegistryPath))
	if err := os.MkdirAll(filepath.Dir(registry), 0o755); err != nil {
		t.Fatal(err)
	}
	rows := `[{"id":"keep-me","journey":"A path","test":"internal/x.TestA","issue":"#1"},{"id":"also-keep","journey":"B path","test":"internal/x.TestB","issue":"#2"}]`
	if err := os.WriteFile(registry, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-qm", "base")
	kept := `[{"id":"also-keep","journey":"B path","test":"internal/x.TestB","issue":"#2"}]`
	if err := os.WriteFile(registry, []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	t.Setenv("CG_ATTEST_DIR", t.TempDir())
	var out, errb bytes.Buffer

	rc := RunCommitGate([]string{"run", "--project-root", repo.Dir, "--reviewers", "docs-review", "--no-install"}, nil, &out, &errb)

	if rc != commitgate.ExitFail || !strings.Contains(errb.String(), "keep-me") || strings.Contains(errb.String(), "also-keep") {
		t.Fatalf("rc %d stderr %q: want a refusal that names keep-me alone", rc, errb.String())
	}
}
