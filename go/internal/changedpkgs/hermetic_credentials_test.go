package changedpkgs

import (
	"os"
	"testing"
)

func TestMain_IsolatesHostCredentialsForEveryTestInChangedpkgs(t *testing.T) {
	if got := os.Getenv("GIT_CONFIG_GLOBAL"); got != os.DevNull {
		t.Errorf("GIT_CONFIG_GLOBAL = %q, want %q, so a git in this package reads the host config", got, os.DevNull)
	}
	configDir := os.Getenv("GH_CONFIG_DIR")
	if configDir == "" {
		t.Fatal("GH_CONFIG_DIR is empty, so a gh in this package reads the host login")
	}
	if _, err := os.Stat(configDir); err != nil {
		t.Errorf("GH_CONFIG_DIR %s: %v", configDir, err)
	}
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v, ok := os.LookupEnv(key); ok {
			t.Errorf("%s is set (%q) in the test binary", key, v)
		}
	}
}
