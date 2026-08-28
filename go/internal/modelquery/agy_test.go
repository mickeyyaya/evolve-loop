package modelquery

// agy_test.go — agy must be enumerated by `agy models`, NOT by driving its
// /model picker.
//
// The live incident this pins (observed 2026-08-28, catalog written
// 2026-08-14 with source:"live"): agy's picker separates the model from a
// SEPARATE effort slider, so the pane reads
//
//	Switch Model
//	  Gemini 3.7 Flash
//	  Gemini 3.1 Pro
//	  Effort  ◂ ●━━━━◉─────○ ▸   low  medium  high
//
// The picker parser faithfully captured those unsuffixed names — and they are
// NOT valid `--model` arguments. agy requires model and effort COMBINED
// ("Gemini 3.7 Flash (Low)"), so every tier resolved to a name agy rejects:
//
//	⎿ model Gemini 3.1 Pro is not recognized as a known model or custom model
//	  in settings. Using "Gemini 3.5 Flash (Medium)" instead.
//
// agy does not exit non-zero on that — it warns once and serves the fallback
// for the whole session, so router/memo silently ran Gemini 3.5 Flash (Medium)
// at every tier. `agy models` emits "<id>\t<display name>" with the effort
// already baked in, which is exactly the string --model accepts (both halves
// verified live), so it is the only faithful source.

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// realAgyModelsOutput is the verbatim shape of `agy models` on agy 1.1.22,
// captured live. The leading progress line is not a model and must not become
// one.
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
	// The whole point: an unsuffixed bare family name is what agy REJECTS, so
	// it must never appear on its own. This is the assertion that would have
	// caught the live defect.
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

// A tab is not proof of a model row. Found by adversarial review: if a future
// `agy models` grows a header, a naive "has a tab" rule contributes
// "DISPLAY NAME" as a model — which would then be written into the catalog as
// a tier model and rejected at launch, reproducing the exact incident class
// this file exists to close. Not a live defect on 1.1.22; pinned so it cannot
// become one.
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

// A listing failure must PROPAGATE. Returning an empty list would let the
// refresh commit a catalog with no agy models, which reads as "agy has none"
// rather than "we could not ask" — the silent-degradation shape this whole
// file exists to prevent.
func TestAgyLister_PropagatesFailure(t *testing.T) {
	l := AgyLister{Run: func(context.Context, string, []string, string) (string, error) {
		return "boom", errors.New("exit 1")
	}}
	if _, err := l.List(context.Background(), "agy"); err == nil {
		t.Fatal("a failed `agy models` must return an error, not an empty list")
	}
}

// WIRING PROOF. The lister is worthless if production still routes agy to the
// picker, and that is precisely how the live defect shipped: the parser was
// correct, the ROUTE was wrong. DefaultRouter is the single registry both
// call sites use, so this pins the route itself rather than a copy of it.
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
