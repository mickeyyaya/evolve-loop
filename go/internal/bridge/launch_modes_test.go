package bridge

import (
	"context"
	"os"
	"strings"
	"testing"
)

// launch_modes_test.go — --validate-only, --dry-run, --require-full
// (ported from bin/bridge cmd_launch). No driver dispatch occurs in these
// modes, so a nil runner is fine.

func TestLaunchArgs_ValidateOnly(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})
	var stdout, stderr strings.Builder
	code := eng.LaunchArgs(context.Background(), fx.args("claude-p", "--validate-only"), nil, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("validate-only exit = %d, want ExitOK", code)
	}
	out := stdout.String()
	for _, want := range []string{"resolved config", "cli             = claude-p", "model           = haiku"} {
		if !strings.Contains(out, want) {
			t.Fatalf("validate-only output missing %q; got:\n%s", want, out)
		}
	}
	if len(fr.calls) != 0 {
		t.Fatal("validate-only must not dispatch a driver")
	}
}

func TestLaunchArgs_DryRun(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})
	var stdout, stderr strings.Builder
	code := eng.LaunchArgs(context.Background(), fx.args("claude-p", "--dry-run"), nil, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("dry-run exit = %d, want ExitOK", code)
	}
	if len(fr.calls) != 0 {
		t.Fatal("dry-run must not invoke the inner CLI")
	}
	b, err := os.ReadFile(fx.artifact)
	if err != nil {
		t.Fatalf("dry-run should write a mock artifact: %v", err)
	}
	if !strings.Contains(string(b), "DRY-RUN-OK") {
		t.Fatalf("dry-run artifact missing marker; got %q", string(b))
	}
}

func TestLaunchArgs_RequireFull_Unmet(t *testing.T) {
	// claude-p needs `claude`; with no binaries available the tier is none
	// → --require-full blocks with ExitRequireFullUnmet.
	fx := newFixture(t, "claude-p", "")
	eng := NewEngine(Deps{
		Runner:    (&fakeRunner{}).runner(),
		LookupEnv: mapLookup(nil),
		LookPath:  func(string) (string, error) { return "", errNoBin },
	})
	var stdout, stderr strings.Builder
	code := eng.LaunchArgs(context.Background(), fx.args("claude-p", "--require-full"), nil, &stdout, &stderr)
	if code != ExitRequireFullUnmet {
		t.Fatalf("require-full exit = %d, want %d (ExitRequireFullUnmet)", code, ExitRequireFullUnmet)
	}
	if !strings.Contains(stderr.String(), "require-full") {
		t.Fatalf("stderr should explain require-full failure; got %q", stderr.String())
	}
}

func TestLaunchArgs_RequireFull_Met(t *testing.T) {
	// With claude available, claude-p tier is full/hybrid → require-full
	// passes and the launch proceeds (artifact produced by the fake).
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	eng := NewEngine(Deps{
		Runner:    fr.runner(),
		LookupEnv: mapLookup(nil),
		LookPath:  func(b string) (string, error) { return "/usr/bin/" + b, nil },
	})
	var stdout, stderr strings.Builder
	code := eng.LaunchArgs(context.Background(), fx.args("claude-p", "--require-full"), nil, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("require-full (met) exit = %d, want ExitOK; stderr=%s", code, stderr.String())
	}
	if len(fr.calls) == 0 {
		t.Fatal("launch should proceed when require-full is met")
	}
}
