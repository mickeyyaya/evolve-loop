package commitgate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func seededClone(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("a.txt", "a\n")
	repo.Git("add", "a.txt")
	repo.Git("commit", "-q", "-m", "init")
	return repo
}

func TestReviewStoreDir_ResolvesTheCommonDirInEachCheckoutKind(t *testing.T) {
	t.Parallel()
	plain := seededClone(t)
	base := realPath(t, plain.Dir)
	linked := filepath.Join(realPath(t, t.TempDir()), "linked")
	plain.Git("worktree", "add", "-q", "-b", "linked", linked)

	sepRoot := realPath(t, t.TempDir())
	sepWork, sepGit := filepath.Join(sepRoot, "work"), filepath.Join(sepRoot, "store.git")
	if err := os.MkdirAll(sepWork, 0o755); err != nil {
		t.Fatal(err)
	}
	plain.Git("clone", "-q", "--separate-git-dir", sepGit, base, sepWork)

	super := seededClone(t)
	superDir := realPath(t, super.Dir)
	super.Git("-c", "protocol.file.allow=always", "submodule", "add", "-q", seededClone(t).Dir, "sub")
	subWork := filepath.Join(superDir, "sub")
	subGit := filepath.Join(superDir, ".git", "modules", "sub")

	cases := map[string]struct{ root, common string }{
		"plain clone":      {base, filepath.Join(base, ".git")},
		"linked worktree":  {linked, filepath.Join(base, ".git")},
		"separate git dir": {sepWork, sepGit},
		"submodule":        {subWork, subGit},
	}
	for name, c := range cases {
		got, err := ReviewStoreDir(context.Background(), sysexec.DefaultRunner, c.root)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if want := filepath.Join(realPath(t, c.common), "evolve", "reviews"); got != want {
			t.Errorf("%s: store = %q, want %q", name, got, want)
		}
	}
}

func TestReviewStoreDir_RefusesWhenGitCannotAnswer(t *testing.T) {
	t.Parallel()
	failing := (&scriptRunner{rules: []scriptRule{{matchPrefix: "git rev-parse", exit: 128}}}).run()
	erroring := (&scriptRunner{rules: []scriptRule{{matchPrefix: "git rev-parse", err: errors.New("spawn")}}}).run()
	empty := (&scriptRunner{rules: []scriptRule{{matchPrefix: "git rev-parse", stdout: "\n"}}}).run()
	for name, run := range map[string]Runner{"exit": failing, "spawn": erroring, "empty": empty} {
		if got, err := ReviewStoreDir(context.Background(), run, "/r"); err == nil || got != "" {
			t.Errorf("%s: got %q, %v; want an error and no path", name, got, err)
		}
	}
}

func TestReviewStoreDir_AsksGitForTheAbsoluteCommonDirFromTheRoot(t *testing.T) {
	t.Parallel()
	sr := &scriptRunner{rules: []scriptRule{{matchPrefix: "git rev-parse", stdout: "/c/.git\n"}}}
	got, err := ReviewStoreDir(context.Background(), sr.run(), "/r")
	if err != nil || got != filepath.Join("/c/.git", "evolve", "reviews") {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(sr.calls) != 1 || sr.calls[0] != "git rev-parse --path-format=absolute --git-common-dir" {
		t.Errorf("calls = %v", sr.calls)
	}
}

func TestRun_AnEmptyReviewStoreIsARefusalNotASkippedWrite(t *testing.T) {
	t.Parallel()
	o, _ := goGate(t, "code-simplifier,go-reviewer")
	o.ReviewDir = ""

	res := o.Run(context.Background())

	logs := strings.Join(res.Logs, "\n")
	if res.ExitCode != ExitGitFatal || res.Attestation != nil || !strings.Contains(logs, "no review store directory") {
		t.Errorf("exit %d attestation %v logs %q, want a git-fatal refusal that names the missing review store directory", res.ExitCode, res.Attestation, logs)
	}
	if _, err := os.Stat(o.attestPath()); err == nil {
		t.Error("a refused gate must not write the attestation")
	}
}

func TestRun_TheGateRecordsTheAttestationAndEachNamedLaunchRecordUnderTheTree(t *testing.T) {
	t.Parallel()
	simplify, golang := reviewRun("r-simplify", "code-simplifier"), reviewRun("r-go", "go-reviewer")
	o, _ := goGate(t, "run:r-simplify,run:r-go", simplify, golang)

	res := o.Run(context.Background())

	if res.ExitCode != ExitPass {
		t.Fatalf("exit %d: %v", res.ExitCode, res.Logs)
	}
	body, err := os.ReadFile(filepath.Join(o.ReviewDir, res.Attestation.TreeStateSHA+".json"))
	if err != nil {
		t.Fatalf("no record under the tree state: %v", err)
	}
	var got ReviewRecord
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Attestation, *res.Attestation) {
		t.Errorf("attestation = %+v, want %+v", got.Attestation, *res.Attestation)
	}
	if !reflect.DeepEqual(got.LaunchRecords, []launchrecord.Record{simplify, golang}) {
		t.Errorf("launch records = %+v, want the two verified runs in order", got.LaunchRecords)
	}
}

func TestRun_ANamedReviewerGateRecordsNoLaunchRecords(t *testing.T) {
	t.Parallel()
	o, _ := goGate(t, "code-simplifier,go-reviewer")

	res := o.Run(context.Background())

	body, err := os.ReadFile(filepath.Join(o.ReviewDir, res.Attestation.TreeStateSHA+".json"))
	if err != nil {
		t.Fatalf("no record: %v", err)
	}
	var got ReviewRecord
	if err := json.Unmarshal(body, &got); err != nil || len(got.LaunchRecords) != 0 || len(got.Attestation.ReviewersRun) != 2 {
		t.Errorf("record %+v err %v, want the attestation and no launch record", got, err)
	}
}

func TestRun_AFailedReviewRecordWriteFailsTheGate(t *testing.T) {
	t.Parallel()
	o, _ := goGate(t, "code-simplifier,go-reviewer")
	blocker := filepath.Join(o.RepoRoot, "blocker")
	mustWrite(t, blocker, "file")
	o.ReviewDir = filepath.Join(blocker, "reviews")

	res := o.Run(context.Background())

	logs := strings.Join(res.Logs, "\n")
	if res.ExitCode != ExitGitFatal || res.Attestation != nil || !strings.Contains(logs, "cannot write review record") {
		t.Errorf("exit %d attestation %v logs %q, want git fatal and the review record reason", res.ExitCode, res.Attestation, logs)
	}
	if _, err := os.Stat(o.attestPath()); err == nil {
		t.Error("a gate that failed to write the review record must not leave an attestation")
	}
}
