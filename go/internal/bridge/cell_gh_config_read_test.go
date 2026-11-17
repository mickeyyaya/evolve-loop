package bridge

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

func ghConfigFixture(t *testing.T, c readCell) (dir string) {
	t.Helper()
	dir = filepath.Join(c.home, ".config", "gh")
	writeFixtureFile(t, filepath.Join(dir, "hosts.yml"), "github.com:\n  oauth_token: fixture\n")
	writeFixtureFile(t, filepath.Join(dir, "config.yml"), "git_protocol: https\n")
	return dir
}

func underGhConfig(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/.config/gh")
}

func TestReadIsolation_TheHostGhConfigDirIsDeniedAndNoGrantReachesIt(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "enforce"} {
		c := newReadCell(t)
		c.index(t)
		ghDir := ghConfigFixture(t, c)
		ri := Deps{ReadStage: stage}.readIsolation(c.request(), c.home)

		found := false
		for _, d := range ri.SecretDirs {
			found = found || d == ghDir
		}
		if !found {
			t.Errorf("%s: SecretDirs %v lacks the gh config dir %s", stage, ri.SecretDirs, ghDir)
		}
		for _, p := range append(append([]string{}, ri.Grants...), ri.ReportOnly...) {
			if underGhConfig(p) {
				t.Errorf("%s: a read grant reaches the gh config: %s", stage, p)
			}
		}

		sbpl := sandbox.GenerateSBPL(sandbox.Config{ReadIsolation: ri})
		want := `(deny file-read-data (subpath "` + ghDir + `") (with message "evolve-cell=d42 rule=read.home.secret inv=I5"))`
		if !strings.Contains(sbpl, want) {
			t.Errorf("%s: the profile lacks %s:\n%s", stage, want, sbpl)
		}
		for _, line := range strings.Split(sbpl, "\n") {
			if underGhConfig(line) && strings.Contains(line, "(allow ") {
				t.Errorf("%s: the profile allows a read under the gh config: %s", stage, line)
			}
		}
	}
}

func TestReadIsolation_AMissingHomeOrAMissingGhDirLeavesNoGhPathInTheGrants(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	for name, home := range map[string]string{"empty home": "", "home without gh": c.home} {
		ri := Deps{ReadStage: "enforce"}.readIsolation(c.request(), home)
		var all []string
		all = append(all, ri.Grants...)
		all = append(all, ri.ReportOnly...)
		all = append(all, ri.SecretDirs...)
		all = append(all, ri.SecretFiles...)
		for _, p := range all {
			if underGhConfig(p) {
				t.Errorf("%s: a gh config path is in the read isolation: %s", name, p)
			}
		}
	}
}

func TestCellEnv_NoOptInOrPassthroughLetsAGitHubTokenIntoACell(t *testing.T) {
	for _, key := range githubTokenKeys {
		t.Setenv(key, "parent-"+key)
	}
	isolationParent(t)
	pipeline := map[string]string{}
	for _, key := range githubTokenKeys {
		pipeline[key] = "pipeline-" + key
	}
	for _, stage := range []string{"off", "shadow", "enforce"} {
		cfg := isolationConfig(t)
		cfg.Realization.EnvPassthrough = append([]string{"GH_HOST"}, githubTokenKeys...)
		deps := Deps{
			EnvStage: stage, Stderr: io.Discard, HostGitConfig: fakeHostGit(nil), Env: pipeline,
			AllowAnthropicBaseURL: true, AllowOpenAIAPIKey: true,
		}
		env, _ := deps.cellEnv(cfg)
		for where, got := range map[string][]string{"cellEnv": env, "headless": dispatchEnv(deps, cfg)} {
			for _, key := range githubTokenKeys {
				if v, ok := lastEnvValue(got, key); ok {
					t.Errorf("%s %s: %s = %q reached the cell although it is on the passthrough list", stage, where, key, v)
				}
			}
		}
	}
}
