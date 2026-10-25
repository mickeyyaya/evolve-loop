package policy

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var todaysClaudeFloor = []string{"auditor", "adversarial-review", "tdd-engineer", "spec-verifier", "spec-verify", "code-reviewer"}

func TestDefaultFloors_HoldTodaysClaudeFloorAsData(t *testing.T) {
	floors := DefaultFloors()
	if !reflect.DeepEqual(floors.Families(), []string{"claude"}) {
		t.Fatalf("the compiled default floors name the families %v, want only claude", floors.Families())
	}
	if !reflect.DeepEqual(floors["claude"], todaysClaudeFloor) {
		t.Fatalf("the claude floor is %v, want today's %v", floors["claude"], todaysClaudeFloor)
	}
	for _, agent := range todaysClaudeFloor {
		if got := floors.FamilyOf(agent); got != "claude" {
			t.Errorf("FamilyOf(%s) = %q, want claude", agent, got)
		}
	}
	for _, agent := range []string{"builder", "scout", ""} {
		if got := floors.FamilyOf(agent); got != "" {
			t.Errorf("FamilyOf(%s) = %q, want no floor", agent, got)
		}
	}
}

func TestDefaultFloors_IsACopyACallerCannotWiden(t *testing.T) {
	first := DefaultFloors()
	first["claude"][0] = "builder"
	first["agy"] = []string{"scout"}
	second := DefaultFloors()
	if second.FamilyOf("builder") != "" || second.FamilyOf("scout") != "" || second.FamilyOf("auditor") != "claude" {
		t.Fatalf("a caller's write leaked into the compiled floors: %v", second)
	}
}

func TestPolicyFloors_AnAbsentKeyReadsTheDefaultAndAnEmptyKeyHasNoFloor(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		auditor string
	}{
		{"no cli_routing block", `{}`, "claude"},
		{"a block with no floors key", `{"cli_routing":{"clis":["agy","claude"]}}`, "claude"},
		{"an empty floors key", `{"cli_routing":{"clis":["agy"],"floors":{}}}`, ""},
		{"a floor on another family", `{"cli_routing":{"clis":["agy"],"floors":{"agy":["auditor"]}}}`, "agy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Parse([]byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if got := p.Floors().FamilyOf("auditor"); got != tc.auditor {
				t.Fatalf("Floors().FamilyOf(auditor) = %q, want %q", got, tc.auditor)
			}
		})
	}
}

func TestPolicyFloors_ACallerCannotWidenTheDeclaredFloors(t *testing.T) {
	p, err := Parse([]byte(`{"cli_routing":{"clis":["agy"],"floors":{"agy":["auditor"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	p.Floors()["agy"][0] = "builder"
	if p.Floors().FamilyOf("auditor") != "agy" {
		t.Fatal("a caller's write leaked into the policy's floors")
	}
}

func TestFloors_FamilyOfReadsTheFirstFamilyInNameOrder(t *testing.T) {
	floors := Floors{"codex": {"auditor"}, "agy": {"auditor"}}
	if got := floors.FamilyOf("auditor"); got != "agy" {
		t.Fatalf("FamilyOf(auditor) = %q, want agy, the first family by name", got)
	}
	if got := Floors(nil).Families(); len(got) != 0 {
		t.Fatalf("nil floors name the families %v", got)
	}
}

func TestSeatDefault_EverySeatHoldsTodaysValue(t *testing.T) {
	want := map[string]Seat{
		SeatPhaseAdvisor:       {CLI: "claude-tmux", Model: "deep"},
		SeatFailureAdvisor:     {CLI: "claude-tmux", Model: "deep"},
		SeatPlanJudge:          {CLI: "claude-tmux", Model: "fast"},
		SeatRouter:             {Model: "deep"},
		SeatContractEscalation: {CLI: "claude-tmux"},
		SeatPostmortem:         {CLI: "claude-tmux"},
		SeatObserver:           {CLI: "claude-tmux"},
		SeatSubagent:           {CLI: "claude-tmux"},
		SeatNoProfile:          {CLI: "claude-tmux"},
		SeatWaveHealth:         {CLI: "claude-tmux"},
	}
	for name, seat := range want {
		if got := SeatDefault(name); got != seat {
			t.Errorf("SeatDefault(%s) = %+v, want %+v", name, got, seat)
		}
	}
	if got := SeatDefault("no-such-seat"); got != (Seat{}) {
		t.Errorf("an unknown seat has no default, got %+v", got)
	}
	if n := len(compiledRoutingDefaults().Seats); n != len(want) {
		t.Errorf("the data file holds %d seats, the test pins %d: pin every seat", n, len(want))
	}
}

func TestRoutingDefaults_TheFamilyListsHoldTodaysOrder(t *testing.T) {
	if got := ClassifierFamilies(); !reflect.DeepEqual(got, []string{"codex", "claude", "agy"}) {
		t.Errorf("ClassifierFamilies() = %v", got)
	}
	if got := UnmeasuredOrder(); !reflect.DeepEqual(got, []string{"claude", "agy", "agy-claude"}) {
		t.Errorf("UnmeasuredOrder() = %v", got)
	}
	ClassifierFamilies()[0] = "ollama"
	UnmeasuredOrder()[0] = "ollama"
	if ClassifierFamilies()[0] != "codex" || UnmeasuredOrder()[0] != "claude" {
		t.Error("a caller's write leaked into the compiled family lists")
	}
	if got := DefaultWaveHealthDrivers(); !reflect.DeepEqual(got, []string{"claude-tmux"}) {
		t.Errorf("DefaultWaveHealthDrivers() = %v", got)
	}
	if got := (Policy{}).WorkflowConfig().UniversalFallbackExclude; !reflect.DeepEqual(got, []string{"agy"}) {
		t.Errorf("the compiled universal_fallback_exclude = %v", got)
	}
}

func TestMustRoutingDefaults_AMalformedDataFileStopsTheBinary(t *testing.T) {
	for _, raw := range []string{`{`, `{"seats":{},"surprise":1}`} {
		func() {
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(r.(error).Error(), "routing_defaults.json") {
					t.Errorf("%s: want a panic that names the data file, got %v", raw, r)
				}
			}()
			mustRoutingDefaults([]byte(raw))
		}()
	}
}

func TestCheckedInPolicy_StrictDecodesWithTheFloorsKey(t *testing.T) {
	p, err := Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if p.CLIRouting == nil || p.CLIRouting.Floors == nil {
		t.Fatal("the checked-in policy declares no cli_routing.floors key")
	}
	if !reflect.DeepEqual(p.Floors(), DefaultFloors()) {
		t.Fatalf("the checked-in floors %v differ from the compiled default %v", p.Floors(), DefaultFloors())
	}
}

func TestCLIRouting_AnEmptyFloorsKeySurvivesAWrite(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     string
		auditor string
	}{
		{"an empty floors key stays no floor", `{"cli_routing":{"clis":["agy"],"floors":{}}}`, ""},
		{"an absent floors key stays the default", `{"cli_routing":{"clis":["agy","claude"]}}`, "claude"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Parse([]byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := PatchBlocks([]byte(tc.raw), map[string]any{"cli_routing": *p.CLIRouting})
			if err != nil {
				t.Fatal(err)
			}
			again, err := Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := again.Floors().FamilyOf("auditor"); got != tc.auditor {
				t.Fatalf("after a write FamilyOf(auditor) = %q, want %q:\n%s", got, tc.auditor, raw)
			}
		})
	}
}
