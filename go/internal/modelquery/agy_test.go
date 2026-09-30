package modelquery

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const realAgyModelsOutput = `Fetching available models...
gemini-3.7-flash-high	Gemini 3.7 Flash (High)
gemini-3.7-flash-medium	Gemini 3.7 Flash (Medium)
gemini-3.7-flash-low	Gemini 3.7 Flash (Low)
gemini-3.1-pro-high	Gemini 3.1 Pro (High)
gemini-3.1-pro-low	Gemini 3.1 Pro (Low)
claude-sonnet-4-6	Claude Sonnet 4.6 (Thinking)
gpt-oss-120b-medium	GPT-OSS 120B (Medium)
`

func TestAgyLister_KeepsTheEffortSuffixThatMakesTheNameValid(t *testing.T) {
	l := AgyLister{Run: func(_ context.Context, name string, args []string, stdin string) (string, error) {
		if name != "agy" || len(args) != 1 || args[0] != "models" {
			t.Fatalf("wrong command: %s %v", name, args)
		}
		if stdin != "" {
			t.Fatalf("model listing must never pass stdin, got %q", stdin)
		}
		return realAgyModelsOutput, nil
	}}
	got, err := l.List(context.Background(), "agy")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"Gemini 3.7 Flash (High)",
		"Gemini 3.7 Flash (Medium)",
		"Gemini 3.7 Flash (Low)",
		"Gemini 3.1 Pro (High)",
		"Gemini 3.1 Pro (Low)",
		"Claude Sonnet 4.6 (Thinking)",
		"GPT-OSS 120B (Medium)",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d models %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("model[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	for _, m := range got {
		if !strings.Contains(m, "(") {
			t.Errorf("model %q carries no effort/capability suffix — agy rejects such names and silently falls back", m)
		}
	}
}

func TestAgyLister_SkipsProgressNoiseAndBlankLines(t *testing.T) {
	l := AgyLister{Run: func(context.Context, string, []string, string) (string, error) {
		return "Fetching available models...\n\n\ngemini-3.1-pro-high\tGemini 3.1 Pro (High)\n", nil
	}}
	got, err := l.List(context.Background(), "agy")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0] != "Gemini 3.1 Pro (High)" {
		t.Fatalf("got %v, want exactly [Gemini 3.1 Pro (High)]", got)
	}
}

func TestAgyLister_HeaderAndBannerRowsNeverBecomeModels(t *testing.T) {
	l := AgyLister{Run: func(context.Context, string, []string, string) (string, error) {
		return "ID\tDISPLAY NAME\n" +
			"MODEL\tNAME\n" +
			"Fetching available models...\n" +
			"gemini-3.7-flash-low\tGemini 3.7 Flash (Low)\n", nil
	}}
	got, err := l.List(context.Background(), "agy")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0] != "Gemini 3.7 Flash (Low)" {
		t.Fatalf("got %v, want exactly [Gemini 3.7 Flash (Low)] — a header row leaked in as a model", got)
	}
}

func TestAgyLister_PropagatesFailure(t *testing.T) {
	l := AgyLister{Run: func(context.Context, string, []string, string) (string, error) {
		return "boom", errors.New("exit 1")
	}}
	if _, err := l.List(context.Background(), "agy"); err == nil {
		t.Fatal("a failed `agy models` must return an error, not an empty list")
	}
}

func TestDefaultRouter_RoutesAgyAndOllamaOffThePicker(t *testing.T) {
	r := DefaultRouter(nil)
	for _, cli := range []string{"agy", "ollama"} {
		l, ok := r.ByCLI[cli]
		if !ok {
			t.Fatalf("%s is not registered — it would fall through to the /model picker, whose pane lacks the information --model needs", cli)
		}
		if _, isRecipe := l.(RecipeLister); isRecipe {
			t.Fatalf("%s is routed to RecipeLister (the picker) — the exact defect this fixes", cli)
		}
	}
	if _, ok := r.ByCLI["codex"]; ok {
		t.Error("codex must NOT be registered: it has no non-interactive listing, so the picker is correct for it")
	}
	if r.Default == nil {
		t.Error("Default must stay set — codex/claude still need the picker")
	}
}
