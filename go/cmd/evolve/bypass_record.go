package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/bypass"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	bypassSandboxOff  = "sandbox-off"
	bypassAutoConfirm = "auto-confirm"
)

const sandboxOffUsage = "turn the OS sandbox off for this run; the value is the reason, which the ledger records (an audited bypass, refused inside an LLM cell)"

type bypassAuditor struct {
	evolveDir string
	signals   *signalcenter.Center
	stderr    io.Writer
	now       func() time.Time
}

func newBypassAuditor(evolveDir string, signals *signalcenter.Center, stderr io.Writer) bypassAuditor {
	return bypassAuditor{evolveDir: evolveDir, signals: signals, stderr: stderr, now: time.Now}
}

func (a bypassAuditor) record(name, reason, origin string) error {
	if err := bypass.Check(name, reason, os.Getenv); err != nil {
		return err
	}
	use := bypass.Use{Name: name, Reason: reason, Actor: bypass.Actor(user.Current), Origin: origin, At: a.now()}
	if err := bypass.Record(context.Background(), ledger.New(a.evolveDir, ledger.WithSignals(a.signals)), a.signals, use); err != nil {
		return fmt.Errorf("--%s: the audit record failed: %w", name, err)
	}
	fmt.Fprintf(a.stderr, "[bypass] --%s used by %s: %s (recorded in the ledger and as %s)\n", name, use.Actor, reason, bypass.CodeUsed)
	return nil
}

type sandboxSwitch interface{ SetSandboxOff() }

func (a bypassAuditor) applySandboxOff(br sandboxSwitch, reason string) error {
	if reason == "" {
		return nil
	}
	if err := a.record(bypassSandboxOff, reason, "wireOrchestratorDeps"); err != nil {
		return err
	}
	br.SetSandboxOff()
	return nil
}
