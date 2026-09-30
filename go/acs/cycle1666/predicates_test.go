//go:build acs

package cycle1666

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	dossierPkg = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

	legacyRetroSkipFloor = 134
)

var (
	evolveBin      string
	evolveBuildErr error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1666-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1666: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
		out, err := exec.Command("go", "-C", filepath.Join(root, "go"), "build", "-o", evolveBin, "./cmd/evolve").CombinedOutput()
		if err != nil {
			evolveBuildErr = fmt.Errorf("go build ./cmd/evolve: %v\n%s", err, out)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func repoRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("RED (harness): %v", evolveBuildErr)
	}
	return evolveBin
}

func assertSuiteTestsPass(t *testing.T, tags, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	var (
		stdout, stderr string
		code           int
		err            error
	)
	if tags == "" {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	} else {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-tags", tags, "-run", pattern, "-count=1", "-v", pkg)
	}
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

func legacyRetroSkipCycles(t *testing.T, root string) []int {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus %s: %v", dir, err)
	}
	var out []int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "cycle-") || filepath.Ext(name) != ".json" {
			continue
		}
		n, cerr := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "cycle-"), ".json"))
		if cerr != nil {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if _, stamped := m["schema_version"]; stamped {
			continue
		}
		if skipped, ok := m["skipped_phases"].([]any); ok {
			for _, s := range skipped {
				if entry, ok := s.(map[string]any); ok && entry["phase"] == "retro" {
					out = append(out, n)
					break
				}
			}
		}
	}
	sort.Ints(out)
	return out
}

