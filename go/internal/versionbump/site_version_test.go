package versionbump

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const siteFixture = `{
  "product": {
    "name": "Evolve Loop",
    "version": "v23.1.0",
    "license": "Apache-2.0"
  },
  "note": "shipped in v23.1.0",
  "other": { "version": "v1.0.0" }
}
`

func writeSite(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "content.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBumpSiteVersion_ChangesOnlyTheProductVersion(t *testing.T) {
	path := writeSite(t, siteFixture)
	changed, err := BumpSiteVersion(path, "23.2.0", false)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(siteFixture, `"version": "v23.1.0"`, `"version": "v23.2.0"`, 1)
	if string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBumpSiteVersion_ADryRunWritesNothing(t *testing.T) {
	path := writeSite(t, siteFixture)
	changed, err := BumpSiteVersion(path, "23.2.0", true)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if got, _ := os.ReadFile(path); string(got) != siteFixture {
		t.Fatalf("a dry run wrote: %s", got)
	}
}

func TestBumpSiteVersion_SkipsWhatItCannotOwn(t *testing.T) {
	cases := map[string]string{
		"current already":        writeSite(t, strings.Replace(siteFixture, "v23.1.0", "v23.2.0", 1)),
		"no product version":     writeSite(t, `{"product":{"name":"x"},"version":"v1.0.0"}`),
		"another site's content": writeSite(t, `{"title":"my landing page"}`),
		"absent file":            filepath.Join(t.TempDir(), "absent.json"),
	}
	for name, path := range cases {
		changed, err := BumpSiteVersion(path, "23.2.0", false)
		if err != nil || changed {
			t.Errorf("%s: changed=%v err=%v", name, changed, err)
		}
	}
}

func TestBumpSiteVersion_AnUnreadableFileFails(t *testing.T) {
	if _, err := BumpSiteVersion(t.TempDir(), "23.2.0", false); err == nil {
		t.Fatal("reading a directory must fail")
	}
}

func TestRun_AnUnreadableSiteFileStopsTheBump(t *testing.T) {
	paths := runFixture(t)
	paths.SiteContentJSON = t.TempDir()
	if _, err := Run(paths, "23.2.0", false, time.Unix(0, 0)); err == nil {
		t.Fatal("a site file that cannot be read must stop the bump")
	}
}

func TestRun_TheReleaseMovesTheSiteVersionAndTheCommitStagesIt(t *testing.T) {
	paths := runFixture(t)
	paths.SiteContentJSON = writeSite(t, siteFixture)
	res, err := Run(paths, "23.2.0", false, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Modified, "landing/shared/content.json") {
		t.Fatalf("modified = %v", res.Modified)
	}
	if !slices.Contains(paths.Files(), paths.SiteContentJSON) {
		t.Fatalf("Files() = %v omits the site content, so the release commit does not stage it", paths.Files())
	}
	if got := DefaultPaths("/r").SiteContentJSON; got != filepath.Join("/r", "landing", "shared", "content.json") {
		t.Fatalf("default path = %s", got)
	}
}
