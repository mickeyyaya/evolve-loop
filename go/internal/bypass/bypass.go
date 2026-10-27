// Package bypass audits each use of a bypass parameter with one ledger entry and one signal.
// See docs/architecture/packages/internal-bypass.md.
package bypass

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeUsed signalcenter.Code = "CONTROL_BYPASS_USED"

const LedgerKind = "control_bypass"

var (
	ErrNoReason = errors.New("a bypass parameter needs a reason")
	ErrInCell   = errors.New("a bypass parameter is refused inside an LLM cell")
)

type Appender interface {
	Append(ctx context.Context, e core.LedgerEntry) error
}

type Use struct {
	Name, Reason, Actor, Origin string
	Cycle                       int
	At                          time.Time
}

func Check(name, reason string, getenv func(string) string) error {
	if reason == "" {
		return fmt.Errorf("--%s: %w (pass --%s=<why>)", name, ErrNoReason, name)
	}
	if getenv(ipcenv.PhaseKey) != "" || ipcenv.DispatchDepth(getenv) > 0 {
		return fmt.Errorf("--%s: %w", name, ErrInCell)
	}
	return nil
}

func Record(ctx context.Context, ld Appender, c *signalcenter.Center, u Use) error {
	err := ld.Append(ctx, core.LedgerEntry{
		TS: u.At.UTC().Format(time.RFC3339), Cycle: u.Cycle, Role: "operator", Kind: LedgerKind, Action: u.Name,
		Message: u.Actor + ": " + u.Reason, Source: u.Origin,
	})
	if err != nil {
		return err
	}
	Emit(c, u)
	return nil
}

func Emit(c *signalcenter.Center, u Use) {
	c.Emit(signalcenter.Event{
		Cycle: u.Cycle, Module: signalcenter.ModuleControl, Origin: u.Origin, Kind: signalcenter.KindControlBypass,
		Severity: signalcenter.SeverityInfo, Code: CodeUsed, Reason: u.Name + " used by " + u.Actor + ": " + u.Reason,
		Fields: map[string]string{"name": u.Name, "actor": u.Actor, "reason": u.Reason, "at": u.At.UTC().Format(time.RFC3339)},
	})
}

func Actor(current func() (*user.User, error)) string {
	u, err := current()
	if err != nil || u.Username == "" {
		return "unknown"
	}
	return u.Username
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleControl, CodeUsed, "an audited bypass is in use (ADR-0156 I-16, X18d): an operator parameter, after its control_bypass ledger entry is written, or a policy opt-in at the bridge adapter start; fields name, actor, reason and at")
}
