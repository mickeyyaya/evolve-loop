package llmroute

import "testing"

func TestBinding_CarriesTheDriverOpaquely(t *testing.T) {
	if !(Binding{}).IsZero() || Bind("agy-tmux").IsZero() {
		t.Fatal("only the zero Binding reports IsZero")
	}
	if got := Bind("agy-tmux").String(); got != "agy-tmux" {
		t.Fatalf("String() = %q, want the bound driver", got)
	}
}

func TestEscalationTarget_PicksTheFirstAllowedDriverOfAnotherFamily(t *testing.T) {
	allowAll := func(string) bool { return true }
	refuseCodex := func(d string) bool { return Family(d) != "codex" }
	cases := []struct {
		name       string
		candidates []string
		failed     string
		allowed    func(string) bool
		want       string
		ok         bool
	}{
		{"another family follows", []string{"claude-tmux", "claude-p", "codex-tmux"}, "claude-tmux", allowAll, "codex-tmux", true},
		{"the same family only", []string{"claude-tmux", "claude-p"}, "claude-p", allowAll, "", false},
		{"an empty slot is skipped", []string{"", "agy-tmux"}, "claude-tmux", allowAll, "agy-tmux", true},
		{"a refused driver is skipped", []string{"codex-tmux", "agy-tmux"}, "claude-tmux", refuseCodex, "agy-tmux", true},
		{"no candidate", nil, "claude-tmux", allowAll, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := EscalationTarget(tc.candidates, Bind(tc.failed), tc.allowed)
			if ok != tc.ok || got != Bind(tc.want) {
				t.Fatalf("EscalationTarget = %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
