package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const seatCanaryToken = "sk-ant-oat01-CANARY-LAUNCH-0123456789"

type seatStubDriver struct {
	cfg      Config
	env      []string
	req      *SandboxWrapRequest
	launches int
	logins   []string
}

func (d *seatStubDriver) Name() string { return "seat-stub" }

func (d *seatStubDriver) Launch(_ context.Context, cfg *Config, deps Deps) (int, error) {
	d.launches++
	d.cfg = *cfg
	d.env = deps.paneEnv(cfg)
	sandboxPrefixForLaunch(deps, cfg, "")
	return ExitOK, nil
}

func (d *seatStubDriver) deps() Deps {
	return Deps{
		EnvStage:    "enforce",
		Stderr:      io.Discard,
		LookupEnv:   func(string) (string, bool) { return "", false },
		SandboxMode: "on",
		SandboxWrap: func(req SandboxWrapRequest) ([]string, bool) {
			d.req = &req
			return []string{"sandbox-exec", "-f", "x"}, true
		},
		onSeatLogin:   func(v string) { d.logins = append(d.logins, v) },
		ListProcesses: func(context.Context) ([]proctree.Process, error) { return nil, nil },
		ListCwds:      func(context.Context) ([]proctree.Cwd, error) { return nil, nil },
		SignalProcess: func(pid int, _ syscall.Signal) error {
			return fmt.Errorf("the test signals no process, got pid %d", pid)
		},
	}
}

func seatHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func plantHolder(t *testing.T, home, body string, mode os.FileMode) {
	t.Helper()
	path := paths.SeatTokenPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func seatEnvValue(env []string, name string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v, true
		}
	}
	return "", false
}

func TestLaunchSwept_ANoTokenHostLaunchesAsTodayWithTheStoredLogin(t *testing.T) {
	home := seatHome(t)
	d := &seatStubDriver{}
	cfg := &Config{CLI: "claude-tmux", Agent: "scout", Worktree: t.TempDir()}
	rc, err := launchSwept(context.Background(), d, cfg, d.deps())
	if rc != ExitOK || err != nil || d.launches != 1 {
		t.Fatalf("rc=%d err=%v launches=%d", rc, err, d.launches)
	}
	if d.cfg.Decision != (seatlogin.Decision{}) || d.cfg.RequireSandbox {
		t.Fatalf("cfg=%+v", d.cfg)
	}
	for _, name := range []string{"CLAUDE_CONFIG_DIR", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if v, ok := seatEnvValue(d.env, name); ok {
			t.Errorf("%s=%q reached a no-token seat", name, v)
		}
	}
	if d.req == nil || d.req.SeatRoot != "" || d.req.HostIsolated {
		t.Fatalf("req=%+v", d.req)
	}
	if _, err := os.Stat(paths.SeatRoot(home)); err == nil {
		t.Fatal("a no-token launch created the seat directory")
	}
	if !slices.Equal(d.logins, []string{"stored"}) {
		t.Fatalf("logins=%v", d.logins)
	}
}

func TestLaunchSwept_APlantedTokenGivesTheSeatShape(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken+"\n", 0o600)
	d := &seatStubDriver{}
	deps := d.deps()
	deps.EnvStage = "shadow"
	cfg := &Config{CLI: "claude-tmux", Agent: "scout"}
	rc, err := launchSwept(context.Background(), d, cfg, deps)
	if rc != ExitOK || err != nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	if dir, _ := seatEnvValue(d.env, "CLAUDE_CONFIG_DIR"); dir != paths.SeatRoot(home) {
		t.Errorf("CLAUDE_CONFIG_DIR=%q", dir)
	}
	if tok, _ := seatEnvValue(d.env, "CLAUDE_CODE_OAUTH_TOKEN"); tok != seatCanaryToken {
		t.Errorf("the token did not reach the hermetic env")
	}
	if !d.cfg.RequireSandbox || d.cfg.Decision.SeatToken != seatCanaryToken || !d.cfg.Decision.HostIsolated {
		t.Errorf("cfg=%+v", d.cfg.Decision)
	}
	if d.req == nil || d.req.SeatRoot != paths.SeatRoot(home) || !d.req.HostIsolated {
		t.Errorf("req=%+v", d.req)
	}
	info, err := os.Stat(paths.SeatRoot(home))
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("seat directory: %v %v", info, err)
	}
	if !slices.Equal(d.logins, []string{"isolated"}) {
		t.Fatalf("logins=%v", d.logins)
	}
}

