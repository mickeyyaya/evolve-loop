package phasespec_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// repoRoot walks up from this test file to the repo root (the dir containing
// .evolve/phases). The test reads the REAL operator-overlay phases, so it proves
// the research-informed phases are added as pure config — no production Go edit.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	// .../go/internal/phasespec/usercatalog_research_test.go → up 4 to repo root.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// TestResearchPhasesAreConfigOnly loads the real merged catalog and asserts the
// 2026-research-informed phases (adversarial-review, perf-profile) exist as
// optional user phases whose spec-derived contract is well-formed — the zero-Go
// proof for WS-A + WS-C.
func TestResearchPhasesAreConfigOnly(t *testing.T) {
	root := repoRoot(t)
	registry := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	builtin, err := phasespec.Load(registry)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	user, warns := phasespec.DiscoverUserSpecs(filepath.Join(root, ".evolve", "phases"))
	for _, w := range warns {
		t.Logf("discover warning: %s", w)
	}
	if len(user) == 0 {
		t.Fatal("DiscoverUserSpecs found no user phases — is .evolve/phases/ present at the repo root?")
	}
	cat, mWarns := builtin.Merge(user)
	for _, w := range mWarns {
		t.Logf("merge warning: %s", w)
	}

	type want struct {
		artifact   string
		sections   []string
		hasVerdict bool
	}
	cases := map[string]want{
		"adversarial-review": {
			artifact:   "adversarial-review-report.md",
			sections:   []string{"## Threat Model", "## Findings", "## Verdict"},
			hasVerdict: true,
		},
		"perf-profile": {
			artifact:   "perf-profile-report.md",
			sections:   []string{"## Benchmarks", "## Findings", "## Verdict"},
			hasVerdict: true,
		},
	}

	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			spec, ok := cat.Get(name)
			if !ok {
				t.Fatalf("%s not in merged catalog — should be a config-only user phase", name)
			}
			if !cat.IsUser(name) {
				t.Errorf("%s should be a user (overlay) phase, not built-in", name)
			}
			if !spec.Optional {
				t.Errorf("%s must be optional (user-phase floor)", name)
			}
			if v := phasespec.ValidateUserSpec(spec); len(v) > 0 {
				t.Errorf("%s fails ValidateUserSpec: %v", name, v)
			}
			c := phasecontract.FromSpec(spec)
			if c.ArtifactName != w.artifact {
				t.Errorf("%s artifact = %q, want %q", name, c.ArtifactName, w.artifact)
			}
			if c.Kind != phasecontract.KindMarkdown {
				t.Errorf("%s kind = %v, want markdown", name, c.Kind)
			}
			for _, want := range w.sections {
				found := false
				for _, s := range c.Sections {
					if s.Canonical == want {
						found = true
					}
				}
				if !found {
					t.Errorf("%s contract missing required section %q (have %+v)", name, want, c.Sections)
				}
			}
			if w.hasVerdict && len(c.Verdicts) == 0 {
				t.Errorf("%s should opt into a verdict vocabulary (evaluate + verdict_on_pass)", name)
			}
		})
	}
}
