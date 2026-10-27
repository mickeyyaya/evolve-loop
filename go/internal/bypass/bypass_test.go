package bypass_test

import (
	"context"
	"errors"
	"os/user"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bypass"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type memLedger struct {
	entries []core.LedgerEntry
	err     error
}

func (m *memLedger) Append(_ context.Context, e core.LedgerEntry) error {
	m.entries = append(m.entries, e)
	return m.err
}

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

func TestCheck_NeedsAReasonAndRefusesInsideACell(t *testing.T) {
	if err := bypass.Check("sandbox-off", "", env(nil)); !errors.Is(err, bypass.ErrNoReason) {
		t.Errorf("no reason: err = %v, want ErrNoReason", err)
	}
	for _, marker := range []string{ipcenv.PhaseKey, ipcenv.DispatchDepthKey} {
		if err := bypass.Check("sandbox-off", "debug", env(map[string]string{marker: "1"})); !errors.Is(err, bypass.ErrInCell) {
			t.Errorf("%s set: err = %v, want ErrInCell", marker, err)
		}
	}
	if err := bypass.Check("sandbox-off", "debug", env(map[string]string{ipcenv.DispatchDepthKey: "0"})); err != nil {
		t.Errorf("depth 0 is the operator shell: err = %v", err)
	}
}

func TestRecord_WritesOneLedgerEntryAndOneSignalWithWhoWhenWhy(t *testing.T) {
	ld := &memLedger{}
	var appender bypass.Appender = ld
	center := signalcenter.New()
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	err := bypass.Record(context.Background(), appender, center, bypass.Use{Name: "sandbox-off", Reason: "debug a pane", Actor: "dan", Origin: "runLoop", Cycle: 7, At: at})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(ld.entries) != 1 || ld.entries[0].Kind != bypass.LedgerKind || ld.entries[0].Action != "sandbox-off" || ld.entries[0].Role != "operator" ||
		ld.entries[0].Message != "dan: debug a pane" || ld.entries[0].Source != "runLoop" || ld.entries[0].TS != "2026-10-27T09:00:00Z" || ld.entries[0].Cycle != 7 {
		t.Errorf("ledger = %+v", ld.entries)
	}
	center.Flush()
	got := center.Recent()
	if len(got) != 1 || got[0].Code != bypass.CodeUsed || got[0].Severity != signalcenter.SeverityInfo || got[0].Kind != signalcenter.KindControlBypass ||
		got[0].Fields["name"] != "sandbox-off" || got[0].Fields["actor"] != "dan" || got[0].Fields["reason"] != "debug a pane" || got[0].Origin != "runLoop" {
		t.Errorf("signals = %+v", got)
	}
}

func TestRecord_ALedgerFailureIsReturnedAndNoSignalClaimsTheUse(t *testing.T) {
	boom := errors.New("disk full")
	center := signalcenter.New()
	if err := bypass.Record(context.Background(), &memLedger{err: boom}, center, bypass.Use{Name: "auto-confirm", Reason: "release", At: time.Now()}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the ledger error", err)
	}
	center.Flush()
	if got := center.Recent(); len(got) != 0 {
		t.Errorf("signals = %+v, want none: a refused use must not be reported as used", got)
	}
}

func TestEmit_IsTheSignalAloneAndANilCenterIsANoOp(t *testing.T) {
	center := signalcenter.New()
	bypass.Emit(center, bypass.Use{Name: "bridge.allow_openai_api_key", Reason: "policy", Actor: "policy.json", Origin: "NewDefault", At: time.Now()})
	center.Flush()
	if got := center.Recent(); len(got) != 1 || got[0].Code != bypass.CodeUsed || got[0].Fields["actor"] != "policy.json" {
		t.Errorf("signals = %+v, want one %s", got, bypass.CodeUsed)
	}
	bypass.Emit(nil, bypass.Use{Name: "x"})
}

func TestActor_NamesTheOperatorOrUnknown(t *testing.T) {
	if got := bypass.Actor(func() (*user.User, error) { return &user.User{Username: "dan"}, nil }); got != "dan" {
		t.Errorf("Actor = %q", got)
	}
	if got := bypass.Actor(func() (*user.User, error) { return nil, errors.New("no user") }); got != "unknown" {
		t.Errorf("Actor = %q, want unknown", got)
	}
	if got := bypass.Actor(func() (*user.User, error) { return &user.User{}, nil }); got != "unknown" {
		t.Errorf("Actor = %q, want unknown for an empty name", got)
	}
}
