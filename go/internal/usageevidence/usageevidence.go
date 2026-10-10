// Package usageevidence queries a failing CLI's usage, records the verdict as evidence and decorates the bridge with it.
// See docs/architecture/packages/internal-usageevidence.md.
package usageevidence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

const CodeUsageEvidence signalcenter.Code = "BRIDGE_USAGE_EVIDENCE"

const (
	exitMissingBinary = 127
	exitEscalated     = 85
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeUsageEvidence, "a CLI failed in a way quota could explain (a REPL boot timeout, no response, an escalation, a stall, a failed doctor probe), so the failure path queried the CLI's usage in a fresh session and recorded the verdict for the failing family: exhausted (a family-scoped window is used up: the verified cause, and the family is benched until the reset), healthy (quota ruled out for the family; an exhausted per-model window is named as a note, never as the cause), unavailable (the usage query failed too, which points at auth, install or network) or unknown (no usage window could be read); the reason is the verdict summary; fields driver, cli, family, verdict, trigger (exit N or stall), exit_code, cached, and record_error when the workspace record could not be written")
}

type Explain func(ctx context.Context, driver string, since time.Time) usageprobe.Evidence

type Record struct {
	Cycle     int
	RunID     string
	Phase     string
	Workspace string
	Origin    string
	Driver    string
	Trigger   string
	ExitCode  int
	Evidence  usageprobe.Evidence
}

type Line struct {
	Origin   string              `json:"origin"`
	Phase    string              `json:"phase,omitempty"`
	Driver   string              `json:"driver"`
	Trigger  string              `json:"trigger"`
	ExitCode int                 `json:"exit_code,omitempty"`
	Summary  string              `json:"summary"`
	Evidence usageprobe.Evidence `json:"evidence"`
}

func QuotaCouldExplain(exitCode int) bool {
	return exitCode != 0 && exitCode != exitMissingBinary
}

func Report(signals *signalcenter.Center, r Record) error {
	var err error
	if r.Workspace != "" {
		err = appendLine(r)
	}
	fields := map[string]string{
		"driver": r.Driver, "cli": r.Evidence.CLI, "family": r.Evidence.Family, "verdict": string(r.Evidence.Verdict),
		"trigger": r.Trigger, "exit_code": itoa(r.ExitCode), "cached": strconv.FormatBool(r.Evidence.Cached),
	}
	if err != nil {
		fields["record_error"] = err.Error()
	}
	signals.Emit(signalcenter.Event{
		Cycle: r.Cycle, RunID: r.RunID, Phase: r.Phase, Module: signalcenter.ModuleBridge, Origin: r.Origin,
		Kind: signalcenter.KindBridgeWarning, Code: CodeUsageEvidence, Severity: severityOf(r.Evidence.Verdict),
		Reason: r.Evidence.Summary(), Fields: fields,
	})
	return err
}

func severityOf(v usageprobe.Verdict) signalcenter.Severity {
	if v == usageprobe.VerdictExhausted || v == usageprobe.VerdictUnavailable {
		return signalcenter.SeverityWarn
	}
	return signalcenter.SeverityInfo
}

func appendLine(r Record) error {
	body, err := json.Marshal(Line{Origin: r.Origin, Phase: r.Phase, Driver: r.Driver, Trigger: r.Trigger, ExitCode: r.ExitCode, Summary: r.Evidence.Summary(), Evidence: r.Evidence})
	if err != nil {
		return fmt.Errorf("usage evidence: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(r.Workspace, core.UsageEvidenceFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("usage evidence: %w", err)
	}
	if _, err := f.Write(append(body, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("usage evidence: %w", err)
	}
	return f.Close()
}

func itoa(n int) string { return strconv.Itoa(n) }

type Bridge struct {
	inner   core.Bridge
	explain Explain
	signals func() *signalcenter.Center
}

func Wrap(inner core.Bridge, explain Explain, signals func() *signalcenter.Center) *Bridge {
	return &Bridge{inner: inner, explain: explain, signals: signals}
}

func (b *Bridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	start := time.Now()
	res, err := b.inner.Launch(ctx, req)
	if b.explain != nil && ctx.Err() == nil && QuotaCouldExplain(res.ExitCode) {
		rec := Record{Cycle: req.Cycle, RunID: req.RunID, Phase: req.Agent, Workspace: req.Workspace,
			Origin: "Bridge.Launch", Driver: req.CLI, Trigger: "exit " + itoa(res.ExitCode), ExitCode: res.ExitCode, Evidence: b.explain(ctx, req.CLI, start)}
		res.UsageExhausted = rec.Evidence.Verdict == usageprobe.VerdictExhausted
		if rerr := Report(b.center(), rec); rerr != nil {
			warnf("%s: %v\n", req.CLI, rerr)
		}
	}
	if res.ExitCode == exitEscalated {
		bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: req.ProjectRoot, Workspace: req.Workspace, CLI: req.CLI, DispatchStart: start, Env: req.Env}, time.Now, warnf)
	}
	return res, err
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[usage-evidence] "+format, args...)
}

func (b *Bridge) center() *signalcenter.Center {
	if b.signals == nil {
		return nil
	}
	return b.signals()
}

func (b *Bridge) Probe(ctx context.Context) (core.BridgeProbe, error) { return b.inner.Probe(ctx) }

func (b *Bridge) Signals() *signalcenter.Center {
	if src, ok := b.inner.(interface{ Signals() *signalcenter.Center }); ok {
		return src.Signals()
	}
	return nil
}

func (b *Bridge) SignalsWired() bool {
	if src, ok := b.inner.(interface{ SignalsWired() bool }); ok {
		return src.SignalsWired()
	}
	return false
}
