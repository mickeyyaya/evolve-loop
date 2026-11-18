package tmuxtest

import (
	"go/build/constraint"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func buildConstraintOf(t *testing.T, file string) constraint.Expr {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if constraint.IsGoBuild(line) {
			expr, err := constraint.Parse(line)
			if err != nil {
				t.Fatal(err)
			}
			return expr
		}
	}
	t.Fatalf("%s has no go:build line", file)
	return nil
}

func TestServeFiles_UseTheExclusionTagOfTheBridgeTierEntry(t *testing.T) {
	tag, ok := ciparity.EnvExclusiveTagOf("internal/bridge")
	if !ok {
		t.Fatal("internal/bridge declares no exclusion tag")
	}
	if got := buildConstraintOf(t, "serve_floor.go").String(); got != tag {
		t.Errorf("serve_floor.go constraint = %q, want %q", got, tag)
	}
	if got := buildConstraintOf(t, "serve_tmux.go").String(); got != "!"+tag {
		t.Errorf("serve_tmux.go constraint = %q, want %q", got, "!"+tag)
	}
}

func TestBuildConstraintOf_ReadsTheGoBuildLineOnly(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/a.go"
	if err := os.WriteFile(file, []byte("// header\n//go:build a && !b\n\npackage x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := buildConstraintOf(t, file).String(); got != "a && !b" {
		t.Errorf("constraint = %q, want %q", got, "a && !b")
	}
}
