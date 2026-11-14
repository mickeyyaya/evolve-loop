package bridgechain_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type deadShellOnce struct{ calls []core.BridgeRequest }

func (d *deadShellOnce) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	d.calls = append(d.calls, req)
	if len(d.calls) == 1 {
		return core.BridgeResponse{ExitCode: 81, CauseCode: "dead_shell", SessionRecoverable: true}, errors.New("bridge: exit 81")
	}
	return core.BridgeResponse{}, nil
}

func (d *deadShellOnce) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestWalking_ADeadShellGetsOneFreshSessionOfTheSameCLIBeforeTheWalkMovesOn(t *testing.T) {
	inner := &deadShellOnce{}
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, []string{"deep"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", Agent: "retrospective"})

	if err != nil || resp.ExitCode != 0 {
		t.Fatalf("resp = %+v err = %v, want the fresh session's success", resp, err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@deep codex-tmux@deep" || !inner.calls[1].FreshSession {
		t.Fatalf("attempts = %s, want one fresh session of codex-tmux before any walk", got)
	}
}

type causeLaunch struct {
	calls []core.BridgeRequest
	cause string
	failN int
}

func (c *causeLaunch) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	c.calls = append(c.calls, req)
	if len(c.calls) <= c.failN {
		return core.BridgeResponse{ExitCode: 81, CauseCode: c.cause}, errors.New("bridge: exit 81")
	}
	return core.BridgeResponse{Stderr: "ok:" + req.CLI}, nil
}

func (c *causeLaunch) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestWalking_TransientUpstreamRetriesPrimaryOnceWhenFallbacksBenched(t *testing.T) {
	root := t.TempDir()
	if _, err := clihealth.NewStore(root, time.Now).BenchWall("agy", clihealth.QuotaExhaustedPattern, "quota"); err != nil {
		t.Fatal(err)
	}
	inner := &causeLaunch{cause: "transient_upstream", failN: 1}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "agy-tmux"}, []string{"deep"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "retrospective", ProjectRoot: root})

	if err != nil || resp.Stderr != "ok:claude-tmux" {
		t.Fatalf("resp = %+v err = %v, want fresh session success", resp, err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "claude-tmux@deep claude-tmux@deep" || !inner.calls[1].FreshSession {
		t.Fatalf("attempts = %s fresh=%v, want one fresh session retry", got, inner.calls[1].FreshSession)
	}
}

func TestWalking_TransientCauseWalksWhenHealthyFallbackExists(t *testing.T) {
	inner := &causeLaunch{cause: "transient_upstream", failN: 1}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "codex-tmux"}, []string{"deep"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "retrospective", ProjectRoot: t.TempDir()})

	if err != nil || resp.Stderr != "ok:codex-tmux" {
		t.Fatalf("resp = %+v err = %v, want immediate fallback to codex-tmux", resp, err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "claude-tmux@deep codex-tmux@deep" {
		t.Fatalf("attempts = %s, want no retry when healthy fallback exists", got)
	}
}

func TestWalking_TransientCauseWalksWithoutProjectRoot(t *testing.T) {
	inner := &causeLaunch{cause: "submit_wedged", failN: 1}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "agy-tmux"}, []string{"deep"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "retrospective"})

	if err != nil || resp.Stderr != "ok:agy-tmux" {
		t.Fatalf("resp = %+v err = %v, want fallback when ProjectRoot is empty", resp, err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "claude-tmux@deep agy-tmux@deep" {
		t.Fatalf("attempts = %s, want no retry without ProjectRoot", got)
	}
}

func TestWalking_BootTimeoutAndModelBenchCountAsBenchedFallbacks(t *testing.T) {
	root := t.TempDir()
	store := clihealth.NewStore(root, time.Now)
	until := time.Now().Add(time.Hour)
	if err := store.Bench(clihealth.Entry{Family: "agy-tmux", Reason: clihealth.BootTimeoutPattern, BenchedAt: time.Now(), BenchedUntil: until}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BenchModelUntil("claude", "Opus", clihealth.Wall{Pattern: clihealth.UsageProbePattern, Reset: until}); err != nil {
		t.Fatal(err)
	}
	inner := &causeLaunch{cause: "submit_wedged", failN: 1}
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "agy-tmux", "claude-tmux"}, []string{"deep"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", Agent: "retrospective", ProjectRoot: root})

	if err != nil || resp.Stderr != "ok:codex-tmux" {
		t.Fatalf("resp = %+v err = %v, want fresh session success", resp, err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@deep codex-tmux@deep" || !inner.calls[1].FreshSession {
		t.Fatalf("attempts = %s fresh=%v, want one fresh session retry", got, inner.calls[1].FreshSession)
	}
}
