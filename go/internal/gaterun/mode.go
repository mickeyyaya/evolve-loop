package gaterun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	bwrapPath = "/usr/bin/bwrap"
	lddPath   = "/usr/bin/ldd"
)

type Target struct {
	Entry   string
	Lane    string
	Scratch string
}

func (t Target) tag() string { return filepath.Base(t.Scratch) }

type Strategy interface {
	Prefix(Target) Prefix
}

type Finisher interface {
	Finish(ctx context.Context, tg Target, since time.Time)
}

type Setup struct {
	Mode        string
	OS          string
	Binary      string
	Policy      string
	HomeDir     string
	SandboxExec string
	Bwrap       string
	Signals     Emitter
	Source      sandbox.ReportSource
	Libraries   func(binary string) ([]string, error)
}

func SetupFor(pol policy.Policy, binary, policyPath string) Setup {
	home, _ := os.UserHomeDir()
	return Setup{
		Mode: pol.HermeticMode(), OS: runtime.GOOS, Binary: binary, Policy: policyPath, HomeDir: home,
		SandboxExec: sandbox.SandboxExecPath, Bwrap: bwrapPath, Libraries: lddWith(runLdd),
		Source: sandbox.ReportSource{
			Reports: func(ctx context.Context, tag string, since time.Time) ([]byte, error) {
				return sandbox.Reports(ctx, sandbox.LogShowPath, tag, since)
			},
			Fence: func(ctx context.Context, tag string) error {
				return sandbox.WriteFence(ctx, sandbox.SandboxExecPath, tag)
			},
			Now: time.Now, Sleep: time.Sleep,
		},
	}
}

func runLdd(ctx context.Context, binary string) ([]byte, error) {
	return sysexec.Command(ctx, lddPath, binary).Output()
}

func lddWith(run func(ctx context.Context, binary string) ([]byte, error)) func(string) ([]string, error) {
	return func(binary string) ([]string, error) {
		out, err := run(context.Background(), binary)
		if err != nil {
			return nil, fmt.Errorf("gaterun: ldd %s: %w", binary, err)
		}
		return sandbox.ParseLibraries(string(out)), nil
	}
}

func NewStrategy(s Setup) (Strategy, error) {
	switch s.Mode {
	case policy.HermeticOff:
		return offStrategy{}, nil
	case policy.HermeticShadow, policy.HermeticEnforce:
	default:
		return nil, fmt.Errorf("gaterun: gates.hermetic.mode %q is not off, shadow or enforce", s.Mode)
	}
	if s.Signals == nil {
		return nil, fmt.Errorf("gaterun: gates.hermetic.mode %s needs a signal emitter, or its evidence is lost", s.Mode)
	}
	shadow := s.Mode == policy.HermeticShadow
	switch {
	case s.OS == "darwin":
		return darwinStrategy{setup: s, report: shadow}, nil
	case s.OS == "linux" && shadow:
		return linuxShadow{setup: s}, nil
	case s.OS == "linux":
		libs, err := s.Libraries(s.Binary)
		if err != nil {
			return nil, err
		}
		return linuxEnforce{setup: s, libraries: libs}, nil
	}
	return nil, fmt.Errorf("gaterun: no hermetic sandbox for GOOS=%s; set gates.hermetic.mode to off", s.OS)
}

func wrap(tg Target, program string, lead []string) Prefix {
	return func(cmd *exec.Cmd) *exec.Cmd {
		inner := append([]string{cmd.Path}, cmd.Args[1:]...)
		cmd.Path = program
		cmd.Args = slices.Concat([]string{filepath.Base(program)}, lead, inner)
		env := cmd.Env
		if env == nil {
			env = os.Environ()
		}
		cmd.Env = append(slices.Clone(env), "TMPDIR="+tg.Scratch)
		return cmd
	}
}

func (s Setup) gate(tg Target, report bool) sandbox.GateConfig {
	return sandbox.GateConfig{Lane: tg.Lane, Binary: s.Binary, Policy: s.Policy, Scratch: tg.Scratch, HomeDir: s.HomeDir, Tag: tg.tag(), Report: report}
}

type offStrategy struct{}

func (offStrategy) Prefix(Target) Prefix { return nil }

type darwinStrategy struct {
	setup  Setup
	report bool
}

func (d darwinStrategy) Prefix(tg Target) Prefix {
	return wrap(tg, d.setup.SandboxExec, []string{"-p", sandbox.GateProfile(d.setup.gate(tg, d.report))})
}

func (d darwinStrategy) Finish(ctx context.Context, tg Target, since time.Time) {
	if !d.report {
		return
	}
	raw, complete, err := d.setup.Source.Cell(ctx, tg.tag(), since)
	if err != nil {
		emitNoEvidence(d.setup.Signals, tg, "the sandbox reports could not be read: "+err.Error())
		return
	}
	canaries := map[string]bool{}
	for _, r := range sandbox.ParseReports(raw) {
		if r.Rule == sandbox.RuleGateCanary {
			canaries[filepath.Base(r.Target)] = true
			continue
		}
		emitViolation(d.setup.Signals, tg, d.setup.Mode, r)
	}
	switch {
	case !complete:
		emitNoEvidence(d.setup.Signals, tg, "the sandbox reports are incomplete: the fence of the run did not show")
	case !canaries[sandbox.CanaryStart] || !canaries[sandbox.CanaryEnd]:
		emitNoEvidence(d.setup.Signals, tg, "the sandbox dropped reports of the child: a canary report is missing")
	}
}

type linuxShadow struct{ setup Setup }

func (linuxShadow) Prefix(Target) Prefix { return nil }

func (l linuxShadow) Finish(_ context.Context, tg Target, _ time.Time) {
	emitNoEvidence(l.setup.Signals, tg, "bwrap cannot report a denied operation, so the child ran with no sandbox")
}

type linuxEnforce struct {
	setup     Setup
	libraries []string
}

func (l linuxEnforce) Prefix(tg Target) Prefix {
	cfg := l.setup.gate(tg, false)
	cfg.Libraries = l.libraries
	return wrap(tg, l.setup.Bwrap, sandbox.GateBwrapPrefix(cfg))
}
