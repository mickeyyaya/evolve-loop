package auditledger

import "testing"

func TestEntry_AuditedBasePrefersTheWorktreeBaseAndFallsBackToTheHead(t *testing.T) {
	for name, tc := range map[string]struct {
		entry Entry
		want  string
	}{
		"the worktree base wins over main's head":       {Entry{WorktreeBaseSHA: "base", GitHEAD: "head"}, "base"},
		"a row written before the field reads its head": {Entry{GitHEAD: "head"}, "head"},
		"a row naming neither names no base":            {Entry{}, ""},
	} {
		if got := tc.entry.AuditedBase(); got != tc.want {
			t.Errorf("%s: AuditedBase() = %q, want %q", name, got, tc.want)
		}
	}
}
