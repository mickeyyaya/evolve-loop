package profiles_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func trackedProfilesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "profiles")
}

// See ADR-0104.
func TestEveryAgentProfileHasAFallbackChain(t *testing.T) {
	dir := trackedProfilesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var p struct {
			Name        string   `json:"name"`
			CLI         string   `json:"cli"`
			CLIFallback []string `json:"cli_fallback"`
			AllowedCLIs []string `json:"allowed_clis"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if p.CLI == "" {
			continue // not an agent, e.g. tool-policy.json
		}
		checked++
		if len(p.CLIFallback) == 0 {
			t.Errorf("%s: cli=%s has NO cli_fallback — one wall on %s fails the phase", e.Name(), p.CLI, p.CLI)
			continue
		}
		allowed := map[string]bool{}
		for _, a := range p.AllowedCLIs {
			allowed[strings.TrimSpace(a)] = true
		}
		for _, fb := range p.CLIFallback {
			switch {
			case !llmroute.KnownDriver(fb):
				t.Errorf("%s: cli_fallback %q is not a registered driver", e.Name(), fb)
			case llmroute.Family(fb) == "agy":
				t.Errorf("%s: agy is banned as a fallback (2026-06-07); found %q", e.Name(), fb)
			case len(allowed) > 0 && !allowed["all"] && !allowed[llmroute.Family(fb)]:
				t.Errorf("%s: cli_fallback %q is outside allowed_clis %v", e.Name(), fb, p.AllowedCLIs)
			}
		}
	}
	if checked < 50 {
		t.Fatalf("only %d dispatchable profiles checked — wrong directory? %s", checked, dir)
	}
}
