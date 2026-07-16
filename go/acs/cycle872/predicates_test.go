//go:build acs

// Package cycle872 materializes the cycle-872 acceptance criteria for this
// fleet lane's sole committed task, wire-tier-fallback-chain (fleet_scope
// todo-id: overlay-injection-dormant-wire-fable-deep).
//
// Scout confirmed the operator-authored `tier_fallbacks` key in
// .evolve/model-catalog.json is dead config: encoding/json silently drops it
// because modelcatalog.CLIEntry declares no matching field, and
// DispatchModel/Lookup are single-shot with no chain traversal. The task adds
// `TierFallbacks map[string][]string` to CLIEntry and makes DispatchModel and
// Lookup walk the tier's chain when the primary TierModels entry is empty,
// returning the first non-empty model. Behavior with no fallbacks configured
// must be byte-identical to today (manifest fallback preserved).
//
// Every predicate below exercises the system under test directly: it imports
// the modelcatalog package, constructs a Catalog by unmarshaling JSON (the
// exact ingestion path .evolve/model-catalog.json takes), and asserts on
// DispatchModel/Lookup return values — no source-grep predicates.
package cycle872

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// catalogFromJSON decodes raw through the same path loadCatalogCached uses for
// .evolve/model-catalog.json, so a silently-dropped key fails these predicates
// exactly the way it fails the operator.
func catalogFromJSON(t *testing.T, raw string) modelcatalog.Catalog {
	t.Helper()
	var c modelcatalog.Catalog
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("catalog JSON failed to parse: %v", err)
	}
	return c
}

// TestC872_001_TierFallbacksJSONRoundTrips pins AC-1: the tier_fallbacks key
// survives an Unmarshal→Marshal round trip instead of being dropped as an
// unknown field. RED today: CLIEntry has no TierFallbacks field, so the
// re-marshaled JSON lacks the key.
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

// TestC872_002_DispatchModelPrimaryWinsOverFallbacks pins the priority order:
// a non-empty primary TierModels entry is returned as-is even when a fallback
// chain is configured. Pre-existing GREEN (current single-shot behavior) —
// kept as a regression guard so the chain implementation cannot invert
// priority.
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

// TestC872_003_DispatchModelWalksFallbackChain pins AC-2 (the core RED): when
// the primary is empty, DispatchModel walks TierFallbacks[tier] in order and
// returns the first non-empty model — including skipping empty chain entries
// (edge axis). RED today: no chain traversal exists, so this returns ok=false.
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

// TestC872_004_DispatchModelExhaustedChainStaysNotOK is the negative
// predicate: primary empty and every chain entry empty must yield ok=false so
// the caller's static-manifest fallback stays byte-identical. Guards against
// an implementation returning ("", true) at chain exhaustion.
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

// TestC872_005_DispatchModelFallbacksStillGatedOnLiveSource is the second
// negative predicate: a detect-sourced entry must never dispatch, even with a
// populated fallback chain — the SourceLive gate (catalog.go) must survive the
// chain change unchanged.
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

// TestC872_006_LookupConsultsFallbackChain pins the Lookup half of AC-2:
// display-path lookups (models list) get the same chain treatment for
// consistency. Lookup has no SourceLive gate, so a detect entry's chain is
// still consulted. RED today.
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

// TestC872_007_AbsentFallbacksPreserveSingleShotBehavior pins AC-3
// (behavior-preserving when tier_fallbacks is absent): with no chain
// configured, an empty primary is still ok=false and a live primary is still
// returned — the exact current contract. Pre-existing GREEN, kept as the
// no-regression anchor.
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
