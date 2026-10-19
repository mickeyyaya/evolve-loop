//go:build acs

package cycle1977

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func headroomWithDeep() convergence.HeadroomTable {
	return convergence.HeadroomTable{
		"claude": {
			"balanced": {Model: "sonnet", Effort: "high"},
			"deep":     {Model: "opus", Effort: "high"},
			"top":      {Model: "opus-top", Effort: "xhigh"},
		},
	}
}

func headroomWithoutDeep() convergence.HeadroomTable {
	return convergence.HeadroomTable{"claude": {"balanced": {Model: "sonnet", Effort: "high"}}}
}

func capabilityTry(rung retryladder.Rung, headroom convergence.HeadroomTable, fingerprint string, earlier ...string) retryladder.Input {
	return retryladder.Input{
		Rung:     rung,
		Earlier:  earlier,
		Verdict:  retryladder.Verdict{Class: retryladder.Capability, Fingerprint: fingerprint},
		Base:     convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "sonnet", Effort: "medium"},
		Chain:    []string{"claude"},
		Headroom: headroom,
	}
}

func TestC1977_001_RungTableFollowsTheSpecFlow(t *testing.T) {
	agy := capabilityTry(retryladder.R0, headroomWithDeep(), "codes:a")
	agy.Base = convergence.TierEffort{Family: "agy", Tier: "balanced", Model: "flash-high"}
	agy.Chain = []string{"agy", "claude"}
	agy.Headroom = convergence.HeadroomTable{"agy": {"balanced": {Model: "flash-high"}, "deep": {Model: "argon"}}, "claude": {"deep": {Model: "opus"}}}

	tests := []struct {
		name       string
		in         retryladder.Input
		wantAction retryladder.Action
		wantRung   retryladder.Rung
		wantClass  retryladder.Class
	}{
		{"R0 to R1 on a new fingerprint with an effort step", capabilityTry(retryladder.R0, headroomWithDeep(), "codes:a"), retryladder.ActionRaise, retryladder.R1, retryladder.Capability},
		{"R0 to R2 on a repeated fingerprint", capabilityTry(retryladder.R0, headroomWithDeep(), "codes:a", "codes:a"), retryladder.ActionRaise, retryladder.R2, retryladder.Capability},
		{"R0 to R2 when the family has no effort step", agy, retryladder.ActionRaise, retryladder.R2, retryladder.Capability},
		{"R1 to R2", capabilityTry(retryladder.R1, headroomWithDeep(), "codes:b", "codes:a"), retryladder.ActionRaise, retryladder.R2, retryladder.Capability},
		{"R2 stops as spec on an earlier fingerprint", capabilityTry(retryladder.R2, headroomWithDeep(), "codes:a", "codes:a", "codes:b"), retryladder.ActionStop, retryladder.R2, retryladder.Spec},
		{"R2 stops as capability on a new fingerprint", capabilityTry(retryladder.R2, headroomWithDeep(), "codes:c", "codes:a", "codes:b"), retryladder.ActionStop, retryladder.R2, retryladder.Capability},
		{"no headroom at R0 goes to R1", capabilityTry(retryladder.R0, headroomWithoutDeep(), "codes:a", "codes:a"), retryladder.ActionRaise, retryladder.R1, retryladder.Capability},
		{"no headroom at R1 stops", capabilityTry(retryladder.R1, headroomWithoutDeep(), "codes:a"), retryladder.ActionStop, retryladder.R1, retryladder.Capability},
	}
	for _, tt := range tests {
		got := retryladder.Next(tt.in)
		if got.Action != tt.wantAction || got.Rung != tt.wantRung || got.Class != tt.wantClass {
			t.Errorf("%s: Next = {%s %s %s}, want {%s %s %s}", tt.name, got.Action, got.Rung, got.Class, tt.wantAction, tt.wantRung, tt.wantClass)
		}
	}
}

