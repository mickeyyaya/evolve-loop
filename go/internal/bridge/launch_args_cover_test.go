package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type cfgProbeDriver struct {
	name string
	seen *Config
	pre  error
}

func (d *cfgProbeDriver) Name() string { return d.name }
func (d *cfgProbeDriver) Preflight(context.Context, *Config, Deps) error {
	return d.pre
}
func (d *cfgProbeDriver) Launch(_ context.Context, cfg *Config, _ Deps) (int, error) {
	c := *cfg
	d.seen = &c
	return ExitOK, nil
}

func TestLaunchArgs_AFailedPreflightIsLoggedAndTheArtifactBudgetReachesTheDriver(t *testing.T) {
	d := &cfgProbeDriver{name: "cover-probe", pre: errors.New("trust write denied")}
	Register(d)
	defer func() { ResetDriversForTesting(); registerBuiltins() }()
	fx := newFixture(t, "cover-probe", "")
	eng := NewEngine(Deps{Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)})
	var stdout, stderr bytes.Buffer

	code := eng.LaunchArgs(context.Background(), fx.args("cover-probe", "--artifact-timeout-s=45"), nil, &stdout, &stderr)

	if code != ExitOK || d.seen == nil {
		t.Fatalf("LaunchArgs=%d launched=%v, want ExitOK and a launch after a failed preflight\n%s", code, d.seen != nil, stderr.String())
	}
	if !strings.Contains(stderr.String(), "[bridge] cover-probe preflight: trust write denied (continuing — best-effort)") {
		t.Errorf("stderr lacks the preflight line:\n%s", stderr.String())
	}
	if d.seen.ArtifactTimeoutS != 45 {
		t.Errorf("ArtifactTimeoutS = %d, want 45 from --artifact-timeout-s", d.seen.ArtifactTimeoutS)
	}
}

func TestLaunchArgs_ABadArtifactBudgetFallsBackToTheBuiltinDeadline(t *testing.T) {
	d := &cfgProbeDriver{name: "cover-probe"}
	Register(d)
	defer func() { ResetDriversForTesting(); registerBuiltins() }()
	fx := newFixture(t, "cover-probe", "")
	eng := NewEngine(Deps{Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)})
	for _, v := range []string{"abc", "-5"} {
		var stdout, stderr bytes.Buffer
		code := eng.LaunchArgs(context.Background(), fx.args("cover-probe", "--artifact-timeout-s="+v), nil, &stdout, &stderr)
		if code != ExitOK || d.seen == nil || d.seen.ArtifactTimeoutS != 0 {
			t.Errorf("--artifact-timeout-s=%s: code=%d cfg=%+v, want ExitOK and no per-phase budget", v, code, d.seen)
		}
	}
}

func TestLaunchArgs_AGlobInASandboxDenialIsABadFlag(t *testing.T) {
	d := &cfgProbeDriver{name: "cover-probe"}
	Register(d)
	defer func() { ResetDriversForTesting(); registerBuiltins() }()
	fx := newFixture(t, "cover-probe", "")
	profile := filepath.Join(fx.ws, "sandboxed.json")
	body := `{"name":"sb","model":"auto","allowed_tools":["Read"],"sandbox":{"enabled":true,"deny_subpaths":[".git/*"]}}`
	if err := os.WriteFile(profile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(Deps{Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)})
	var stdout, stderr bytes.Buffer

	code := eng.LaunchArgs(context.Background(), fx.args("cover-probe", "--profile="+profile, "--worktree="+t.TempDir()), nil, &stdout, &stderr)

	if code != ExitBadFlags || d.seen != nil || !strings.Contains(stderr.String(), `[bridge] invalid sandbox policy: denial must be a literal path: ".git/*"`) {
		t.Errorf("code=%d launched=%v stderr=%q, want ExitBadFlags, no launch and the denial error", code, d.seen != nil, stderr.String())
	}
}
