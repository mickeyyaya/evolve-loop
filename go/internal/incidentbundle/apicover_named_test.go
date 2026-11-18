package incidentbundle_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentbundle"
)

const bundleID = "2026-11-18-fixture"

var fixedNow = time.Date(2026, 11, 18, 1, 2, 3, 0, time.UTC)

func writeSource(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAdd_RedactsTheCopyAndKeepsTheRawDigest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	raw := "ok line\napi_key=hunter2hunter2 loaded\n"
	entry, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", raw), 1<<20, fixedNow)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	copied, err := os.ReadFile(filepath.Join(incidentbundle.Dir(root, bundleID), entry.Path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(copied), "hunter2hunter2") || entry.Redactions != 1 || entry.State != incidentbundle.StateFull {
		t.Errorf("copy %q entry %+v: want the secret gone, one redaction, state full", copied, entry)
	}
	if entry.SourceSHA256 == entry.SHA256 {
		t.Errorf("source digest must be of the raw bytes, copy digest of the redacted bytes")
	}
}

func TestAdd_SameSourceTwiceStagesOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := writeSource(t, "a.log", "same\n")
	first, _ := incidentbundle.Add(root, bundleID, src, 1<<20, fixedNow)
	second, err := incidentbundle.Add(root, bundleID, src, 1<<20, fixedNow)
	if err != nil || second.ID != first.ID {
		t.Fatalf("second Add = %+v, %v; want the first entry %s", second, err, first.ID)
	}
	files, _ := os.ReadDir(incidentbundle.Dir(root, bundleID))
	if len(files) != 2 {
		t.Errorf("bundle holds %d files, want the manifest and one copy", len(files))
	}
}

func TestAdd_RejectsAnInvalidID(t *testing.T) {
	t.Parallel()
	if incidentbundle.ValidID("../escape") || !incidentbundle.ValidID(bundleID) {
		t.Errorf("ValidID must refuse a path and accept yyyy-mm-dd-slug")
	}
	if _, err := incidentbundle.Add(t.TempDir(), "../escape", writeSource(t, "a.log", "x"), 1<<20, fixedNow); err == nil {
		t.Errorf("Add must refuse an id that escapes the bundle root")
	}
	if _, err := incidentbundle.Check(t.TempDir(), "../escape"); err == nil {
		t.Errorf("Check must refuse an id that escapes the bundle root")
	}
}

func TestAdd_OverBudgetPointersTheOldestRepeatAndKeepsTemplates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pad := strings.Repeat("INFO filler line\n", 100)
	s1 := "ERROR disk full on node 1\n" + pad
	s2 := "ERROR disk full on node 2\nERROR lock held\n" + pad
	s3 := "ERROR quota hit\n" + pad
	budget := len(s2) + len(s3) + 50
	for i, body := range []string{s1, s2, s3} {
		if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "s.log", body), budget, fixedNow); err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}
	manifest := readManifest(t, root)
	states := []string{manifest.Entries[0].State, manifest.Entries[1].State, manifest.Entries[2].State}
	if strings.Join(states, ",") != "pointer,full,full" {
		t.Errorf("states = %v, want the oldest repeat as a pointer and the rest full", states)
	}
	if manifest.Entries[0].SourceSHA256 == "" || manifest.Entries[0].Path != "" {
		t.Errorf("pointer entry %+v must keep the digest and drop the path", manifest.Entries[0])
	}
}

func TestAdd_OverBudgetWithNoRepeatKeepsEverySource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for i, body := range []string{"ERROR one\n", "ERROR two\n"} {
		if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "s.log", body), 1, fixedNow); err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}
	for _, e := range readManifest(t, root).Entries {
		if e.State != incidentbundle.StateFull {
			t.Errorf("entry %s became %s though its template is held nowhere else", e.ID, e.State)
		}
	}
}

func TestCheck_FailsOnChangedUnlistedSecretAndEntropy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, _ := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "fine\n"), 1<<20, fixedNow)
	if problems, err := incidentbundle.Check(root, bundleID); err != nil || len(problems) != 0 {
		t.Fatalf("fresh bundle: %v %v", problems, err)
	}
	dir := incidentbundle.Dir(root, bundleID)
	appendTo(t, filepath.Join(dir, entry.Path), "handle Zq8Xv3Lp0Rt7Ym2Kc9Wd4Hs6Jb1Nf5Ga8Ue3OiPx\npassword: letmein\n")
	if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	problems, _ := incidentbundle.Check(root, bundleID)
	joined := strings.Join(problems, "\n")
	for _, want := range []string{"changed since capture", "secret", "high-entropy", "stray.txt"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Check problems lack %q:\n%s", want, joined)
		}
	}
}

func TestCheckAll_ReportsEachBundleAndNothingWhenAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if got, err := incidentbundle.CheckAll(root); err != nil || len(got) != 0 {
		t.Fatalf("absent root: %v %v", got, err)
	}
	entry, _ := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "fine\n"), 1<<20, fixedNow)
	appendTo(t, filepath.Join(incidentbundle.Dir(root, bundleID), entry.Path), "more\n")
	got, err := incidentbundle.CheckAll(root)
	if err != nil || len(got[bundleID]) == 0 {
		t.Errorf("CheckAll = %v, %v; want the changed bundle reported", got, err)
	}
}

func TestSchemaAndLayoutConstants(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := incidentbundle.Add(root, bundleID, writeSource(t, "a.log", "x\n"), 1<<20, fixedNow); err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, root)
	var first incidentbundle.Entry = manifest.Entries[0]
	if first.ID != "src-01" || first.State != incidentbundle.StateFull {
		t.Errorf("first entry = %+v, want src-01 in state full", first)
	}
	if manifest.Schema != incidentbundle.Schema || manifest.ID != bundleID {
		t.Errorf("manifest = %+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(root, incidentbundle.RootDir, bundleID, incidentbundle.ManifestName)); err != nil {
		t.Errorf("manifest is not at RootDir/<id>/ManifestName: %v", err)
	}
	if incidentbundle.StatePointer == incidentbundle.StateFull {
		t.Errorf("states must differ")
	}
}

func readManifest(t *testing.T, root string) incidentbundle.Manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(incidentbundle.Dir(root, bundleID), incidentbundle.ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var got incidentbundle.Manifest
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	return got
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
