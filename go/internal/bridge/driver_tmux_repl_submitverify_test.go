package bridge

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// submitVerifyStderrMarker is the loud-logging contract: each re-send attempt
// must name itself on stderr so an operator reading a stalled cycle's log can
// see the driver noticed and acted.
const submitVerifyStderrMarker = "submit-verify"

// maxSubmitVerifyResends bounds the re-send loop so it can never spin forever.
const maxSubmitVerifyResends = 3

// unsubmittedPane renders text parked at the input line, never submitted.
func unsubmittedPane(text string) string {
	return "● earlier scrollback\n\n" + tmuxPromptMarkerDefault + " " + text
}

// stickyInputTmux simulates a pane whose input line does NOT clear when keys
// are sent with enter=true. `trigger` selects which send to sabotage: the
// send whose keys contain `trigger` (the nudge names the artifact path; ""
// means the prompt-paste Enter). When clearOnResend is set, a subsequent bare
// Enter clears the input line, so the test can observe recovery rather than
// an unbounded stall.
type stickyInputTmux struct {
	*fakeTmux
	mu            sync.Mutex
	trigger       string // substring identifying the send to sabotage
	stuckText     string // what stays parked at the input line
	stuck         bool
	stuckAtSeq    int // len(sentSeq) right after the sabotaged send
	clearOnResend bool
}

func (s *stickyInputTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	s.mu.Lock()
	switch {
	case !s.stuck && s.trigger != "" && strings.Contains(keys, s.trigger):
		s.stuck, s.stuckText = true, keys
	case s.stuck && s.clearOnResend && keys == "" && enter:
		s.stuck = false
	}
	s.mu.Unlock()
	err := s.fakeTmux.SendKeys(ctx, session, keys, enter)
	s.mu.Lock()
	if s.stuck && s.stuckAtSeq == 0 {
		s.stuckAtSeq = len(s.fakeTmux.sentSeq)
	}
	s.mu.Unlock()
	return err
}

// pasteStickyTmux sabotages the prompt-paste delivery instead: the paste
// lands but the delivery Enter does not submit it, so the prompt text sits
// at the input line.
type pasteStickyTmux struct {
	*fakeTmux
	mu       sync.Mutex
	pasted   bool
	stuck    bool
	pasteSeq int
}

func (p *pasteStickyTmux) PasteBuffer(ctx context.Context, session string) error {
	err := p.fakeTmux.PasteBuffer(ctx, session)
	p.mu.Lock()
	p.pasted, p.stuck = true, true
	p.pasteSeq = len(p.fakeTmux.sentSeq)
	p.mu.Unlock()
	return err
}

