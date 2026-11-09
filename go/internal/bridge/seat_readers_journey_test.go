package bridge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

var nonAlnumFolder = regexp.MustCompile(`[^A-Za-z0-9]`)

const seatTranscriptInput = 321

func plantSeatTranscript(t *testing.T, root, worktree string, at time.Time) string {
	t.Helper()
	dir := filepath.Join(root, "projects", nonAlnumFolder.ReplaceAllString(worktree, "-"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	first, last := at.Add(-2*time.Minute).UTC().Format(time.RFC3339), at.Add(-time.Minute).UTC().Format(time.RFC3339)
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"` + first + `","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}` + "\n" +
		`{"type":"assistant","cwd":"` + worktree + `","timestamp":"` + last + `","message":{"id":"m1","usage":{"input_tokens":321,"output_tokens":40,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n"
	path := filepath.Join(dir, "seat.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readerWindow(worktree string, at time.Time) tokenusage.Window {
	return tokenusage.Window{
		Collector: tokenusage.FormatClaudeJSONL, ContextWindow: 200_000, Worktree: worktree,
		Start: at.Add(-time.Hour), End: at.Add(time.Hour),
	}
}

func TestSeatReadersJourney_ASeatOnTheTokenFileIsFoundBySpendAndPostMortemUnderTheSeatRoot(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := seatHome(t)
	param := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(param, []byte(seatCanaryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	j := newSeatJourney(t)
	resp, err := j.launch(t, func(r *core.BridgeRequest) { r.SeatTokenFile = param; r.Worktree = worktree })
	if err != nil {
		t.Fatal(err)
	}
	if resp.SeatLogin != "isolated" {
		t.Fatalf("the seat did not run isolated: %q", resp.SeatLogin)
	}
	configDir, ok := seatEnvValue(j.callEnv(t), "CLAUDE_CONFIG_DIR")
	if !ok || configDir != paths.SeatRoot(home) {
		t.Fatalf("CLAUDE_CONFIG_DIR=%q ok=%v, want the seat root %q", configDir, ok, paths.SeatRoot(home))
	}

	at := time.Now()
	transcript := plantSeatTranscript(t, configDir, worktree, at)
	env := map[string]string{"HOME": home}
	w := readerWindow(worktree, at)

	res, err := tokenusage.HostResolver(env)(w)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != tokenusage.SourceTranscript || res.Usage.Input != seatTranscriptInput {
		t.Errorf("the spend reader missed the seat usage: source %q input %d, want %q %d", res.Source, res.Usage.Input, tokenusage.SourceTranscript, seatTranscriptInput)
	}
	if got := tokenusage.LocateTranscript(tokenusage.ClaudeTranscriptRoots(env), w); got != transcript {
		t.Errorf("the post-mortem reader found %q, want the seat transcript %q", got, transcript)
	}
	if !strings.HasPrefix(transcript, paths.SeatRoot(home)+string(filepath.Separator)) {
		t.Errorf("the transcript %q is not under the seat root", transcript)
	}
	if got := tokenusage.LocateTranscript([]string{filepath.Join(home, ".claude")}, w); got != "" {
		t.Errorf("the stored root alone found %q: the journey does not prove the seat root", got)
	}
}

func TestSeatReadersJourney_AStoredLoginSeatStillGivesTheSameSpendAndPostMortem(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := seatHome(t)
	worktree := t.TempDir()
	j := newSeatJourney(t)
	resp, err := j.launch(t, func(r *core.BridgeRequest) { r.Worktree = worktree })
	if err != nil {
		t.Fatal(err)
	}
	if resp.SeatLogin != "stored" {
		t.Fatalf("the seat did not run on the stored login: %q", resp.SeatLogin)
	}
	if v, _ := seatEnvValue(j.callEnv(t), "CLAUDE_CONFIG_DIR"); v != "" {
		t.Fatalf("CLAUDE_CONFIG_DIR=%q on a stored-login seat", v)
	}

	at := time.Now()
	transcript := plantSeatTranscript(t, filepath.Join(home, ".claude"), worktree, at)
	env := map[string]string{"HOME": home}
	w := readerWindow(worktree, at)

	res, err := tokenusage.HostResolver(env)(w)
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != tokenusage.SourceTranscript || res.Usage.Input != seatTranscriptInput {
		t.Errorf("the spend reader missed the stored usage: source %q input %d", res.Source, res.Usage.Input)
	}
	if got := tokenusage.LocateTranscript(tokenusage.ClaudeTranscriptRoots(env), w); got != transcript {
		t.Errorf("the post-mortem reader found %q, want %q", got, transcript)
	}
}
