//go:build acs

package cycle1747

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/routingtest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const artifactBytesField = "ArtifactBytes"

const artifactBytesKey = "run_dir.artifact_bytes"

const junkBytes = 256 << 10

func specWithArtifactBytes(t *testing.T, base routingtest.SignalSpec, n int) routingtest.SignalSpec {
	t.Helper()
	f := reflect.ValueOf(&base).Elem().FieldByName(artifactBytesField)
	if !f.IsValid() || !f.CanInt() {
		t.Fatalf("routingtest.SignalSpec has no integer field %s", artifactBytesField)
	}
	f.SetInt(int64(n))
	return base
}

func scoutArtifactBytes(t *testing.T, s router.ScoutSignals) int64 {
	t.Helper()
	f := reflect.ValueOf(s).FieldByName(artifactBytesField)
	if !f.IsValid() || !f.CanInt() {
		t.Fatalf("router.ScoutSignals has no integer field %s", artifactBytesField)
	}
	return f.Int()
}

func newWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return ws
}

func withRunDirJunk(files map[string]string) map[string]string {
	out := map[string]string{"scout-stdout.log": strings.Repeat("x", junkBytes)}
	for k, v := range files {
		out[k] = v
	}
	return out
}

type renderCase struct {
	name string
	base routingtest.SignalSpec
	n    int
}

var renderCases = []renderCase{
	{name: "artifact-bytes-only", base: routingtest.SignalSpec{}, n: 4096},
	{name: "with-scout-neighbours", base: routingtest.SignalSpec{CycleSize: "medium", ScoutBacklog: 9, ScoutCarryover: 2}, n: 1 << 20},
}

func assertRenderingsAgree(t *testing.T, render func(routingtest.SignalSpec) map[string]string) {
	t.Helper()
	for _, tc := range renderCases {
		t.Run(tc.name, func(t *testing.T) {
			spec := specWithArtifactBytes(t, tc.base, tc.n)
			want := spec.Signals()
			if !want.Scout.Present {
				t.Fatalf("Signals().Scout.Present = false for an ArtifactBytes=%d fixture; the field must participate in scout presence", tc.n)
			}
			if got := scoutArtifactBytes(t, want.Scout); got != int64(tc.n) {
				t.Fatalf("Signals().Scout.ArtifactBytes = %d, want %d from the spec", got, tc.n)
			}
			ws := newWorkspace(t, withRunDirJunk(render(spec)))
			got, err := router.Digest(ws, []string{"scout"})
			if err != nil {
				t.Fatalf("Digest: %v", err)
			}
			if b := scoutArtifactBytes(t, got.Scout); b != int64(tc.n) {
				t.Errorf("Digest().Scout.ArtifactBytes = %d, want %d from the rendered handoff", b, tc.n)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Digest != Signals\n got=%+v\nwant=%+v", got, want)
			}
		})
	}
}

func TestC1747_001_ArtifactBytesFixtureRendersIdenticallyThroughFlatHandoff(t *testing.T) {
	assertRenderingsAgree(t, routingtest.SignalSpec.HandoffFiles)
}

func TestC1747_002_ArtifactBytesFixtureRendersIdenticallyThroughWrappedEnvelope(t *testing.T) {
	assertRenderingsAgree(t, routingtest.SignalSpec.WrappedHandoffFiles)
}

func TestC1747_003_DigestReadsArtifactBytesFromScoutHandoffNotRunDirSize(t *testing.T) {
	ws := newWorkspace(t, withRunDirJunk(map[string]string{
		"handoff-scout.json": `{"cycle_size_estimate":"small","backlog_size":3,"` + artifactBytesKey + `":4096}`,
	}))
	sig, err := router.Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got := scoutArtifactBytes(t, sig.Scout); got != 4096 {
		t.Errorf("Scout.ArtifactBytes = %d, want 4096 read from handoff-scout.json key %q", got, artifactBytesKey)
	}
	if sig.Scout.BacklogSize != 3 || sig.Scout.CycleSizeEstimate != "small" {
		t.Errorf("neighbouring scout fields lost: %+v", sig.Scout)
	}
	if v, ok := sig.GenericValue(artifactBytesKey); ok {
		t.Errorf("Generic[%q] = %v; the signal must stay typed on ScoutSignals, never injected into Generic", artifactBytesKey, v)
	}
}

