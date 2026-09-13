package landing

// sourcescan_test.go — test 31: the leaf never writes the process stderr
// itself and never reads the environment (the fleet toggle arrives as a
// bool the host read); test 45: every whitelisted Debug key the leaf stamps
// is spelled through its shiperr constant, never as a bare literal. All are
// structural, so a source scan pins them.

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// leafSources returns the leaf's non-test Go sources by file name.
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

// Test 45 — the consumer pin of shiperr's Debug-key constants: a key the ONE
// ship.error producer projects (shiperr.SignalDebugKeys) appears in no leaf
// source as a quoted literal, so a producer here cannot respell a key out of
// the signal line; the needles follow the whitelist, so a key added there
// must be adopted here too.
func TestLeaf_SpellsTheWhitelistedDebugKeysThroughShiperr(t *testing.T) {
	if len(shiperr.SignalDebugKeys) < 6 {
		t.Fatalf("the whitelist shrank to %v: the scan would pass vacuously", shiperr.SignalDebugKeys)
	}
	for name, src := range leafSources(t) {
		for _, key := range shiperr.SignalDebugKeys {
			if needle := `"` + key + `"`; strings.Contains(src, needle) {
				t.Errorf("%s spells the projected Debug key %s as a literal: use its shiperr constant", name, needle)
			}
		}
	}
}
