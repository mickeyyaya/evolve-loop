package bridge

import (
	"context"
	"io"
	"slices"
	"testing"
)

var githubTokenKeys = []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}

func TestCellEnv_NoGitHubTokenReachesAnyCarrier(t *testing.T) {
	for _, key := range githubTokenKeys {
		t.Setenv(key, "parent-"+key)
	}
	isolationParent(t)
	manifest := map[string]string{"KEEP": "1"}
	pipeline := map[string]string{}
	for _, key := range githubTokenKeys {
		manifest[key] = "manifest-" + key
		pipeline[key] = "pipeline-" + key
	}
	for _, stage := range []string{"off", "shadow", "enforce"} {
		cfg := isolationConfig(t)
		cfg.Realization.Env = manifest
		deps := Deps{EnvStage: stage, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil), Env: pipeline}
		env, _ := deps.cellEnv(cfg)
		for _, key := range githubTokenKeys {
			if v, ok := lastEnvValue(env, key); ok {
				t.Errorf("%s cell env: %s = %q reached the cell", stage, key, v)
			}
			if v, ok := lastEnvValue(dispatchEnv(deps, cfg), key); ok {
				t.Errorf("%s headless env: %s = %q reached the cell", stage, key, v)
			}
		}
	}

	tm := &fakeTmux{}
	paneCfg := isolationConfig(t)
	if err := primePaneShell(context.Background(), Deps{Tmux: tm, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil)}, paneCfg, "s", "/wt"); err != nil {
		t.Fatal(err)
	}
	for _, key := range githubTokenKeys {
		if !slices.Contains(tm.sentKeys, "unset "+key) {
			t.Errorf("the pane keys lack %q: %q", "unset "+key, tm.sentKeys)
		}
	}
}
