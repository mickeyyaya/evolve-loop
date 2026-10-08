package ship

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

func coverPackLog() (packLog, *bytes.Buffer) {
	var buf bytes.Buffer
	return packLog{notes: &buf, raw: &buf}, &buf
}

func TestDefaultRepoContractTest_AModuleDirThatIsGoneIsAStartErrorNotARed(t *testing.T) {
	t.Parallel()
	log, _ := coverPackLog()

	got := defaultRepoContractTest(context.Background(), filepath.Join(t.TempDir(), "gone"), log)

	if got.err == nil || strings.Count(got.err.Error(), "go test start: ") != 2 || len(got.failures) != 0 {
		t.Errorf("outcome = %+v, want two start errors (the pack and the selections) and no named red", got)
	}
}

func TestRunImporterBackstop_DiscoveryThatFailsTwiceIsAnInfraError(t *testing.T) {
	t.Parallel()
	log, buf := coverPackLog()
	cleared := false

	err := runImporterBackstop(context.Background(), log, "", t.TempDir(), t.TempDir(),
		[]changedpkgs.ChangedFile{{Path: "go/internal/x/x.go"}}, nil, func([]string) { cleared = true })

	if err == nil || !strings.Contains(err.Error(), "repo-contract importer backstop discovery: `go list` could not walk the module's import graph") || cleared {
		t.Errorf("err = %v cleared=%v log=%q, want the INFRA discovery error and nothing cleared", err, cleared, buf.String())
	}
}

func TestRunAddedTestBackstop_AnUnreadableAddedTestIsAnInfraError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	t.Parallel()
	repo := makeRepo(t)
	added := filepath.Join(repo, "go", "internal", "x", "x_test.go")
	mustMkdir(t, filepath.Dir(added))
	mustWrite(t, added, "package x\n")
	if err := os.Chmod(added, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(added, 0o644) })
	log, _ := coverPackLog()

	files, untagged, err := runAddedTestBackstop(context.Background(), log, repo, "HEAD", filepath.Join(repo, "go"), t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "could not inspect an added test's build constraints") || files != nil || untagged != nil {
		t.Errorf("runAddedTestBackstop = %v, %v, %v, want the INFRA inspect error and nothing run", files, untagged, err)
	}
}
