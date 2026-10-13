package subagent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAppendAbnormalEvent_AWriteFailureNeverBreaksTheCaller(t *testing.T) {
	orig := abnormalEventWrite
	t.Cleanup(func() { abnormalEventWrite = orig })
	var wrote string
	abnormalEventWrite = func(f *os.File, s string) (int, error) { wrote = s; return 0, errors.New("disk full") }
	ws := t.TempDir()

	err := AppendAbnormalEvent(ws, AbnormalEvent{EventType: "x", Severity: "warn"}, fixedClock(t, "2026-10-01T00:00:00Z"))

	if err != nil {
		t.Fatalf("event logging is best-effort and must never fail the caller: %v", err)
	}
	if wrote == "" || wrote[len(wrote)-1] != '\n' {
		t.Errorf("the line is written newline-terminated: %q", wrote)
	}
	if info, statErr := os.Stat(filepath.Join(ws, "abnormal-events.jsonl")); statErr != nil || info.Size() != 0 {
		t.Errorf("a failed write leaves the file empty: %v %v", info, statErr)
	}
}

func TestDefaultResolveLLM_ARouterThatCannotBeBuiltIsReturnedNotResolvedAround(t *testing.T) {
	orig := newSingleProfileRouter
	t.Cleanup(func() { newSingleProfileRouter = orig })
	boom := errors.New("host rejected")
	newSingleProfileRouter = func(policy.Policy, cliroute.SingleProfile, cliroute.Host) (*cliroute.Router, error) {
		return nil, boom
	}

	got, err := defaultResolveLLM("builder")

	if !errors.Is(err, boom) || got.CLI != "" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