func TestC1977_002_NoInputBeatsTwoRaisedTriesOrTheBounds(t *testing.T) {
	classes := []retryladder.Class{retryladder.Infra, retryladder.Quota, retryladder.Halt, retryladder.Spec, retryladder.Capability, retryladder.Unknown}
	boundsSet := []retryladder.Bounds{{}, {MaxTier: "balanced"}, {MaxTier: "top", MaxEffort: "xhigh"}, {MaxEffort: "medium"}}
	headrooms := []convergence.HeadroomTable{headroomWithDeep(), headroomWithoutDeep(), {}}
	efforts := append([]string{""}, policy.EffortLevels()...)

	for _, bounds := range boundsSet {
		resolved := bounds.Resolved()
		tierCap, effortCap := slices.Index(modelcatalog.CanonicalTiers, resolved.MaxTier), slices.Index(policy.EffortLevels(), resolved.MaxEffort)
		for _, headroom := range headrooms {
			for _, baseEffort := range efforts {
				if slices.Index(policy.EffortLevels(), baseEffort) > effortCap {
					continue
				}
				for _, first := range classes {
					for _, second := range classes {
						rung, raised, earlier := retryladder.R0, 0, []string(nil)
						for step, class := range []retryladder.Class{first, second, retryladder.Capability, retryladder.Capability, retryladder.Unknown} {
							fp := []string{"codes:a", "codes:b", "codes:a", "codes:c", "codes:c"}[step]
							in := capabilityTry(rung, headroom, fp, earlier...)
							in.Verdict.Class = class
							in.Bounds = bounds
							in.Base.Effort = baseEffort
							got := retryladder.Next(in)
							if got.Action == retryladder.ActionStop {
								break
							}
							earlier = append(earlier, fp)
							if got.Action == retryladder.ActionRetry {
								continue
							}
							raised++
							rung = got.Rung
							if raised > 2 {
								t.Fatalf("bounds %+v classes %s,%s: %d raised tries", bounds, first, second, raised)
							}
							if slices.Index(modelcatalog.CanonicalTiers, got.Target.Tier) > tierCap {
								t.Fatalf("bounds %+v: tier %q above max_tier %q", bounds, got.Target.Tier, resolved.MaxTier)
							}
							if got.Target.Effort != "" && slices.Index(policy.EffortLevels(), got.Target.Effort) > effortCap {
								t.Fatalf("bounds %+v: effort %q above max_effort %q", bounds, got.Target.Effort, resolved.MaxEffort)
							}
						}
					}
				}
			}
		}
	}
}

func TestC1977_003_HaltAndSpecNeverClimbAndUnknownNeverPassesR1(t *testing.T) {
	for _, rung := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
		for _, class := range []retryladder.Class{retryladder.Halt, retryladder.Spec} {
			in := capabilityTry(rung, headroomWithDeep(), "codes:a", "codes:a")
			in.Verdict.Class = class
			if got := retryladder.Next(in); got.Action != retryladder.ActionStop {
				t.Errorf("%s at %s: Action = %s, want stop", class, rung, got.Action)
			}
		}
		in := capabilityTry(rung, headroomWithDeep(), "codes:a", "codes:a")
		in.Verdict.Class = retryladder.Unknown
		got := retryladder.Next(in)
		if got.Action == retryladder.ActionRaise && got.Rung != retryladder.R1 {
			t.Errorf("unknown at %s raised to %s, only R1 is allowed", rung, got.Rung)
		}
		if rung != retryladder.R0 && got.Action != retryladder.ActionStop {
			t.Errorf("unknown at %s: Action = %s, want stop", rung, got.Action)
		}
	}
}

func TestC1977_004_NoHeadroomRecordsTheSignalAndStopsAtR1(t *testing.T) {
	r0 := retryladder.Next(capabilityTry(retryladder.R0, headroomWithoutDeep(), "codes:a", "codes:a"))
	if r0.Rung != retryladder.R1 || !slices.Contains(r0.Signals, "LADDER_NO_HEADROOM") {
		t.Errorf("R0 without headroom = rung %s signals %v, want R1 and LADDER_NO_HEADROOM", r0.Rung, r0.Signals)
	}
	r1 := retryladder.Next(capabilityTry(retryladder.R1, headroomWithoutDeep(), "codes:a"))
	if r1.Action != retryladder.ActionStop || !slices.Contains(r1.Signals, "LADDER_NO_HEADROOM") {
		t.Errorf("R1 without headroom = %s signals %v, want a stop with LADDER_NO_HEADROOM", r1.Action, r1.Signals)
	}
	withHeadroom := retryladder.Next(capabilityTry(retryladder.R1, headroomWithDeep(), "codes:a"))
	if slices.Contains(withHeadroom.Signals, "LADDER_NO_HEADROOM") {
		t.Errorf("a climb with headroom recorded LADDER_NO_HEADROOM: %v", withHeadroom.Signals)
	}
}

