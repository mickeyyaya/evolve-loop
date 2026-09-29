package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReapPipelineTemp_RemovesOnlyStalePipelineArtifacts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 9, 29, 23, 0, 0, 0, time.UTC)
	stale, fresh := now.Add(-30*time.Hour), now.Add(-time.Hour)
	entries := map[string]struct {
		mtime time.Time
		reap  bool
	}{
		"go-build1234567":                       {stale, true},
		"acs-cycle1515-bin-2718281828":          {stale, true},
		"acs1435-bin-1106288950":                {stale, true},
		"cycle1498-evolve-bin4242":              {stale, true},
		"TestRawFixturePattern_LosesRace99":     {stale, false},
		"release-pipeline-dryrun-81632.json":    {stale, true},
		"release-pipeline-dryrun-22.26.0.json":  {stale, true},
		"go-build7654321":                       {fresh, false},
		"com.apple.launchd.x":                   {stale, false},
		"tmpabc123":                             {stale, false},
		"go-buildcache":                         {stale, false},
		"acs":                                   {stale, false},
		"cyclone-notes":                         {stale, false},
		"release-pipeline-dryrun-notes.json.bk": {stale, false},
	}
	for name, e := range entries {
		p := filepath.Join(dir, name)
		writeFile(t, filepath.Join(p, "payload"), "0123456789")
		if err := os.Chtimes(p, e.mtime, e.mtime); err != nil {
			t.Fatal(err)
		}
	}

	rep := ReapPipelineTemp(dir, now.Add(-24*time.Hour), os.RemoveAll)

	if rep.Entries != 6 || rep.Bytes != 60 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v, want 6 entries / 60 bytes / no errors", rep)
	}
	for name, e := range entries {
		_, err := os.Stat(filepath.Join(dir, name))
		if gone := os.IsNotExist(err); gone != e.reap {
			t.Errorf("%s: removed=%v, want %v", name, gone, e.reap)
		}
	}
}

func TestReapPipelineTemp_APreviewRemovesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "go-build1234567")
	writeFile(t, filepath.Join(p, "payload"), "0123456789")
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}

	rep := ReapPipelineTemp(dir, time.Now().Add(-24*time.Hour), func(string) error { return nil })

	if rep.Entries != 1 {
		t.Errorf("preview counted %d entries, want 1", rep.Entries)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("preview removed %s", p)
	}
}
