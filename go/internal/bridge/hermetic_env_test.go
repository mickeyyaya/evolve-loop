package bridge

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var launcherLeaks = map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_ENTRYPOINT": "cli", "FOO_LEAK": "secret-value-1"}

var pollutedEnv = map[string]string{
	"CLAUDECODE": "1", "CLAUDE_CODE_ENTRYPOINT": "cli", "FOO_LEAK": "secret-value-1",
	"PATH": "/opt/fake/bin:/usr/bin", "HOME": "/Users/cell",
}

func envNames(env []string) []string {
	names := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		names = append(names, name)
	}
	return names
}

func manifestCLIs(t *testing.T) []string {
	t.Helper()
	entries, err := embeddedManifests.ReadDir("manifests")
	if err != nil {
		t.Fatal(err)
	}
	var clis []string
	for _, e := range entries {
		clis = append(clis, strings.TrimSuffix(e.Name(), ".json"))
	}
	if len(clis) < 8 {
		t.Fatalf("found %d manifests, want the 8 families", len(clis))
	}
	return clis
}

func seatConfig(cli string) *Config {
	return &Config{CLI: cli, Agent: "build", Cycle: 7, RunID: "run-n1", DispatchID: thisDispatch, Realization: RealizeFor(cli, LaunchIntent{})}
}

func TestN1_LauncherClaudeVariablesReachNoSeatAtEnforce(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	t.Setenv("CODEX_HOME", "/Users/cell/.codex-alt")
	for _, cli := range manifestCLIs(t) {
		cfg := seatConfig(cli)
		env := dispatchEnv(Deps{EnvStage: "enforce", Env: map[string]string{"EVOLVE_PROJECT_ROOT": "/repo"}}, cfg)
		names := envNames(env)
		for leak := range launcherLeaks {
			if slices.Contains(names, leak) {
				t.Errorf("%s: launcher variable %s reached the seat at enforce", cli, leak)
			}
		}
		for _, want := range []string{"PATH=/opt/fake/bin:/usr/bin", "HOME=/Users/cell", "EVOLVE_PROJECT_ROOT=/repo", ipcenv.DispatchIDKey + "=" + thisDispatch} {
			if !slices.Contains(env, want) {
				t.Errorf("%s: the seat lost %s; env names %v", cli, want, names)
			}
		}
		for k, v := range cfg.Realization.Env {
			if !slices.Contains(env, k+"="+v) {
				t.Errorf("%s: the seat lost its manifest default_env %s", cli, k)
			}
		}
		if got := slices.Contains(names, "CODEX_HOME"); got != slices.Contains(cfg.Realization.EnvPassthrough, "CODEX_HOME") {
			t.Errorf("%s: CODEX_HOME present=%v, but only a manifest that lists it may pass it", cli, got)
		}
	}
}

func TestCellEnv_ShadowKeepsTodaysEnvAndReportsEachDroppedNameWithoutItsValue(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	center := signalcenter.New()
	deps := Deps{Signals: center, EnvDrops: &EnvDropLog{}}
	cfg := seatConfig("claude-tmux")

	env := dispatchEnv(deps, cfg)

	if !slices.Contains(env, "FOO_LEAK=secret-value-1") {
		t.Fatal("shadow must keep today's environment for the child")
	}
	got := isolationEvents(center)
	for leak := range launcherLeaks {
		e, ok := got[leak]
		if !ok {
			t.Errorf("no %s for the dropped name %s", CodeIsolationI2Violation, leak)
			continue
		}
		if e.Fields["stage"] != "shadow" || e.Fields["action"] != "logged" || e.Fields["family"] != "claude-tmux" || e.Fields["cell"] != "build" || e.Cycle != 7 {
			t.Errorf("%s event = %+v", leak, e)
		}
	}
	for _, e := range center.Recent() {
		for k, v := range e.Fields {
			if strings.Contains(v, "secret-value-1") {
				t.Errorf("field %s carries a variable value: %q", k, v)
			}
		}
		if strings.Contains(e.Reason, "secret-value-1") {
			t.Errorf("reason carries a variable value: %q", e.Reason)
		}
	}
	if _, ok := got["PATH"]; ok {
		t.Error("PATH is in the base allowlist; it is not a dropped name")
	}
}

