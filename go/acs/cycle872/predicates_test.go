//go:build acs

package cycle872

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func catalogFromJSON(t *testing.T, raw string) modelcatalog.Catalog {
	t.Helper()
	var c modelcatalog.Catalog
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("catalog JSON failed to parse: %v", err)
	}
	return c
}

func TestC872_001_TierFallbacksJSONRoundTrips(t *testing.T) {
	c := catalogFromJSON(t, `{
		"fetched_at": "2026-07-17T00:00:00Z",
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
			t.Errorf("tier_fallbacks did not round-trip: %q missing from re-marshaled catalog %s", want, out)
		}
	}
}

func TestC872_002_DispatchModelPrimaryWinsOverFallbacks(t *testing.T) {
	c := catalogFromJSON(t, `{
		"clis": {
			"claude": {
				"tier_models": {"deep": "claude-fable-5"},
				"tier_fallbacks": {"deep": ["claude-opus-4-8"]},
				"source": "live"
			}
		}
	}`)
	model, ok := c.DispatchModel("claude", "deep")
	if !ok || model != "claude-fable-5" {
		t.Errorf("primary must win over fallbacks: got (%q, %v), want (%q, true)", model, ok, "claude-fable-5")
	}
}

func TestC872_003_DispatchModelWalksFallbackChain(t *testing.T) {
	c := catalogFromJSON(t, `{
		"clis": {
			"claude": {
				"tier_models": {"deep": "", "fast": "claude-haiku-4-5-20251001"},
				"tier_fallbacks": {"deep": ["", "claude-opus-4-8", "claude-sonnet-5"]},
				"source": "live"
			}
		}
	}`)
	model, ok := c.DispatchModel("claude", "deep")
	if !ok || model != "claude-opus-4-8" {
		t.Errorf("empty primary must fall back through the chain to the first non-empty model: got (%q, %v), want (%q, true)", model, ok, "claude-opus-4-8")
	}
}

func TestC872_004_DispatchModelExhaustedChainStaysNotOK(t *testing.T) {
	c := catalogFromJSON(t, `{
		"clis": {
			"claude": {
				"tier_models": {"deep": ""},
				"tier_fallbacks": {"deep": ["", ""]},
				"source": "live"
			}
		}
	}`)
	if model, ok := c.DispatchModel("claude", "deep"); ok || model != "" {
		t.Errorf("exhausted chain must report ok=false: got (%q, %v)", model, ok)
	}
}

func TestC872_005_DispatchModelFallbacksStillGatedOnLiveSource(t *testing.T) {
	c := catalogFromJSON(t, `{
		"clis": {
			"claude": {
				"tier_models": {"deep": ""},
				"tier_fallbacks": {"deep": ["claude-opus-4-8"]},
				"source": "detect"
			}
		}
	}`)
	if model, ok := c.DispatchModel("claude", "deep"); ok || model != "" {
		t.Errorf("detect-sourced entry must never dispatch, chain or not: got (%q, %v)", model, ok)
	}
}

func TestC872_006_LookupConsultsFallbackChain(t *testing.T) {
	c := catalogFromJSON(t, `{
		"clis": {
			"codex": {
				"tier_models": {"balanced": ""},
				"tier_fallbacks": {"balanced": ["gpt-5-codex"]},
				"source": "detect"
			}
		}
	}`)
	model, ok := c.Lookup("codex", "balanced")
	if !ok || model != "gpt-5-codex" {
		t.Errorf("Lookup must consult the fallback chain when the primary is empty: got (%q, %v), want (%q, true)", model, ok, "gpt-5-codex")
	}
}

func TestC872_007_AbsentFallbacksPreserveSingleShotBehavior(t *testing.T) {
	c := catalogFromJSON(t, `{
		"clis": {
			"claude": {
				"tier_models": {"deep": "claude-fable-5", "fast": ""},
				"source": "live"
			}
		}
	}`)
	if model, ok := c.DispatchModel("claude", "deep"); !ok || model != "claude-fable-5" {
		t.Errorf("live primary without fallbacks must dispatch unchanged: got (%q, %v)", model, ok)
	}
	if model, ok := c.DispatchModel("claude", "fast"); ok || model != "" {
		t.Errorf("empty primary without fallbacks must stay ok=false: got (%q, %v)", model, ok)
	}
	if model, ok := c.Lookup("claude", "fast"); ok || model != "" {
		t.Errorf("Lookup empty primary without fallbacks must stay ok=false: got (%q, %v)", model, ok)
	}
}
