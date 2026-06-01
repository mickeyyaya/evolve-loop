package modelquery

import (
	"context"
	"errors"
	"testing"
)

// fakeCapturer returns a canned pane per CLI, or an error if set.
type fakeCapturer struct {
	panes map[string]string
	err   error
}

func (f fakeCapturer) CaptureModelPicker(_ context.Context, cli string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.panes[cli], nil
}

func TestRecipeListerParsesCapturedPicker(t *testing.T) {
	l := RecipeLister{Capturer: fakeCapturer{panes: map[string]string{
		"codex":  codexPickerPane,
		"agy":    agyPickerPane,
		"claude": claudePickerPane,
	}}}

	codex, err := l.List(context.Background(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, codex, []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2"})

	claude, err := l.List(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, claude, []string{"opus", "sonnet", "haiku"})

	if agy, err := l.List(context.Background(), "agy"); err != nil || len(agy) != 8 {
		t.Fatalf("agy = %v (err %v)", agy, err)
	}
}

func TestRecipeListerUnknownCLI(t *testing.T) {
	l := RecipeLister{Capturer: fakeCapturer{}}
	if _, err := l.List(context.Background(), "ollama"); err == nil {
		t.Fatal("expected error: ollama has no /model picker parser")
	}
}

func TestRecipeListerCaptureError(t *testing.T) {
	l := RecipeLister{Capturer: fakeCapturer{err: errors.New("tmux dead")}}
	if _, err := l.List(context.Background(), "codex"); err == nil {
		t.Fatal("expected capture error to propagate")
	}
}

func TestRecipeListerEmptyPicker(t *testing.T) {
	l := RecipeLister{Capturer: fakeCapturer{panes: map[string]string{"codex": "no picker rendered"}}}
	if _, err := l.List(context.Background(), "codex"); err == nil {
		t.Fatal("expected error when the picker yields no models")
	}
}

func TestRecipeListerNilCapturer(t *testing.T) {
	if _, err := (RecipeLister{}).List(context.Background(), "codex"); err == nil {
		t.Fatal("expected error when Capturer is nil")
	}
}

func TestRecipeListerCustomParsers(t *testing.T) {
	// The parser registry is overridable (open for extension).
	l := RecipeLister{
		Capturer: fakeCapturer{panes: map[string]string{"x": "anything"}},
		Parsers:  map[string]PickerParser{"x": func(string) []string { return []string{"only-model"} }},
	}
	got, err := l.List(context.Background(), "x")
	if err != nil || len(got) != 1 || got[0] != "only-model" {
		t.Fatalf("custom parser = %v (err %v)", got, err)
	}
}