func TestC1977_005_EveryDecisionRecordsTheShadowCriticRung(t *testing.T) {
	for _, class := range []retryladder.Class{retryladder.Infra, retryladder.Quota, retryladder.Halt, retryladder.Spec, retryladder.Capability, retryladder.Unknown} {
		for _, rung := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
			in := capabilityTry(rung, headroomWithDeep(), "codes:a")
			in.Verdict.Class = class
			plain := retryladder.Next(in)
			in.Critic = retryladder.R2
			got := retryladder.Next(in)
			if got.CriticRung != retryladder.R2 {
				t.Errorf("%s at %s: CriticRung = %q, want R2", class, rung, got.CriticRung)
			}
			if got.Action != plain.Action || got.Rung != plain.Rung {
				t.Errorf("%s at %s: the shadow critic arm changed the decision from {%s %s} to {%s %s}", class, rung, plain.Action, plain.Rung, got.Action, got.Rung)
			}
		}
	}
}

func TestC1977_006_UnsetBoundsAreDeepAndHigh(t *testing.T) {
	got := retryladder.Bounds{}.Resolved()
	if got.MaxTier != "deep" || got.MaxEffort != "high" {
		t.Errorf("Bounds{}.Resolved() = %+v, want max_tier deep and max_effort high", got)
	}
	deepBase := capabilityTry(retryladder.R0, headroomWithDeep(), "codes:a", "codes:a")
	deepBase.Base = convergence.TierEffort{Family: "claude", Tier: "deep", Model: "opus", Effort: "high"}
	if climbed := retryladder.Next(deepBase); climbed.Rung == retryladder.R2 {
		t.Errorf("a deep base with unset bounds climbed to %+v, max_tier deep must stop it", climbed.Target)
	}
	highEffort := capabilityTry(retryladder.R0, headroomWithDeep(), "codes:a")
	highEffort.Base.Effort = "high"
	if stepped := retryladder.Next(highEffort); stepped.Rung == retryladder.R1 && stepped.Target.Effort != "high" {
		t.Errorf("a high effort base with unset bounds stepped to %q, max_effort high must stop it", stepped.Target.Effort)
	}
}

func TestC1977_007_InfraAndQuotaRetryOnTheSameRung(t *testing.T) {
	for _, class := range []retryladder.Class{retryladder.Infra, retryladder.Quota} {
		in := capabilityTry(retryladder.R1, headroomWithDeep(), "codes:a")
		in.Verdict.Class = class
		got := retryladder.Next(in)
		if got.Action != retryladder.ActionRetry || got.Rung != retryladder.R1 {
			t.Errorf("%s at R1: Next = {%s %s}, want a retry at R1", class, got.Action, got.Rung)
		}
	}
}

func TestC1977_008_PackageStillDoesNotImportCore(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, stderr, code, err := acsassert.SubprocessOutput("go", "list", "-C", filepath.Join(root, "go"), "-deps", "-test", "./internal/retryladder")
	if err != nil || code != 0 {
		t.Fatalf("go list exit=%d err=%v: %s", code, err, stderr)
	}
	for _, dep := range strings.Fields(out) {
		if strings.HasSuffix(dep, "/go/internal/core") || strings.Contains(dep, "/go/internal/core/") {
			t.Errorf("retryladder depends on %s", dep)
		}
	}
	if !strings.Contains(out, "/go/internal/convergence") {
		t.Errorf("retryladder does not reuse internal/convergence: the decider must not carry a second tier table")
	}
}

// acs-predicate: config-check
func TestC1977_009_EveryExportedSymbolOfTheDeciderIsNamedInTheApicoverTest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, "go", "internal", "retryladder")
	fset := token.NewFileSet()
	decide, err := parser.ParseFile(fset, filepath.Join(dir, "decide.go"), nil, 0)
	if err != nil {
		t.Fatalf("decide.go does not parse: %v", err)
	}
	named, err := parser.ParseFile(fset, filepath.Join(dir, "apicover_named_test.go"), nil, 0)
	if err != nil {
		t.Fatalf("apicover_named_test.go does not parse: %v", err)
	}
	referenced := map[string]bool{}
	ast.Inspect(named, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			referenced[sel.Sel.Name] = true
		}
		return true
	})
	var exported []string
	for _, decl := range decide.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				exported = append(exported, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						exported = append(exported, s.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							exported = append(exported, name.Name)
						}
					}
				}
			}
		}
	}
	if len(exported) == 0 {
		t.Fatalf("decide.go declares no exported symbol")
	}
	for _, name := range exported {
		if !referenced[name] {
			t.Errorf("apicover_named_test.go does not name retryladder.%s", name)
		}
	}
}
