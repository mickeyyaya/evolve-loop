package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

func TestSeatSettingsAllowlist_CarriesNoKeyThatWidensAuthority(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"permissions", "hooks", "env", "apiKeyHelper", "skipDangerousModePermissionPrompt", "awsAuthRefresh", "awsCredentialExport", "forceLoginMethod", "enableAllProjectMcpServers", "disableAllHooks", "enabledPlugins", "statusLine", "autoMemoryEnabled"} {
		if seatCarriedSettings[k] {
			t.Errorf("settings key %q must not be carried into a seat", k)
		}
	}
	if len(seatCarriedSettings) != 9 {
		t.Errorf("allowlist has %d keys, want 9", len(seatCarriedSettings))
	}
}

func TestProvisionSeatSettings_AFileInTheWayOfTheDirectoryFails(t *testing.T) {
	home := seatHome(t)
	blocker := filepath.Join(home, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provisionSeatSettings(home, filepath.Join(blocker, "seat")); err == nil {
		t.Fatal("want an error when the seat directory cannot be created")
	}
}

func TestProvisionSeatSettings_ParallelLaunchesAllSucceedAndLeaveWholeJSON(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, ".evolve-seat")
	var wg sync.WaitGroup
	errs := make(chan error, 8*50)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 50; round++ {
				errs <- provisionSeatSettings(home, dir)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a parallel launch failed: %v", err)
		}
	}
	if got := readSeatSettings(t, dir); got["autoMemoryEnabled"] != false {
		t.Fatalf("got %v", got)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*.tmp*"))
	if len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestProvisionSeatSettings_ABlockedTargetFailsTheRename(t *testing.T) {
	home := seatHome(t)
	dir := filepath.Join(home, ".evolve-seat")
	if err := os.MkdirAll(filepath.Join(dir, "settings.json", "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := provisionSeatSettings(home, dir); err == nil {
		t.Fatal("want an error when the settings file cannot be replaced")
	}
}

func TestSeatSettings_CarriesOnlyTheCarriedKeysAndForcesAutoMemoryOff(t *testing.T) {
	home := seatHome(t)
	user := map[string]any{
		"model": "opus", "theme": "dark", "hooks": map[string]any{"Stop": []any{}},
		"env": map[string]any{"X": "1"}, "autoMemoryEnabled": true, "skipDangerousModePermissionPrompt": true,
		"someKeyNobodyDecided": 1,
	}
	body, _ := json.Marshal(user)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".evolve-seat")
	if err := provisionSeatSettings(home, dir); err != nil {
		t.Fatal(err)
	}
	got := readSeatSettings(t, dir)
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"autoMemoryEnabled", "model", "theme"}) || got["autoMemoryEnabled"] != false || got["model"] != "opus" {
		t.Fatalf("got %v", got)
	}
	info, _ := os.Stat(filepath.Join(dir, "settings.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
}

func TestSeatSettings_AMissingOrBadUserFileStillGivesTheMemoryOffFile(t *testing.T) {
	for name, body := range map[string]*string{"missing": nil, "bad": ptr("{not json")} {
		home := seatHome(t)
		if body != nil {
			os.MkdirAll(filepath.Join(home, ".claude"), 0o700)
			os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(*body), 0o600)
		}
		dir := filepath.Join(home, ".evolve-seat")
		if err := provisionSeatSettings(home, dir); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := readSeatSettings(t, dir); len(got) != 1 || got["autoMemoryEnabled"] != false {
			t.Fatalf("%s: got %v", name, got)
		}
	}
}

func TestLaunchSwept_AnIsolatedLaunchProvisionsTheSeatSettings(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	d := &seatStubDriver{}
	if rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, d.deps()); rc != ExitOK || err != nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	if got := readSeatSettings(t, filepath.Join(home, ".evolve-seat")); got["autoMemoryEnabled"] != false {
		t.Fatalf("got %v", got)
	}
}

func TestLaunchSwept_ANoTokenLaunchWritesNoSeatSettings(t *testing.T) {
	home := seatHome(t)
	d := &seatStubDriver{}
	launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, d.deps())
	if _, err := os.Stat(filepath.Join(home, ".evolve-seat")); !os.IsNotExist(err) {
		t.Fatalf("err=%v: a host with no token must get no seat directory", err)
	}
}

func ptr(s string) *string { return &s }

func readSeatSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func absentStageDeps(t *testing.T, d *seatStubDriver, policyBody string) Deps {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyBody), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := d.deps()
	deps.EnvStage = ""
	deps.LookupEnv = func(k string) (string, bool) {
		if k == "EVOLVE_PROJECT_ROOT" {
			return root, true
		}
		return "", false
	}
	return deps
}

func TestLaunchSwept_AnAbsentIsolationI2WithATokenIsHermeticWithNoRefusal(t *testing.T) {
	for name, body := range map[string]string{"no isolation block": `{}`, "i2 without a stage": `{"isolation":{"i2":{}}}`} {
		home := seatHome(t)
		plantHolder(t, home, seatCanaryToken, 0o600)
		d := &seatStubDriver{}
		rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, absentStageDeps(t, d, body))
		if rc != ExitOK || err != nil {
			t.Fatalf("%s: rc=%d err=%v", name, rc, err)
		}
		if tok, _ := seatEnvValue(d.env, "CLAUDE_CODE_OAUTH_TOKEN"); tok != seatCanaryToken {
			t.Errorf("%s: the token did not reach the hermetic env", name)
		}
		if dir, _ := seatEnvValue(d.env, "CLAUDE_CONFIG_DIR"); dir != filepath.Join(home, ".evolve-seat") {
			t.Errorf("%s: CLAUDE_CONFIG_DIR=%q", name, dir)
		}
	}
}

func TestLaunchSwept_AnAbsentIsolationI2WithNoTokenCarriesNoSeatEnv(t *testing.T) {
	seatHome(t)
	d := &seatStubDriver{}
	rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, absentStageDeps(t, d, `{}`))
	if rc != ExitOK || err != nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	for _, name := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CONFIG_DIR"} {
		if _, ok := seatEnvValue(d.env, name); ok {
			t.Errorf("%s present with no token", name)
		}
	}
}

func TestLaunchSwept_AnExplicitI2OffStillRefusesATokenSeat(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	d := &seatStubDriver{}
	rc, _ := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, absentStageDeps(t, d, `{"isolation":{"i2":{"stage":"off"}}}`))
	if rc != ExitSeatTokenRefused {
		t.Fatalf("rc=%d", rc)
	}
}
