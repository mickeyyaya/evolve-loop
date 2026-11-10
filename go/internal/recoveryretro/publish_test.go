package recoveryretro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const publishID = "cycle-2003-judgment-plan-review"

func planeWithPending(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("README.md", "plane\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	dir := PendingDir(r.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ext := range pairExts {
		if err := os.WriteFile(filepath.Join(dir, publishID+ext), []byte("body"+ext+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestPublishPending_CommitsThePairAndClearsThePendingCopy(t *testing.T) {
	r := planeWithPending(t)

	published, err := PublishPending(r.Dir, &strings.Builder{})

	if err != nil || len(published) != 1 || published[0] != publishID {
		t.Fatalf("PublishPending = %v, %v", published, err)
	}
	if names := r.Git("show", "--name-only", "--format=", "HEAD"); !strings.Contains(names, DocsHome+"/"+publishID+".yaml") {
		t.Fatalf("HEAD lists %q", names)
	}
	if _, err := os.Lstat(filepath.Join(PendingDir(r.Dir), publishID+".md")); err == nil {
		t.Fatal("the pending copy remains")
	}
}

func TestPublishPending_ADocumentInDocsIsKeptAndThePendingPairIsDropped(t *testing.T) {
	r := planeWithPending(t)
	held := filepath.Join(r.Dir, DocsHome, publishID+".md")
	if err := os.MkdirAll(filepath.Dir(held), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(held, []byte("on main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	published, err := PublishPending(r.Dir, &strings.Builder{})

	if err != nil || len(published) != 0 {
		t.Fatalf("PublishPending = %v, %v", published, err)
	}
	if got, _ := os.ReadFile(held); string(got) != "on main\n" {
		t.Fatalf("the document was replaced: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(PendingDir(r.Dir), publishID+".yaml")); err == nil {
		t.Fatal("the pending copy of a held document remains")
	}
}

func TestPublishPending_AHalfPairWarnsAndStaysPending(t *testing.T) {
	r := planeWithPending(t)
	if err := os.Remove(filepath.Join(PendingDir(r.Dir), publishID+".yaml")); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder

	published, err := PublishPending(r.Dir, &log)

	if err != nil || len(published) != 0 || !strings.Contains(log.String(), "WARN") {
		t.Fatalf("PublishPending = %v, %v, %q", published, err, log.String())
	}
}

func TestPublishPending_ALinkAtAPendingPathIsRefused(t *testing.T) {
	r := planeWithPending(t)
	md := filepath.Join(PendingDir(r.Dir), publishID+".md")
	if err := os.Remove(md); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "x"), md); err != nil {
		t.Fatal(err)
	}

	published, err := PublishPending(r.Dir, &strings.Builder{})

	if err == nil || len(published) != 0 {
		t.Fatalf("PublishPending = %v, %v; want a refusal", published, err)
	}
}

func TestPublishPending_ACommitFailureKeepsThePairPendingAndLeavesNoDocument(t *testing.T) {
	r := planeWithPending(t)
	gittest.FaultOn(t, "commit")

	published, err := PublishPending(r.Dir, &strings.Builder{})

	if err == nil || len(published) != 0 {
		t.Fatalf("PublishPending = %v, %v", published, err)
	}
	if _, err := os.Lstat(filepath.Join(PendingDir(r.Dir), publishID+".md")); err != nil {
		t.Fatalf("the pair is not pending: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(r.Dir, DocsHome, publishID+".md")); err == nil {
		t.Fatal("the failed publish left a document")
	}
}

func TestPublishPending_AGitFailureOutsideATreeIsAnError(t *testing.T) {
	root := t.TempDir()
	dir := PendingDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ext := range pairExts {
		if err := os.WriteFile(filepath.Join(dir, "cycle-1-x"+ext), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := PublishPending(root, &strings.Builder{}); err == nil {
		t.Fatal("PublishPending outside a git tree returned no error")
	}
}

func TestReadRegular_AMissingPathIsAnError(t *testing.T) {
	if _, err := readRegular(filepath.Join(t.TempDir(), "absent.md")); err == nil {
		t.Fatal("readRegular of a missing path returned no error")
	}
}

func TestRemoveAll_ANonEmptyDirectoryIsAnErrorAndAMissingPathIsNot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeAll([]string{filepath.Join(dir, "absent")}); err != nil {
		t.Fatalf("removeAll of a missing path = %v", err)
	}
	if err := removeAll([]string{dir}); err == nil {
		t.Fatal("removeAll of a non-empty directory returned no error")
	}
}

func TestPublishPending_ADocsPathThatIsAFileIsAnError(t *testing.T) {
	r := planeWithPending(t)
	if err := os.WriteFile(filepath.Join(r.Dir, "docs"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	published, err := PublishPending(r.Dir, &strings.Builder{})

	if err == nil || len(published) != 0 {
		t.Fatalf("PublishPending = %v, %v; want an error", published, err)
	}
}
