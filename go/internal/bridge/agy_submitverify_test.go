package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const agyParkedTypedPrompt = "Reason step by step about which three prime numbers below 20 sum to 31, then create a file named out.txt in the current directory listing them one per line, then reply DONE."

func agySubmitVerifyDeps(frames ...string) (Deps, *FakeTmuxController, *bytes.Buffer) {
	fake := &FakeTmuxController{CaptureFrames: frames}
	stderr := &bytes.Buffer{}
	return Deps{Tmux: fake, Stderr: stderr, Sleep: func(time.Duration) {}}, fake, stderr
}

func agyLaunchForTest() tmuxLaunch {
	return agyTmuxLaunch("agy-tmux", &Config{}, Deps{}, "evolve-bridge-agy-c1-router-pid1-n1-1", false)
}

func bareEnters(sent []string) int {
	n := 0
	for _, s := range sent {
		if s == "|true" {
			n++
		}
	}
	return n
}

func TestAgyTmux_ParkedPasteChipIsResubmitted(t *testing.T) {
	deps, fake, stderr := agySubmitVerifyDeps(agy1217Frame(t, "editing-a.txt"))
	parked := agy1217Frame(t, "parked-paste.txt")
	got := verifySubmitted(context.Background(), deps, agyLaunchForTest(), "[agy-tmux]", "prompt", parked,
		promptSubmitEcho("=== HARMLESS TEST PROMPT"), tmuxPastePlaceholderEcho)
	if got.Result != interaction.ResultSubmittedAfterResend || bareEnters(fake.SentSeq) != 1 {
		t.Fatalf("parked [Pasted text] chip: got %+v with %d Enter(s), want submitted_after_resend after one Enter\n%s",
			got, bareEnters(fake.SentSeq), stderr.String())
	}
}

func TestAgyTmux_ParkedTypedNudgeIsResubmitted(t *testing.T) {
	deps, fake, _ := agySubmitVerifyDeps(agy1217Frame(t, "editing-a.txt"))
	got := verifySubmitted(context.Background(), deps, agyLaunchForTest(), "[agy-tmux]", "nudge",
		agy1217Frame(t, "parked-typed.txt"), agyParkedTypedPrompt)
	if got.Result != interaction.ResultSubmittedAfterResend || bareEnters(fake.SentSeq) != 1 {
		t.Fatalf("parked typed text: got %+v with %d Enter(s), want submitted_after_resend", got, bareEnters(fake.SentSeq))
	}
}

func TestAgyTmux_SubmittedPanesAreVerifiedWithoutResend(t *testing.T) {
	for _, name := range []string{"editing-a.txt", "generating-a.txt", "answer.txt", "idle.txt"} {
		deps, fake, stderr := agySubmitVerifyDeps()
		got := verifySubmitted(context.Background(), deps, agyLaunchForTest(), "[agy-tmux]", "prompt",
			agy1217Frame(t, name), agyParkedTypedPrompt, promptSubmitEcho(agyParkedTypedPrompt), tmuxPastePlaceholderEcho)
		if got.Result != interaction.ResultSubmitVerified || len(fake.SentSeq) != 0 {
			t.Errorf("%s: got %+v and sent %v, want submit_verified with no re-send\n%s", name, got, fake.SentSeq, stderr.String())
		}
	}
}

func TestAgyTmux_TranscriptAngleBracketsNeverShadowTheInputBox(t *testing.T) {
	pane := agy1217Frame(t, "editing-a.txt")
	if !strings.Contains(pane, "> "+agyParkedTypedPrompt[:20]) {
		t.Fatal("fixture precondition: the submitted prompt is echoed above the input box with a > prefix")
	}
	if pendingAtInputLine(pane, agyLaunchForTest().inputLineMarker, []string{agyParkedTypedPrompt}) {
		t.Error("the echoed prompt above the input box was read as parked; only the last > is the live input line")
	}
}

func TestAgyTmux_ParkedPromptContainingAngleBracketsIsStillResubmitted(t *testing.T) {
	const prompt = "Make sure x > 0 holds, then run go test ./... > out.txt and report the count."
	parked := strings.Replace(agy1217Frame(t, "parked-typed.txt"), agyParkedTypedPrompt, prompt, 1)
	if !strings.Contains(parked, "> "+prompt) {
		t.Fatal("fixture precondition: the prompt with > characters sits parked on the input line")
	}
	deps, fake, stderr := agySubmitVerifyDeps(agy1217Frame(t, "editing-a.txt"))
	got := verifySubmitted(context.Background(), deps, agyLaunchForTest(), "[agy-tmux]", "prompt", parked,
		promptSubmitEcho(prompt), firstNonEmptyLine(prompt), tmuxPastePlaceholderEcho)
	if got.Result != interaction.ResultSubmittedAfterResend || bareEnters(fake.SentSeq) != 1 {
		t.Fatalf("a parked prompt whose text contains > was read as submitted: got %+v with %d Enter(s)\n%s",
			got, bareEnters(fake.SentSeq), stderr.String())
	}
}
