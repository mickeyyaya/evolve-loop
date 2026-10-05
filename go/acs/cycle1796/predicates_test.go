//go:build acs

package cycle1796

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const scopedeltaPkg = "./internal/scopedelta"

var gateConfiguration = []string{"go/.apicover-enforce", "go/go.mod", "go/go.sum"}

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func runScopedeltaTests(t *testing.T, run string) string {
	t.Helper()
	args := []string{"test", "-C", goDir(t), "-count=1", "-v"}
	if run != "" {
		args = append(args, "-run", run)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", append(args, scopedeltaPkg)...)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("go test %s (run=%q) exited %d\n%s", scopedeltaPkg, run, code, out)
	}
	return out
}

func namesPath(errs []error, p string) bool {
	return slices.ContainsFunc(errs, func(err error) bool { return strings.Contains(err.Error(), p) })
}

func TestC1796_001_ProtectedPathLabelledDiscoveredKeepIsRefused(t *testing.T) {
	const work = "go/internal/salvage/extract.go"
	const protected = "go/internal/phases/ship/gitops.go"
	scope := scopedelta.Scope{Cycle: 1796, Declared: []string{work}, Protected: []string{"go/internal/phases/ship"}}
	changed := []string{work, protected}
	rules := scopedelta.DefaultClosureRules()
	corroborated := scopedelta.Corroboration{FailsWithout: true, Command: "go test -count=1 ./internal/phases/ship/ -run TestStage"}

	discoveredKeep := scopedelta.Entry{
		Path: protected, Class: scopedelta.ClassDiscovered, Disposition: scopedelta.DispositionKeep,
		Effect: scopedelta.EffectTightens, Corroboration: corroborated,
		Reason: "the staging bug is real and the fix is one line",
	}
	res := scopedelta.Account(changed, scope, rules, []scopedelta.Entry{discoveredKeep})
	if res.OK() {
		t.Errorf("RED: protected %s labelled discovered with keep, a declared tightens and corroboration accounted clean — Account trusted the producer's label instead of re-deriving the boundary", protected)
	} else if !namesPath(res.Invalid, protected) {
		t.Errorf("RED: protected %s refused without an Invalid error naming it: unaccounted=%v invalid=%v", protected, res.Unaccounted, res.Invalid)
	}

	for _, label := range []scopedelta.Class{scopedelta.ClassOpportunistic, scopedelta.ClassMisunderstood, scopedelta.ClassCrossLane, ""} {
		for _, d := range []scopedelta.Disposition{scopedelta.DispositionKeep, scopedelta.DispositionCarve} {
			relabelled := discoveredKeep
			relabelled.Class, relabelled.Disposition, relabelled.PatchRef = label, d, ".evolve/carved/cycle-1796/gitops.patch"
			if r := scopedelta.Account(changed, scope, rules, []scopedelta.Entry{relabelled}); r.OK() {
				t.Errorf("RED: protected %s labelled %q with %s accounted clean — a boundary may only be refused", protected, label, d)
			}
		}
	}

	refused := scopedelta.Entry{Path: protected, Class: scopedelta.ClassBoundary, Disposition: scopedelta.DispositionRefuse,
		Reason: "operator-owned ship gate; the finding is preserved for console review"}
	if r := scopedelta.Account(changed, scope, rules, []scopedelta.Entry{refused}); !r.OK() {
		t.Errorf("refusing the protected path must account (anti-overreach): unaccounted=%v invalid=%v", r.Unaccounted, r.Invalid)
	}

	const unprotected = "go/internal/router/pick.go"
	adjacentFix := scopedelta.Entry{Path: unprotected, Class: scopedelta.ClassDiscovered, Disposition: scopedelta.DispositionKeep,
		Corroboration: corroborated, Reason: "genuine nil-deref on the empty-candidate path"}
	if r := scopedelta.Account([]string{work, unprotected}, scope, rules, []scopedelta.Entry{adjacentFix}); !r.OK() {
		t.Errorf("a corroborated discovered keep off the protected surface must still account (anti-overreach): unaccounted=%v invalid=%v", r.Unaccounted, r.Invalid)
	}
}

func TestC1796_002_ApicoverEnforceLineRemovalNeedsCorroboration(t *testing.T) {
	const work = "go/internal/scopedelta/scopedelta.go"
	scope := scopedelta.Scope{Cycle: 1796, Declared: []string{work}}
	rules := scopedelta.DefaultClosureRules()
	for _, p := range gateConfiguration {
		changed := []string{work, p}

		if got := scopedelta.Classify(p, scopedelta.Declaration{}, scope, rules); got.Class == scopedelta.ClassClosure {
			t.Errorf("RED: %s classified closure in a Go cycle — closure is exempt from evidence, so the edit ships uncorroborated", p)
		}

		res := scopedelta.Account(changed, scope, rules, nil)
		if res.OK() || !slices.Contains(res.Unaccounted, p) {
			t.Errorf("RED: an unadjudicated %s edit accounted clean (unaccounted=%v)", p, res.Unaccounted)
		}

		removesAnotherPackagesLine := scopedelta.Entry{
			Path: p, Class: scopedelta.ClassClosure, Disposition: scopedelta.DispositionKeep, Effect: scopedelta.EffectLoosens,
			Reason: "drops the enrollment line of go/internal/router, which my change no longer needs gated",
		}
		if r := scopedelta.Account(changed, scope, rules, []scopedelta.Entry{removesAnotherPackagesLine}); r.OK() {
			t.Errorf("RED: a %s edit removing another package's line was kept as closure with no corroboration", p)
		}

		corroboratedEnrollment := scopedelta.Entry{
			Path: p, Class: scopedelta.ClassDiscovered, Disposition: scopedelta.DispositionKeep, Effect: scopedelta.EffectTightens,
			Reason:        "enrolls the package whose coverage gate this change completes",
			Corroboration: scopedelta.Corroboration{FailsWithout: true, Command: "go test -count=1 ./internal/apicover/"},
		}
		if r := scopedelta.Account(changed, scope, rules, []scopedelta.Entry{corroboratedEnrollment}); !r.OK() {
			t.Errorf("a corroborated tightening of %s must account (anti-overreach): unaccounted=%v invalid=%v", p, r.Unaccounted, r.Invalid)
		}
		uncorroborated := corroboratedEnrollment
		uncorroborated.Corroboration = scopedelta.Corroboration{}
		if r := scopedelta.Account(changed, scope, rules, []scopedelta.Entry{uncorroborated}); r.OK() {
			t.Errorf("a %s keep with no corroboration accounted clean", p)
		}
	}
}

