//go:build e2e

package e2e

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
)

func TestE2E_VersionSubcommandRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH; skipping e2e build")
	}
	builds := gotooltest.NewCache(gotooltest.ExecRunner)
	t.Cleanup(func() { _ = builds.Close() })
	bin, err := builds.Build(gotooltest.Spec{Pkg: "../../cmd/evolve", Name: "evolve"})
	if err != nil {
		t.Fatalf("build evolve: %v", err)
	}

	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("run `evolve version`: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if !strings.HasPrefix(got, "evolve") {
		t.Fatalf("`evolve version` output not self-identifying: %q", got)
	}
}
