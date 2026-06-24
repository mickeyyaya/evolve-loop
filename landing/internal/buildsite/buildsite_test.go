package buildsite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Build renders each version page + the gallery from the real content and copies
// assets — proving the whole wiring end to end against a tiny fixture template.
func TestBuild_RendersPagesAndCopiesAssets(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"),
		`{{define "mini"}}<h1>{{.Site.Hero.Headline}}</h1><img src="{{.Assets}}/hero.png">{{end}}`)
	mustWrite(t, filepath.Join(dir, "gallery.html"),
		`{{define "gallery"}}{{range .Versions}}<a href="{{.Slug}}/">{{.Title}}</a>{{end}}{{end}}`)

	assets := filepath.Join(dir, "assets", "mini")
	mustMkdir(t, assets)
	mustWrite(t, filepath.Join(assets, "hero.png"), "PNGBYTES")

	out := filepath.Join(dir, "dist")
	written, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		AssetsDir:    filepath.Join(dir, "assets"),
		OutDir:       out,
		Gallery:      "gallery",
		Versions:     []Version{{Slug: "mini", Title: "Mini", Template: "mini"}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("Build reported no written files")
	}

	page := readFile(t, filepath.Join(out, "mini", "index.html"))
	if !strings.Contains(page, "Ship the code your agent wrote") {
		t.Errorf("page missing real headline, got: %s", page)
	}
	if !strings.Contains(page, `src="../assets/mini/hero.png"`) {
		t.Errorf("page missing correct relative asset path, got: %s", page)
	}

	gallery := readFile(t, filepath.Join(out, "index.html"))
	if !strings.Contains(gallery, "Mini") {
		t.Errorf("gallery missing version title, got: %s", gallery)
	}

	if _, err := os.Stat(filepath.Join(out, "assets", "mini", "hero.png")); err != nil {
		t.Errorf("asset not copied into dist: %v", err)
	}
}

// Build copies RootFiles verbatim into the dist root — how install.sh reaches
// /install.sh on GitHub Pages.
func TestBuild_CopiesRootFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"), `{{define "mini"}}x{{end}}`)
	mustWrite(t, filepath.Join(dir, "install.sh"), "#!/bin/sh\necho hi\n")

	out := filepath.Join(dir, "dist")
	written, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		OutDir:       out,
		RootFiles:    []RootFile{{Src: filepath.Join(dir, "install.sh"), Dst: "install.sh"}},
		Versions:     []Version{{Slug: "mini", Title: "Mini", Template: "mini"}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := readFile(t, filepath.Join(out, "install.sh")); got != "#!/bin/sh\necho hi\n" {
		t.Errorf("install.sh not copied verbatim, got: %q", got)
	}
	found := false
	for _, w := range written {
		if strings.HasSuffix(w, "install.sh") {
			found = true
		}
	}
	if !found {
		t.Errorf("install.sh missing from written list: %v", written)
	}
}

func TestBuild_UnknownTemplateFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"), `{{define "real"}}x{{end}}`)
	_, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		OutDir:       filepath.Join(dir, "dist"),
		Versions:     []Version{{Slug: "ghost", Title: "Ghost", Template: "does-not-exist"}},
	})
	if err == nil {
		t.Fatal("Build with unknown template = nil error, want failure")
	}
}

func TestBuild_BadContentFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"), `{{define "x"}}x{{end}}`)
	_, err := Build(Config{
		ContentPath:  filepath.Join(dir, "nonexistent.json"),
		TemplateGlob: filepath.Join(dir, "*.html"),
		OutDir:       filepath.Join(dir, "dist"),
		Versions:     []Version{{Slug: "x", Template: "x"}},
	})
	if err == nil {
		t.Fatal("Build with missing content = nil error, want failure")
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
