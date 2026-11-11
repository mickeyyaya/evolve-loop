package gaterun

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	fakeSandboxExec = "/fake/sandbox-exec"
	fakeBwrap       = "/fake/bwrap"
)

func modeSetup(t *testing.T, mode, goos string) (Setup, *recorder) {
	t.Helper()
	root := t.TempDir()
	rec := &recorder{}
	return Setup{
		Mode: mode, OS: goos, Binary: "/opt/evolve", Policy: root + "/policy.json", HomeDir: "/home/x",
		SandboxExec: fakeSandboxExec, Bwrap: fakeBwrap, Signals: rec,
		Libraries: func(string) ([]string, error) { return []string{"/lib/libc.so.6"}, nil },
		Source:    sandbox.ReportSource{Reports: func(context.Context, string, time.Time) ([]byte, error) { return nil, nil }},
	}, rec
}

func childCommand() *exec.Cmd {
	return &exec.Cmd{Path: "/opt/evolve", Args: []string{"evolve", "gate", "run", "--entry", "x"}}
}

func target(t *testing.T) Target {
	t.Helper()
	return Target{Entry: "x", Lane: t.TempDir(), Scratch: t.TempDir()}
}

func TestNewStrategy_AnUnknownModeAndAnUnsupportedOSAreErrors(t *testing.T) {
	t.Parallel()
	if _, err := NewStrategy(Setup{Mode: "strict", OS: "darwin"}); err == nil || !strings.Contains(err.Error(), "strict") {
		t.Errorf("unknown mode = %v", err)
	}
	if _, err := NewStrategy(Setup{Mode: policy.HermeticShadow, OS: "plan9", Signals: &recorder{}}); err == nil || !strings.Contains(err.Error(), "plan9") {
		t.Errorf("unsupported OS = %v", err)
	}
	if _, err := NewStrategy(Setup{Mode: policy.HermeticOff, OS: "plan9"}); err != nil {
		t.Errorf("off needs no sandbox on any OS: %v", err)
	}
}

func TestStrategy_OffRunsTheChildWithNoPrefixAndReadsNoReports(t *testing.T) {
	t.Parallel()
	setup, rec := modeSetup(t, policy.HermeticOff, "darwin")
	setup.Source.Reports = func(context.Context, string, time.Time) ([]byte, error) {
		t.Error("off reads no reports")
		return nil, nil
	}
	s, err := NewStrategy(setup)
	if err != nil {
		t.Fatal(err)
	}
	if s.Prefix(target(t)) != nil {
		t.Error("off must give no prefix")
	}
	if _, ok := s.(Finisher); ok || len(rec.events) != 0 {
		t.Errorf("off has nothing to finish: events = %+v", rec.events)
	}
}

func TestStrategy_DarwinEnforceWrapsTheChildInTheDenyProfile(t *testing.T) {
	t.Parallel()
	setup, rec := modeSetup(t, policy.HermeticEnforce, "darwin")
	setup.Source.Reports = func(context.Context, string, time.Time) ([]byte, error) {
		t.Error("enforce must read no reports: its deny profile tags no report, so a read only waits for the fence")
		return nil, nil
	}
	s, _ := NewStrategy(setup)
	cmd := s.Prefix(target(t))(childCommand())
	if cmd.Path != fakeSandboxExec || !slices.Equal(cmd.Args[:2], []string{"sandbox-exec", "-p"}) {
		t.Fatalf("cmd = %v %v", cmd.Path, cmd.Args)
	}
	profile := cmd.Args[2]
	if !strings.Contains(profile, "(deny network*)") || strings.Contains(profile, "with report") {
		t.Errorf("the enforce profile denies and reports nothing:\n%s", profile)
	}
	if !slices.Equal(cmd.Args[3:], []string{"/opt/evolve", "gate", "run", "--entry", "x"}) {
		t.Errorf("the original command must follow the profile: %v", cmd.Args)
	}
	s.(Finisher).Finish(context.Background(), target(t), time.Now())
	if len(rec.events) != 0 {
		t.Errorf("enforce emits nothing at the end: %+v", rec.events)
	}
}

