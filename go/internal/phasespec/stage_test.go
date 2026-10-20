package phasespec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func preBuildOrder() []string {
	return []string{"scout", "triage", "tdd", "build", "audit", "ship"}
}

func stagedSpec(name, stage, after string) PhaseSpec {
	return PhaseSpec{Name: name, Optional: true, Role: "evaluate", Stage: stage, After: after}
}

func TestStage_DecodesFromPhaseJSON(t *testing.T) {
	t.Parallel()
	var s PhaseSpec
	if err := json.Unmarshal([]byte(`{"name":"bug-reproduction","stage":"pre-build"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Stage != StagePreBuild {
		t.Fatalf("Stage = %q, want %q", s.Stage, StagePreBuild)
	}
	if StagePreBuild != "pre-build" || StagePostBuild != "post-build" {
		t.Fatalf("stage words are the declared vocabulary: %q %q", StagePreBuild, StagePostBuild)
	}
}

func TestValidateUserSpec_StageIsAClosedSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stage   string
		refused bool
	}{
		{"", false},
		{"pre-build", false},
		{"post-build", false},
		{"bogus", true},
		{"Pre-Build", true},
		{" pre-build", true},
		{"pre_build", true},
		{"prebuild", true},
		{"build", true},
	}
	for _, c := range cases {
		t.Run("stage="+c.stage, func(t *testing.T) {
			t.Parallel()
			v := ValidateUserSpec(stagedSpec("my-check", c.stage, ""))
			var stageViolations []string
			for _, msg := range v {
				if strings.Contains(msg, "stage") {
					stageViolations = append(stageViolations, msg)
				}
			}
			if c.refused && len(stageViolations) == 0 {
				t.Errorf("stage %q must be refused, violations = %v", c.stage, v)
			}
			if !c.refused && len(v) != 0 {
				t.Errorf("stage %q must validate clean, violations = %v", c.stage, v)
			}
			if c.refused && !strings.Contains(strings.Join(stageViolations, "\n"), c.stage) {
				t.Errorf("the violation must quote the offending value %q, got %v", c.stage, stageViolations)
			}
		})
	}
}

func TestApplyUserRouting_AnUnknownStageIsNotRouted(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Order: preBuildOrder()}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{stagedSpec("odd-check", "bogus", "triage")}, Catalog{})
	if slices.Contains(cfg.Order, "odd-check") {
		t.Errorf("a spec with an unknown stage must not enter the order: %v", cfg.Order)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "odd-check") || !strings.Contains(warns[0], "stage") {
		t.Errorf("warnings = %v, want exactly one naming the phase and the stage", warns)
	}
}

func TestLoad_ARegistrySpecWithAnUnknownStageIsAHardError(t *testing.T) {
	t.Parallel()
	path := writeRegistry(t, `{"phases":[{"name":"scout"},{"name":"odd","stage":"bogus"}]}`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "stage") || !strings.Contains(err.Error(), "odd") {
		t.Fatalf("Load must refuse a registry phase with an unknown stage, naming it; err = %v", err)
	}
}

func TestApplyUserRouting_RefusesAPreBuildPhaseAnchoredAfterBuild(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Order: preBuildOrder()}
	spec := stagedSpec("bug-reproduction", StagePreBuild, "build")
	spec.Routing = &config.RoutingBlock{}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{spec}, Catalog{})
	if slices.Contains(cfg.Order, "bug-reproduction") {
		t.Errorf("a pre-build phase anchored after build must be refused, order = %v", cfg.Order)
	}
	if !slices.Equal(cfg.Order, preBuildOrder()) {
		t.Errorf("a refused phase leaves the order untouched: %v", cfg.Order)
	}
	if _, enabled := cfg.PhaseEnable["bug-reproduction"]; enabled {
		t.Errorf("a refused phase must not be content-enabled: %v", cfg.PhaseEnable)
	}
	if _, triggered := cfg.Triggers["bug-reproduction"]; triggered {
		t.Errorf("a refused phase must not keep its insert_when triggers: %v", cfg.Triggers)
	}
	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warns)
	}
	for _, want := range []string{"bug-reproduction", "pre-build", "build"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("warning %q must name %q", warns[0], want)
		}
	}
	if ev := RoutingDefectEvent("test", warns[0]); ev.Code != CodeRoutingDefect || ev.Reason != warns[0] {
		t.Errorf("the refusal travels as the registered routing-defect signal: %+v", ev)
	}
}

func TestApplyUserRouting_RefusesAPreBuildPhaseWithNoAnchor(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Order: preBuildOrder()}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{stagedSpec("late-repro", StagePreBuild, "")}, Catalog{})
	if slices.Contains(cfg.Order, "late-repro") {
		t.Errorf("the pre-audit default slot is after build, so a pre-build phase there is refused: %v", cfg.Order)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "late-repro") || !strings.Contains(warns[0], "pre-build") {
		t.Errorf("warnings = %v, want exactly one naming the phase and its stage", warns)
	}
}

func TestApplyUserRouting_RefusesAPreBuildPhaseChainedBehindAnAfterBuildAnchor(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Order: preBuildOrder()}
	specs := []PhaseSpec{
		stagedSpec("second-repro", StagePreBuild, "first-note"),
		{Name: "first-note", Optional: true, After: "build"},
	}
	warns := ApplyUserRouting(&cfg, specs, Catalog{})
	if slices.Contains(cfg.Order, "second-repro") {
		t.Errorf("a pre-build phase behind a post-build anchor must be refused: %v", cfg.Order)
	}
	if !slices.Contains(cfg.Order, "first-note") {
		t.Errorf("the unstaged batch-mate stays routed: %v", cfg.Order)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "second-repro") {
		t.Errorf("warnings = %v, want exactly one naming the refused phase", warns)
	}
}

func TestApplyUserRouting_PreBuildPhasesBeforeBuildAreRouted(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Order: preBuildOrder()}
	specs := []PhaseSpec{
		stagedSpec("bug-reproduction", StagePreBuild, "fault-localization"),
		stagedSpec("fault-localization", StagePreBuild, "triage"),
	}
	if warns := ApplyUserRouting(&cfg, specs, Catalog{}); len(warns) != 0 {
		t.Fatalf("both pre-build phases precede build — no warnings expected, got %v", warns)
	}
	want := []string{"scout", "triage", "fault-localization", "bug-reproduction", "tdd", "build", "audit", "ship"}
	if !slices.Equal(cfg.Order, want) {
		t.Errorf("Order = %v, want %v", cfg.Order, want)
	}
}

func TestApplyUserRouting_OnlyAPreBuildStageIsFencedFromFollowingBuild(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		stage string
	}{
		{"no stage", ""},
		{"post-build", StagePostBuild},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cfg := config.RoutingConfig{Order: preBuildOrder()}
			warns := ApplyUserRouting(&cfg, []PhaseSpec{stagedSpec("after-check", c.stage, "build")}, Catalog{})
			if len(warns) != 0 {
				t.Fatalf("a %s phase after build is legal, got warnings %v", c.name, warns)
			}
			want := []string{"scout", "triage", "tdd", "build", "after-check", "audit", "ship"}
			if !slices.Equal(cfg.Order, want) {
				t.Errorf("Order = %v, want %v", cfg.Order, want)
			}
		})
	}
}

func TestApplyUserRouting_APreBuildPhaseIsLegalWhenTheOrderHasNoBuild(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Order: []string{"scout", "audit", "ship"}}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{stagedSpec("early-check", StagePreBuild, "scout")}, Catalog{})
	if len(warns) != 0 || !slices.Contains(cfg.Order, "early-check") {
		t.Errorf("without a build phase there is nothing to follow: warns=%v order=%v", warns, cfg.Order)
	}
}

func TestRepoPhaseCatalog_ThePreBuildEvaluatePhasesDeclareTheirStage(t *testing.T) {
	t.Parallel()
	root := repoRoot()
	dir := filepath.Join(root, ".evolve", "phases")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("phase catalog not present at %s: %v", dir, err)
	}
	tracked := TrackedPhaseDirs(t, root)
	specs, _ := DiscoverUserSpecs(dir)
	byName := map[string]PhaseSpec{}
	for _, s := range specs {
		byName[s.Name] = s
	}
	for _, name := range []string{"bug-reproduction", "fault-localization"} {
		if tracked != nil && !tracked[name] {
			t.Fatalf("%s is not a tracked phase directory", name)
		}
		s, ok := byName[name]
		if !ok {
			t.Fatalf("%s not discovered under %s", name, dir)
		}
		if s.Stage != StagePreBuild {
			t.Errorf("%s declares stage %q, want %q", name, s.Stage, StagePreBuild)
		}
	}
}

func TestRepoPhaseCatalog_EveryTrackedStageIsInTheClosedSet(t *testing.T) {
	t.Parallel()
	root := repoRoot()
	dir := filepath.Join(root, ".evolve", "phases")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("phase catalog not present at %s: %v", dir, err)
	}
	tracked := TrackedPhaseDirs(t, root)
	specs, _ := DiscoverUserSpecs(dir)
	declared := 0
	for _, s := range specs {
		if tracked != nil && !tracked[s.Name] {
			continue
		}
		if s.Stage == "" {
			continue
		}
		declared++
		if s.Stage != StagePreBuild && s.Stage != StagePostBuild {
			t.Errorf("%s declares stage %q outside the closed set", s.Name, s.Stage)
		}
	}
	if declared == 0 {
		t.Error("no tracked phase declares a stage — the field is unused")
	}
}

func TestDescriptorSchema_StageEnumIsTheClosedSet(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(repoRoot(), "docs", "architecture", "phase-descriptor.schema.json"))
	if err != nil {
		t.Skipf("descriptor schema not present: %v", err)
	}
	var doc struct {
		Properties map[string]struct {
			Type string   `json:"type"`
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	stage, ok := doc.Properties["stage"]
	if !ok {
		t.Fatal("the descriptor schema has no `stage` property, and additionalProperties:false would reject a phase.json that declares one")
	}
	got := slices.Clone(stage.Enum)
	slices.Sort(got)
	want := []string{StagePostBuild, StagePreBuild}
	if !slices.Equal(got, want) {
		t.Errorf("schema stage enum = %v, want %v", got, want)
	}
}

func TestLoad_ARegistrySpecWithAnUnpartitionedOutputIsAHardError(t *testing.T) {
	t.Parallel()
	path := writeRegistry(t, `{"phases":[{"name":"odd","outputs":{"files":["a.md","b.md"],"agent_owed":["sub/x.md"]}}]}`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "odd") {
		t.Fatalf("Load must refuse a bad outputs partition, naming the phase; err = %v", err)
	}
}

func TestValidateStage_AcceptsTheClosedSetAndRefusesTheRest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage string
		bad   bool
	}{{"", false}, {StagePreBuild, false}, {StagePostBuild, false}, {"mid-build", true}} {
		if got := len(ValidateStage(PhaseSpec{Stage: tc.stage})) > 0; got != tc.bad {
			t.Errorf("ValidateStage(%q) refused = %v, want %v", tc.stage, got, tc.bad)
		}
	}
}