func TestCellEnv_EnforceReportsTheDropAsRefused(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	center := signalcenter.New()
	dispatchEnv(Deps{Signals: center, EnvStage: "enforce"}, seatConfig("codex"))
	if e, ok := isolationEvents(center)["FOO_LEAK"]; !ok || e.Fields["action"] != "refused" || e.Fields["stage"] != "enforce" {
		t.Fatalf("enforce must emit the drop with action=refused; got %+v (ok=%v)", e, ok)
	}
}

func TestCellEnv_OffKeepsTodaysEnvAndStaysSilent(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	center := signalcenter.New()
	env := dispatchEnv(Deps{Signals: center, EnvStage: "off"}, seatConfig("codex"))
	if !slices.Contains(env, "FOO_LEAK=secret-value-1") || len(isolationEvents(center)) != 0 {
		t.Fatalf("off = today's env and no signal; leak kept=%v events=%d", slices.Contains(env, "FOO_LEAK=secret-value-1"), len(isolationEvents(center)))
	}
}

func TestEnvDropLog_ReportsEachNameOnceForEachFamily(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	center := signalcenter.New()
	deps := Deps{Signals: center, EnvDrops: &EnvDropLog{}}
	dispatchEnv(deps, seatConfig("codex"))
	dispatchEnv(deps, seatConfig("codex"))
	dispatchEnv(deps, seatConfig("agy"))
	count := 0
	for _, e := range center.Recent() {
		if e.Code == CodeIsolationI2Violation && e.Fields["target"] == "FOO_LEAK" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("FOO_LEAK reported %d times, want once for codex and once for agy", count)
	}
	var none *EnvDropLog
	if got := none.firstSeen("codex", []string{"A", "B"}); !slices.Equal(got, []string{"A", "B"}) {
		t.Fatalf("a nil log reports every name; got %v", got)
	}
}

func isolationEvents(c *signalcenter.Center) map[string]signalcenter.Event {
	out := map[string]signalcenter.Event{}
	for _, e := range c.Recent() {
		if e.Code == CodeIsolationI2Violation {
			out[e.Fields["target"]] = e
		}
	}
	return out
}

func TestHermeticEnv_IsTheAllowlistThePassthroughTheManifestAndThePipeline(t *testing.T) {
	t.Parallel()
	parent := []string{
		"PATH=/usr/bin", "HOME=/h", "USER=u", "LOGNAME=u", "SHELL=/bin/zsh", "LANG=en_US.UTF-8", "LC_ALL=C", "LC_CTYPE=UTF-8",
		"TERM=xterm", "TMPDIR=/t", "TMP=/t", "TEMP=/t", "TZ=UTC", "EVOLVE_CYCLE_STATE_FILE=/c.json", "EVOLVE_BYPASS_SHIP_GATE=1", ipcenv.DispatchIDKey + "=stale",
		"HTTPS_PROXY=http://p", "CLAUDECODE=1", "GEMINI_API_KEY=k", "LC_BAD-NAME=x", "TMUX=/tmp/s",
	}
	got := hermeticEnv(parent, []string{"HTTPS_PROXY"}, map[string]string{"B_DEFAULT": "2", "A_DEFAULT": "1"}, map[string]string{"EVOLVE_PROJECT_ROOT": "/repo"})
	want := []string{
		"PATH=/usr/bin", "HOME=/h", "USER=u", "LOGNAME=u", "SHELL=/bin/zsh", "LANG=en_US.UTF-8", "LC_ALL=C", "LC_CTYPE=UTF-8",
		"TERM=xterm", "TMPDIR=/t", "TMP=/t", "TEMP=/t", "TZ=UTC", "EVOLVE_CYCLE_STATE_FILE=/c.json", "HTTPS_PROXY=http://p",
		"A_DEFAULT=1", "B_DEFAULT=2", "EVOLVE_PROJECT_ROOT=/repo",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("hermeticEnv =\n  %q\nwant\n  %q", got, want)
	}
	if dropped := droppedEnvNames(parent, []string{"HTTPS_PROXY"}); !slices.Equal(dropped, []string{"CLAUDECODE", "EVOLVE_BYPASS_SHIP_GATE", "GEMINI_API_KEY", "LC_BAD-NAME", "TMUX"}) {
		t.Fatalf("dropped = %q", dropped)
	}
}