func TestC1747_004_AbsentOrCorruptArtifactBytesFailsOpen(t *testing.T) {
	cases := []struct {
		name    string
		handoff string
	}{
		{name: "absent", handoff: `{"backlog_size":3}`},
		{name: "non-numeric", handoff: `{"backlog_size":3,"` + artifactBytesKey + `":"lots"}`},
		{name: "null", handoff: `{"backlog_size":3,"` + artifactBytesKey + `":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := newWorkspace(t, withRunDirJunk(map[string]string{"handoff-scout.json": tc.handoff}))
			sig, err := router.Digest(ws, []string{"scout"})
			if err != nil {
				t.Fatalf("Digest: %v", err)
			}
			if got := scoutArtifactBytes(t, sig.Scout); got != 0 {
				t.Errorf("Scout.ArtifactBytes = %d, want 0 (fail-open, never the run dir's %d+ bytes)", got, junkBytes)
			}
			if !sig.Scout.Present || sig.Scout.BacklogSize != 3 {
				t.Errorf("a bad %s value must not cost the rest of the scout digest: %+v", artifactBytesKey, sig.Scout)
			}
			if v, ok := sig.GenericValue(artifactBytesKey); ok {
				t.Errorf("Generic[%q] = %v injected with no handoff value", artifactBytesKey, v)
			}
		})
	}
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func goTestPackage(t *testing.T, pkg string, extra ...string) string {
	t.Helper()
	var out bytes.Buffer
	args := append([]string{"test", "-count=1", "-v"}, extra...)
	cmd := exec.Command("go", append(args, pkg)...)
	cmd.Dir = goModuleDir(t)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test %s: %v\n%s", pkg, err, out.String())
	}
	return out.String()
}

func TestC1747_005_RoutingtestSuiteRunsTheArtifactBytesKeystone(t *testing.T) {
	out := goTestPackage(t, "./internal/routingtest")
	for _, name := range []string{
		"TestSignalSpec_DualRenderingAgree",
		"TestSignalSpec_WrappedDualRenderingAgree",
		"TestSignalSpec_ArtifactBytesReachesBothRenderings",
	} {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass in ./internal/routingtest", name)
		}
	}
	keystone := filepath.Join(goModuleDir(t), "internal", "routingtest", "consistency_test.go")
	acsassert.FileContains(t, keystone, "ArtifactBytes: 4096}")
}

func TestC1747_006_RouterSuitePasses(t *testing.T) {
	goTestPackage(t, "./internal/router")
}

func TestC1747_007_RouterChangeSelectsRoutingtestForRegression(t *testing.T) {
	const name = "TestChangedScope_WidensByReverseDependency"
	out := goTestPackage(t, "./internal/regressiontia", "-run", "^"+name+"$")
	if !strings.Contains(out, "--- PASS: "+name+" ") {
		t.Errorf("%s did not run and pass: a router change must widen the regression scope to routingtest\n%s", name, out)
	}
}

const probeArtifactBytes = 200000

var falseNoConsumerClaim = regexp.MustCompile(`(?i)no\s+consumer\s+(asks|reads|uses|needs|wants|requests)`)

func registryRoutingConfig(t *testing.T, root string) config.RoutingConfig {
	t.Helper()
	cfg := router.PolicyForProject(root, nil).Cfg
	builtin, err := phasespec.Load(config.RegistryPath(root))
	if err != nil {
		t.Fatalf("load builtin phase registry: %v", err)
	}
	user, _, _ := phasespec.DiscoverUserSpecsFromRoots(phasespec.Roots(root))
	phasespec.ApplyUserRouting(&cfg, user, builtin)
	return cfg
}

func artifactBytesConsumers(cfg config.RoutingConfig) []string {
	var consumers []string
	for _, phase := range slices.Sorted(maps.Keys(cfg.Triggers)) {
		block := cfg.Triggers[phase]
		for _, c := range slices.Concat(block.InsertWhen, block.SkipWhen) {
			if c.Field == artifactBytesKey {
				consumers = append(consumers, phase)
				break
			}
		}
	}
	return consumers
}

func buildExplanation(t *testing.T, root string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1747-*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want exactly one cycle-1747 build explanation, got %v (err=%v)", matches, err)
	}
	rel, _ := filepath.Rel(root, matches[0])
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is not tracked, so ship would drop the corrected explanation", rel)
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func docSection(t *testing.T, doc, heading string) string {
	t.Helper()
	_, rest, ok := strings.Cut(doc, "\n## "+heading+"\n")
	if !ok {
		t.Errorf("build explanation has no ## %s section", heading)
		return ""
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return body
}

// acs-predicate: config-check — the repair is document-only; the registry and a router probe derive what the document must say.
func TestC1747_008_ExplanationNamesTheInsertWhenConsumerTheTypedFieldCannotReach(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cfg := registryRoutingConfig(t, root)
	consumers := artifactBytesConsumers(cfg)
	if len(consumers) == 0 {
		t.Fatalf("no registry phase routes on %s; the explanation's consumer account cannot be derived", artifactBytesKey)
	}

	ws := newWorkspace(t, map[string]string{
		"handoff-scout.json": `{"backlog_size":3,"` + artifactBytesKey + `":200000}`,
	})
	sig, err := router.Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got := scoutArtifactBytes(t, sig.Scout); got != probeArtifactBytes {
		t.Fatalf("Scout.ArtifactBytes = %d, want %d from the handoff", got, probeArtifactBytes)
	}
	viaGeneric := sig
	viaGeneric.Generic = map[string]any{artifactBytesKey: float64(probeArtifactBytes)}

	pol := router.NewPhasePolicy(cfg)
	var unreached []string
	for _, phase := range consumers {
		if !pol.Enabled(phase, viaGeneric) {
			t.Fatalf("control: %s does not fire even with Generic[%q]=%d, so the probe cannot tell an unreached field from a disabled phase", phase, artifactBytesKey, probeArtifactBytes)
		}
		if !pol.Enabled(phase, sig) {
			unreached = append(unreached, phase)
		}
	}

	doc := buildExplanation(t, root)
	if claim := falseNoConsumerClaim.FindString(doc); claim != "" {
		t.Errorf("explanation claims %q, but %v route on %s via insert_when", claim, consumers, artifactBytesKey)
	}
	design := docSection(t, doc, "Design Decisions")
	for _, phase := range consumers {
		if !strings.Contains(design, phase) || !strings.Contains(design, "insert_when") {
			t.Errorf("## Design Decisions does not name the %s insert_when consumer of %s", phase, artifactBytesKey)
		}
	}
	if len(unreached) == 0 {
		return
	}
	if !strings.Contains(design, "resolveField") {
		t.Errorf("## Design Decisions does not say that ScoutSignals.ArtifactBytes has no resolveField case, yet a handoff with %d bytes leaves %v unfired", probeArtifactBytes, unreached)
	}
	limits := docSection(t, doc, "Limitations")
	if !strings.Contains(strings.ToLower(limits), "writer") {
		t.Errorf("## Limitations dropped the missing scout-side writer")
	}
	if !strings.Contains(limits, "insert-when-fields-need-a-producer") {
		t.Errorf("## Limitations does not name the queued consumer-side item insert-when-fields-need-a-producer, yet %v stay unreachable through the typed field", unreached)
	}
}
