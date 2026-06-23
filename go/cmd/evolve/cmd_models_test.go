package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/modelcatalog"
)

func runModelsCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := runModels(args, nil, &out, &errb)
	return code, out.String(), errb.String()
}

// seedCatalog writes a small catalog into a fresh temp .evolve dir.
func seedCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cat := modelcatalog.Catalog{
		FetchedAt: time.Now().UTC(),
		CLIs: map[string]modelcatalog.CLIEntry{
			"codex": {TierModels: map[string]string{"fast": "gpt-5.4-mini", "balanced": "gpt-5.4", "deep": "gpt-5.5"}},
		},
	}
	if err := modelcatalog.Write(dir, cat); err != nil {
		t.Fatalf("seed Write: %v", err)
	}
	return dir
}

func TestModelsUnknownSubcommand(t *testing.T) {
	if code, _, _ := runModelsCLI("frobnicate"); code != 10 {
		t.Fatalf("unknown subcommand code=%d, want 10", code)
	}
	if code, _, _ := runModelsCLI(); code != 10 {
		t.Fatalf("no subcommand code=%d, want 10", code)
	}
}

func TestModelsListEmpty(t *testing.T) {
	code, out, _ := runModelsCLI("list", "--evolve-dir", t.TempDir())
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "No model catalog yet") {
		t.Fatalf("expected empty-cache hint, got %q", out)
	}
}

func TestModelsListEmptyJSON(t *testing.T) {
	// `list --json` on a fresh repo must emit an explicit empty signal, not a
	// zero-value {"clis":null}.
	code, out, _ := runModelsCLI("list", "--evolve-dir", t.TempDir(), "--json")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, "empty") {
		t.Fatalf("expected explicit empty JSON signal, got %q", out)
	}
	if strings.Contains(out, "null") {
		t.Fatalf("empty --json should not emit null, got %q", out)
	}
}

func TestModelsListHuman(t *testing.T) {
	dir := seedCatalog(t)
	code, out, _ := runModelsCLI("list", "--evolve-dir", dir)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "codex") || !strings.Contains(out, "gpt-5.5") {
		t.Fatalf("human output missing catalog data: %q", out)
	}
}

// TestModelsListJSONFlagOrdering is the regression guard for the reorderArgs
// value-flag bug: `--json` AFTER `--evolve-dir <path>` must still emit JSON,
// not fall through to the human/empty path.
func TestModelsListJSONFlagOrdering(t *testing.T) {
	dir := seedCatalog(t)
	cases := [][]string{
		{"list", "--evolve-dir", dir, "--json"},   // --json last (the bug case)
		{"list", "--json", "--evolve-dir", dir},   // --json first
		{"list", "--evolve-dir=" + dir, "--json"}, // = form
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, out, errb := runModelsCLI(args...)
			if code != 0 {
				t.Fatalf("code=%d stderr=%q", code, errb)
			}
			if !strings.HasPrefix(strings.TrimSpace(out), "{") {
				t.Fatalf("expected JSON output, got %q", out)
			}
			if !strings.Contains(out, `"fetched_at"`) || !strings.Contains(out, "gpt-5.5") {
				t.Fatalf("JSON missing catalog fields: %q", out)
			}
		})
	}
}
