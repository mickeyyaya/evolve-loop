package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var effortChannelCLIs = []string{"claude-tmux", "codex-tmux", "agy-claude-tmux", "claude-p"}

func shippedProfiles(t *testing.T) map[string]Profile {
	t.Helper()
	dir := filepath.Join(repoRootForEffort(t), ".evolve", "profiles")
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no shipped profiles under %s: %v", dir, err)
	}
	out := map[string]Profile{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var head struct {
			CLI string `json:"cli"`
		}
		if json.Unmarshal(raw, &head) != nil || head.CLI == "" {
			continue
		}
		p, err := LoadProfile(path)
		if err != nil {
			t.Fatalf("LoadProfile(%s): %v", path, err)
		}
		out[p.Name] = p
	}
	return out
}

func effortRealized(m Manifest, r Realization, effort string) bool {
	spec := m.Params["effort"]
	switch {
	case spec.Channel == effortChannelModelVariant:
		want := modelVariantSuffix(spec.Values[effort])
		return want != "" && slices.ContainsFunc(r.LaunchFlags, func(f string) bool { return strings.HasSuffix(f, want) })
	case len(spec.Values) > 0:
		toks, ok := spec.Values[effort]
		return ok && containsSubsequence(r.LaunchFlags, toks)
	default:
		return spec.Flag != "" && containsSubsequence(r.LaunchFlags, []string{spec.Flag, effort})
	}
}

func TestEveryShippedProfileAndTierLaunchesWithAnEffort(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	tiers := policy.BridgePolicy{}.TierEfforts()
	profiles := shippedProfiles(t)
	launches := 0
	for _, cli := range effortChannelCLIs {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", cli, err)
		}
		if ch := m.Params["effort"].Channel; ch == "" || ch == "noop" {
			t.Fatalf("%s has no effort channel (channel=%q): its launches cannot carry an effort", cli, ch)
		}
		for name, prof := range profiles {
			for _, tier := range policy.TierNames() {
				intent := launchIntentFor(&Config{CLI: cli, Model: tier}, prof, tiers)
				launches++
				if !slices.Contains(policy.EffortLevels(), intent.Effort) {
					t.Errorf("%s/%s/%s: effort %q is not a level", cli, name, tier, intent.Effort)
					continue
				}
				if r := Realize(m, intent); !effortRealized(m, r, intent.Effort) {
					t.Errorf("%s/%s/%s: effort %q did not reach the launch: flags %v", cli, name, tier, intent.Effort, r.LaunchFlags)
				}
			}
		}
	}
	if launches < len(effortChannelCLIs)*4*80 {
		t.Fatalf("walked %d launches, want every profile x tier x CLI (>= %d)", launches, len(effortChannelCLIs)*4*80)
	}
}
