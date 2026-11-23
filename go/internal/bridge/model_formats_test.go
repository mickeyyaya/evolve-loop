package bridge

import (
	"strings"
	"testing"
)

func TestModelsFormat_EachListerManifestDeclaresAKnownID(t *testing.T) {
	t.Parallel()
	want := map[string]string{"agy-tmux": "agy-models-v1", "agy-claude-tmux": "agy-models-v1", "ollama-tmux": "ollama-list-v1"}
	for cli, id := range want {
		m, err := LoadManifest(cli)
		if err != nil || m.Parsers[ParserModels] != id {
			t.Errorf("%s: parsers.models = %q, err %v; want %q", cli, m.Parsers[ParserModels], err, id)
		}
	}
	for _, cli := range []string{"claude-tmux", "codex-tmux"} {
		if m, err := LoadManifest(cli); err != nil || m.Parsers[ParserModels] != "" {
			t.Errorf("%s declares a models parser %q (err %v); it has no non-interactive listing", cli, m.Parsers[ParserModels], err)
		}
	}
}

func TestModelsFormat_LoadAcceptsARegisteredID(t *testing.T) {
	t.Parallel()
	if err := validateFormats("x", Manifest{Parsers: map[string]string{ParserModels: "agy-models-v1"}}); err != nil {
		t.Fatalf("a registered id was refused: %v", err)
	}
	if err := validateFormats("x", Manifest{Parsers: map[string]string{ParserModels: "bogus"}}); err == nil {
		t.Fatal("an unknown id was accepted")
	}
}

func TestModelsFormat_EachIDOwnsItsArgvAndItsParse(t *testing.T) {
	t.Parallel()
	agy, ok := modelsFormats["agy-models-v1"]
	if !ok || strings.Join(agy.argv, " ") != "models" {
		t.Fatalf("agy-models-v1 = %+v", agy)
	}
	ollama, ok := modelsFormats["ollama-list-v1"]
	if !ok || strings.Join(ollama.argv, " ") != "list" {
		t.Fatalf("ollama-list-v1 = %+v", ollama)
	}
}