func TestLaunchSwept_TheSeatDirectoryHoldsNoCredential(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	d := &seatStubDriver{}
	for i := 0; i < 2; i++ {
		if rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-p", Agent: "scout"}, d.deps()); rc != ExitOK || err != nil {
			t.Fatalf("pass %d: rc=%d err=%v", i, rc, err)
		}
	}
	err := filepath.WalkDir(paths.SeatRoot(home), func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || p == paths.SeatTokenPath(home) {
			return err
		}
		body, rerr := os.ReadFile(p)
		if rerr != nil || strings.Contains(string(body), seatCanaryToken) {
			t.Errorf("%s holds the token or cannot be read: %v", p, rerr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLaunchSwept_AParameterSeatBeatsTheHolderAndLeavesItUntouched(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, "sk-ant-oat01-HOLDER", 0o600)
	before, _ := os.ReadFile(paths.SeatTokenPath(home))
	param := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(param, []byte(seatCanaryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &seatStubDriver{}
	rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout", SeatTokenFile: param}, d.deps())
	if rc != ExitOK || err != nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	if tok, _ := seatEnvValue(d.env, "CLAUDE_CODE_OAUTH_TOKEN"); tok != seatCanaryToken {
		t.Error("the parameter token did not win")
	}
	after, _ := os.ReadFile(paths.SeatTokenPath(home))
	if string(before) != string(after) {
		t.Error("the holder changed")
	}
}

func TestLaunchSwept_AParameterSeatRunsOnAHostWithNoHolder(t *testing.T) {
	home := seatHome(t)
	param := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(param, []byte(seatCanaryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &seatStubDriver{}
	if rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout", SeatTokenFile: param}, d.deps()); rc != ExitOK || err != nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	if _, err := os.Stat(paths.SeatTokenPath(home)); err == nil {
		t.Fatal("a parameter seat wrote the holder")
	}
	if tok, _ := seatEnvValue(d.env, "CLAUDE_CODE_OAUTH_TOKEN"); tok != seatCanaryToken {
		t.Error("the parameter token did not reach the seat")
	}
}

func TestLaunchSwept_ABadHolderRefusesAClaudeSeatWithThePinnedTextAndNeverFallsBack(t *testing.T) {
	home := seatHome(t)
	for name, tc := range map[string]struct {
		body string
		mode os.FileMode
		want string
	}{
		"empty": {" \n", 0o600, "bridge: the seat token at %s is empty, so no claude seat can start. Next: run claude setup-token, then evolve auth seat-token set, or run evolve auth seat-token clear to go back to the stored login."},
		"mode":  {seatCanaryToken, 0o644, "bridge: the seat token at %s has mode 0644, not 0600, so no claude seat can start. Next: run evolve auth seat-token set again."},
	} {
		plantHolder(t, home, tc.body, tc.mode)
		for _, cli := range []string{"claude-tmux", "claude-p"} {
			d := &seatStubDriver{}
			rc, err := launchSwept(context.Background(), d, &Config{CLI: cli, Agent: "scout"}, d.deps())
			want := strings.Replace(tc.want, "%s", paths.SeatTokenPath(home), 1)
			if rc != ExitSeatTokenRefused || err == nil || err.Error() != want || d.launches != 0 {
				t.Errorf("%s/%s: rc=%d err=%v launches=%d", name, cli, rc, err, d.launches)
			}
			if err != nil && strings.Contains(err.Error(), seatCanaryToken) {
				t.Errorf("%s/%s: the refusal holds the token", name, cli)
			}
		}
	}
}

func TestLaunchSwept_ABadHolderLeavesACodexSeatWithNoRefusalNoTokenAndTheStoredLoginDeny(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, "  ", 0o600)
	d := &seatStubDriver{}
	rc, err := launchSwept(context.Background(), d, &Config{CLI: "codex-tmux", Agent: "scout", Worktree: t.TempDir()}, d.deps())
	if rc != ExitOK || err != nil || d.launches != 1 {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	if d.cfg.Decision != (seatlogin.Decision{HostIsolated: true}) || d.cfg.RequireSandbox {
		t.Fatalf("decision=%+v", d.cfg.Decision)
	}
	if d.req == nil || !d.req.HostIsolated || d.req.SeatRoot != "" {
		t.Fatalf("req=%+v", d.req)
	}
	if len(d.logins) != 0 {
		t.Fatalf("a codex launch recorded a claude login: %v", d.logins)
	}
}

func TestLaunchSwept_TheCloudSeatStaysOnTheStoredLoginAndTakesNoToken(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	d := &seatStubDriver{}
	if rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-cloud", Agent: "scout"}, d.deps()); rc != ExitOK || err != nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	if _, ok := seatEnvValue(d.env, "CLAUDE_CODE_OAUTH_TOKEN"); ok || d.cfg.Decision.SeatToken != "" || d.cfg.RequireSandbox {
		t.Fatalf("the cloud seat took the token: %+v", d.cfg.Decision)
	}
	if !d.cfg.Decision.HostIsolated || !slices.Equal(d.logins, []string{"stored"}) {
		t.Fatalf("decision=%+v logins=%v", d.cfg.Decision, d.logins)
	}
}

func TestLaunchSwept_ATokenSeatRefusesWhenConfinementIsOffOrTheEnvStageIsOff(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	for name, mutate := range map[string]func(*Deps){
		"sandbox off": func(d *Deps) { d.SandboxMode = "off" },
		"env off":     func(d *Deps) { d.EnvStage = "off" },
	} {
		d := &seatStubDriver{}
		deps := d.deps()
		mutate(&deps)
		rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, deps)
		if rc != ExitSeatTokenRefused || err == nil || d.launches != 0 || strings.Contains(err.Error(), seatCanaryToken) {
			t.Errorf("%s: rc=%d err=%v launches=%d", name, rc, err, d.launches)
		}
	}
}

func TestLaunchSwept_AWrapThatFailsRefusesATokenSeatAndNotAHostWithNoToken(t *testing.T) {
	home := seatHome(t)
	failing := func(SandboxWrapRequest) ([]string, bool) { return nil, false }
	run := func() (bool, *Config) {
		cfg := &Config{CLI: "claude-tmux", Agent: "scout"}
		d := &seatStubDriver{}
		deps := d.deps()
		deps.SandboxWrap = failing
		deps.LookupEnv = func(string) (string, bool) { return "", false }
		if _, err := launchSwept(context.Background(), d, cfg, deps); err != nil {
			t.Fatal(err)
		}
		_, wrapped := sandboxPrefixForLaunch(deps, &d.cfg, "")
		return sandboxRequiredButUnavailable(deps, &d.cfg, wrapped), &d.cfg
	}
	if refused, _ := run(); refused {
		t.Fatal("a host with no token started to refuse")
	}
	plantHolder(t, home, seatCanaryToken, 0o600)
	if refused, _ := run(); !refused {
		t.Fatal("a token seat with a failed wrap was not refused")
	}
}

func TestLaunchPane_ASmokeTakesTheSeatShapeAndAProbeStaysOnTheStoredLogin(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	d := &seatStubDriver{}
	if rc, err := launchPane(context.Background(), PurposeSmoke, d, &Config{CLI: "claude-tmux"}, d.deps()); rc != ExitOK || err != nil {
		t.Fatalf("smoke: rc=%d err=%v", rc, err)
	}
	if tok, _ := seatEnvValue(d.env, "CLAUDE_CODE_OAUTH_TOKEN"); tok != seatCanaryToken || !d.cfg.RequireSandbox {
		t.Fatalf("a smoke did not take the seat shape: %+v", d.cfg.Decision)
	}
	p := &seatStubDriver{}
	if rc, err := launchPane(context.Background(), PurposeProbe, p, &Config{CLI: "claude-tmux"}, p.deps()); rc != ExitOK || err != nil {
		t.Fatalf("probe: rc=%d err=%v", rc, err)
	}
	if p.cfg.Decision != (seatlogin.Decision{}) || p.cfg.RequireSandbox {
		t.Fatalf("a probe left the stored login: %+v", p.cfg.Decision)
	}
	b := &seatStubDriver{}
	plantHolder(t, home, " ", 0o600)
	if rc, err := launchPane(context.Background(), PurposeSmoke, b, &Config{CLI: "claude-tmux"}, b.deps()); rc != ExitSeatTokenRefused || err == nil || b.launches != 0 {
		t.Fatalf("a bad holder with a smoke: rc=%d err=%v", rc, err)
	}
}

func TestLaunch_TheCanaryReachesOnlyTheCellEnvAndNothingTheLaunchWrites(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	center, events := recordingSignals()
	workspace := t.TempDir()
	var stderr strings.Builder
	d := &seatStubDriver{}
	deps := d.deps()
	deps.Signals, deps.Stderr = center, &stderr
	cfg := &Config{CLI: "claude-tmux", Agent: "scout", Workspace: workspace}
	if _, err := launchSwept(context.Background(), d, cfg, deps); err != nil {
		t.Fatal(err)
	}
	var seen []string
	seen = append(seen, stderr.String())
	for _, e := range *events {
		seen = append(seen, e.Reason)
		for _, v := range e.Fields {
			seen = append(seen, v)
		}
	}
	seen = append(seen, exportLines(cfg.Realization.Env)...)
	seen = append(seen, exportLines(deps.cellMarkers(cfg))...)
	seen = append(seen, probeEnv(deps, "claude")...)
	seen = append(seen, cellEnvHeader(workspace), strings.Join(d.req.WriteSubpaths, " "))
	for _, s := range seen {
		if strings.Contains(s, seatCanaryToken) {
			t.Fatalf("the canary leaked: %q", s)
		}
	}
	_ = filepath.WalkDir(workspace, func(p string, e os.DirEntry, _ error) error {
		if e != nil && !e.IsDir() {
			if body, _ := os.ReadFile(p); strings.Contains(string(body), seatCanaryToken) {
				t.Errorf("%s holds the canary", p)
			}
		}
		return nil
	})
}

func TestResolveSeat_AnEmptySeamFallsToTheRealResolverAndNeverToAStoredLoginOnAFailure(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o644)
	d := &seatStubDriver{}
	deps := d.deps()
	deps.SeatLogin = seatlogin.Deps{}
	if rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, deps); rc != ExitSeatTokenRefused || err == nil {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
	deps.SeatLogin = seatlogin.Deps{
		Stat:     func(string) (os.FileInfo, error) { return nil, errors.New("denied") },
		ReadFile: os.ReadFile,
	}
	if rc, err := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, deps); rc != ExitSeatTokenRefused || err == nil || !strings.Contains(err.Error(), "cannot be read: denied") {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
}

func TestSandboxWrap_ASeatRootIsWritableAndTheStoredLoginIsNotWhenHostIsolated(t *testing.T) {
	home := seatHome(t)
	seatRoot := paths.SeatRoot(home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(seatRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	ws, wt, root := t.TempDir(), t.TempDir(), t.TempDir()
	profile := func(isolated bool) string {
		req := SandboxWrapRequest{Phase: "itest", CLI: "claude-tmux", Worktree: wt, Workspace: ws, RepoRoot: root}
		if isolated {
			req.HostIsolated, req.SeatRoot = true, seatRoot
		}
		deps := Deps{SandboxMode: "on"}
		prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))(req)
		if !ok {
			t.Fatal("the wrap was refused")
		}
		body, err := os.ReadFile(prefix[2])
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	claudeWrite := `(allow file-write* (subpath "` + filepath.Join(home, ".claude") + `"))`
	seatWrite := `(allow file-write* (subpath "` + canonicalOrSelf(seatRoot) + `"))`
	plain, iso := profile(false), profile(true)
	if !strings.Contains(plain, claudeWrite) || strings.Contains(plain, seatWrite) {
		t.Errorf("a no-token wrap changed:\n%s", plain)
	}
	if strings.Contains(iso, claudeWrite) || !strings.Contains(iso, seatWrite) {
		t.Errorf("an isolated wrap must grant the seat root and not ~/.claude:\n%s", iso)
	}
	if !strings.Contains(iso, `(deny file-read* (subpath "`+filepath.Join(home, ".claude")+`"))`) {
		t.Errorf("an isolated wrap must deny the read of the stored login:\n%s", iso)
	}
}

func TestSeatLogin_TheHolderIsReadAtOneDoorHelperAndEachDoorCallsIt(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"seatlogin.Resolve(", "SeatTokenPath("} {
			if strings.Contains(string(body), banned) && f != "seat_launch.go" {
				t.Errorf("%s reads the holder through %q: only seat_launch.go may", f, banned)
			}
		}
	}
	for file, door := range map[string]string{"dispatch_reap.go": "launchSwept", "launch_llm.go": "launchPane"} {
		body, _ := os.ReadFile(file)
		start := strings.Index(string(body), "func "+door+"(")
		if start < 0 || !strings.Contains(string(body)[start:], "resolveSeat(") {
			t.Errorf("%s does not call resolveSeat", door)
		}
	}
}

func TestSeatLogin_TheSignalCenterIsNotNeededForARefusal(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, "", 0o600)
	d := &seatStubDriver{}
	deps := d.deps()
	deps.Signals = signalcenter.New()
	if rc, _ := launchSwept(context.Background(), d, &Config{CLI: "claude-tmux", Agent: "scout"}, deps); rc != ExitSeatTokenRefused {
		t.Fatalf("rc=%d", rc)
	}
}

