//go:build integration

package coverstrict

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

var goTestCoverage = regexp.MustCompile(`coverage: ([0-9.]+)% of statements`)

func TestCheck_PercentEqualsWhatGoTestCoverPrints(t *testing.T) {
	root := t.TempDir()
	for rel, text := range map[string]string{
		"go.mod":        "module sample\n\ngo 1.23\n",
		"p/p.go":        "package p\n\nfunc F(n int) int {\n\tif n > 0 {\n\t\treturn 1\n\t}\n\tif n < -5 {\n\t\treturn 2\n\t}\n\tswitch n {\n\tcase -1:\n\t\treturn 3\n\t}\n\treturn 0\n}\n\nfunc G() int { return 7 }\n",
		"p/p_test.go":   "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) { _ = F(1); _ = F(0) }\n",
		".cover-strict": "./p 0\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-cover", "-coverprofile=coverage.txt", "./p")
	cmd.Dir = root
	cmd.Env = gotooltest.Env(append(os.Environ(), "GOWORK=off", "GOFLAGS="))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
	match := goTestCoverage.FindSubmatch(out)
	if match == nil {
		t.Fatalf("go test printed no coverage figure:\n%s", out)
	}

	results, err := Check(root, filepath.Join(root, "coverage.txt"), filepath.Join(root, ".cover-strict"))

	if err != nil || len(results) != 1 {
		t.Fatalf("results = %+v, %v", results, err)
	}
	if got := fmt.Sprintf("%.1f", results[0].Percent); got != string(match[1]) || results[0].Percent >= 100 {
		t.Errorf("cover strict percent = %s, go test -cover = %s (want equal, and below 100 so the figure is not trivial)", got, match[1])
	}
}
