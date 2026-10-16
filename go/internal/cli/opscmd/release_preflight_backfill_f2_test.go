package opscmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
)

func TestRunReleasePreflight_MapsEachRunOutcomeToExitCodeAndMessageF2(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantRC     int
		wantStderr string
	}{
		{"all checks pass", nil, 0, ""},
		{"a check failed", fmt.Errorf("%w: working tree dirty", releasepreflight.ErrCheckFailed), 1, "[preflight] FAIL: releasepreflight: check failed: working tree dirty"},
		{"an unclassified error", errors.New("gh unavailable"), 1, "[preflight] FAIL: gh unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got releasepreflight.Options
			releasePreflightRun = func(opts releasepreflight.Options) (releasepreflight.Result, error) {
				got = opts
				return releasepreflight.Result{}, tc.err
			}
			t.Cleanup(func() { releasePreflightRun = releasepreflight.Run })
			t.Setenv("EVOLVE_PROJECT_ROOT", "/the/root")
			var out, errb bytes.Buffer

			rc := RunReleasePreflight([]string{"1.2.3", "--dry-run", "--skip-tests", "--strict-pass", "--allow-red-ci", "--require-ci"}, nil, &out, &errb)

			if rc != tc.wantRC || (tc.wantStderr == "") != (errb.Len() == 0) || !strings.Contains(errb.String(), tc.wantStderr) {
				t.Errorf("rc=%d stderr=%q, want %d containing %q", rc, errb.String(), tc.wantRC, tc.wantStderr)
			}
			if got.Target != "1.2.3" || got.RepoRoot != "/the/root" || !got.DryRun || !got.SkipTests || !got.StrictPass || !got.AllowRedCI || !got.RequireCI || got.Stderr != &errb {
				t.Errorf("options = %+v, want every flag, the target and the project root forwarded", got)
			}
		})
	}
}

func TestRunReleasePreflight_ARepoWithoutAPluginManifestFailsTheSemverStepAgainstRealGitF2(t *testing.T) {
	repo := gittest.Fixture(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", repo.Dir)
	var out, errb bytes.Buffer

	rc := RunReleasePreflight([]string{"1.2.3", "--dry-run", "--skip-tests"}, nil, &out, &errb)

	if rc != 1 || !strings.Contains(errb.String(), "[preflight] FAIL: releasepreflight: check failed: plugin.json missing at") {
		t.Errorf("rc=%d stderr=%q, want exit 1 naming the missing plugin.json", rc, errb.String())
	}
}
