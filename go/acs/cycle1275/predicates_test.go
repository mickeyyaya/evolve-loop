//go:build acs

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

func routeInput(t *testing.T) router.RouteInput {
	t.Helper()
	return router.RouteInput{
		Current:   "build",
		Workspace: t.TempDir(),
		Cycle:     1275,
		Env:       map[string]string{"EVOLVE_CLI": "claude-tmux"},
	}
}

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
	if !strings.Contains(mc.Prompt, "wire-schema drift") {
		t.Errorf("mint prompt not carried: %q", mc.Prompt)
	}
	if mc.Dispatch.ModelTierDefault != "balanced" || mc.Dispatch.CLI != "claude" {
		t.Errorf("mint dispatch=%+v, want tier=balanced cli=claude", mc.Dispatch)
	}
}

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
				`"description"`,
				`"when_to_use"`,
			} {
				if !strings.Contains(fb.prompt, want) {
					t.Errorf("plan prompt (%s) never mentions %s — the advisor is not instructed to supply SELECT metadata", tc.name, want)
				}
			}
			if !mintExampleHasMetadata(fb.prompt) {
				t.Errorf("plan prompt (%s) mint JSON example omits description/when_to_use", tc.name)
			}
		})
	}
}

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
