package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

const pendingRetroID = "cycle-2003-judgment-plan-review"

func writePendingRetrospective(t *testing.T, root string) {
	t.Helper()
	dir := recoveryretro.PendingDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for ext, body := range map[string]string{".md": "# judgment\n", ".yaml": "id: " + pendingRetroID + "\n"} {
		if err := os.WriteFile(filepath.Join(dir, pendingRetroID+ext), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func headFiles(r *gittest.Repo) string { return r.Git("show", "--name-only", "--format=", "HEAD") }

func assertRetrospectiveCommitted(t *testing.T, r *gittest.Repo) {
	t.Helper()
	files := headFiles(r)
	for _, ext := range []string{".md", ".yaml"} {
		if !strings.Contains(files, recoveryretro.DocsHome+"/"+pendingRetroID+ext) {
			t.Errorf("HEAD does not commit %s%s; HEAD lists %q", pendingRetroID, ext, files)
		}
	}
	if _, err := os.Stat(filepath.Join(recoveryretro.PendingDir(r.Dir), pendingRetroID+".md")); err == nil {
		t.Error("the committed retrospective is still pending")
	}
}

func TestPublishPendingDossiers_PublishesThePendingRetrospectivesWithNoDossierPending(t *testing.T) {
	t.Parallel()
	r := gitPlane(t, "main")
	writePendingRetrospective(t, r.Dir)
	var warn bytes.Buffer

	publishPendingDossiers(r.Dir, &warn)

	assertRetrospectiveCommitted(t, r)
}

func TestPublishPendingDossiers_PublishesTheRetrospectivesBesideTheDossiers(t *testing.T) {
	t.Parallel()
	r := planeWithAPendingCloseout(t, "main")
	writePendingRetrospective(t, r.Dir)
	var warn bytes.Buffer

	publishPendingDossiers(r.Dir, &warn)

	if log := r.Git("log", "--format=%s", "-3"); !strings.Contains(log, "dossier: cycle-1705 closeout") {
		t.Errorf("the dossier is not committed: %q", log)
	}
	if all := r.Git("log", "--name-only", "--format=", "-3"); !strings.Contains(all, recoveryretro.DocsHome+"/"+pendingRetroID+".md") {
		t.Errorf("the retrospective is not committed beside the dossier: %q", all)
	}
}

func TestPublishDossierPairs_PublishesThePendingRetrospectives(t *testing.T) {
	t.Parallel()
	r := gitPlane(t, "main")
	writePendingRetrospective(t, r.Dir)
	var stdout, stderr bytes.Buffer

	rc := publishDossierPairs(r.Dir, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("publishDossierPairs rc = %d, stderr = %q", rc, stderr.String())
	}
	assertRetrospectiveCommitted(t, r)
}

func TestPublishPendingDossiers_ARetrospectivePublishFailureIsAWarnAndTheBoundaryGoesOn(t *testing.T) {
	r := gitPlane(t, "main")
	writePendingRetrospective(t, r.Dir)
	head := r.Git("rev-parse", "HEAD")
	gittest.FaultOn(t, "commit")
	var warn bytes.Buffer

	publishPendingDossiers(r.Dir, &warn)

	if after := r.Git("rev-parse", "HEAD"); after != head {
		t.Error("HEAD moved on a failed publish")
	}
	if _, err := os.Stat(filepath.Join(recoveryretro.PendingDir(r.Dir), pendingRetroID+".md")); err != nil {
		t.Errorf("the failed publish dropped the pending pair: %v", err)
	}
	if !strings.Contains(warn.String(), "WARN") || !strings.Contains(strings.ToLower(warn.String()), "retrospective") {
		t.Errorf("warn = %q, want a WARN line about the retrospective", warn.String())
	}
}

func TestPublishDossierPairs_ARetrospectivePublishFailureReturnsTwo(t *testing.T) {
	r := gitPlane(t, "main")
	writePendingRetrospective(t, r.Dir)
	gittest.FaultOn(t, "commit")
	var stdout, stderr bytes.Buffer

	rc := publishDossierPairs(r.Dir, &stdout, &stderr)

	if rc != 2 {
		t.Errorf("publishDossierPairs rc = %d, want 2; stderr = %q", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "retrospective") {
		t.Errorf("stderr = %q, want a retrospective WARN", stderr.String())
	}
}

func TestHasPendingRetrospectives_AnUnreadablePendingDirWarnsAndReportsNone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pending := recoveryretro.PendingDir(root)
	if err := os.MkdirAll(filepath.Dir(pending), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pending, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer

	got := hasPendingRetrospectives(root, &warn)

	if got {
		t.Error("hasPendingRetrospectives = true for an unreadable dir")
	}
	if !strings.Contains(warn.String(), "WARN") || !strings.Contains(warn.String(), "cannot check") {
		t.Errorf("warn = %q, want a cannot-check WARN", warn.String())
	}
}