func TestProcessEnv_IsTheAllowlistPlusTheManifest(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	enforcePolicyRoot(t, "enforce")
	t.Setenv("CLAUDE_CONFIG_DIR", "/Users/cell/.claude-alt")
	agy := ProcessEnv("agy")
	if !slices.Contains(agy, agyAutoUpdateOffEnv+"=1") {
		t.Errorf("a bare agy exec must carry %s=1 from the agy-tmux manifest", agyAutoUpdateOffEnv)
	}
	claude := ProcessEnv("claude")
	if !slices.Contains(claude, "CLAUDE_CONFIG_DIR=/Users/cell/.claude-alt") {
		t.Error("claude lists CLAUDE_CONFIG_DIR in env_passthrough, so a bare claude exec keeps it")
	}
	for _, family := range []string{"agy", "claude", "codex", "no-such-family"} {
		env := ProcessEnv(family)
		names := envNames(env)
		for leak := range launcherLeaks {
			if slices.Contains(names, leak) {
				t.Errorf("%s: %s reached a bare exec", family, leak)
			}
		}
		if !slices.Contains(env, "PATH=/opt/fake/bin:/usr/bin") || !slices.Contains(env, "HOME=/Users/cell") {
			t.Errorf("%s: a bare exec lost PATH or HOME", family)
		}
	}
	if slices.Contains(ProcessEnv("codex"), "CLAUDE_CONFIG_DIR=/Users/cell/.claude-alt") {
		t.Error("codex does not list CLAUDE_CONFIG_DIR, so it must not get it")
	}
}

func TestDoctorProbe_RunsWithTheFamilyCellEnv(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	seen := map[string][]string{}
	e := envCapturingEngine(seen)
	e.deps.EnvStage = "enforce"
	if v := doctorVersion(context.Background(), e.deps, "agy"); v != "1.2.17" {
		t.Fatalf("version = %q", v)
	}
	if slices.Contains(envNames(seen["agy --version"]), "CLAUDECODE") {
		t.Error("the doctor's version probe must not inherit CLAUDECODE at enforce")
	}
}

func TestNewEngine_GivesEachEngineADropLog(t *testing.T) {
	t.Parallel()
	shared := &EnvDropLog{}
	if NewEngine(Deps{}).deps.EnvDrops == nil {
		t.Fatal("an engine with no drop log must get its own, so one dispatch reports each name once")
	}
	if NewEngine(Deps{EnvDrops: shared}).deps.EnvDrops != shared {
		t.Fatal("a caller's drop log must be kept")
	}
}

func TestHermeticEnv_EachNameOnceAndOnlyShellIdentifiers(t *testing.T) {
	t.Parallel()
	got := hermeticEnv(
		[]string{"PATH=/usr/bin", "HOME=/h", "EVOLVE_PROJECT_ROOT=/old"},
		nil,
		map[string]string{"PATH": "/manifest/bin", "X;rm -rf ~": "1"},
		map[string]string{"EVOLVE_PROJECT_ROOT": "/repo", "$(touch pwned)": "1", "PATH": "/pipeline/bin"},
	)
	want := []string{"HOME=/h", "EVOLVE_PROJECT_ROOT=/repo", "PATH=/manifest/bin"}
	if !slices.Equal(got, want) {
		t.Fatalf("hermeticEnv =\n  %q\nwant\n  %q", got, want)
	}
	args := newSessionEnvArgs("s", 1, 1, "", []string{"OK=1", "X;rm -rf ~=1"})
	if strings.Contains(args[len(args)-1], "rm -rf") || slices.Contains(args, "X;rm -rf ~=1") {
		t.Fatalf("a non-identifier name must never reach the pane command: %q", args)
	}
}
