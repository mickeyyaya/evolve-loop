package journeys

import (
	"testing"
)

func assessRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, RegistryPath, `[
 {"id":"live","journey":"A path works","test":"internal/x.TestOne","issue":"#1"},
 {"id":"orphan","journey":"B path works","test":"internal/x.TestGone","issue":"#2"},
 {"id":"old","journey":"C path works","test":"internal/x.TestOne","issue":"#3","retired":"removed"}
]`)
	writeFile(t, root, "go/internal/x/one_test.go", "package x\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n")
	return root
}

func TestAssess_ReadsTheTrailerOfAFixCommit(t *testing.T) {
	root := assessRoot(t)
	writeFile(t, root, "../elsewhere/x/x_test.go", "package x\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n")
	tests := []struct {
		name    string
		message string
		want    Assessment
	}{
		{"feat has no duty", "feat: add a thing", Assessment{}},
		{"docs has no duty", "docs: say a thing\n\nFix-Journey: none", Assessment{}},
		{"fix with no trailer", "fix: a defect\n\nbody text", Assessment{Applies: true, Problem: ProblemMissing}},
		{"fix with other trailers only", "fix: a defect\n\nCloses-Inbox: x", Assessment{Applies: true, Problem: ProblemMissing}},
		{"trailer not in the closing block", "fix: a defect\n\nFix-Journey: live\n\nclosing prose", Assessment{Applies: true, Problem: ProblemMissing}},
		{"registered journey", "fix: a defect\n\nFix-Journey: live", Assessment{Applies: true}},
		{"breaking fix with scope", "fix(bridge)!: a defect\n\nFix-Journey: live", Assessment{Applies: true}},
		{"unknown journey", "fix: a defect\n\nFix-Journey: nope", Assessment{Applies: true, Problem: ProblemUnknownJourney}},
		{"retired journey", "fix: a defect\n\nFix-Journey: old", Assessment{Applies: true, Problem: ProblemUnknownJourney}},
		{"row whose test is gone", "fix: a defect\n\nFix-Journey: orphan", Assessment{Applies: true, Problem: ProblemMissingTest}},
		{"waiver with a reason", "fix: a defect\n\nFix-Journey: none behavior-preserving rename", Assessment{Applies: true}},
		{"waiver without a reason", "fix: a defect\n\nFix-Journey: none", Assessment{Applies: true, Problem: ProblemMalformed}},
		{"lane test that exists", "fix: a defect\n\nFix-Journey: test:internal/x.TestOne", Assessment{Applies: true}},
		{"lane test that is absent", "fix: a defect\n\nFix-Journey: test:internal/x.TestGone", Assessment{Applies: true, Problem: ProblemMissingTest}},
		{"lane test with no package", "fix: a defect\n\nFix-Journey: test:TestOne", Assessment{Applies: true, Problem: ProblemMissingTest}},
		{"lane test outside the tree", "fix: a defect\n\nFix-Journey: test:../../elsewhere/x.TestOne", Assessment{Applies: true, Problem: ProblemMissingTest}},
		{"journey id that starts with none", "fix: a defect\n\nFix-Journey: nonesuch", Assessment{Applies: true, Problem: ProblemUnknownJourney}},
		{"two trailers", "fix: a defect\n\nFix-Journey: live\nFix-Journey: none why", Assessment{Applies: true, Problem: ProblemMalformed}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Assess(root, tc.message); got != tc.want {
				t.Errorf("Assess = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestAssess_NamesAnUnreadableRegistry(t *testing.T) {
	root := t.TempDir()
	if got := Assess(root, "fix: a defect\n\nFix-Journey: live"); got != (Assessment{Applies: true, Problem: ProblemRegistryUnreadable}) {
		t.Errorf("Assess = %+v, want an unreadable registry", got)
	}
	if got := Assess(root, "fix: a defect\n\nFix-Journey: none a reason"); got != (Assessment{Applies: true}) {
		t.Errorf("a waiver reads no registry: %+v", got)
	}
}
