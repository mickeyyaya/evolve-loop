//go:build acs

package cycle2091

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const documentSkill = "docs-write"

func codeSignals() map[string]string {
	return map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
}

func documentOnlyRule(paths ...string) policy.Policy {
	return policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{Paths: paths, Skills: []string{documentSkill}},
	}}}
}

func resolveFor(pol policy.Policy, scope []string) []string {
	return pol.ResolveOverlays(policy.OverlayDispatch{
		Phase: "build", Tier: "balanced", WritesSource: true,
		Signals: codeSignals(), ScopePaths: scope,
	})
}

func TestC2091_001_PathsRuleLoadsOnlyForAMatchingScope(t *testing.T) {
	pol := documentOnlyRule("docs/**", "**/*.md")
	cases := []struct {
		name  string
		scope []string
		want  []string
	}{
		{"code-only scope", []string{"go/internal/policy/overlays.go", "go/internal/policy/overlays_test.go"}, nil},
		{"scope with a docs file", []string{"go/internal/policy/overlays.go", "docs/architecture/guidance-layers.md"}, []string{documentSkill}},
		{"root markdown file", []string{"README.md"}, []string{documentSkill}},
		{"deep docs path", []string{"docs/a/b/c/d.txt"}, []string{documentSkill}},
		{"no scope paths at all", nil, nil},
		{"empty scope paths", []string{}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveFor(pol, tc.scope)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveOverlays(scope=%v) = %v, want %v", tc.scope, got, tc.want)
			}
		})
	}
}

func TestC2091_002_EmptyPathsSelectorMatchesEveryScope(t *testing.T) {
	pol := documentOnlyRule()
	for _, scope := range [][]string{nil, {}, {"go/a.go"}, {"docs/x.md"}} {
		got := resolveFor(pol, scope)
		if !reflect.DeepEqual(got, []string{documentSkill}) {
			t.Errorf("rule with no paths, scope=%v: got %v, want [%s]", scope, got, documentSkill)
		}
	}
}

func TestC2091_003_GlobSemanticsAreSegmentAware(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"double star matches nested", "docs/**", "docs/a/b/c.md", true},
		{"double star matches direct child", "docs/**", "docs/a.md", true},
		{"double star anchored at the start", "docs/**", "go/docs/a.md", false},
		{"double star needs a whole segment", "docs/**", "docsx/a.md", false},
		{"leading double star reaches the root", "**/*.md", "README.md", true},
		{"leading double star reaches depth", "**/*.md", "docs/a/b.md", true},
		{"leading double star rejects other suffix", "**/*.md", "go/a.go", false},
		{"single star stays inside a segment", "*.md", "docs/a.md", false},
		{"single star matches at the root", "*.md", "a.md", true},
		{"middle single star is one segment", "go/*/x.go", "go/internal/x.go", true},
		{"middle single star is not two segments", "go/*/x.go", "go/internal/policy/x.go", false},
		{"question mark is one character", "docs/?.md", "docs/ab.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveFor(documentOnlyRule(tc.pattern), []string{tc.path})
			matched := reflect.DeepEqual(got, []string{documentSkill})
			if matched != tc.want {
				t.Errorf("pattern %q against %q: matched=%v, want %v", tc.pattern, tc.path, matched, tc.want)
			}
		})
	}
}

func TestC2091_004_PathsSelectorIntersectsWithTheOtherSelectors(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{Phases: []string{"build"}, Paths: []string{"docs/**"}, WritesSource: true, Skills: []string{documentSkill}},
	}}}
	docScope := []string{"docs/x.md"}
	cases := []struct {
		name string
		d    policy.OverlayDispatch
		want []string
	}{
		{"phase and paths match", policy.OverlayDispatch{Phase: "build", WritesSource: true, ScopePaths: docScope}, []string{documentSkill}},
		{"paths match but the phase differs", policy.OverlayDispatch{Phase: "audit", WritesSource: true, ScopePaths: docScope}, nil},
		{"paths match but the dispatch writes no source", policy.OverlayDispatch{Phase: "build", ScopePaths: docScope}, nil},
		{"phase matches but the paths do not", policy.OverlayDispatch{Phase: "build", WritesSource: true, ScopePaths: []string{"go/a.go"}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pol.ResolveOverlays(tc.d)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveOverlays(%+v) = %v, want %v", tc.d, got, tc.want)
			}
		})
	}
}

