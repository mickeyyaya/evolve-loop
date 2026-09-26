package bridge

// manifest_default_env_test.go — a CLI manifest declares the environment its process runs in.
//
// `default_env` is to environment variables what `default_args` is to flags: the manifest's always-on
// channel, read once by the realizer. A headless driver hands the variables to the process; a tmux driver
// exports them in the pane shell before the launch command (driver_tmux_boot.go). Keys must be shell
// identifiers because the pane path writes `export KEY=value`.

import (
	"maps"
	"reflect"
	"strings"
	"testing"
)

func TestParseManifest_DefaultEnvIsRead(t *testing.T) {
	m, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","default_env":{"CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION":"false"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.DefaultEnv["CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"]; got != "false" {
		t.Fatalf("default_env not read: %v", m.DefaultEnv)
	}
}

func TestParseManifest_DefaultEnvRefusesANonIdentifierKey(t *testing.T) {
	for _, key := range []string{"A B", "", "1ABC", "K=V", "K;rm"} {
		_, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","default_env":{"`+key+`":"1"}}`))
		if err == nil {
			t.Fatalf("key %q must be refused: the pane path writes `export KEY=value`", key)
		}
		if !strings.Contains(err.Error(), "default_env") {
			t.Fatalf("the refusal must name default_env; got %v", err)
		}
	}
	if _, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","default_env":{"_ok_9":"1"}}`)); err != nil {
		t.Fatalf("an identifier key must be accepted: %v", err)
	}
}

func TestParseManifest_DefaultEnvRefusesTheLoopsBridgesAndCredentialVariables(t *testing.T) {
	for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "OPENAI_API_KEY", "EVOLVE_PROJECT_ROOT", "EVOLVE_FLEET", "BRIDGE_TESTING", "BRIDGE_ALLOW_ANTHROPIC_BASE_URL"} {
		_, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","default_env":{"`+key+`":"x"}}`))
		if err == nil || !strings.Contains(err.Error(), "default_env") {
			t.Fatalf("key %q must be refused: the credential guards and the loop's IPC never see a manifest's map; got %v", key, err)
		}
	}
}

func TestParseManifest_DefaultEnvRefusesControlBytesInAValue(t *testing.T) {
	for _, value := range []string{`a\nb`, `a\u001bb`, `a\rb`, `\u007f`} {
		_, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","default_env":{"K":"`+value+`"}}`))
		if err == nil || !strings.Contains(err.Error(), "default_env") {
			t.Fatalf("value %q must be refused: it is typed into a pane; got %v", value, err)
		}
	}
	if _, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","default_env":{"K":"it's ok; $HOME"}}`)); err != nil {
		t.Fatalf("shell metacharacters are quoted, not refused: %v", err)
	}
}

func TestRealize_DefaultEnvIsCopiedIntoTheRealization(t *testing.T) {
	m := Manifest{CLI: "hypo", DefaultEnv: map[string]string{"B": "2", "A": "1"}}
	got := Realize(m, LaunchIntent{})
	if !maps.Equal(got.Env, m.DefaultEnv) {
		t.Fatalf("Realization.Env = %v, want the manifest's %v", got.Env, m.DefaultEnv)
	}
	m.DefaultEnv["A"] = "changed"
	if got.Env["A"] != "1" {
		t.Fatal("the realization must hold its own copy of the manifest's map")
	}
	if bare := Realize(Manifest{CLI: "bare"}, LaunchIntent{}); bare.Env != nil {
		t.Fatalf("no default_env must leave Env nil; got %v", bare.Env)
	}
}

func TestDriverEnv_LayersTheCLIEnvUnderTheRequestOverrides(t *testing.T) {
	env := driverEnv(Deps{Env: map[string]string{"REQ": "r"}}, map[string]string{"Z": "1", "A": "two words"})
	tail := env[len(env)-3:]
	want := []string{"A=two words", "Z=1", "REQ=r"}
	if !reflect.DeepEqual(tail, want) {
		t.Fatalf("env tail = %v, want the CLI's variables sorted, then the request's (the last value of a key wins)", tail)
	}
	if n := len(driverEnv(Deps{}, nil)); n == 0 {
		t.Fatal("the process environment must still be the base")
	}
}

func TestExportLines_SortedAndShellQuoted(t *testing.T) {
	got := exportLines(map[string]string{"Z": "1", "A": "it's false"})
	want := []string{"export A='it'\\''s false'", "export Z=1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exportLines = %q, want %q", got, want)
	}
	if got := exportLines(nil); got != nil {
		t.Fatalf("no variables → no lines; got %q", got)
	}
}
