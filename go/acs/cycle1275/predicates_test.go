//go:build acs

// Package cycle1275 materializes the cycle-1275 acceptance criteria for this
// fleet lane's sole committed task, mint-carries-select-metadata (triage top_n
// id: mint-carries-select-metadata).
//
// Goal: the advisor's phase MINTER must satisfy the catalog SELECT-metadata
// contract itself instead of being papered over after the fact by
// metadataAllowlist entries. Today router.MintSpec carries no
// Description/WhenToUse channel, so mintConfigsFrom
// (go/internal/core/phase_advisor.go:986) constructs every minted
// phasespec.PhaseSpec with empty SELECT metadata, and
// TestPhaseCatalog_OptionalPhasesHaveSelectMetadata only stays green because
// each concrete minted name is hand-added to the shrinking allowlist (#404,
// #406).
//
// Every predicate below EXERCISES THE SYSTEM UNDER TEST through its PRODUCTION
// caller: core.PhaseAdvisor.Plan — the router.Planner entry point the
// orchestrator calls — which composes the real plan prompt
// (composePlanPrompt → writePlanResponseSchema) and parses the advisor's
// response through the real parsePhasePlan → mintConfigsFrom path. None call
// the minter directly and none grep source text, so none can pass on dead code
// or on a magic string (the cycle-85 degenerate-predicate failure mode).
//
// RED today: 001/003 fail on empty Description/WhenToUse and on a plan prompt
// that never mentions the two keys. The Builder makes them GREEN by adding the
// two omitempty fields to router.MintSpec, threading them through
// mintConfigsFrom into the minted phasespec.PhaseSpec, and extending the
// plan-prompt mint-block documentation + JSON example — WITHOUT modifying this
// file.
//
// SUT CONTRACT the Builder must implement (see test-report.md handoff):
//
//	package router
//	    type MintSpec struct {
//	        …
//	        Description string `json:"description,omitempty"`   // one line: what the phase produces
//	        WhenToUse   string `json:"when_to_use,omitempty"`   // the signal that should trigger SELECTing it
//	    }
//
//	package core
//	    mintConfigsFrom copies e.Mint.Description / e.Mint.WhenToUse into the
//	    constructed phasespec.PhaseSpec (same JSON keys as phasespec's own tags).
//	    writePlanResponseSchema documents both keys and shows them in the
//	    strict-JSON mint example.
package cycle1275

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// planBridge is a core.Bridge stub: it records the prompt the advisor composed
// and replays a canned advisor response, so a predicate drives the REAL
// PhaseAdvisor.Plan path (compose → dispatch → parse → mint) with no subprocess
// and no LLM.
type planBridge struct {
	stdout string
	prompt string
}

func (b *planBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.prompt = req.Prompt
	return core.BridgeResponse{Stdout: b.stdout, ExitCode: 0}, nil
}

func (b *planBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// routeInput builds the minimum viable RouteInput for a Plan call. Workspace is
// a per-test temp dir because the advisor's WS3 capture writes prompt/response
// artifacts there (fail-open, but keep it off the repo).
func routeInput(t *testing.T) router.RouteInput {
	t.Helper()
	return router.RouteInput{
		Current:   "build",
		Workspace: t.TempDir(),
		Cycle:     1275,
		Env:       map[string]string{"EVOLVE_CLI": "claude-tmux"},
	}
}

// planWithMint runs the production Plan path over a one-mint advisor response
// and returns the resulting plan.
func planWithMint(t *testing.T, mintJSON string) *router.PhasePlan {
	t.Helper()
	stdout := `[{"phase":"scout","run":true,"justification":"fresh"},` +
		`{"phase":"schema-drift-check","run":true,"justification":"wire types changed","mint":` + mintJSON + `}]`
	plan, err := core.NewPhaseAdvisor(&planBridge{stdout: stdout}).Plan(routeInput(t))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return plan
}

// TestC1275_001_MintedPhaseCarriesSelectMetadata is the load-bearing predicate:
// an advisor-authored mint block that supplies description/when_to_use must
// land on the minted phaseconfig.PhaseConfig's PhaseSpec, which is what the
// catalog SELECT-metadata gate reads. Driven end-to-end through the production
// planner entry point (PhaseAdvisor.Plan), never through mintConfigsFrom
// directly — a minter reachable only from a test would leave this RED.
func TestC1275_001_MintedPhaseCarriesSelectMetadata(t *testing.T) {
	plan := planWithMint(t, `{"prompt":"You detect wire-schema drift between the advisor and the router.",`+
		`"tier":"balanced","cli":"claude",`+
		`"description":"Reports wire-schema drift between advisor output and router types.",`+
		`"when_to_use":"Select when a cycle edits router wire structs or the advisor response contract."}`)

	if len(plan.MintPhases) != 1 {
		t.Fatalf("MintPhases=%d, want 1 (%+v)", len(plan.MintPhases), plan.MintPhases)
	}
	mc := plan.MintPhases[0]
	if mc.Name != "schema-drift-check" {
		t.Fatalf("mint name=%q, want schema-drift-check", mc.Name)
	}
	if got, want := mc.Description, "Reports wire-schema drift between advisor output and router types."; got != want {
		t.Errorf("minted PhaseSpec.Description=%q, want %q — the minter drops the advisor's SELECT metadata", got, want)
	}
	if got, want := mc.WhenToUse, "Select when a cycle edits router wire structs or the advisor response contract."; got != want {
		t.Errorf("minted PhaseSpec.WhenToUse=%q, want %q — the minter drops the advisor's SELECT metadata", got, want)
	}
	// Existing carried fields must be unaffected by the new plumbing.
	if !strings.Contains(mc.Prompt, "wire-schema drift") {
		t.Errorf("mint prompt not carried: %q", mc.Prompt)
	}
	if mc.Dispatch.ModelTierDefault != "balanced" || mc.Dispatch.CLI != "claude" {
		t.Errorf("mint dispatch=%+v, want tier=balanced cli=claude", mc.Dispatch)
	}
}

// TestC1275_002_MintSpecWireContract proves the advisor actually has a CHANNEL
// for the metadata: the two JSON keys must decode onto router.MintSpec under
// the same names phasespec.PhaseSpec already uses (description/when_to_use), so
// advisor output and catalog schema speak one vocabulary. Read reflectively so
// a missing field is a clear assertion failure rather than a package-wide
// compile break that hides the other predicates' verdicts.
func TestC1275_002_MintSpecWireContract(t *testing.T) {
	var spec router.MintSpec
	raw := `{"prompt":"p","tier":"deep","cli":"claude","description":"what it produces","when_to_use":"when to select it"}`
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal MintSpec: %v", err)
	}
	v := reflect.ValueOf(spec)
	for _, tc := range []struct{ field, want string }{
		{"Description", "what it produces"},
		{"WhenToUse", "when to select it"},
	} {
		f := v.FieldByName(tc.field)
		if !f.IsValid() {
			t.Errorf("router.MintSpec has no %s field — the advisor has no channel to supply SELECT metadata", tc.field)
			continue
		}
		if f.Kind() != reflect.String || f.String() != tc.want {
			t.Errorf("MintSpec.%s=%q, want %q (check the json tag)", tc.field, f.String(), tc.want)
		}
	}
}

