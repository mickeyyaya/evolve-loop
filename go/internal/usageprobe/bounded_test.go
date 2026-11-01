package usageprobe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

func warnLines(log string) []string {
	var warns []string
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "WARN") {
			warns = append(warns, line)
		}
	}
	return warns
}

func TestProber_Run_AFailedProbeBenchesNothingAndLogsOneWarn(t *testing.T) {
	st := newStore(t)
	var log bytes.Buffer
	rp := &recordingProbe{err: map[string]error{"agy": errors.New("boot timed out")}}
	p := &Prober{Families: []string{"agy"}, Probe: rp.probe, Classify: func(_, _ string) bool { return true }, Store: st, Log: &log}

	p.Run(context.Background())

	if len(st.Active()) != 0 {
		t.Fatalf("a failed probe benched %v", st.Active())
	}
	warns := warnLines(log.String())
	if len(warns) != 1 || !strings.Contains(warns[0], "agy") || !strings.Contains(warns[0], "boot timed out") || !strings.Contains(warns[0], "nothing benched") {
		t.Errorf("want one WARN naming agy, the cause and that nothing is benched; log:\n%s", log.String())
	}
}

func TestProber_Run_AProbeThatIgnoresItsDeadlineIsOneWarnAndRunReturns(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	st := newStore(t)
	var log bytes.Buffer
	p := &Prober{
		Families: []string{"claude", "codex"},
		Probe: func(_ context.Context, family string) (string, error) {
			if family == "codex" {
				<-block
			}
			return "healthy", nil
		},
		Classify: bridge.ClassifyExhausted,
		Store:    st,
		Log:      &log,
		Timeout:  50 * time.Millisecond,
	}
	done := make(chan struct{})
	go func() { p.Run(context.Background()); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run must return at its timeout even though a probe never returns")
	}
	warns := warnLines(log.String())
	if len(warns) != 1 || !strings.Contains(warns[0], "codex") || !strings.Contains(warns[0], "50ms") || !strings.Contains(warns[0], "nothing benched") {
		t.Errorf("want one WARN for codex naming the 50ms bound; log:\n%s", log.String())
	}
	if len(st.Active()) != 0 {
		t.Errorf("a timed-out probe benched %v", st.Active())
	}
}

func TestProbeOne_AProbeThatAnswersPastTheBoundBeforeTheRoundClosesIsOneWarn(t *testing.T) {
	var log bytes.Buffer
	p := &Prober{
		Probe:    func(ctx context.Context, _ string) (string, error) { return "", ctx.Err() },
		Classify: bridge.ClassifyExhausted,
		Store:    newStore(t),
		Log:      &log,
	}
	probeCtx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	round := newProbeRound(context.Background(), []string{"agy"})

	p.probeOne(probeCtx, round, "agy")

	if warns := warnLines(log.String()); len(warns) != 1 || !strings.Contains(warns[0], "agy") || len(round.close()) != 0 {
		t.Fatalf("want one WARN for agy and nothing left to abandon; log:\n%s", log.String())
	}
}

func TestProber_Run_WaitsForAJudgementThatStartedBeforeTheBound(t *testing.T) {
	st := newStore(t)
	p := &Prober{
		Families: []string{"agy"},
		Probe:    func(context.Context, string) (string, error) { return "a walled pane", nil },
		Classify: func(string, string) bool { time.Sleep(150 * time.Millisecond); return true },
		Store:    st,
		Log:      &bytes.Buffer{},
		Timeout:  20 * time.Millisecond,
	}

	p.Run(context.Background())

	if _, ok := st.Active()["agy"]; !ok {
		t.Fatalf("Run returned before the started judgement wrote the agy bench: %v", st.Active())
	}
}

func TestProber_Run_AProbeThatHonoursItsDeadlineIsAlwaysOneWarn(t *testing.T) {
	for i := 0; i < 200; i++ {
		var log bytes.Buffer
		p := &Prober{
			Families: []string{"agy"},
			Probe:    func(ctx context.Context, _ string) (string, error) { <-ctx.Done(); return "", ctx.Err() },
			Classify: bridge.ClassifyExhausted,
			Store:    newStore(t),
			Log:      &log,
			Timeout:  5 * time.Millisecond,
		}

		p.Run(context.Background())

		if warns := warnLines(log.String()); len(warns) != 1 {
			t.Fatalf("run %d: want exactly one WARN for agy; log:\n%s", i, log.String())
		}
	}
}

func TestProber_Run_AFamilyWithoutAUsageControlIsNotAWarn(t *testing.T) {
	var log bytes.Buffer
	p := &Prober{
		Families: []string{"ollama"},
		Probe: func(context.Context, string) (string, error) {
			return "", fmt.Errorf("wrap: %w", clicontrol.ErrUnsupported)
		},
		Classify: bridge.ClassifyExhausted,
		Store:    newStore(t),
		Log:      &log,
		Timeout:  time.Second,
	}

	p.Run(context.Background())

	if warns := warnLines(log.String()); len(warns) != 0 {
		t.Errorf("a CLI with no usage control is no WARN; log:\n%s", log.String())
	}
}

func TestProber_Run_AnAnswerAfterTheBoundWritesNothing(t *testing.T) {
	release := make(chan struct{})
	judged := make(chan struct{}, 1)
	st := newStore(t)
	var log bytes.Buffer
	p := &Prober{
		Families: []string{"agy"},
		Probe:    func(context.Context, string) (string, error) { <-release; return "a walled pane", nil },
		Classify: func(string, string) bool { judged <- struct{}{}; return true },
		Store:    st,
		Log:      &log,
		Timeout:  10 * time.Millisecond,
	}
	p.Run(context.Background())
	after := log.String()

	close(release)

	select {
	case <-judged:
		t.Fatal("an answer after the bound was judged")
	case <-time.After(200 * time.Millisecond):
	}
	if log.String() != after || len(st.Active()) != 0 {
		t.Errorf("an answer after the bound wrote: log %q -> %q, benches %v", after, log.String(), st.Active())
	}
}
