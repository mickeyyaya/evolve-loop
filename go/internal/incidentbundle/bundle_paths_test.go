package incidentbundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentbundle"
)

func bundleDir(root string) string { return incidentbundle.Dir(root, bundleID) }

func writeManifest(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(bundleDir(root), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir(root), incidentbundle.ManifestName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAdd_FailsOnAMissingSource(t *testing.T) {
	t.Parallel()
	_, err := incidentbundle.Add(t.TempDir(), bundleID, filepath.Join(t.TempDir(), "absent.log"), 1000, fixedNow)
	if err == nil || !strings.Contains(err.Error(), "read source") {
		t.Fatalf("want a read source error, got %v", err)
	}
}

func TestAdd_FailsOnACorruptManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifest(t, root, "{not json")
	_, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "x\n"), 1000, fixedNow)
	if err == nil || !strings.Contains(err.Error(), "manifest.json") {
		t.Fatalf("want a manifest error, got %v", err)
	}
}

func TestAdd_FailsWhenTheCopyPathIsADirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundleDir(root), "src-01-a.log"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "x\n"), 1000, fixedNow); err == nil {
		t.Fatal("want an error when the copy cannot be written")
	}
}

func TestAdd_FailsWhenAnEvictedCopyCannotBeRemoved(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "ERROR same 1\n"), 1000, fixedNow); err != nil {
		t.Fatal(err)
	}
	stuck := filepath.Join(bundleDir(root), "src-01-a.log")
	if err := os.Remove(stuck); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stuck, "keep"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "b.log", "ERROR same 2\n"), 1, fixedNow); err == nil || !strings.Contains(err.Error(), "drop") {
		t.Fatalf("want a drop error, got %v", err)
	}
}

func TestAdd_ToleratesAnEvictedCopyThatIsAlreadyGone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "ERROR same 1\n"), 1000, fixedNow); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(bundleDir(root), "src-01-a.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "b.log", "ERROR same 2\n"), 1, fixedNow); err != nil {
		t.Fatalf("a copy already gone must not fail the eviction: %v", err)
	}
}

func TestAdd_LeadingNumberIsDroppedFromTheErrorTemplate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "12:00:01 ERROR job 7 failed\n"), 1000, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Templates) != 1 || entry.Templates[0] != "ERROR job # failed" {
		t.Fatalf("templates = %v", entry.Templates)
	}
}

func TestCheck_FailsOnAMissingBundleAndACorruptManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := incidentbundle.Check(root, bundleID); err == nil {
		t.Error("want an error for a bundle that is not there")
	}
	writeManifest(t, root, "{not json")
	if _, err := incidentbundle.Check(root, bundleID); err == nil {
		t.Error("want an error for a corrupt manifest")
	}
}

func TestCheck_ReportsAnUnreadableCopy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "ok\n"), 1000, fixedNow); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(bundleDir(root), "src-01-a.log")); err != nil {
		t.Fatal(err)
	}
	problems, err := incidentbundle.Check(root, bundleID)
	if err != nil || len(problems) != 1 || !strings.Contains(problems[0], "unreadable") {
		t.Fatalf("problems = %v, err = %v", problems, err)
	}
}

func TestCheckAll_SkipsFilesAndWrapsABundleError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, filepath.FromSlash(incidentbundle.RootDir))
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "README.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := incidentbundle.CheckAll(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("files must be skipped: %v %v", got, err)
	}
	if err := os.MkdirAll(filepath.Join(base, "not-an-incident-id"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := incidentbundle.CheckAll(root); err == nil || !strings.Contains(err.Error(), "not-an-incident-id") {
		t.Fatalf("want an error naming the bundle, got %v", err)
	}
}

func TestCheckAll_FailsWhenTheIncidentRootIsNotADirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "knowledge-base"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := incidentbundle.CheckAll(root); err == nil {
		t.Fatal("want a read error")
	}
}
