package bridge

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const strictMCPFlag = "--strict-mcp-config"

func embeddedClaudeManifests(t *testing.T) []Manifest {
	t.Helper()
	entries, err := embeddedManifests.ReadDir("manifests")
	if err != nil {
		t.Fatalf("read embedded manifests: %v", err)
	}
	var out []Manifest
	for _, e := range entries {
		raw, err := embeddedManifests.ReadFile("manifests/" + e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var head struct {
			Binary string `json:"binary"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			t.Fatalf("decode %s: %v", e.Name(), err)
		}
		if head.Binary != "claude" {
			continue
		}
		m, err := parseManifest(strings.TrimSuffix(e.Name(), ".json"), raw)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		t.Fatal("no embedded manifest launches the claude binary")
	}
	return out
}

func mcpViolations(flags []string) []string {
	var bad []string
	if !slices.Contains(flags, strictMCPFlag) {
		bad = append(bad, "missing "+strictMCPFlag)
	}
	for _, f := range flags {
		if strings.HasPrefix(f, "--mcp-config") {
			bad = append(bad, "passes "+f)
		}
	}
	return bad
}

func TestEveryClaudeLaunchIsStrictMCPAndNamesNoMCPConfig(t *testing.T) {
	t.Parallel()
	profiles := shippedProfiles(t)
	for _, m := range embeddedClaudeManifests(t) {
		if bad := mcpViolations(Realize(m, LaunchIntent{}).LaunchFlags); len(bad) > 0 {
			t.Errorf("%s with no profile flags: %v", m.CLI, bad)
		}
		for name, p := range profiles {
			got := Realize(m, LaunchIntent{RawByCLI: p.ExtraFlagsByCLI}).LaunchFlags
			if bad := mcpViolations(got); len(bad) > 0 {
				t.Errorf("%s launch of profile %s: %v (flags %v)", m.CLI, name, bad, got)
			}
		}
	}
}

func TestMCPViolations_FlagsAMissingStrictFlagAndANamedConfig(t *testing.T) {
	t.Parallel()
	got := mcpViolations([]string{"--mcp-config", "servers.json"})
	want := []string{"missing " + strictMCPFlag, "passes --mcp-config"}
	if !slices.Equal(got, want) {
		t.Fatalf("mcpViolations = %v, want %v", got, want)
	}
}
