//go:build acs

package cycle2152

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

const judgmentID = "cycle-2003-judgment-plan-review"

func judgmentTodos() []cyclestate.CarryoverTodo {
	return []cyclestate.CarryoverTodo{{ID: judgmentID, Action: "cycle 2003 plan-review FAIL: the plan widens the lane scope", Priority: "P1"}}
}

func planeWithBaseCommit(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("README.md", "plane\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func writeFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func pendingPair(root string) (md, yaml string) {
	dir := recoveryretro.PendingDir(root)
	return filepath.Join(dir, judgmentID+".md"), filepath.Join(dir, judgmentID+".yaml")
}

func TestC2152_001_EnforceWritesThePairIntoThePendingDirAndThePublishCommitsItIntoDocsRetrospectives(t *testing.T) {
	r := planeWithBaseCommit(t)
	workspace := t.TempDir()

	written, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, workspace, 2152, judgmentTodos())
	if err != nil {
		t.Fatalf("WriteInto: %v", err)
	}
	md, yaml := pendingPair(r.Dir)
	if len(written) != 2 {
		t.Fatalf("WriteInto wrote %v, want %s and %s", written, md, yaml)
	}
	if got := recoveryretro.PendingDir(r.Dir); got != filepath.Join(r.Dir, ".evolve", "retrospectives-pending") {
		t.Fatalf("PendingDir = %q, want <root>/.evolve/retrospectives-pending", got)
	}
	wantDoc, err := os.ReadFile(md)
	if err != nil {
		t.Fatalf("the pending document is missing: %v", err)
	}

	published, err := recoveryretro.PublishPending(r.Dir, &bytes.Buffer{})

	if err != nil || len(published) != 1 || published[0] != judgmentID {
		t.Fatalf("PublishPending = %v, %v; want only %s", published, err, judgmentID)
	}
	gotDoc, err := os.ReadFile(filepath.Join(r.Dir, recoveryretro.DocsHome, judgmentID+".md"))
	if err != nil || !bytes.Equal(gotDoc, wantDoc) {
		t.Fatalf("docs/retrospectives holds %q (%v), want the pending bytes", gotDoc, err)
	}
	names := r.Git("show", "--name-only", "--format=", "HEAD")
	for _, ext := range []string{".md", ".yaml"} {
		if !strings.Contains(names, recoveryretro.DocsHome+"/"+judgmentID+ext) {
			t.Errorf("HEAD does not commit %s%s; HEAD lists %q", judgmentID, ext, names)
		}
	}
	for _, path := range []string{md, yaml} {
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("%s is still pending after the publish", path)
		}
	}
	if status := r.Git("status", "--porcelain"); strings.Contains(status, "docs/retrospectives") {
		t.Errorf("the publish left uncommitted retrospective files: %q", status)
	}
}

func TestC2152_002_ShadowWritesTheWorkspaceOnlyAndTouchesNeitherThePendingDirNorDocs(t *testing.T) {
	r := planeWithBaseCommit(t)
	workspace := t.TempDir()

	written, err := recoveryretro.WriteShadow(r.Dir, workspace, 2152, judgmentTodos())

	if err != nil || len(written) != 2 {
		t.Fatalf("WriteShadow = %v, %v; want one pair", written, err)
	}
	for _, w := range written {
		if filepath.Dir(w) != filepath.Join(workspace, recoveryretro.DirName) {
			t.Errorf("WriteShadow wrote %s outside <workspace>/%s", w, recoveryretro.DirName)
		}
	}
	for _, rel := range []string{filepath.Join(".evolve", "retrospectives-pending"), recoveryretro.DocsHome} {
		if _, err := os.Lstat(filepath.Join(r.Dir, rel)); err == nil {
			t.Errorf("shadow created %s", rel)
		}
	}
}

