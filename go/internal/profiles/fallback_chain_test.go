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

// trackedProfilesDir is the repository's .evolve/profiles — the tracked
// source of every phase agent's routing config.
func trackedProfilesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "profiles")
}

// TestEveryAgentProfileHasAFallbackChain — operator policy (2026-09-14): a
// phase must try every available CLI before giving up, so no agent that names
// a primary CLI may leave cli_fallback empty. Each entry must be a registered
// driver, must be a family the profile's allowed_clis permits, and must not be
// the agy family (the 2026-06-07 ban on agy as a rescue). The two audit-spine
// agents that had no chain at all (auditor, tdd-engineer) are the ones this
// pins hardest: with an empty chain, one Claude wall failed the whole cycle.
// The Claude-family floor (family_floor_test.go) keeps those five agents'
// chains INSIDE the claude family on purpose; this guard only requires a chain.
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
			continue // not a dispatchable agent (e.g. the shared tool-policy fragment)
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
