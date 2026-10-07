package policy_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestPatchBlocks_SetsAndRemovesTopLevelBlocksAndKeepsTheRest(t *testing.T) {
	existing := []byte(`{"gc": {"mode": "enforce"}, "pins": {"scout": {"cli": "claude"}}, "workflow": {"universal_fallback_exclude": []}}`)

	out, err := policy.PatchBlocks(existing, map[string]any{
		"pins":        nil,
		"cli_routing": policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}},
	})

	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if _, kept := got["pins"]; kept {
		t.Error("a nil value removes its block")
	}
	var gc map[string]string
	_ = json.Unmarshal(got["gc"], &gc)
	if !reflect.DeepEqual(gc, map[string]string{"mode": "enforce"}) || got["workflow"] == nil {
		t.Errorf("untouched blocks survive: gc=%v workflow=%s", gc, got["workflow"])
	}
	pol, err := policy.Parse(out)
	if err != nil || pol.CLIRouting == nil || !reflect.DeepEqual(pol.CLIRouting.CLIs, []string{"agy", "claude"}) {
		t.Fatalf("the written block parses back: %+v %v", pol.CLIRouting, err)
	}
	if !strings.HasSuffix(string(out), "}\n") {
		t.Error("the file ends with one newline")
	}
}

func TestPatchBlocks_AMalformedFileIsNeverClobbered(t *testing.T) {
	if _, err := policy.PatchBlocks([]byte(`{"gc": `), map[string]any{"pins": nil}); err == nil || !strings.Contains(err.Error(), "refusing to clobber") {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

func TestPatchBlocks_AnEmptyFileStartsFromNothing(t *testing.T) {
	out, err := policy.PatchBlocks(nil, map[string]any{"cli_routing": policy.CLIRouting{CLIs: []string{"claude"}}})
	if err != nil || !strings.Contains(string(out), `"cli_routing"`) {
		t.Fatalf("out = %s err = %v", out, err)
	}
}

func TestParse_IsLoadWithoutTheFile(t *testing.T) {
	if _, err := policy.Parse([]byte(`{"cli_routing": null}`)); err == nil {
		t.Fatal("Parse refuses a null block as Load does")
	}
	pol, err := policy.Parse([]byte(`{"pins": {"scout": {"cli": "claude"}}}`))
	if err != nil || pol.Pins["scout"].CLI != "claude" {
		t.Fatalf("pol = %+v err = %v", pol, err)
	}
}
