//go:build acs

package cycle2325

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const contractTestPattern = "BoundaryWriters"

const (
	dossierHomeLiteral = `"knowledge-base/cycles/"`
	inboxHomeLiteral   = `".evolve/inbox/"`
)

var (
	coreListLiteral   = regexp.MustCompile(`boundaryOwnedPrefixes\s*=\s*\[\]string\{[^}]*\}`)
	retroHomeEntry    = regexp.MustCompile(`recoveryretro\.DocsHome\s*\+\s*"/"`)
	dossierHomeDecl   = regexp.MustCompile(`\bHome\s*=\s*"knowledge-base/cycles"`)
	inboxPathspecDecl = regexp.MustCompile(`\bPathspec\s*=\s*"\.evolve/inbox/"`)
	contractPass      = regexp.MustCompile(`--- PASS: \S*` + contractTestPattern)
	contractFail      = regexp.MustCompile(`--- FAIL: \S*` + contractTestPattern)
)

type mutation struct {
	pkgDir  string
	pattern *regexp.Regexp
	inList  bool
	replace func(match string) string
}

func TestC2325_001_ContractTestPassesOnTheUnmutatedTree(t *testing.T) {
	out, code := runContract(t, nil)
	if code != 0 {
		t.Fatalf("RED: go test -run %s ./cmd/evolve exited %d\n%s", contractTestPattern, code, acsassert.OutputTail(out, 40))
	}
	if !contractPass.MatchString(out) {
		t.Fatalf("RED: no passing %s test ran in cmd/evolve — the contract test does not exist yet\n%s",
			contractTestPattern, acsassert.OutputTail(out, 20))
	}
}

func TestC2325_002_DroppingTheDossierHomeFailsTheContract(t *testing.T) {
	requireContractFails(t, mutation{
		pkgDir: "internal/core", pattern: regexp.MustCompile(regexp.QuoteMeta(dossierHomeLiteral)), inList: true,
		replace: func(string) string { return inboxHomeLiteral },
	})
}

func TestC2325_003_DroppingTheInboxHomeFailsTheContract(t *testing.T) {
	requireContractFails(t, mutation{
		pkgDir: "internal/core", pattern: regexp.MustCompile(regexp.QuoteMeta(inboxHomeLiteral)), inList: true,
		replace: func(string) string { return dossierHomeLiteral },
	})
}

func TestC2325_004_DroppingTheRetrospectiveHomeFailsTheContract(t *testing.T) {
	requireContractFails(t, mutation{
		pkgDir: "internal/core", pattern: retroHomeEntry, inList: true,
		replace: func(string) string { return `recoveryretro.DocsHome[:0] + ` + inboxHomeLiteral },
	})
}

func TestC2325_005_ANewDossierHomeWithoutAListEntryFailsTheContract(t *testing.T) {
	requireContractFails(t, mutation{
		pkgDir: "internal/dossier", pattern: dossierHomeDecl,
		replace: func(string) string { return `Home = "knowledge-base/zz-new-publisher"` },
	})
}

func TestC2325_006_ANewInboxPublisherHomeWithoutAListEntryFailsTheContract(t *testing.T) {
	requireContractFails(t, mutation{
		pkgDir: "internal/inboxstamps", pattern: inboxPathspecDecl,
		replace: func(string) string { return `Pathspec = ".evolve/zz-new-publisher/"` },
	})
}

func requireContractFails(t *testing.T, m mutation) {
	t.Helper()
	overlay := writeOverlay(t, m)
	out, code := runContract(t, []string{"-overlay=" + overlay})
	if code == 0 {
		t.Fatalf("RED: the %s test passed against the mutated tree — the contract does not bind this home\n%s",
			contractTestPattern, acsassert.OutputTail(out, 20))
	}
	if !contractFail.MatchString(out) {
		t.Fatalf("RED: exit %d but no failing %s test — the mutated tree failed for another reason (build error or no such test)\n%s",
			code, contractTestPattern, acsassert.OutputTail(out, 30))
	}
}

func runContract(t *testing.T, extra []string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	args := append([]string{"-C", goDir, "test", "-count=1", "-v", "-timeout=600s"}, extra...)
	args = append(args, "-run", contractTestPattern, "./cmd/evolve")
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + stderr, code
}

func writeOverlay(t *testing.T, m mutation) string {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", filepath.FromSlash(m.pkgDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if mutated, ok := mutate(string(raw), m); ok {
			return overlayFile(t, path, mutated)
		}
	}
	t.Fatalf("RED: no non-test file in go/%s holds the anchor %s — the exported home constant or the core list is not where the contract needs it",
		m.pkgDir, m.pattern)
	return ""
}

func mutate(src string, m mutation) (string, bool) {
	if !m.inList {
		loc := m.pattern.FindStringIndex(src)
		if loc == nil {
			return "", false
		}
		return src[:loc[0]] + m.replace(src[loc[0]:loc[1]]) + src[loc[1]:], true
	}
	list := coreListLiteral.FindStringIndex(src)
	if list == nil {
		return "", false
	}
	body := src[list[0]:list[1]]
	loc := m.pattern.FindStringIndex(body)
	if loc == nil {
		return "", false
	}
	changed := body[:loc[0]] + m.replace(body[loc[0]:loc[1]]) + body[loc[1]:]
	return src[:list[0]] + changed + src[list[1]:], true
}

func overlayFile(t *testing.T, original, mutated string) string {
	t.Helper()
	dir := t.TempDir()
	mutantPath := filepath.Join(dir, "mutant.go")
	if err := os.WriteFile(mutantPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	payload, err := json.Marshal(map[string]map[string]string{"Replace": {original: mutantPath}})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, payload, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return overlayPath
}