func TestStrategy_DarwinShadowUsesTheReportProfileAndEmitsEachViolation(t *testing.T) {
	t.Parallel()
	setup, rec := modeSetup(t, policy.HermeticShadow, "darwin")
	var gotTag string
	setup.Source.Reports = func(_ context.Context, tag string, _ time.Time) ([]byte, error) {
		gotTag = tag
		return []byte(`{"eventMessage":"Sandbox: git(9) allow file-read-data \/lane\/.git\/HEAD\nevolve-cell=` + tag + ` rule=gate.git inv=I0"}` + "\n" + canaryReports(tag)), nil
	}
	s, _ := NewStrategy(setup)
	tg := target(t)
	cmd := s.Prefix(tg)(childCommand())
	if !strings.Contains(cmd.Args[2], "with report") || strings.Contains(cmd.Args[2], "(deny network*)") {
		t.Errorf("the shadow profile reports and denies nothing:\n%s", cmd.Args[2])
	}
	s.(Finisher).Finish(context.Background(), tg, time.Now())
	if len(rec.events) != 1 || rec.events[0].Code != CodeViolation || rec.events[0].Fields["path"] != "/lane/.git/HEAD" || rec.events[0].Fields["operation"] != "file-read-data" || rec.events[0].Fields["entry"] != "x" {
		t.Fatalf("events = %+v", rec.events)
	}
	if gotTag == "" || !strings.Contains(cmd.Args[2], gotTag) {
		t.Errorf("the profile and the query must share the tag %q", gotTag)
	}
}

func TestStrategy_DarwinShadowWithAReportsErrorEmitsNoEvidence(t *testing.T) {
	t.Parallel()
	setup, rec := modeSetup(t, policy.HermeticShadow, "darwin")
	setup.Source.Reports = func(context.Context, string, time.Time) ([]byte, error) { return nil, errors.New("log: denied") }
	s, _ := NewStrategy(setup)
	s.(Finisher).Finish(context.Background(), target(t), time.Now())
	if len(rec.events) != 1 || rec.events[0].Code != CodeNoEvidence || !strings.Contains(rec.events[0].Fields["cause"], "log: denied") {
		t.Errorf("events = %+v", rec.events)
	}
}

func TestStrategy_LinuxEnforceWrapsTheChildInBwrap(t *testing.T) {
	t.Parallel()
	setup, _ := modeSetup(t, policy.HermeticEnforce, "linux")
	s, _ := NewStrategy(setup)
	tg := target(t)
	cmd := s.Prefix(tg)(childCommand())
	joined := strings.Join(cmd.Args, " ")
	if cmd.Path != fakeBwrap || !strings.Contains(joined, "--unshare-net") || !strings.Contains(joined, "--ro-bind /lib/libc.so.6 /lib/libc.so.6") || !strings.HasSuffix(joined, "/opt/evolve gate run --entry x") {
		t.Errorf("cmd = %v", cmd.Args)
	}
	if !slices.Contains(cmd.Env, "TMPDIR="+tg.Scratch) {
		t.Errorf("the child must write its temp files to the scratch: %v", cmd.Env)
	}
}

func TestStrategy_LinuxShadowRunsTheSameChildWithNoPrefixAndEmitsOneNoEvidence(t *testing.T) {
	t.Parallel()
	setup, rec := modeSetup(t, policy.HermeticShadow, "linux")
	s, _ := NewStrategy(setup)
	if s.Prefix(target(t)) != nil {
		t.Error("linux shadow gives no bwrap prefix")
	}
	s.(Finisher).Finish(context.Background(), target(t), time.Now())
	if len(rec.events) != 1 || rec.events[0].Code != CodeNoEvidence || rec.events[0].Fields["entry"] != "x" {
		t.Errorf("events = %+v", rec.events)
	}
}

func TestStrategy_AnExistingEnvironmentGainsTheScratchAsTmpdir(t *testing.T) {
	t.Parallel()
	setup, _ := modeSetup(t, policy.HermeticEnforce, "darwin")
	s, _ := NewStrategy(setup)
	cmd := childCommand()
	cmd.Env = []string{"A=1"}
	tg := target(t)
	cmd = s.Prefix(tg)(cmd)
	if !slices.Equal(cmd.Env, []string{"A=1", "TMPDIR=" + tg.Scratch}) {
		t.Errorf("Env = %v", cmd.Env)
	}
}

