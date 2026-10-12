package core

import (
	"context"
	"errors"
	"testing"
)

type scriptedLaunches struct {
	responses []BridgeResponse
	sent      []BridgeRequest
	onLaunch  func(int)
}

func (s *scriptedLaunches) launch(_ context.Context, req BridgeRequest) (BridgeResponse, error) {
	i := len(s.sent)
	s.sent = append(s.sent, req)
	if s.onLaunch != nil {
		s.onLaunch(i)
	}
	resp := s.responses[i]
	if resp.ExitCode != 0 {
		return resp, errors.New("bridge: launch exit=81")
	}
	return resp, nil
}

var deadShell = BridgeResponse{ExitCode: 81, CauseCode: "dead_shell", SessionRecoverable: true}

func TestLaunchRecoverable_ANonRecoverableEndIsNotRetried(t *testing.T) {
	t.Parallel()

	s := &scriptedLaunches{responses: []BridgeResponse{{ExitCode: 81, CauseCode: "review_pause"}}}
	var launch LaunchFunc = s.launch

	resp, err := LaunchRecoverable(context.Background(), launch, BridgeRequest{Agent: "retro", Prompt: "p"}, nil)

	if len(s.sent) != 1 || err == nil || resp.ExitCode != 81 {
		t.Fatalf("launches = %d resp = %+v err = %v, want one launch and its exit 81", len(s.sent), resp, err)
	}
}

func TestLaunchRecoverable_ARecoverableEndGetsOneFreshSessionWithTheHookRequest(t *testing.T) {
	t.Parallel()

	s := &scriptedLaunches{responses: []BridgeResponse{deadShell, {Stdout: "ok"}}}
	var seen BridgeResponse

	resp, err := LaunchRecoverable(context.Background(), s.launch, BridgeRequest{Agent: "build", Prompt: "p"}, func(dead BridgeResponse) BridgeRequest {
		seen = dead
		return BridgeRequest{Agent: "build", Prompt: "p + record"}
	})

	if err != nil || resp.Stdout != "ok" || len(s.sent) != 2 {
		t.Fatalf("resp = %+v err = %v launches = %d, want the fresh session's success", resp, err, len(s.sent))
	}
	if seen.CauseCode != "dead_shell" || s.sent[1].Prompt != "p + record" || !s.sent[1].FreshSession || s.sent[0].FreshSession {
		t.Fatalf("hook saw %+v, sent %+v; want the hook request marked as the fresh session", seen, s.sent)
	}
}

func TestLaunchRecoverable_ACancelledContextIsNotRetried(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	s := &scriptedLaunches{responses: []BridgeResponse{deadShell, {}}, onLaunch: func(int) { cancel() }}

	resp, _ := LaunchRecoverable(ctx, s.launch, BridgeRequest{Agent: "build"}, SameRequest(BridgeRequest{Agent: "build"}))

	if len(s.sent) != 1 || resp.ExitCode != 81 {
		t.Fatalf("launches = %d resp = %+v, want no fresh session after the context was cancelled", len(s.sent), resp)
	}
}

func TestLaunchRecoverable_TheSecondRecoverableEndIsIgnoredAndCleared(t *testing.T) {
	t.Parallel()

	s := &scriptedLaunches{responses: []BridgeResponse{deadShell, deadShell, {}}}

	resp, err := LaunchRecoverable(context.Background(), s.launch, BridgeRequest{Agent: "judge"}, SameRequest(BridgeRequest{Agent: "judge"}))

	if len(s.sent) != 2 || err == nil || resp.ExitCode != 81 || resp.SessionRecoverable {
		t.Fatalf("launches = %d resp = %+v, want at most one retry and the flag cleared so no outer caller retries", len(s.sent), resp)
	}
}

func TestSameRequest_RetriesWithTheRequestUnchanged(t *testing.T) {
	t.Parallel()

	req := BridgeRequest{Agent: "judge", Prompt: "same"}
	if got := SameRequest(req)(deadShell); got.Prompt != "same" || got.Agent != "judge" {
		t.Fatalf("SameRequest = %+v, want %+v", got, req)
	}
}

type scriptedBridge struct{ scriptedLaunches }

func (b *scriptedBridge) Launch(ctx context.Context, req BridgeRequest) (BridgeResponse, error) {
	return b.launch(ctx, req)
}

func (b *scriptedBridge) Probe(context.Context) (BridgeProbe, error) { return BridgeProbe{}, nil }

func TestLaunchWithFreshSession_RetriesTheSameRequestOnceOnABridge(t *testing.T) {
	t.Parallel()

	b := &scriptedBridge{scriptedLaunches{responses: []BridgeResponse{deadShell, {Stdout: "ok"}}}}
	req := BridgeRequest{Agent: "judge", Prompt: "same"}

	resp, err := LaunchWithFreshSession(context.Background(), b, req)

	if err != nil || resp.Stdout != "ok" || len(b.sent) != 2 || b.sent[1].Prompt != "same" || !b.sent[1].FreshSession {
		t.Fatalf("resp = %+v err = %v sent = %+v, want one fresh session with the same request", resp, err, b.sent)
	}
}
