//go:build acs

package apicover

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestApicoverEnforce_CoversEveryInternalPackage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	enfData, err := os.ReadFile(filepath.Join(goDir, ".apicover-enforce"))
	if err != nil {
		t.Fatalf("read .apicover-enforce: %v", err)
	}
	enforced := map[string]bool{}
	for _, line := range strings.Split(string(enfData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		enforced[line] = true
	}
	if len(enforced) == 0 {
		t.Fatal(".apicover-enforce has no enforced packages")
	}

	modPath := goList(t, goDir, "-m")
	listOut := goListPkgs(t, goDir)

	internal := map[string]bool{}
	var missing []string
	for _, ip := range listOut {
		pat := "." + strings.TrimPrefix(ip, modPath)
		internal[pat] = true
		if !enforced[pat] {
			missing = append(missing, pat)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("apicover completeness regression: %d internal package(s) NOT in .apicover-enforce — graduate them (add an apicover_named_test.go) or they escape the public-API gate:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	var stale []string
	for pat := range enforced {
		if !strings.HasPrefix(pat, "./internal/") {
			continue
		}
		if !internal[pat] {
			stale = append(stale, pat)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("apicover stale entries: %d .apicover-enforce line(s) are not real ./internal/... packages:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

func goListPkgs(t *testing.T, goDir string) []string {
	t.Helper()
	out := goListRaw(t, goDir, "./internal/...")
	return strings.Fields(out)
}

func goList(t *testing.T, goDir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(goListRaw(t, goDir, args...))
}

func goListRaw(t *testing.T, goDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Dir = goDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %v (in %s): %v", args, goDir, err)
	}
	return string(out)
}
