package policy_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_TheFileCellsKeyNamesTheLeafPackagesWithFileKeys(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"fleet":{"count":4,"orchestrator":{"file_cells":["cmd/evolve","cmd/other"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p.FleetConfig().Orchestrator.FileCells(), []string{"cmd/evolve", "cmd/other"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FileCells = %v, want %v", got, want)
	}
}

func TestFleetConfig_AnUnsetFileCellsKeyIsEmptyAndLeavesTheConfigAtItsDefaults(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string]string{
		"no orchestrator block":  `{"fleet":{"count":4}}`,
		"an orchestrator block":  `{"fleet":{"count":4,"orchestrator":{"stage":"shadow"}}}`,
		"an explicit empty list": `{"fleet":{"count":4,"orchestrator":{"file_cells":[]}}}`,
	} {
		p, err := policy.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		cfg := p.FleetConfig().Orchestrator
		if got := cfg.FileCells(); len(got) != 0 {
			t.Errorf("%s: FileCells = %v, want none", name, got)
		}
		defaults := policy.Policy{Fleet: &policy.FleetPolicy{Count: 4}}.FleetConfig().Orchestrator
		if cfg != defaults {
			t.Errorf("%s: Orchestrator = %+v, want the defaults %+v", name, cfg, defaults)
		}
	}
}

func TestFleetConfig_NoFleetBlockLeavesNoFileCells(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FleetConfig().Orchestrator.FileCells(); len(got) != 0 {
		t.Fatalf("FileCells = %v, want none", got)
	}
}

func TestParse_RefusesABadFileCellsKey(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ raw, want string }{
		"an empty entry":        {`{"file_cells":["cmd/evolve",""]}`, "file_cells"},
		"a blank entry":         {`{"file_cells":["  "]}`, "file_cells"},
		"a non-string entry":    {`{"file_cells":[7]}`, "file_cells"},
		"a string for the list": {`{"file_cells":"cmd/evolve"}`, "file_cells"},
		"a line break":          {`{"file_cells":["cmd/evolve\ncmd/other"]}`, "file_cells"},
		"a repeated entry":      {`{"file_cells":["cmd/evolve","cmd/evolve"]}`, "file_cells"},
	} {
		_, err := policy.Parse([]byte(`{"fleet":{"orchestrator":` + tc.raw + `}}`))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want a refusal that names %q", name, err, tc.want)
		}
	}
}
