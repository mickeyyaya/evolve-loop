package content

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var versionToken = regexp.MustCompile(`\bv?\d+\.\d+\.\d+\b`)

func TestPinnedVersionCopiesMatchProductVersion(t *testing.T) {
	site := loadRealSite(t)
	llms, err := os.ReadFile("../../shared/llms.txt")
	if err != nil {
		t.Fatalf("read llms.txt: %v", err)
	}
	want := strings.TrimPrefix(site.Product.Version, "v")
	copies := []struct {
		name string
		text string
	}{
		{"tryIt.noAccount", site.TryIt.NoAccount},
		{"tryIt.terminal", strings.Join(site.TryIt.Terminal, "\n")},
		{"examples release command", releaseCommand(t, site)},
		{"llms.txt", string(llms)},
	}
	for _, c := range copies {
		found := versionToken.FindAllString(c.text, -1)
		if len(found) == 0 {
			t.Errorf("%s pins no version, want %s", c.name, site.Product.Version)
		}
		for _, v := range found {
			if strings.TrimPrefix(v, "v") != want {
				t.Errorf("%s pins %s, want product.version %s", c.name, v, site.Product.Version)
			}
		}
	}
}

func releaseCommand(t *testing.T, site *Site) string {
	t.Helper()
	for _, it := range site.Examples.Items {
		if strings.HasPrefix(it.Command, "evolve release ") {
			return it.Command
		}
	}
	t.Fatal("examples.items has no evolve release command")
	return ""
}
