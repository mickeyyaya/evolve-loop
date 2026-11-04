package main

import (
	"os"
	"testing"
)

func TestMain_IsolatesHostCredentialsForEveryTestInThePackage(t *testing.T) {
	t.Parallel()
	if got := os.Getenv("GIT_CONFIG_GLOBAL"); got != os.DevNull {
		t.Errorf("GIT_CONFIG_GLOBAL = %q, want %q", got, os.DevNull)
	}
	if got := os.Getenv("GIT_CONFIG_NOSYSTEM"); got != "1" {
		t.Errorf("GIT_CONFIG_NOSYSTEM = %q, want 1", got)
	}
	if got := os.Getenv("GIT_TERMINAL_PROMPT"); got != "0" {
		t.Errorf("GIT_TERMINAL_PROMPT = %q, want 0", got)
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
