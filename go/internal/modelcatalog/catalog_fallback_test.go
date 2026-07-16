package modelcatalog

import (
	"encoding/json"
	"strings"
	"testing"
)

// Catalogs are constructed by unmarshaling JSON — the same ingestion path
// .evolve/model-catalog.json takes — so these tests compile before the
// TierFallbacks field exists and fail (RED) on behavior, not syntax. Do not
// rewrite them as struct literals; the JSON path is part of the contract
// (unknown keys are silently dropped by encoding/json, which is the exact
// defect being fixed).
func mustCatalog(t *testing.T, raw string) Catalog {
	t.Helper()
	var c Catalog
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("catalog JSON failed to parse: %v", err)
	}
	return c
}

func TestCLIEntry_TierFallbacksRoundTrip(t *testing.T) {
	c := mustCatalog(t, `{
		"clis": {
			"claude": {
				"tier_models": {"deep": "claude-fable-5"},
				"tier_fallbacks": {"deep": ["claude-opus-4-8", "claude-sonnet-5"]},
				"source": "live"
			}
		}
	}`)
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	for _, want := range []string{"tier_fallbacks", "claude-opus-4-8", "claude-sonnet-5"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("tier_fallbacks did not round-trip: %q missing from %s", want, out)
		}
	}
}

func TestDispatchModel_FallsBackThroughChain(t *testing.T) {
	tests := []struct {
		name      string
		entryJSON string
		tier      string
		wantModel string
		wantOK    bool
	}{
		{
			name: "primary present wins, fallbacks ignored",
			entryJSON: `{
				"tier_models": {"deep": "claude-fable-5"},
				"tier_fallbacks": {"deep": ["claude-opus-4-8"]},
				"source": "live"
			}`,
			tier:      "deep",
			wantModel: "claude-fable-5",
			wantOK:    true,
		},
		{
			name: "primary empty walks chain, skips empty entries",
			entryJSON: `{
				"tier_models": {"deep": ""},
				"tier_fallbacks": {"deep": ["", "claude-opus-4-8", "claude-sonnet-5"]},
				"source": "live"
			}`,
			tier:      "deep",
			wantModel: "claude-opus-4-8",
			wantOK:    true,
		},
		{
			name: "exhausted chain returns ok=false",
			entryJSON: `{
				"tier_models": {"deep": ""},
				"tier_fallbacks": {"deep": ["", ""]},
				"source": "live"
			}`,
			tier:      "deep",
			wantModel: "",
			wantOK:    false,
		},
		{
			name: "tier absent from chain map returns ok=false",
			entryJSON: `{
				"tier_models": {"deep": ""},
				"tier_fallbacks": {"fast": ["claude-haiku-4-5-20251001"]},
				"source": "live"
			}`,
			tier:      "deep",
			wantModel: "",
			wantOK:    false,
		},
		{
			name: "detect source stays gated even with a live chain",
			entryJSON: `{
				"tier_models": {"deep": ""},
				"tier_fallbacks": {"deep": ["claude-opus-4-8"]},
				"source": "detect"
			}`,
			tier:      "deep",
			wantModel: "",
			wantOK:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := mustCatalog(t, `{"clis": {"claude": `+tt.entryJSON+`}}`)
			model, ok := c.DispatchModel("claude", tt.tier)
			if model != tt.wantModel || ok != tt.wantOK {
				t.Errorf("DispatchModel(claude, %s) = (%q, %v), want (%q, %v)", tt.tier, model, ok, tt.wantModel, tt.wantOK)
			}
		})
	}
}

func TestLookup_FallsBackThroughChain(t *testing.T) {
	tests := []struct {
		name      string
		entryJSON string
		tier      string
		wantModel string
		wantOK    bool
	}{
		{
			name: "primary empty walks chain (no source gate on Lookup)",
			entryJSON: `{
				"tier_models": {"balanced": ""},
				"tier_fallbacks": {"balanced": ["gpt-5-codex"]},
				"source": "detect"
			}`,
			tier:      "balanced",
			wantModel: "gpt-5-codex",
			wantOK:    true,
		},
		{
			name: "exhausted chain returns ok=false",
			entryJSON: `{
				"tier_models": {"balanced": ""},
				"tier_fallbacks": {"balanced": [""]},
				"source": "detect"
			}`,
			tier:      "balanced",
			wantModel: "",
			wantOK:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := mustCatalog(t, `{"clis": {"codex": `+tt.entryJSON+`}}`)
			model, ok := c.Lookup("codex", tt.tier)
			if model != tt.wantModel || ok != tt.wantOK {
				t.Errorf("Lookup(codex, %s) = (%q, %v), want (%q, %v)", tt.tier, model, ok, tt.wantModel, tt.wantOK)
			}
		})
	}
}
