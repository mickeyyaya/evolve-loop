package main

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/modelcatalog"
)

func TestShouldRefreshCatalog(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	stale := modelcatalog.Catalog{FetchedAt: now.Add(-48 * time.Hour)}
	fresh := modelcatalog.Catalog{FetchedAt: now.Add(-1 * time.Hour)}
	empty := modelcatalog.Catalog{}

	tests := []struct {
		name        string
		cat         modelcatalog.Catalog
		autoRefresh bool
		want        bool
	}{
		{"stale → refresh", stale, true, true},
		{"empty (never fetched) → refresh", empty, true, true},
		{"fresh within TTL → skip", fresh, true, false},
		{"disabled overrides stale", stale, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRefreshCatalog(tt.cat, now, tt.autoRefresh); got != tt.want {
				t.Fatalf("shouldRefreshCatalog = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPickClassifierCLI(t *testing.T) {
	tests := []struct {
		name  string
		ready []string
		want  string
	}{
		{"prefers codex", []string{"agy", "ollama", "codex"}, "codex"},
		{"falls to claude when no codex", []string{"agy", "claude"}, "claude"},
		{"falls to agy when only agy of the preferred", []string{"ollama", "agy"}, "agy"},
		{"any ready when none preferred", []string{"ollama"}, "ollama"},
		{"empty when none ready", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickClassifierCLI(tt.ready, ""); got != tt.want {
				t.Fatalf("pickClassifierCLI(%v) = %q, want %q", tt.ready, got, tt.want)
			}
		})
	}
}

func TestPickClassifierCLIEnvOverride(t *testing.T) {
	// Honored when the override names a READY CLI.
	if got := pickClassifierCLI([]string{"codex", "agy"}, "agy"); got != "agy" {
		t.Fatalf("ready override = %q, want agy", got)
	}
	// Ignored (falls through to preference) when the override is NOT ready —
	// a stale override must not classify against a blocked CLI.
	if got := pickClassifierCLI([]string{"codex", "agy"}, "gemini"); got != "codex" {
		t.Fatalf("non-ready override should fall through to codex, got %q", got)
	}
}