func TestC2091_005_PolicyLoadReadsPathsAndRefusesAMalformedGlob(t *testing.T) {
	good := []byte(`{"overlays":{"rules":[{"paths":["docs/**"],"skills":["docs-write"]}]}}`)
	pol, err := policy.Parse(good)
	if err != nil {
		t.Fatalf("Parse refused a valid paths selector: %v", err)
	}
	if got := resolveFor(pol, []string{"go/a.go"}); len(got) != 0 {
		t.Errorf("code-only scope loaded %v from a parsed docs/** rule", got)
	}
	if got := resolveFor(pol, []string{"docs/a.md"}); !reflect.DeepEqual(got, []string{documentSkill}) {
		t.Errorf("docs scope loaded %v from a parsed docs/** rule, want [%s]", got, documentSkill)
	}

	for _, bad := range []string{"[", "docs/[a-", "a/**/[", `docs\`} {
		raw := []byte(`{"overlays":{"rules":[{"paths":["` + strings.ReplaceAll(bad, `\`, `\\`) + `"],"skills":["docs-write"]}]}}`)
		_, err := policy.Parse(raw)
		if err == nil {
			t.Errorf("Parse accepted the malformed glob %q; it must fail at load, never silently never match", bad)
			continue
		}
		if !strings.Contains(err.Error(), "paths") {
			t.Errorf("Parse error for %q does not name the paths selector: %v", bad, err)
		}
	}
}

func TestC2091_006_DefaultCodeRuleIsUnchangedUntilTheWiringLands(t *testing.T) {
	var pol policy.Policy
	want := []string{"engineering-craft", "code-review-simplify", documentSkill}
	for _, scope := range [][]string{nil, {"go/internal/policy/overlays.go"}, {"docs/a.md"}} {
		if got := resolveFor(pol, scope); !reflect.DeepEqual(got, want) {
			t.Errorf("default overlays for scope %v = %v, want %v", scope, got, want)
		}
	}
}

func TestC2091_007_ChangedPathFunctionsAreFullyCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	moduleDir, err := acsassert.ModuleDir(filepath.Join(root, "go"))
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	profile := filepath.Join(t.TempDir(), "policy.cov")

	testOut, testErr, code, runErr := acsassert.SubprocessOutput(
		"go", "-C", moduleDir, "test", "-count=1", "-coverprofile="+profile, "./internal/policy")
	if runErr != nil || code != 0 {
		t.Fatalf("go test ./internal/policy exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(testOut+testErr, 40))
	}
	coverOut, coverErr, code, runErr := acsassert.SubprocessOutput(
		"go", "-C", moduleDir, "tool", "cover", "-func="+profile)
	if runErr != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(coverOut+coverErr, 40))
	}
	out := coverOut

	for _, fn := range []string{"matches", "matchPaths"} {
		pct, err := acsassert.FuncCoverage(out, fn)
		if err != nil {
			t.Errorf("coverage of %s: %v", fn, err)
			continue
		}
		if pct < 100.0 {
			t.Errorf("%s coverage = %.1f%%, want 100.0%%", fn, pct)
		}
	}

	seenPathFunc := false
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "overlays.go:") || !strings.Contains(strings.ToLower(fields[1]), "path") {
			continue
		}
		seenPathFunc = true
		if fields[len(fields)-1] != "100.0%" {
			t.Errorf("overlays.go function %s coverage = %s, want 100.0%%", fields[1], fields[len(fields)-1])
		}
	}
	if !seenPathFunc {
		t.Errorf("no function with path in its name found in overlays.go coverage output")
	}
}

func TestC2091_008_PackageNoteAndSpecDescribeThePathsSelector(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	for _, rel := range []string{
		"docs/architecture/packages/internal-policy.md",
		"docs/architecture/guidance-layers.md",
	} {
		path := filepath.Join(root, rel)
		if !acsassert.FileContains(t, path, "ScopePaths") {
			t.Errorf("%s does not describe the ScopePaths field of the dispatch", rel)
		}
	}
	if !acsassert.FileContains(t, filepath.Join(root, "docs/architecture/packages/internal-policy.md"), "OverlayRule.Paths") {
		t.Errorf("the package note does not describe OverlayRule.Paths")
	}
}
