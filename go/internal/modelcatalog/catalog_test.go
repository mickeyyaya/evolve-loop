package modelcatalog

import (
	"testing"
	"time"
)

func sampleCatalog(fetched time.Time) Catalog {
	return Catalog{
		FetchedAt: fetched,
		CLIs: map[string]CLIEntry{
			"claude": {TierModels: map[string]string{
				"fast":     "claude-haiku-4-5",
				"balanced": "claude-sonnet-4-6",
				"deep":     "claude-opus-4-8",
			}},
			"codex": {TierModels: map[string]string{
				"balanced": "gpt-5.4",
			}},
		},
	}
}

func TestLookup(t *testing.T) {
	c := sampleCatalog(time.Unix(0, 0))
	tests := []struct {
		name      string
		cli, tier string
		wantModel string
		wantOK    bool
	}{
		{"hit fast", "claude", "fast", "claude-haiku-4-5", true},
		{"hit deep", "claude", "deep", "claude-opus-4-8", true},
		{"hit other cli", "codex", "balanced", "gpt-5.4", true},
		{"miss unknown cli", "gemini", "fast", "", false},
		{"miss unknown tier", "claude", "ultra", "", false},
		{"miss tier absent for cli", "codex", "fast", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := c.Lookup(tt.cli, tt.tier)
			if got != tt.wantModel || ok != tt.wantOK {
				t.Fatalf("Lookup(%q,%q) = (%q,%v), want (%q,%v)",
					tt.cli, tt.tier, got, ok, tt.wantModel, tt.wantOK)
			}
		})
	}
}

func TestLookupEmptyModelIsMiss(t *testing.T) {
	// An explicit empty-string model must be treated as a miss so the caller
	// falls back to the static manifest instead of dispatching with no model.
	c := Catalog{CLIs: map[string]CLIEntry{
		"claude": {TierModels: map[string]string{"fast": ""}},
	}}
	if got, ok := c.Lookup("claude", "fast"); ok || got != "" {
		t.Fatalf("Lookup with empty model = (%q,%v), want (\"\",false)", got, ok)
	}
}

func TestLookupNilCatalog(t *testing.T) {
	var c Catalog // zero value, nil CLIs map
	if got, ok := c.Lookup("claude", "fast"); ok || got != "" {
		t.Fatalf("zero Catalog Lookup = (%q,%v), want (\"\",false)", got, ok)
	}
}

func TestIsStale(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	ttl := 24 * time.Hour
	tests := []struct {
		name      string
		fetchedAt time.Time
		want      bool
	}{
		{"never fetched (zero time)", time.Time{}, true},
		{"fresh (1h ago)", now.Add(-1 * time.Hour), false},
		{"exactly at ttl boundary is stale", now.Add(-24 * time.Hour), true},
		{"just inside ttl", now.Add(-24*time.Hour + time.Second), false},
		{"well past ttl", now.Add(-48 * time.Hour), true},
		{"future fetch (clock skew) is fresh", now.Add(1 * time.Hour), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Catalog{FetchedAt: tt.fetchedAt}
			if got := c.IsStale(now, ttl); got != tt.want {
				t.Fatalf("IsStale(fetched=%v) = %v, want %v", tt.fetchedAt, got, tt.want)
			}
		})
	}
}

func TestEmpty(t *testing.T) {
	if !(Catalog{}).Empty() {
		t.Fatal("zero Catalog should be Empty")
	}
	if !(Catalog{CLIs: map[string]CLIEntry{}}).Empty() {
		t.Fatal("Catalog with empty CLIs map should be Empty")
	}
	if sampleCatalog(time.Now()).Empty() {
		t.Fatal("populated Catalog should not be Empty")
	}
}