// TestC1275_003_PlanPromptDocumentsMintMetadata proves the other half of the
// fix: an advisor that is never TOLD about the keys will never emit them, so
// the composed plan prompt must document description/when_to_use and show them
// in its strict-JSON mint example. Asserted on the prompt the bridge actually
// received, for BOTH prompt-assembly paths (persona = production,
// no-persona = legacy inline fallback) — the pair that silently diverged at
// #293.
func TestC1275_003_PlanPromptDocumentsMintMetadata(t *testing.T) {
	stdout := `[{"phase":"scout","run":true,"justification":"fresh"}]`
	cases := []struct {
		name string
		opts []core.PhaseAdvisorOption
	}{
		{name: "persona", opts: []core.PhaseAdvisorOption{core.WithPersona("You are the evolve router.")}},
		{name: "legacy-inline", opts: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &planBridge{stdout: stdout}
			if _, err := core.NewPhaseAdvisor(fb, tc.opts...).Plan(routeInput(t)); err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if fb.prompt == "" {
				t.Fatalf("bridge received an empty prompt")
			}
			for _, want := range []string{
				`"description"`, // named in the mint-block instructions
				`"when_to_use"`, // named in the mint-block instructions
			} {
				if !strings.Contains(fb.prompt, want) {
					t.Errorf("plan prompt (%s) never mentions %s — the advisor is not instructed to supply SELECT metadata", tc.name, want)
				}
			}
			// The strict-JSON example is what the advisor pattern-matches on, so
			// the keys must appear INSIDE the mint example object, not only in prose.
			if !mintExampleHasMetadata(fb.prompt) {
				t.Errorf("plan prompt (%s) mint JSON example omits description/when_to_use", tc.name)
			}
		})
	}
}

// mintExampleHasMetadata reports whether the prompt's `"mint":{…}` example
// object itself names both metadata keys.
func mintExampleHasMetadata(prompt string) bool {
	i := strings.Index(prompt, `"mint":{`)
	if i < 0 {
		return false
	}
	rest := prompt[i:]
	if j := strings.Index(rest, "}"); j >= 0 {
		rest = rest[:j]
	}
	return strings.Contains(rest, `"description"`) && strings.Contains(rest, `"when_to_use"`)
}

// TestC1275_004_MintWithoutMetadataStaysBackwardCompatible is the negative /
// edge case: today's entire installed base emits mint blocks with NO metadata
// keys. Those must keep minting exactly as before (empty metadata, all other
// fields carried, no error), and a MintSpec with unset metadata must marshal
// WITHOUT the new keys — omitempty, so the pre-fix wire form stays
// byte-identical and no stale allowlist entry is invalidated by this change.
func TestC1275_004_MintWithoutMetadataStaysBackwardCompatible(t *testing.T) {
	plan := planWithMint(t, `{"prompt":"legacy persona","tier":"deep","cli":"claude","writes_source":false}`)
	if len(plan.MintPhases) != 1 {
		t.Fatalf("MintPhases=%d, want 1 — a metadata-less mint must still register", len(plan.MintPhases))
	}
	mc := plan.MintPhases[0]
	if mc.Description != "" || mc.WhenToUse != "" {
		t.Errorf("metadata-less mint invented metadata: description=%q when_to_use=%q", mc.Description, mc.WhenToUse)
	}
	if mc.Prompt != "legacy persona" || mc.Dispatch.ModelTierDefault != "deep" || mc.WritesSource {
		t.Errorf("legacy mint fields regressed: %+v (writes_source=%v)", mc.Dispatch, mc.WritesSource)
	}

	got, err := json.Marshal(router.MintSpec{Prompt: "p", Tier: "deep", CLI: "claude"})
	if err != nil {
		t.Fatalf("marshal MintSpec: %v", err)
	}
	if want := `{"prompt":"p","tier":"deep","cli":"claude"}`; string(got) != want {
		t.Errorf("MintSpec wire form changed for unset metadata:\n got %s\nwant %s (both new fields need omitempty)", got, want)
	}
}
