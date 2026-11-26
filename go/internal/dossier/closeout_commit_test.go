package dossier

import "testing"

func TestIsCloseoutCommit(t *testing.T) {
	t.Parallel()
	const j7, m7 = "knowledge-base/cycles/cycle-7.json", "knowledge-base/cycles/cycle-7.md"
	cases := []struct {
		name    string
		subject string
		paths   []string
		want    bool
	}{
		{"both files", "dossier: cycle-7 closeout", []string{j7, m7}, true},
		{"one file", "dossier: cycle-7 closeout", []string{m7}, true},
		{"wrong subject", "dossier: cycle-x closeout", []string{j7}, false},
		{"padded number", "dossier: cycle-007 closeout", []string{j7}, false},
		{"unrelated subject", "feat: something", []string{j7}, false},
		{"code path too", "dossier: cycle-7 closeout", []string{j7, "go/internal/core/x.go"}, false},
		{"other cycle path", "dossier: cycle-7 closeout", []string{"knowledge-base/cycles/cycle-8.json"}, false},
		{"empty paths", "dossier: cycle-7 closeout", nil, false},
	}
	for _, c := range cases {
		if got := IsCloseoutCommit(c.subject, c.paths); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