func TestResolveSeat_AnUnprovisionableDirRefusesWithTheProvisionText(t *testing.T) {
	home := seatHome(t)
	if err := os.WriteFile(filepath.Join(home, ".evolve-seat"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tok := filepath.Join(home, "tok")
	if err := os.WriteFile(tok, []byte(seatCanaryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &seatStubDriver{}
	rc, err := d.deps().resolveSeat(&Config{CLI: "claude-tmux", SeatTokenFile: tok})
	if rc != ExitSeatTokenRefused || err == nil || !strings.Contains(err.Error(), "cannot be provisioned") {
		t.Fatalf("rc=%d err=%v", rc, err)
	}
}

func TestSeatConfinementRefusal_IsAPureQueryAndWritesNothing(t *testing.T) {
	home := seatHome(t)
	dir := filepath.Join(home, ".evolve-seat")
	d := &seatStubDriver{}
	if got := d.deps().seatConfinementRefusal(); got != "" {
		t.Fatalf("refusal=%q", got)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("err=%v: the confinement check wrote into the seat directory", err)
	}
}

func TestSeatEnv_CopiesTheCLIEnvAndAddsTheSeatPair(t *testing.T) {
	t.Parallel()
	got := seatEnv(map[string]string{"A": "1"}, seatlogin.Decision{Dir: "/d", SeatToken: "tok"})
	want := map[string]string{"A": "1", seatConfigDirEnv: "/d", seatOAuthTokenEnv: "tok"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