func retroRanReceipts(t *testing.T, root string, cycles []int) map[int]bool {
	t.Helper()
	got := map[int]bool{}
	for _, c := range cycles {
		runDir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", c))
		for _, name := range []string{"retrospective-report.md", "retro-report.md"} {
			if _, err := os.Stat(filepath.Join(runDir, name)); err == nil {
				got[c] = true
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Logf("no readable ledger at %s (%v) — ledger receipts contribute nothing to the oracle", filepath.Join(root, ".evolve", "ledger.jsonl"), err)
		return got
	}
	want := map[int]bool{}
	for _, c := range cycles {
		want[c] = true
	}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"role":"retro"`)) || !bytes.Contains(line, []byte(`"kind":"agent_subprocess"`)) {
			continue
		}
		var e struct {
			Cycle int    `json:"cycle"`
			Role  string `json:"role"`
			Kind  string `json:"kind"`
		}
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		if e.Role == "retro" && e.Kind == "agent_subprocess" && want[e.Cycle] {
			got[e.Cycle] = true
		}
	}
	return got
}

type mislabelReport struct {
	Candidates     int   `json:"candidates"`
	Mislabeled     []int `json:"mislabeled"`
	Uncorroborated []int `json:"uncorroborated"`
}

func TestC1666_001_DerivedAffectedCountCrossChecksArtifacts(t *testing.T) {
	assertSuiteTestsPass(t, "", cmdPkg,
		"TestDossierRetroMislabel_DerivedCountCrossChecksArtifacts",
		"TestDossierRetroMislabel_AbsentCorpusFailsLoudly",
	)

	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "dossier", "retro-mislabel", "--project-root", root, "--json")
	if err != nil || code != 0 {
		t.Fatalf("RED: evolve dossier retro-mislabel over the real corpus exited %d: %v\nstderr:\n%s", code, err, stderr)
	}
	var rep mislabelReport
	if uerr := json.Unmarshal([]byte(stdout), &rep); uerr != nil {
		t.Fatalf("--json output is not a JSON object: %v\n%s", uerr, stdout)
	}

	candidates := legacyRetroSkipCycles(t, root)
	receipts := retroRanReceipts(t, root, candidates)
	var wantMislabeled, wantUncorroborated []int
	for _, c := range candidates {
		if receipts[c] {
			wantMislabeled = append(wantMislabeled, c)
		} else {
			wantUncorroborated = append(wantUncorroborated, c)
		}
	}
	t.Logf("oracle: candidates=%d mislabeled=%d uncorroborated=%d; cli: candidates=%d mislabeled=%d uncorroborated=%d",
		len(candidates), len(wantMislabeled), len(wantUncorroborated), rep.Candidates, len(rep.Mislabeled), len(rep.Uncorroborated))
	if rep.Candidates != len(candidates) {
		t.Errorf("candidates = %d, independent derivation says %d", rep.Candidates, len(candidates))
	}
	if rep.Candidates != len(rep.Mislabeled)+len(rep.Uncorroborated) {
		t.Errorf("candidates (%d) != mislabeled (%d) + uncorroborated (%d)", rep.Candidates, len(rep.Mislabeled), len(rep.Uncorroborated))
	}
	if !reflect.DeepEqual(nonNil(rep.Mislabeled), nonNil(wantMislabeled)) {
		t.Errorf("mislabeled set disagrees with the independent receipt cross-check\n cli:    %v\n oracle: %v", rep.Mislabeled, wantMislabeled)
	}
	if !reflect.DeepEqual(nonNil(rep.Uncorroborated), nonNil(wantUncorroborated)) {
		t.Errorf("uncorroborated set disagrees with the independent receipt cross-check\n cli:    %v\n oracle: %v", rep.Uncorroborated, wantUncorroborated)
	}
	if len(candidates) < legacyRetroSkipFloor {
		t.Errorf("only %d legacy retro-skip records remain in the corpus (RED-time derivation: %d) — history was rewritten", len(candidates), legacyRetroSkipFloor)
	}
}

func nonNil(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}

func gitC(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func laneBase(t *testing.T, root string) string {
	t.Helper()
	base, err := gitC(t, root, "merge-base", "main", "HEAD")
	if err != nil || base == "" {
		t.Logf("merge-base main HEAD unavailable (%v); comparing against HEAD", err)
		return "HEAD"
	}
	return base
}

func TestC1666_002_DiscriminatorStampedForwardOnlyAndHistoryUntouched(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestSchemaVersion_BuildStampsTheDiscriminator",
		"TestSchemaVersion_LegacyRecordStaysUnstamped",
	)
	root := acsassert.RepoRoot(t)
	base := laneBase(t, root)
	modified, err := gitC(t, root, "diff", "--diff-filter=M", "--name-only", base, "--", "knowledge-base/cycles/")
	if err != nil {
		t.Fatalf("git diff against %s: %v", base, err)
	}
	if modified != "" {
		t.Errorf("committed dossiers were MODIFIED on this lane (a backfill — the remedy this cycle must NOT also do):\n%s", modified)
	}
	if n := len(legacyRetroSkipCycles(t, root)); n < legacyRetroSkipFloor {
		t.Errorf("legacy retro-skip records = %d, RED-time floor %d — the corpus was rewritten", n, legacyRetroSkipFloor)
	}
}

func goldenMinusVersion(t *testing.T, raw []byte, label string) (map[string]any, any, bool) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s: not JSON: %v", label, err)
	}
	v, stamped := m["schema_version"]
	delete(m, "schema_version")
	return m, v, stamped
}

func TestC1666_003_SchemaLockstepAndGoldensChangeOnlyByTheStamp(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestSchema_NoDrift",
		"TestSchemaVersion_SchemaDeclaresTheField",
	)
	assertSuiteTestsPass(t, "", corePkg,
		"TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes",
		"TestDossierSystemFailure_OrdinaryPassStaysByteClean",
		"TestWriteCycleDossier_WritesValidArtifact",
	)
	root := acsassert.RepoRoot(t)
	base := laneBase(t, root)
	for _, rel := range []string{
		"go/internal/core/testdata/dossierparams/cycle-4242.golden.json",
		"go/internal/core/testdata/dossierparams/cycle-4243.golden.json",
	} {
		now, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		then, err := gitC(t, root, "show", base+":"+rel)
		if err != nil {
			t.Fatalf("git show %s:%s: %v", base, rel, err)
		}
		nowRest, nowVersion, stamped := goldenMinusVersion(t, now, rel)
		thenRest, _, _ := goldenMinusVersion(t, []byte(then), rel+"@"+base)
		if !stamped {
			t.Errorf("RED: %s carries no schema_version — the producer golden was not regenerated from the stamping Build", rel)
		} else if n, ok := nowVersion.(float64); !ok || n < 2 || n != float64(int(n)) {
			t.Errorf("%s schema_version = %#v, want an integer >= 2", rel, nowVersion)
		}
		if !reflect.DeepEqual(nowRest, thenRest) {
			t.Errorf("%s changed beyond the schema_version stamp relative to %s — regenerate goldens only for the discriminator, nothing else", rel, base)
		}
	}
}

func TestC1666_004_ConsumerRefusesLegacyRetroSkipAsEvidence(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestPhaseSkipEvidence_LegacyRetroSkipIsNeverTrusted",
		"TestPhaseSkipEvidence_VersionedAndAbsentEntries",
	)
	root := acsassert.RepoRoot(t)
	callers := 0
	for _, sub := range []string{"go/internal", "go/cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, sub), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			calls := strings.Count(string(raw), "PhaseSkipEvidence(") - strings.Count(string(raw), "func PhaseSkipEvidence(")
			if calls > 0 {
				callers += calls
				t.Logf("production caller of PhaseSkipEvidence: %s", strings.TrimPrefix(path, root+"/"))
			}
			return nil
		})
	}
	if callers == 0 {
		t.Errorf("RED: no production (non-test) file calls PhaseSkipEvidence( — the consumer seam is dead code until the audit command reaches it")
	}
}
