package guardcmd

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func noCenter(string) (*signalcenter.Center, func()) { return nil, func() {} }

func runVerb(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return Run(noCenter)(args, stdin, stdout, stderr)
}

var streamCenter CenterFor = func(evolveDir string) (*signalcenter.Center, func()) {
	c := signalcenter.New()
	c.Subscribe(c.NDJSONSink(func(int) string { return filepath.Join(evolveDir, signalcenter.StreamFileName) }))
	return c, c.Flush
}

func denialStream(t *testing.T, dir string) []signalcenter.Event {
	t.Helper()
	chunk, err := signalcenter.ReadStream(filepath.Join(dir, signalcenter.StreamFileName), 0)
	if err != nil {
		return nil
	}
	return chunk.Events
}

func TestRunGuard_AShipDenialWritesOneNeutralGuardDeniedEventBeforeExit2(t *testing.T) {
	dir := t.TempDir()
	cwd := t.TempDir()
	input := `{"tool_name":"Bash","cwd":"` + cwd + `","tool_input":{"command":"git push origin main"}}`
	var stdout, stderr bytes.Buffer

	rc := Run(streamCenter)([]string{"ship", "--evolve-dir", dir}, strings.NewReader(input), &stdout, &stderr)

	events := denialStream(t, dir)
	if rc != 2 || len(events) != 1 {
		t.Fatalf("rc=%d events=%d (stderr %q): a deny exits 2 after exactly one event", rc, len(events), stderr.String())
	}
	e := events[0]
	if e.Code != core.CodeGuardDenied || e.Fields[core.DenialFieldGuard] != "ship" || e.Fields[core.DenialFieldPath] != "git push origin main" ||
		e.Fields[core.DenialFieldCWD] != cwd || e.Fields[core.DenialFieldNeutral] != "true" || e.Fields[core.DenialFieldReason] == "" {
		t.Fatalf("the event names the guard, the command, the cwd, the neutral flag and the reason: %+v", e)
	}
}

func TestRunGuard_AnAllowedCallEmitsNoEvent(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	built := false
	rc := Run(func(d string) (*signalcenter.Center, func()) { built = true; return streamCenter(d) })([]string{"ship", "--evolve-dir", dir}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &stdout, &stderr)

	if events := denialStream(t, dir); rc != 0 || len(events) != 0 || built {
		t.Fatalf("rc=%d events=%+v: an allow leaves no denial", rc, events)
	}
}

func TestRun_ADenialWithNoCenterStillDeniesAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	rc := runVerb([]string{"ship", "--evolve-dir", dir}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}`), &stdout, &stderr)

	if events := denialStream(t, dir); rc != 2 || len(events) != 0 {
		t.Fatalf("rc=%d events=%+v: the plain verb denies and sends no signal", rc, events)
	}
}

func TestDenialPath_PrefersTheFileOverTheCommand(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		input map[string]any
		want  string
	}{
		"a file path wins":  {map[string]any{"file_path": "/w/a.go", "command": "x"}, "/w/a.go"},
		"a command is next": {map[string]any{"command": "git push"}, "git push"},
		"neither is empty":  {map[string]any{}, ""},
	} {
		if got := denialPath(core.GuardInput{ToolInput: tc.input}); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

func TestRunGuard_AnUnknownFlagBeforeOrAfterTheGuardNameIsAUsageError(t *testing.T) {
	for name, args := range map[string][]string{
		"before the name": {"--no-such-flag", "ship"},
		"after the name":  {"ship", "--no-such-flag"},
	} {
		var stdout, stderr bytes.Buffer
		built := false
		rc := Run(func(d string) (*signalcenter.Center, func()) { built = true; return streamCenter(d) })(args, strings.NewReader(""), &stdout, &stderr)
		if rc != 10 || built || stdout.Len() != 0 {
			t.Errorf("%s: rc=%d built=%v stdout=%q: a usage error decides nothing and emits nothing", name, rc, built, stdout.String())
		}
	}
}
