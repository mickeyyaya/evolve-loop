package guidance

import "testing"

func TestSelect_KeepsActiveRulesThatMatchEachGivenFilter(t *testing.T) {
	rules := []Rule{
		{ID: "all", Layer: "L0", Status: StatusActive, Audience: []string{"all"}},
		{ID: "console", Layer: "L1", Status: StatusActive, Audience: []string{"console"}},
		{ID: "build", Layer: "L2", Status: StatusActive, Audience: []string{"phase"}, Phases: []string{"build"}},
		{ID: "claude", Layer: "L2", Status: StatusActive, Audience: []string{"phase"}, Families: []string{"claude"}},
		{ID: "gone", Layer: "L0", Status: "retired", Audience: []string{"all"}},
	}
	cases := []struct {
		name string
		f    Filter
		want []string
	}{
		{"no filter", Filter{}, []string{"all", "console", "build", "claude"}},
		{"console", Filter{Audience: "console"}, []string{"all", "console"}},
		{"phase scout", Filter{Audience: "phase", Phase: "scout"}, []string{"all", "claude"}},
		{"phase build on gpt", Filter{Audience: "phase", Phase: "build", Family: "gpt"}, []string{"all", "build"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Select(rules, c.f)
			if len(got) != len(c.want) {
				t.Fatalf("Select = %+v, want %v", got, c.want)
			}
			for i, id := range c.want {
				if got[i].ID != id {
					t.Fatalf("Select[%d] = %s, want %s", i, got[i].ID, id)
				}
			}
		})
	}
}