func TestSetup_ForPolicyReadsTheModeFromThePolicyAndTheHostFromTheRuntime(t *testing.T) {
	t.Parallel()
	pol, err := policy.Parse([]byte(`{"gates":{"hermetic":{"mode":"enforce"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	s := SetupFor(pol, "/opt/evolve", "/p/policy.json")
	if s.Bwrap != "/usr/bin/bwrap" {
		t.Errorf("Bwrap = %q: a PATH lookup lets a planted bwrap run the child with no sandbox", s.Bwrap)
	}
	if s.Mode != policy.HermeticEnforce || s.Binary != "/opt/evolve" || s.Policy != "/p/policy.json" || s.OS == "" || s.Source.Reports == nil || s.Libraries == nil {
		t.Errorf("SetupFor = %+v", s)
	}
	home, _ := os.UserHomeDir()
	if s.HomeDir != home {
		t.Errorf("HomeDir = %q, want %q", s.HomeDir, home)
	}
}

func TestLddWith_ParsesTheOutputAndAFailureIsAnError(t *testing.T) {
	t.Parallel()
	ok := lddWith(func(context.Context, string) ([]byte, error) {
		return []byte("\tlibc.so.6 => /lib/libc.so.6 (0x1)\n"), nil
	})
	if got, err := ok("/b"); err != nil || !slices.Equal(got, []string{"/lib/libc.so.6"}) {
		t.Errorf("libraries = %v, %v", got, err)
	}
	bad := lddWith(func(context.Context, string) ([]byte, error) { return nil, errors.New("no ldd") })
	if got, err := bad("/b"); err == nil || got != nil {
		t.Errorf("a failed ldd = %v, %v, want an error and no libraries", got, err)
	}
	if _, err := runLdd(context.Background(), "/no/such/binary"); err == nil {
		t.Error("runLdd of a missing binary must fail")
	}
}

func TestSetupFor_TheSourceClosuresReachTheHostTools(t *testing.T) {
	t.Parallel()
	s := SetupFor(policy.Policy{}, "/opt/evolve", "/p")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Source.Reports(ctx, "tag", time.Now()); err == nil {
		t.Error("a cancelled context must fail the report read")
	}
	if err := s.Source.Fence(ctx, "tag"); err == nil {
		t.Error("a cancelled context must fail the fence")
	}
	if s.Mode != policy.HermeticShadow {
		t.Errorf("Mode = %q, want the default shadow", s.Mode)
	}
}

type fakeStrategy struct {
	targets  []Target
	finished []Target
}

func (f *fakeStrategy) Prefix(tg Target) Prefix {
	f.targets = append(f.targets, tg)
	return func(cmd *exec.Cmd) *exec.Cmd { return cmd }
}

func (f *fakeStrategy) Finish(_ context.Context, tg Target, _ time.Time) {
	f.finished = append(f.finished, tg)
}

func TestRun_AStrategyPrefixesTheChildAndFinishesAfterIt(t *testing.T) {
	t.Parallel()
	fs := &fakeStrategy{}
	ports := gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}}
	_, err := Run(context.Background(), gatecontract.Entry{Name: "serves"}, gatecontract.Input{Root: "/lane"}, Options{Command: helperCommand, Ports: ports, Strategy: fs})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.targets) != 1 || len(fs.finished) != 1 || fs.targets[0].Entry != "serves" || fs.targets[0].Lane != "/lane" || fs.targets[0].Scratch == "" || fs.finished[0] != fs.targets[0] {
		t.Errorf("targets = %+v finished = %+v", fs.targets, fs.finished)
	}
}

func TestRun_AStrategyFinishesEvenWhenTheChildFails(t *testing.T) {
	t.Parallel()
	fs := &fakeStrategy{}
	_, err := Run(context.Background(), gatecontract.Entry{Name: "x"}, gatecontract.Input{}, Options{Command: shellCommand("exit 3"), Strategy: fs})
	if err == nil || len(fs.finished) != 1 {
		t.Errorf("err = %v finished = %+v", err, fs.finished)
	}
}

type prefixStrategy Prefix

func (p prefixStrategy) Prefix(Target) Prefix { return Prefix(p) }

func TestStrategy_DarwinShadowWithAnIncompleteReadEmitsItsViolationsAndNoEvidence(t *testing.T) {
	t.Parallel()
	setup, rec := modeSetup(t, policy.HermeticShadow, "darwin")
	setup.Source.Reports = func(_ context.Context, tag string, _ time.Time) ([]byte, error) {
		return []byte(`{"eventMessage":"Sandbox: git(9) allow file-read-data \/lane\/.git\/HEAD\nevolve-cell=` + tag + ` rule=gate.git inv=I0"}` + "\n"), nil
	}
	setup.Source.Fence = func(context.Context, string) error { return errors.New("fence refused") }
	s, _ := NewStrategy(setup)
	s.(Finisher).Finish(context.Background(), target(t), time.Now())
	if len(rec.events) != 2 || rec.events[0].Code != CodeViolation || rec.events[1].Code != CodeNoEvidence || !strings.Contains(rec.events[1].Fields["cause"], "incomplete") {
		t.Errorf("events = %+v, want the violation and then a no-evidence signal: a read with no fence may miss later reports", rec.events)
	}
}

func TestNewStrategy_AModeThatMeasuresNeedsAnEmitterAndLinuxEnforceNeedsItsLibraries(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{policy.HermeticShadow, policy.HermeticEnforce} {
		setup, _ := modeSetup(t, mode, "darwin")
		setup.Signals = nil
		if _, err := NewStrategy(setup); err == nil || !strings.Contains(err.Error(), "signal") {
			t.Errorf("%s with no emitter = %v, want an error: the evidence would be dropped", mode, err)
		}
	}
	setup, _ := modeSetup(t, policy.HermeticOff, "darwin")
	setup.Signals = nil
	if _, err := NewStrategy(setup); err != nil {
		t.Errorf("off measures nothing, so it needs no emitter: %v", err)
	}
	setup, _ = modeSetup(t, policy.HermeticEnforce, "linux")
	setup.Libraries = func(string) ([]string, error) { return nil, errors.New("ldd: not found") }
	if _, err := NewStrategy(setup); err == nil || !strings.Contains(err.Error(), "ldd: not found") {
		t.Errorf("linux enforce with no libraries = %v, want the ldd error", err)
	}
}

func canaryReports(tag string) string {
	var b strings.Builder
	for _, name := range []string{sandbox.CanaryStart, sandbox.CanaryEnd} {
		b.WriteString(`{"eventMessage":"Sandbox: x(9) allow file-read-data \/s\/` + name + `\nevolve-cell=` + tag + ` rule=gate.canary inv=I0"}` + "\n")
	}
	return b.String()
}

func TestStrategy_DarwinShadowWithNoCanaryReportSaysTheChildReportsWereDropped(t *testing.T) {
	t.Parallel()
	for name, lines := range map[string]func(tag string) string{
		"none":       func(string) string { return "" },
		"start only": func(tag string) string { return strings.SplitAfter(canaryReports(tag), "\n")[0] },
	} {
		setup, rec := modeSetup(t, policy.HermeticShadow, "darwin")
		setup.Source.Reports = func(_ context.Context, tag string, _ time.Time) ([]byte, error) { return []byte(lines(tag)), nil }
		s, _ := NewStrategy(setup)
		s.(Finisher).Finish(context.Background(), target(t), time.Now())
		if len(rec.events) != 1 || rec.events[0].Code != CodeNoEvidence || !strings.Contains(rec.events[0].Fields["cause"], "canary") {
			t.Errorf("%s: events = %+v, want one no-evidence signal: with no canary a clean run and a run whose reports were dropped look the same", name, rec.events)
		}
	}
}

func TestRun_TheScratchHoldsBothCanariesWhenTheChildStarts(t *testing.T) {
	t.Parallel()
	var missing []string
	probe := prefixStrategy(func(cmd *exec.Cmd) *exec.Cmd { return cmd })
	check := &canaryCheck{inner: probe, missing: &missing}
	ports := gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}}
	if _, err := Run(context.Background(), gatecontract.Entry{Name: "serves"}, gatecontract.Input{}, Options{Command: helperCommand, Ports: ports, Strategy: check}); err != nil {
		t.Fatal(err)
	}
	if !check.called || len(missing) != 0 {
		t.Errorf("called = %v, missing canaries %v: the child reads them, so they must exist before it starts", check.called, missing)
	}
}

type canaryCheck struct {
	inner   Strategy
	missing *[]string
	called  bool
}

func (c *canaryCheck) Prefix(tg Target) Prefix {
	c.called = true
	for _, name := range []string{sandbox.CanaryStart, sandbox.CanaryEnd} {
		if _, err := os.Stat(filepath.Join(tg.Scratch, name)); err != nil {
			*c.missing = append(*c.missing, name)
		}
	}
	return c.inner.Prefix(tg)
}