func (p *pasteStickyTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	p.mu.Lock()
	// The first bare Enter AFTER the paste is the (failed) delivery Enter; a
	// SECOND bare Enter is the re-send under test, and that one submits.
	resend := p.stuck && p.pasted && keys == "" && enter && len(p.fakeTmux.sentSeq) > p.pasteSeq
	if resend {
		p.stuck = false
	}
	p.mu.Unlock()
	return p.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func (p *pasteStickyTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	p.mu.Lock()
	stuck := p.stuck
	p.mu.Unlock()
	out, err := p.fakeTmux.CapturePane(ctx, session, scrollback)
	if stuck {
		return unsubmittedPane("Use your Write tool to create artifact containing:"), err
	}
	return out, err
}

func (s *stickyInputTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	s.mu.Lock()
	stuck, text := s.stuck, s.stuckText
	s.mu.Unlock()
	out, err := s.fakeTmux.CapturePane(ctx, session, scrollback)
	if stuck {
		return unsubmittedPane(text), err
	}
	return out, err
}

// runSubmitVerify drives the production launch path with a custom controller.
func runSubmitVerify(t *testing.T, fx launchFixture, tm TmuxController) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{
		Tmux:      tm,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=1526"), nil, &stdout, &stderr)
	return code, stderr.String()
}

// bareEnterIdxAfter returns the indices of bare-Enter sends ("" with enter) in
// sentSeq at or after `from` — the driver's submit key.
func bareEnterIdxAfter(seq []string, from int) []int {
	var idx []int
	for i := from; i < len(seq); i++ {
		if seq[i] == "|true" {
			idx = append(idx, i)
		}
	}
	return idx
}

// nudgeSeqIdx returns the index of the one-shot nudge send in sentSeq, or -1.
func nudgeSeqIdx(seq []string, artifact string) int {
	for i, e := range seq {
		if strings.Contains(e, artifact) && strings.Contains(e, "complete the phase") {
			return i
		}
	}
	return -1
}

func TestTmuxREPL_NudgeUnsubmitted_ResendsEnter(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tm := &stickyInputTmux{
		fakeTmux:      &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		trigger:       fx.artifact,
		clearOnResend: true,
	}
	code, stderr := runSubmitVerify(t, fx, tm)
	if code != ExitArtifactTimeout && code != ExitOK {
		t.Fatalf("exit = %d, want ExitArtifactTimeout or ExitOK (run must terminate, not hang)", code)
	}
	ni := nudgeSeqIdx(tm.fakeTmux.sentSeq, fx.artifact)
	if ni < 0 {
		t.Fatalf("precondition: the one-shot nudge was never sent; sentSeq=%v", tm.fakeTmux.sentSeq)
	}
	resends := bareEnterIdxAfter(tm.fakeTmux.sentSeq, ni+1)
	if len(resends) == 0 {
		t.Errorf("nudge left unsubmitted at the `%s` input line and the driver never re-sent Enter "+
			"(the cycles 1505/1510/1517 shape: nudge parked at the prompt, result=no_effect); sentSeq=%v",
			tmuxPromptMarkerDefault, tm.fakeTmux.sentSeq)
	}
	if !strings.Contains(stderr, submitVerifyStderrMarker) {
		t.Errorf("re-send must be loud: stderr missing %q marker; got:\n%s", submitVerifyStderrMarker, stderr)
	}
}

func TestTmuxREPL_NudgeSubmitted_NoResend(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Plain fake: every capture is a clean prompt marker, so the nudge is
	// submitted the moment it is sent.
	tm := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	code, _ := runSubmitVerify(t, fx, tm)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout", code)
	}
	ni := nudgeSeqIdx(tm.sentSeq, fx.artifact)
	if ni < 0 {
		t.Fatalf("precondition: the one-shot nudge was never sent; sentSeq=%v", tm.sentSeq)
	}
	if got := bareEnterIdxAfter(tm.sentSeq, ni+1); len(got) != 0 {
		t.Errorf("input line was clear — driver must NOT re-send Enter; got %d extra Enter(s); sentSeq=%v",
			len(got), tm.sentSeq)
	}
}

func TestTmuxREPL_NudgeUnsubmitted_ResendBounded(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tm := &stickyInputTmux{
		fakeTmux:      &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		trigger:       fx.artifact,
		clearOnResend: false, // never clears — the pathological pane
	}
	code, _ := runSubmitVerify(t, fx, tm)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout (bounded give-up, not a hang)", code)
	}
	ni := nudgeSeqIdx(tm.fakeTmux.sentSeq, fx.artifact)
	if ni < 0 {
		t.Fatalf("precondition: the one-shot nudge was never sent; sentSeq=%v", tm.fakeTmux.sentSeq)
	}
	n := len(bareEnterIdxAfter(tm.fakeTmux.sentSeq, ni+1))
	if n < 1 {
		t.Errorf("unsubmitted nudge must be re-sent at least once; got %d", n)
	}
	if n > maxSubmitVerifyResends {
		t.Errorf("re-sends = %d, want <= %d (bounded); an unbounded re-send loop hammers the pane",
			n, maxSubmitVerifyResends)
	}
}

// TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter generalizes the nudge fix
// to the prompt-paste delivery site; no observed run shows an unsubmitted
// prompt, so this pane state is stipulated, not replayed.
func TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tm := &pasteStickyTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	code, stderr := runSubmitVerify(t, fx, tm)
	if code != ExitArtifactTimeout && code != ExitOK {
		t.Fatalf("exit = %d, want ExitArtifactTimeout or ExitOK (run must terminate)", code)
	}
	if tm.fakeTmux.pasteContext == "" && tm.pasteSeq == 0 {
		t.Fatalf("precondition: the prompt was never pasted; sentSeq=%v", tm.fakeTmux.sentSeq)
	}
	enters := bareEnterIdxAfter(tm.fakeTmux.sentSeq, tm.pasteSeq)
	if len(enters) < 2 {
		t.Errorf("prompt left unsubmitted at the input line after paste — driver sent %d bare Enter(s) "+
			"and never verified delivery; want the delivery Enter plus at least one re-send; sentSeq=%v",
			len(enters), tm.fakeTmux.sentSeq)
	}
	if !strings.Contains(stderr, submitVerifyStderrMarker) {
		t.Errorf("prompt re-send must be loud: stderr missing %q marker; got:\n%s", submitVerifyStderrMarker, stderr)
	}
}
