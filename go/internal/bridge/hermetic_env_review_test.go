package bridge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func enforcePolicyRoot(t *testing.T, stage string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"isolation":{"i2":{"stage":"` + stage + `"}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
}

func TestCellEnv_EveryEntryWithNoStageReadsThePolicy(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	enforcePolicyRoot(t, "enforce")
	cfg := seatConfig("claude-tmux")
	entries := map[string][]string{
		"dispatchEnv": dispatchEnv(Deps{}, cfg),
		"paneEnv":     Deps{}.paneEnv(cfg),
		"probeEnv":    probeEnv(Deps{}, "claude"),
	}
	for entry, env := range entries {
		if env == nil || slices.Contains(envNames(env), "FOO_LEAK") {
			t.Errorf("%s with an empty Deps.EnvStage must follow the policy enforce; env names %v", entry, envNames(env))
		}
	}
	if !slices.Contains(dispatchEnv(Deps{EnvStage: "off"}, cfg), "FOO_LEAK=secret-value-1") {
		t.Error("an explicit Deps.EnvStage wins over the policy")
	}
}

func TestCellEnv_NoPolicyFileMeansShadow(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	if !slices.Contains(dispatchEnv(Deps{}, seatConfig("codex")), "FOO_LEAK=secret-value-1") {
		t.Fatal("with no policy the stage is the compiled default shadow, so the child keeps today's env")
	}
}

func TestProcessEnv_FollowsTheStage(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	enforcePolicyRoot(t, "off")
	if !slices.Contains(probeEnv(Deps{}, "codex"), "FOO_LEAK=secret-value-1") {
		t.Fatal("probeEnv at off must keep today's env")
	}
}

var gateAndBypassNames = []string{
	"EVOLVE_BYPASS_SHIP_GATE", "EVOLVE_BYPASS_ROLE_GATE", "EVOLVE_BYPASS_SHIP_VERIFY", "EVOLVE_BYPASS_PREFIX_GATE",
	"EVOLVE_SANDBOX", "EVOLVE_SANDBOX_MODE", ipcenv.MsgTokenKey, "BRIDGE_ALLOW_BYPASS", "BRIDGE_TESTING",
}

func TestN1_GateAndBypassVariablesReachNoSeatAtEnforce(t *testing.T) {
	pipeline := map[string]string{"EVOLVE_PROJECT_ROOT": "/repo", ipcenv.CycleStateFileKey: "/runs/c/cycle-state.json"}
	for _, name := range gateAndBypassNames {
		t.Setenv(name, "1")
		pipeline[name] = "1"
	}
	for _, cli := range manifestCLIs(t) {
		env := dispatchEnv(Deps{EnvStage: "enforce", Env: pipeline}, seatConfig(cli))
		names := envNames(env)
		for _, name := range gateAndBypassNames {
			if slices.Contains(names, name) {
				t.Errorf("%s: %s reached the seat at enforce", cli, name)
			}
		}
		for _, marker := range []string{"EVOLVE_PROJECT_ROOT", ipcenv.CycleStateFileKey} {
			if !slices.Contains(names, marker) {
				t.Errorf("%s: the pipeline marker %s must reach the seat", cli, marker)
			}
		}
	}
}

func TestCellEnv_AnOptInPassesItsTargetAtEnforce(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "https://proxy.example")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("BRIDGE_ALLOW_ANTHROPIC_BASE_URL", "")
	t.Setenv("BRIDGE_ALLOW_OPENAI_API_KEY", "")
	if names := envNames(dispatchEnv(Deps{EnvStage: "enforce"}, seatConfig("claude-p"))); slices.Contains(names, "ANTHROPIC_BASE_URL") {
		t.Fatal("without the opt-in, ANTHROPIC_BASE_URL must not pass")
	}
	t.Setenv("BRIDGE_ALLOW_ANTHROPIC_BASE_URL", "1")
	if !slices.Contains(dispatchEnv(Deps{EnvStage: "enforce"}, seatConfig("claude-p")), "ANTHROPIC_BASE_URL=https://proxy.example") {
		t.Fatal("with BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1 the opted-in endpoint must reach claude, or the route changes silently")
	}
	deps := Deps{EnvStage: "enforce", Env: map[string]string{"BRIDGE_ALLOW_OPENAI_API_KEY": "1"}}
	if !slices.Contains(dispatchEnv(deps, seatConfig("codex")), "OPENAI_API_KEY=sk-test") {
		t.Fatal("an opt-in set through Deps.Env must pass its target too")
	}
}

func TestNewSessionEnvArgs_AlwaysKeepsTheTermThatTmuxSets(t *testing.T) {
	t.Parallel()
	args := newSessionEnvArgs("s", 1, 1, "", "/e/env", "fp")
	if !slices.ContainsFunc(args, func(a string) bool { return strings.Contains(a, `TERM="$TERM"`) }) {
		t.Fatalf("the pane must keep the TERM that tmux sets, even when the launcher has none: %q", args)
	}
}

func TestManifests_DeclareTheDocumentedAuthAndProxyNames(t *testing.T) {
	t.Parallel()
	proxies := []string{"https_proxy", "http_proxy", "no_proxy", "ALL_PROXY", "all_proxy"}
	want := map[string][]string{
		"claude-tmux":     {"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "AWS_PROFILE", "AWS_REGION", "ANTHROPIC_VERTEX_PROJECT_ID", "CLOUD_ML_REGION"},
		"claude-p":        {"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX"},
		"agy":             {"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS", "GOOGLE_CLOUD_PROJECT"},
		"agy-tmux":        {"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS"},
		"agy-claude-tmux": {"GEMINI_API_KEY"},
		"codex":           {},
		"codex-tmux":      {},
		"ollama-tmux":     {},
	}
	for cli, names := range want {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range append(names, proxies...) {
			if !slices.Contains(m.EnvPassthrough, name) {
				t.Errorf("%s: env_passthrough lacks %s", cli, name)
			}
		}
	}
	codex, _ := LoadManifest("codex")
	if slices.Contains(codex.EnvPassthrough, "GEMINI_API_KEY") || slices.Contains(codex.EnvPassthrough, "CLAUDE_CODE_OAUTH_TOKEN") {
		t.Error("a family declares only its own auth names, never another family's")
	}
}

func TestEnvStage_ReadsThePolicyOfTheProjectRootInDepsEnv(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	enforcePolicyRoot(t, "enforce")
	enforced := os.Getenv("EVOLVE_PROJECT_ROOT")
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	deps := Deps{Env: map[string]string{"EVOLVE_PROJECT_ROOT": enforced}}
	if slices.Contains(envNames(dispatchEnv(deps, seatConfig("codex"))), "FOO_LEAK") {
		t.Fatal("the stage must come from the project root that the dispatch carries in Deps.Env")
	}
}

func TestEnvStage_AnUnreadablePolicyWarnsLoudly(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stderr strings.Builder
	env := dispatchEnv(Deps{Stderr: &stderr}, seatConfig("codex"))
	if !strings.Contains(stderr.String(), "WARN isolation stage") || !slices.Contains(env, "FOO_LEAK=secret-value-1") {
		t.Fatalf("a bad policy must warn and keep the shadow env; stderr=%q", stderr.String())
	}
	if got := (Deps{}).envStage(); got != "shadow" {
		t.Fatalf("with no stderr the warning goes to the process stderr and the stage is shadow; got %q", got)
	}
}
