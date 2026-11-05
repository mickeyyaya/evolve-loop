package bridge

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/credsource"
)

func TestCredentialSource_ACellEnvironmentHasNoSourceVariable(t *testing.T) {
	if len(credsource.ChildVariables) == 0 {
		t.Fatal("credsource.ChildVariables names no variable, so this test proves nothing")
	}
	for _, name := range credsource.ChildVariables {
		t.Setenv(name, "tok-from-the-operator-shell")
	}
	for _, cli := range manifestCLIs(t) {
		cfg := seatConfig(cli)
		names := envNames(dispatchEnv(Deps{EnvStage: "enforce"}, cfg))
		for _, name := range credsource.ChildVariables {
			if slices.Contains(names, name) {
				t.Errorf("%s: the cell environment carries the source variable %s", cli, name)
			}
			if _, ok := cfg.Realization.Env[name]; ok || slices.Contains(cfg.Realization.EnvPassthrough, name) {
				t.Errorf("%s: the manifest names the source variable %s", cli, name)
			}
		}
	}
}

func TestParseManifest_RefusesEachCredentialSourceVariable(t *testing.T) {
	t.Parallel()
	for _, name := range credsource.ChildVariables {
		for field, body := range map[string]string{
			"env_passthrough": `{"cli":"hypo","binary":"h","env_passthrough":["` + name + `"]}`,
			"default_env":     `{"cli":"hypo","binary":"h","default_env":{"` + name + `":"x"}}`,
		} {
			if _, err := parseManifest("hypo", []byte(body)); err == nil || !strings.Contains(err.Error(), field) {
				t.Errorf("%s %s: err = %v, want a refusal", field, name, err)
			}
		}
	}
}
