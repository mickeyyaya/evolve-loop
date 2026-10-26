package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func writeHeldPolicy(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestHeldProposalsStageFor_DefaultsToShadowAndReadsTheKey(t *testing.T) {
	cases := []struct {
		name string
		body string
		want config.Stage
	}{
		{"no key", `{}`, config.StageShadow},
		{"an empty stage", `{"held_proposals":{}}`, config.StageShadow},
		{"enforce", `{"held_proposals":{"stage":"enforce"}}`, config.StageEnforce},
		{"off", `{"held_proposals":{"stage":"off"}}`, config.StageOff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := HeldProposalsStageFor(writeHeldPolicy(t, tc.body))
			if err != nil || got != tc.want {
				t.Errorf("HeldProposalsStageFor = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
	if got, err := HeldProposalsStageFor(t.TempDir()); err != nil || got != config.StageShadow {
		t.Errorf("no policy file = %v, %v, want shadow", got, err)
	}
}

func TestHeldProposalsStageFor_ABadPolicyFailsClosedToEnforce(t *testing.T) {
	for _, body := range []string{
		`{"held_proposals":{"stage":"sometimes"}}`,
		`{"held_proposals":{"stage":"shadow","approve":"lane"}}`,
	} {
		got, err := HeldProposalsStageFor(writeHeldPolicy(t, body))
		if err == nil || got != config.StageEnforce || !strings.Contains(err.Error(), "held_proposals") {
			t.Errorf("body %s = %v, %v, want enforce and a held_proposals error", body, got, err)
		}
	}
}

func TestHeldProposalsPolicy_UnmarshalJSONRefusesAnUnknownKeyAndABadStage(t *testing.T) {
	var h HeldProposalsPolicy
	if err := h.UnmarshalJSON([]byte(`{"stage":"shadow","approve":"lane"}`)); err == nil || !strings.Contains(err.Error(), "held_proposals") {
		t.Errorf("unknown key: %v, want a held_proposals error", err)
	}
	if err := h.UnmarshalJSON([]byte(`{"stage":"sometimes"}`)); err == nil {
		t.Error("a bad stage must be refused")
	}
	if err := h.UnmarshalJSON([]byte(`{"stage":"enforce"}`)); err != nil || h.Stage != "enforce" {
		t.Errorf("enforce: %v, %+v", err, h)
	}
}

func TestPolicy_HeldProposalsStage_DefaultsToShadowAndReadsTheBlock(t *testing.T) {
	if got := (Policy{}).HeldProposalsStage(); got != config.StageShadow {
		t.Errorf("no block = %v, want shadow", got)
	}
	if got := (Policy{HeldProposals: &HeldProposalsPolicy{Stage: "off"}}).HeldProposalsStage(); got != config.StageOff {
		t.Errorf("off block = %v, want off", got)
	}
}