func TestC2152_003_CreateOnlyKeepsAnExistingPendingDocumentAndAnExistingPublishedDocument(t *testing.T) {
	r := planeWithBaseCommit(t)
	md, _ := pendingPair(r.Dir)
	writeFile(t, md, []byte("the Judgment text of a person\n"))

	written, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, t.TempDir(), 2152, judgmentTodos())

	if err != nil || len(written) != 1 || !strings.HasSuffix(written[0], ".yaml") {
		t.Fatalf("WriteInto = %v, %v; want only the missing yaml", written, err)
	}
	if got, _ := os.ReadFile(md); string(got) != "the Judgment text of a person\n" {
		t.Fatalf("WriteInto replaced the pending document: %q", got)
	}

	published := filepath.Join(r.Dir, recoveryretro.DocsHome, judgmentID+".md")
	writeFile(t, published, []byte("on main\n"))
	writeFile(t, filepath.Join(r.Dir, recoveryretro.DocsHome, judgmentID+".yaml"), []byte("on main\n"))
	r.Git("add", recoveryretro.DocsHome)
	r.Git("commit", "-q", "-m", "retrospective on main")
	head := r.Git("rev-parse", "HEAD")

	if _, err := recoveryretro.PublishPending(r.Dir, &bytes.Buffer{}); err != nil {
		t.Fatalf("PublishPending: %v", err)
	}

	if got, _ := os.ReadFile(published); string(got) != "on main\n" {
		t.Fatalf("PublishPending replaced the document on main: %q", got)
	}
	if after := r.Git("rev-parse", "HEAD"); after != head {
		t.Fatalf("PublishPending committed over a document on main: %s -> %s", head, after)
	}
	if _, err := os.Lstat(md); err == nil {
		t.Error("the pending copy of a published document stays pending and would be tried at each boundary")
	}
}

func TestC2152_004_APublishFailureReturnsAnErrorAndKeepsThePairPendingWithNoFileInDocs(t *testing.T) {
	r := planeWithBaseCommit(t)
	if _, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, t.TempDir(), 2152, judgmentTodos()); err != nil {
		t.Fatal(err)
	}
	head := r.Git("rev-parse", "HEAD")
	gittest.FaultOn(t, "commit")

	published, err := recoveryretro.PublishPending(r.Dir, &bytes.Buffer{})

	if err == nil || len(published) != 0 {
		t.Fatalf("PublishPending = %v, %v; want an error and nothing published", published, err)
	}
	md, yaml := pendingPair(r.Dir)
	for _, path := range []string{md, yaml} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("the failed publish dropped the pending file %s", path)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(r.Dir, recoveryretro.DocsHome)); len(entries) != 0 {
		t.Errorf("the failed publish left %d files in docs/retrospectives", len(entries))
	}
	if after := r.Git("rev-parse", "HEAD"); after != head {
		t.Errorf("HEAD moved on a failed publish")
	}
}

func TestC2152_005_APublishRefusesAHalfPairAndALinkAtAPendingPath(t *testing.T) {
	r := planeWithBaseCommit(t)
	dir := recoveryretro.PendingDir(r.Dir)
	writeFile(t, filepath.Join(dir, "cycle-1-half-pair.md"), []byte("md without yaml\n"))
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, []byte("outside the tree\n"))
	writeFile(t, filepath.Join(dir, "cycle-2-linked.yaml"), []byte("id: x\n"))
	if err := os.Symlink(outside, filepath.Join(dir, "cycle-2-linked.md")); err != nil {
		t.Fatal(err)
	}
	head := r.Git("rev-parse", "HEAD")

	published, _ := recoveryretro.PublishPending(r.Dir, &bytes.Buffer{})

	if len(published) != 0 {
		t.Fatalf("PublishPending published %v, want neither the half pair nor the linked pair", published)
	}
	if after := r.Git("rev-parse", "HEAD"); after != head {
		t.Fatal("a refused pending pair was committed")
	}
	if _, err := os.Lstat(filepath.Join(dir, "cycle-1-half-pair.md")); err != nil {
		t.Error("the half pair did not stay pending")
	}
}