func looseningKeeps(n int) []scopedelta.Entry {
	var out []scopedelta.Entry
	for i := 0; i < n; i++ {
		out = append(out, scopedelta.Entry{
			Path: fmt.Sprintf("go/internal/router/pick%d_test.go", i), Class: scopedelta.ClassDiscovered,
			Disposition: scopedelta.DispositionKeep, Effect: scopedelta.EffectLoosens,
			Reason: "the assertion was too strict for my change",
		})
	}
	return out
}

func skippedPadding(n int) []scopedelta.Entry {
	var out []scopedelta.Entry
	for i := 0; i < n; i++ {
		out = append(out,
			scopedelta.Entry{Path: fmt.Sprintf("go/internal/salvage/extract%d.go", i), Class: scopedelta.ClassInScope,
				Disposition: scopedelta.DispositionKeep, Reason: "declared in this cycle's scope"},
			scopedelta.Entry{Path: fmt.Sprintf("go/internal/salvage/extract%d_test.go", i), Class: scopedelta.ClassClosure,
				Disposition: scopedelta.DispositionKeep, Reason: "necessary closure (same-package-test)"})
	}
	return out
}

func TestC1796_003_GamingSignalsMajoritiesIgnoreSkippedEntries(t *testing.T) {
	gamed := looseningKeeps(3)
	unpadded := scopedelta.GamingSignals(gamed)
	if len(unpadded) != 3 {
		t.Fatalf("baseline: 3 uncorroborated loosening keeps on the apparatus must raise all three signals, got %v", unpadded)
	}
	for _, pad := range []int{1, 2, 5} {
		padded := slices.Concat(skippedPadding(pad), gamed)
		got := scopedelta.GamingSignals(padded)
		joined := strings.Join(got, " | ")
		if len(got) != len(unpadded) {
			t.Errorf("RED: %d skipped in-scope/closure entries diluted the majority: %d signals unpadded, %d padded (%q)", 2*pad, len(unpadded), len(got), joined)
		}
		for _, want := range []string{"signal", "loosen", "narrative"} {
			if !strings.Contains(joined, want) {
				t.Errorf("RED: no %q signal behind %d skipped entries; got %q", want, 2*pad, joined)
			}
		}
		if strings.Contains(joined, fmt.Sprintf("of %d", len(padded))) {
			t.Errorf("RED: a signal's denominator counted skipped entries: %q", joined)
		}
	}

	lone := slices.Concat(skippedPadding(3), looseningKeeps(1))
	if s := scopedelta.GamingSignals(lone); len(s) != 0 {
		t.Errorf("one counted entry behind skipped padding is below the floor and must raise nothing (floor over counted entries), got %v", s)
	}
	if s := scopedelta.GamingSignals(skippedPadding(3)); len(s) != 0 {
		t.Errorf("a delta of only in-scope and closure entries must raise nothing, got %v", s)
	}
}

func TestC1796_004_GateConfigurationWhyIsATestNotAComment(t *testing.T) {
	const name = "TestSurfaceOf_GateConfigurationIsSignal"
	out := runScopedeltaTests(t, "^"+name+"$")
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Errorf("RED: no PASS line for %s in %s (missing, renamed or skipped)\n%s", name, scopedeltaPkg, out)
	}
	for _, p := range gateConfiguration {
		if got := scopedelta.SurfaceOf(p); got != scopedelta.SurfaceSignal {
			t.Errorf("SurfaceOf(%q) = %q, want signal", p, got)
		}
	}
	gaming := filepath.Join(goDir(t), "internal", "scopedelta", "gaming.go")
	src, err := os.ReadFile(gaming)
	if err != nil {
		t.Fatalf("read %s: %v", gaming, err)
	}
	if stats := commentaudit.Stats(src); stats.Comment != 0 {
		t.Errorf("RED: gaming.go carries %d comment line(s) per commentaudit.Stats — the why above signalFileHints must live in %s, not a comment", stats.Comment, name)
	}
}

func TestC1796_005_ScopedeltaPackageSuiteIsGreen(t *testing.T) {
	out := runScopedeltaTests(t, "")
	for _, name := range []string{
		"TestAccount_ProtectedPathIsBoundaryWhateverTheLabel",
		"TestAccount_GateConfigurationEditNeedsCorroboration",
		"TestDefaultClosureRules_GateConfigurationIsNeverClosure",
		"TestGamingSignals_MajoritiesIgnoreSkippedEntries",
		"TestGamingSignals_PaddingCannotLiftALoneEntryOverTheFloor",
		"TestSurfaceOf_GateConfigurationIsSignal",
	} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: no PASS line for %s in %s", name, scopedeltaPkg)
		}
	}
}
