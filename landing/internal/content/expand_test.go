package content

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestExpand_GivesBothFormsOfTheVersion(t *testing.T) {
	got := Expand("EVO_VERSION={{version}} evolve release {{semver}} since v1.2.3", "v23.2.0")
	want := "EVO_VERSION=v23.2.0 evolve release 23.2.0 since v1.2.3"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestLoad_ExpandsTheVersionInEachCopy(t *testing.T) {
	site := loadRealSite(t)
	for name, text := range map[string]string{
		"noAccount": site.TryIt.NoAccount,
		"terminal":  strings.Join(site.TryIt.Terminal, "\n"),
		"release":   releaseCommand(t, site),
	} {
		if strings.Contains(text, "{{") {
			t.Errorf("%s keeps a token: %s", name, text)
		}
	}
}

func TestTheSiteVersionHasOneHome(t *testing.T) {
	site := loadRealSite(t)
	version := regexp.QuoteMeta(strings.TrimPrefix(site.Product.Version, "v"))
	literal := regexp.MustCompile(`\bv?` + version + `\b`)
	raw, err := os.ReadFile("../../shared/content.json")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(literal.FindAll(raw, -1)); n != 1 {
		t.Errorf("content.json holds the version %d times, want 1 (product.version)", n)
	}
	llms, err := os.ReadFile("../../shared/llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	if literal.Match(llms) {
		t.Error("llms.txt holds a literal copy of the version")
	}
}
