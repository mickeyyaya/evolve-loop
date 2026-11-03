package bridge

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var cellToolViewKeys = []string{"GH_CONFIG_DIR", "GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_GLOBAL", "GIT_TERMINAL_PROMPT", "GIT_SSH_COMMAND"}

const cellSSHCommand = "ssh -F /dev/null -o IdentitiesOnly=yes -o IdentityFile=/dev/null -o BatchMode=yes"

func toolViewSeat(t *testing.T) *Config {
	t.Helper()
	cfg := seatConfig("claude-tmux")
	cfg.Workspace = t.TempDir()
	return cfg
}

func TestCellEnv_EnforceHoldsEachKeyOfTheCellToolView(t *testing.T) {
	t.Parallel()
	cfg := toolViewSeat(t)
	env, hermetic := Deps{EnvStage: "enforce"}.cellEnv(cfg)
	if !hermetic {
		t.Fatal("enforce must give the hermetic env")
	}
	cell := filepath.Join(cfg.Workspace, ".cell")
	want := map[string]string{
		"GH_CONFIG_DIR":       filepath.Join(cell, "gh"),
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   filepath.Join(cell, "gitconfig"),
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_SSH_COMMAND":     cellSSHCommand,
	}
	for _, key := range cellToolViewKeys {
		if got := envValue(env, key); got != want[key] {
			t.Errorf("%s = %q, want %q", key, got, want[key])
		}
	}
	entries, err := os.ReadDir(filepath.Join(cell, "gh"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("the bridge must make an empty gh config dir: entries=%v err=%v", entries, err)
	}
	gitconfig, err := os.ReadFile(filepath.Join(cell, "gitconfig"))
	if err != nil || string(gitconfig) != "[credential]\n\thelper =\n" {
		t.Fatalf("the bridge must write a gitconfig that resets the credential helper: %q err=%v", gitconfig, err)
	}
}

func TestCellEnv_TheCellToolViewBeatsAManifestDefaultEnv(t *testing.T) {
	t.Parallel()
	cfg := toolViewSeat(t)
	cfg.Realization.Env = map[string]string{"GIT_TERMINAL_PROMPT": "1", "KEEP_ME": "yes"}
	env, _ := Deps{EnvStage: "enforce"}.cellEnv(cfg)
	if got := envValue(env, "GIT_TERMINAL_PROMPT"); got != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, want the tool view value 0", got)
	}
	if !slices.Contains(env, "KEEP_ME=yes") {
		t.Fatal("the cell lost its manifest default_env")
	}
}

func TestCellEnv_ShadowAndOffAddNoCellToolView(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "off"} {
		cfg := toolViewSeat(t)
		env, _ := Deps{EnvStage: stage}.cellEnv(cfg)
		if got := envValue(env, "GIT_CONFIG_GLOBAL"); strings.HasPrefix(got, cfg.Workspace) {
			t.Errorf("%s: the cell got the tool view %q; it follows isolation.i2.stage", stage, got)
		}
		if _, err := os.Stat(filepath.Join(cfg.Workspace, ".cell")); !os.IsNotExist(err) {
			t.Errorf("%s: the bridge wrote the tool view: %v", stage, err)
		}
	}
}

func TestCellEnv_ACellWithNoWorkspaceGetsNoCellToolView(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", "/host/gh")
	cfg := seatConfig("claude-tmux")
	cfg.Realization.Env = nil
	env, _ := Deps{EnvStage: "enforce"}.cellEnv(cfg)
	for _, key := range cellToolViewKeys {
		if envValue(env, key) != "" {
			t.Errorf("%s is set for a cell with no workspace", key)
		}
	}
}

func TestCellEnv_AnUnwritableToolViewWarnsAndKeepsTheEnv(t *testing.T) {
	t.Parallel()
	for name, block := range map[string]func(ws string) error{
		"dir":  func(ws string) error { return os.WriteFile(filepath.Join(ws, ".cell"), nil, 0o600) },
		"file": func(ws string) error { return os.MkdirAll(filepath.Join(ws, ".cell", "gitconfig"), 0o700) },
	} {
		cfg := toolViewSeat(t)
		if err := block(cfg.Workspace); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		env, _ := Deps{EnvStage: "enforce", Stderr: &stderr}.cellEnv(cfg)
		if !strings.Contains(stderr.String(), "[bridge] WARN cell tool view:") {
			t.Errorf("%s: no warning on stderr: %q", name, stderr.String())
		}
		if got := envValue(env, "GIT_CONFIG_GLOBAL"); got != filepath.Join(cfg.Workspace, ".cell", "gitconfig") {
			t.Errorf("%s: the cell lost the tool view env: %q", name, got)
		}
	}
}

func TestParseCellToolView_TheEmbeddedDataHoldsEachKey(t *testing.T) {
	t.Parallel()
	v, err := parseCellToolView(cellToolViewRaw)
	if err != nil {
		t.Fatal(err)
	}
	keys := slices.Sorted(maps.Keys(v.Env))
	if !slices.Equal(keys, slices.Sorted(slices.Values(cellToolViewKeys))) {
		t.Fatalf("tool view keys = %v, want %v", keys, cellToolViewKeys)
	}
	if v.Dir != ".cell" || !slices.Equal(v.Dirs, []string{"gh"}) || len(v.Files) != 1 {
		t.Fatalf("tool view = %+v", v)
	}
}

func TestParseCellToolView_RefusesBadData(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string]string{
		"json":         `{`,
		"no dir":       `{"env":{"A":"1"}}`,
		"escaping dir": `{"dir":"../x","env":{"A":"1"}}`,
		"key":          `{"dir":".cell","env":{"A-B":"1"}}`,
		"reserved key": `{"dir":".cell","env":{"EVOLVE_PHASE":"x"}}`,
		"control byte": `{"dir":".cell","env":{"A":"x\u001b"}}`,
		"sub dir":      `{"dir":".cell","dirs":["/etc"]}`,
		"file":         `{"dir":".cell","files":{"../gitconfig":""}}`,
	} {
		if _, err := parseCellToolView([]byte(raw)); err == nil {
			t.Errorf("%s: parse accepted %s", name, raw)
		}
	}
}

func TestMustCellToolView_PanicsOnBadData(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("bad embedded data must stop the binary at start")
		}
	}()
	mustCellToolView([]byte(`{`))
}

func TestCellEnv_ARelativeWorkspaceGetsAnAbsoluteCellToolView(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	cfg := seatConfig("claude-tmux")
	cfg.Workspace = "ws"
	env, _ := Deps{EnvStage: "enforce"}.cellEnv(cfg)
	want, err := filepath.Abs(filepath.Join("ws", ".cell", "gitconfig"))
	if err != nil {
		t.Fatal(err)
	}
	if got := envValue(env, "GIT_CONFIG_GLOBAL"); got != want {
		t.Fatalf("GIT_CONFIG_GLOBAL = %q, want the absolute path %q: the cell runs in the worktree, not in the workspace", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the bridge must write the gitconfig at the absolute path: %v", err)
	}
}
