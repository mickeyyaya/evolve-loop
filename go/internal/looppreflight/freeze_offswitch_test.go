package looppreflight

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func stubManifestOffSwitch(t *testing.T, env string, set bool) {
	t.Helper()
	orig := manifestAutoUpdateOff
	t.Cleanup(func() { manifestAutoUpdateOff = orig })
	manifestAutoUpdateOff = func(bin string) (string, bool) {
		if bin != "agy" {
			return "", false
		}
		return env, set
	}
}

func agyFreezeOptions(t *testing.T, pinned []string) Options {
	t.Helper()
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "agy-tmux"}, nil
	}
	opts.PinnedLister = func() ([]string, error) { return pinned, nil }
	opts.SelfUpdateEvidence = nil
	return opts
}

func TestDefaultSelfUpdateEvidence_AManifestThatSetsItsOffSwitchIsFrozenAtTheSource(t *testing.T) {
	stubManifestOffSwitch(t, "AGY_CLI_DISABLE_AUTO_UPDATE", true)

	risky, evidence, err := defaultSelfUpdateEvidence("agy")

	if risky || evidence != "" || err != nil {
		t.Fatalf("agy launched with its self-updater off is frozen at the source; got risky=%v evidence=%q err=%v", risky, evidence, err)
	}
}

func TestDefaultSelfUpdateEvidence_AManifestThatLeavesItsOffSwitchUnsetIsRisky(t *testing.T) {
	stubManifestOffSwitch(t, "AGY_CLI_DISABLE_AUTO_UPDATE", false)

	risky, evidence, err := defaultSelfUpdateEvidence("agy")

	if !risky || err != nil || !strings.Contains(evidence, "AGY_CLI_DISABLE_AUTO_UPDATE") || !strings.Contains(evidence, "agy-tmux") {
		t.Fatalf("an unset off switch lets agy update itself on launch; got risky=%v evidence=%q err=%v", risky, evidence, err)
	}
}

func TestRun_VersionFreeze_TheShippedAgyManifestFreezesAgyWithoutABrewPin(t *testing.T) {
	r, err := Run(agyFreezeOptions(t, nil))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCheck(t, r, "cli-version-freeze"); c.Level != LevelPass {
		t.Fatalf("agy-tmux ships with AGY_CLI_DISABLE_AUTO_UPDATE in default_env, so agy is frozen; got %s (%s)", c.Level, c.Detail)
	}
}

func TestRun_VersionFreeze_AnAgyManifestWithoutItsOffSwitchHaltsNamingTheManifestFix(t *testing.T) {
	stubManifestOffSwitch(t, "AGY_CLI_DISABLE_AUTO_UPDATE", false)

	r, err := Run(agyFreezeOptions(t, nil))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-freeze")
	if c.Level != LevelHalt {
		t.Fatalf("a self-updating agy must halt the batch; got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "set AGY_CLI_DISABLE_AUTO_UPDATE in the agy-tmux manifest's default_env") || strings.Contains(c.Detail, "brew pin agy") {
		t.Errorf("agy is not brew-installed; the remedy is the manifest's off switch, not a brew pin: %q", c.Detail)
	}
}
