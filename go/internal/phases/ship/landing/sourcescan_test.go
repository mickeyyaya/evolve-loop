package landing

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func leafSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(src)
	}
	return sources
}

var structTag = regexp.MustCompile("`[^`]*`")

func TestLeaf_NoHandWrittenStderrOrEnvReads(t *testing.T) {
	banned := []string{"os.Stderr", "os.Stdout", "Fprintf(os.", "os.Getenv", "os.LookupEnv", `"EVOLVE_`, "ipcenv"}
	for name, src := range leafSources(t) {
		for _, needle := range banned {
			if strings.Contains(src, needle) {
				t.Errorf("%s spells %q: the landing reports through the Center and takes the fleet flag from the host", name, needle)
			}
		}
	}
}

func TestLeaf_SpellsTheWhitelistedDebugKeysThroughShiperr(t *testing.T) {
	if len(shiperr.SignalDebugKeys) < 6 {
		t.Fatalf("the whitelist shrank to %v: the scan would pass vacuously", shiperr.SignalDebugKeys)
	}
	for name, src := range leafSources(t) {
		code := structTag.ReplaceAllString(src, "")
		for _, key := range shiperr.SignalDebugKeys {
			if needle := `"` + key + `"`; strings.Contains(code, needle) {
				t.Errorf("%s spells the projected Debug key %s as a literal: use its shiperr constant", name, needle)
			}
		}
	}
}
