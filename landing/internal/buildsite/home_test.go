package buildsite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func homeConfig(t *testing.T, home string) Config {
	t.Helper()
	return Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: "../../templates/*.html",
		OutDir:       t.TempDir(),
		Gallery:      "gallery",
		Home:         home,
		Versions: []Version{
			{Slug: "noir", Title: "Keynote Noir", Tagline: "Dark.", Template: "noir"},
			{Slug: "luminous", Title: "Luminous Minimal", Tagline: "Light.", Template: "luminous"},
		},
	}
}

func readPage(t *testing.T, parts ...string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestBuild_TheRootServesTheHomeVersionAndNotTheDesignGallery(t *testing.T) {
	cfg := homeConfig(t, "noir")
	if _, err := Build(cfg); err != nil {
		t.Fatal(err)
	}
	root := readPage(t, cfg.OutDir, "index.html")
	if strings.Contains(root, "landing page concepts") || strings.Contains(root, "Five ways") {
		t.Fatal("the root still serves the design gallery")
	}
	if !strings.Contains(root, "Who decides it ships") {
		t.Fatal("the root does not serve the product page")
	}
	if !strings.Contains(root, `assets/noir/hero.png`) || strings.Contains(root, `../assets/noir`) {
		t.Fatal("the root page must reach its assets from the root")
	}
}

func TestBuild_TheGalleryMovesToConceptsAndItsLinksStillResolve(t *testing.T) {
	cfg := homeConfig(t, "noir")
	if _, err := Build(cfg); err != nil {
		t.Fatal(err)
	}
	gallery := readPage(t, cfg.OutDir, "concepts", "index.html")
	if !strings.Contains(gallery, `href="../noir/index.html"`) {
		t.Fatalf("a gallery link does not resolve from /concepts/: %.200s", gallery)
	}
	if !strings.Contains(gallery, `url('../assets/noir/hero.png')`) || strings.Contains(gallery, `'assets/noir`) {
		t.Fatal("a gallery thumbnail does not resolve from /concepts/")
	}
	if _, err := os.Stat(filepath.Join(cfg.OutDir, "noir", "index.html")); err != nil {
		t.Fatal(err)
	}
}

func TestBuild_WithoutAHomeTheGalleryStaysAtTheRoot(t *testing.T) {
	cfg := homeConfig(t, "")
	if _, err := Build(cfg); err != nil {
		t.Fatal(err)
	}
	if root := readPage(t, cfg.OutDir, "index.html"); !strings.Contains(root, `href="noir/index.html"`) {
		t.Fatalf("the gallery link changed with no home: %.200s", root)
	}
}

func TestBuild_AHomeThatNamesNoVersionFails(t *testing.T) {
	cfg := homeConfig(t, "absent")
	_, err := Build(cfg)
	if err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("err = %v, want one that names the absent home", err)
	}
}

func TestBuild_EachPageCarriesTheLinkPreviewOfTheHomeURL(t *testing.T) {
	cfg := homeConfig(t, "noir")
	if _, err := Build(cfg); err != nil {
		t.Fatal(err)
	}
	const site = "https://mickeyyaya.github.io/evolve-loop/"
	for _, page := range [][]string{{"index.html"}, {"noir", "index.html"}, {"luminous", "index.html"}} {
		body := readPage(t, append([]string{cfg.OutDir}, page...)...)
		for _, want := range []string{
			`property="og:title"`,
			`property="og:description"`,
			`property="og:image" content="` + site + `assets/noir/hero.png"`,
			`property="og:url" content="` + site + `"`,
			`name="twitter:card" content="summary_large_image"`,
			`rel="canonical" href="` + site + `"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%v lacks %s", page, want)
			}
		}
	}
}

func TestBuild_AnExpandedRootFileGetsTheVersion(t *testing.T) {
	cfg := homeConfig(t, "noir")
	cfg.RootFiles = []RootFile{{Src: "../../shared/llms.txt", Dst: "llms.txt", Expand: true}, {Src: "../../shared/llms.txt", Dst: "raw.txt"}}
	if _, err := Build(cfg); err != nil {
		t.Fatal(err)
	}
	if body := readPage(t, cfg.OutDir, "llms.txt"); strings.Contains(body, "{{") || !strings.Contains(body, "Current release: v") {
		t.Fatalf("llms.txt was not expanded: %.300s", body)
	}
	if body := readPage(t, cfg.OutDir, "raw.txt"); !strings.Contains(body, "{{version}}") {
		t.Fatalf("a plain root file changed: %.300s", body)
	}
}

func TestBuild_AnExpandedRootFileThatCannotBeReadFails(t *testing.T) {
	cfg := homeConfig(t, "noir")
	cfg.RootFiles = []RootFile{{Src: "absent.txt", Dst: "x.txt", Expand: true}}
	if _, err := Build(cfg); err == nil || !strings.Contains(err.Error(), "absent.txt") {
		t.Fatalf("err = %v", err)
	}
}
