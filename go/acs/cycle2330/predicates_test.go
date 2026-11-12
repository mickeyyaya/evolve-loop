//go:build acs

package cycle2330

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const manifestsRel = "go/internal/bridge/manifests"

func settingsAfterFlag(flags []string) (string, bool) {
	i := slices.Index(flags, "--settings")
	if i < 0 || i+1 >= len(flags) {
		return "", false
	}
	return flags[i+1], true
}

func parseSettings(t *testing.T, raw string) map[string]any {
	t.Helper()
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("--settings value %q is not JSON: %v", raw, err)
	}
	return settings
}

func TestC2330_001_ProjectScopedClaudeSeatTurnsThePRStatusFooterOff(t *testing.T) {
	r := bridge.RealizeFor("claude-tmux", bridge.LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SettingsScope: "project", SessionMode: "ephemeral"})
	raw, ok := settingsAfterFlag(r.LaunchFlags)
	if !ok {
		t.Fatalf("a project-scoped claude seat realizes no --settings: %v", r.LaunchFlags)
	}
	settings := parseSettings(t, raw)
	if settings["prStatusFooterEnabled"] != false {
		t.Errorf("--settings %q must set prStatusFooterEnabled false so the claude footer stops polling gh from the cell", raw)
	}
}

func TestC2330_002_TheBypassAcceptanceSurvivesNextToTheFooterKey(t *testing.T) {
	r := bridge.RealizeFor("claude-tmux", bridge.LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SettingsScope: "project", SessionMode: "ephemeral"})
	raw, ok := settingsAfterFlag(r.LaunchFlags)
	if !ok {
		t.Fatalf("a project-scoped claude seat realizes no --settings: %v", r.LaunchFlags)
	}
	settings := parseSettings(t, raw)
	if settings["skipDangerousModePermissionPrompt"] != true {
		t.Errorf("--settings %q must keep skipDangerousModePermissionPrompt true", raw)
	}
	if _, has := settings["prStatusFooterEnabled"]; !has {
		t.Errorf("--settings %q must declare prStatusFooterEnabled next to the bypass acceptance", raw)
	}
}

func TestC2330_003_EveryManifestSettingsValueDeclaresTheFooterOff(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), manifestsRel)
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no manifests under %s: %v", dir, err)
	}
	seen := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Params map[string]struct {
				Values map[string][]string `json:"values"`
			} `json:"params"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", filepath.Base(path), err)
		}
		for name, spec := range m.Params {
			for scope, flags := range spec.Values {
				value, ok := settingsAfterFlag(flags)
				if !ok {
					continue
				}
				seen++
				if parseSettings(t, value)["prStatusFooterEnabled"] != false {
					t.Errorf("%s params.%s.values.%s: --settings %q lacks prStatusFooterEnabled false", filepath.Base(path), name, scope, value)
				}
			}
		}
	}
	if seen == 0 {
		t.Fatalf("no manifest declares --settings data, so the walk checked nothing")
	}
}

func TestC2330_004_TheBridgeGoldenLaunchCommandPinsTheNewSettingsValue(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-run", "^TestRealizerWiring_NoCrossCLILeak$", "./internal/bridge")
	if code == -1 {
		t.Fatalf("go test did not run: %s", errOut)
	}
	if code != 0 {
		t.Errorf("the bridge golden launch command disagrees with the manifest --settings value (exit %d):\n%s", code, acsassert.OutputTail(out+errOut, 12))
	}
}
