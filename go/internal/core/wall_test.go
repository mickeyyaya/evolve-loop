package core

import (
	"testing"
	"time"
)

var wallReset = time.Date(2026, 11, 6, 21, 0, 0, 0, time.FixedZone("", 8*3600))

func TestWallClauses_EachRowKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ends []FamilyEnd
		want string
	}{
		{"login", []FamilyEnd{{CLI: "claude", Pattern: "auth_recheck"}}, "claude needs a login"},
		{"quota with reset", []FamilyEnd{{CLI: "agy", Pattern: "rate_limit", ResetAt: wallReset}}, "agy is on quota until 2026-11-06T21:00+08:00"},
		{"quota without reset", []FamilyEnd{{CLI: "agy", Pattern: "rate_limit"}}, "agy is on quota, reset unknown"},
		{"other", []FamilyEnd{{CLI: "codex", Pattern: "model_not_found"}}, "codex ended on model_not_found"},
		{"empty", []FamilyEnd{{CLI: "claude"}}, "claude ended with no recorded cause"},
		{"mixed", []FamilyEnd{{CLI: "claude", Pattern: "auth_recheck"}, {CLI: "agy", Pattern: "rate_limit", ResetAt: wallReset}}, "claude needs a login; agy is on quota until 2026-11-06T21:00+08:00"},
		{"none", nil, "no cause was recorded for this pause"},
	}
	for _, tc := range cases {
		var w Wall
		for _, e := range tc.ends {
			w = w.Observe(e)
		}
		if got := w.Clauses(); got != tc.want {
			t.Errorf("%s: Clauses()=%q want %q", tc.name, got, tc.want)
		}
	}
}

func TestWallStopReason(t *testing.T) {
	t.Parallel()
	login := FamilyEnd{CLI: "claude", Pattern: "auth_recheck"}
	quota := FamilyEnd{CLI: "agy", Pattern: "rate_limit"}
	other := FamilyEnd{CLI: "codex", Pattern: "model_not_found"}
	cases := []struct {
		name string
		ends []FamilyEnd
		want string
	}{
		{"login and quota", []FamilyEnd{login, quota}, "login-required"},
		{"all quota", []FamilyEnd{quota, {CLI: "codex", Pattern: "exhausted"}}, "quota-pause"},
		{"quota and other", []FamilyEnd{quota, other}, "wall-unknown"},
		{"quota and empty", []FamilyEnd{quota, {CLI: "codex"}}, "wall-unknown"},
		{"empty wall", nil, "wall-unknown"},
	}
	for _, tc := range cases {
		var w Wall
		for _, e := range tc.ends {
			w = w.Observe(e)
		}
		if got := w.StopReason(); got != tc.want {
			t.Errorf("%s: StopReason()=%q want %q", tc.name, got, tc.want)
		}
	}
}

func TestWallObserve_RankKeepsTheStrongestFact(t *testing.T) {
	t.Parallel()
	w := Wall{}.
		Observe(FamilyEnd{CLI: "claude", Pattern: "auth_recheck"}).
		Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "repl_boot_timeout"}).
		Observe(FamilyEnd{CLI: "claude"})
	if got := w.Clauses(); got != "claude needs a login" {
		t.Errorf("a later lower attempt replaced the login row: %q", got)
	}
	if len(w.Ends) != 1 {
		t.Errorf("claude and claude-tmux are one family: rows=%d", len(w.Ends))
	}
	q := Wall{}.
		Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit", ResetAt: wallReset}).
		Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"}).
		Observe(FamilyEnd{CLI: "agy", Pattern: "model_not_found"})
	if got := q.Clauses(); got != "agy is on quota until 2026-11-06T21:00+08:00" {
		t.Errorf("a quota row lost its reset: %q", got)
	}
	up := Wall{}.Observe(FamilyEnd{CLI: "claude"}).Observe(FamilyEnd{CLI: "claude", Pattern: "auth_recheck"})
	if got := up.Clauses(); got != "claude needs a login" {
		t.Errorf("a stronger later attempt must replace: %q", got)
	}
}

func TestWallMerge_AppliesTheRankAcrossDispatches(t *testing.T) {
	t.Parallel()
	d1 := Wall{}.Observe(FamilyEnd{CLI: "claude", Pattern: "auth_recheck"}).Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"})
	d2 := Wall{}.Observe(FamilyEnd{CLI: "claude"}).Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"})
	m := d1.Merge(d2)
	if m.StopReason() != "login-required" || m.Clauses() != "claude needs a login; agy is on quota, reset unknown" {
		t.Errorf("merge lost the login: %q %q", m.StopReason(), m.Clauses())
	}
	if len(d1.Ends) != 2 || d1.Ends[0].Pattern != "auth_recheck" {
		t.Errorf("merge mutated its receiver: %+v", d1.Ends)
	}
}

func TestWallFamilies(t *testing.T) {
	t.Parallel()
	w := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux"}).Observe(FamilyEnd{CLI: "agy"})
	got := w.Families()
	if len(got) != 2 || got[0] != "claude" || got[1] != "agy" {
		t.Errorf("Families()=%v", got)
	}
}

func wallWorkspace(t *testing.T, scoped bool) string {
	t.Helper()
	ws := t.TempDir()
	if scoped {
		materializeLaneScope(ws, "todo-1", "goal")
	}
	return ws
}

func TestWallNextStep(t *testing.T) {
	t.Parallel()
	login := Wall{}.Observe(FamilyEnd{CLI: "claude", Pattern: "auth_recheck"})
	mixed := login.Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit", ResetAt: wallReset})
	quota := Wall{}.Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"})
	other := Wall{}.Observe(FamilyEnd{CLI: "codex", Pattern: "model_not_found", Evidence: "ws/escalation-report.json"})
	cases := []struct {
		name   string
		wall   Wall
		scoped bool
		want   string
	}{
		{"login in a lane", login, true, "open claude and run /login. Then evolve cycle resume --all, then evolve wave next."},
		{"login in a sequential loop", login, false, "open claude and run /login. Then evolve loop --resume."},
		{"mixed", mixed, true, "open claude and run /login, or wait for agy at 2026-11-06T21:00+08:00. Then evolve cycle resume --all, then evolve wave next."},
		{"quota only", quota, false, "wait for agy, reset unknown. Then evolve loop --resume."},
		{"other row reads its evidence", other, false, "read ws/escalation-report.json. Then evolve loop --resume."},
		{"empty wall", Wall{}, false, "evolve loop --resume."},
	}
	for _, tc := range cases {
		if got := WallNextStep(tc.wall, wallWorkspace(t, tc.scoped)); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

func TestWallAdmits(t *testing.T) {
	t.Parallel()
	w := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"})
	cases := []struct {
		name string
		end  FamilyEnd
		want bool
	}{
		{"new family", FamilyEnd{CLI: "codex", Pattern: ""}, true},
		{"same rank", FamilyEnd{CLI: "claude", Pattern: "auth_recheck"}, true},
		{"lower rank", FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"}, false},
		{"empty", FamilyEnd{CLI: "claude", Pattern: ""}, false},
	}
	for _, tc := range cases {
		if got := w.Admits(tc.end); got != tc.want {
			t.Errorf("%s: Admits=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestWallCountsAsQuotaPause(t *testing.T) {
	t.Parallel()
	login := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"})
	other := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "bridge_crash"})
	quota := Wall{}.Observe(FamilyEnd{CLI: "agy", Pattern: "rate_limit"})
	cases := []struct {
		name string
		wall Wall
		want bool
	}{
		{"no rows", Wall{}, true},
		{"empty cause", Wall{}.Observe(FamilyEnd{CLI: "agy"}), true},
		{"login", login, false},
		{"other", other, false},
		{"quota", quota, true},
		{"login and quota", login.Merge(quota), true},
	}
	for _, c := range cases {
		if got := c.wall.CountsAsQuotaPause(); got != c.want {
			t.Errorf("%s: CountsAsQuotaPause=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestWallStopReasonFor(t *testing.T) {
	t.Parallel()
	login := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"})
	other := Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "bridge_crash"})
	cases := []struct {
		name  string
		wall  Wall
		timed bool
		want  string
	}{
		{"an old checkpoint with a wake time", Wall{}, true, "quota-pause"},
		{"no rows and no wake time", Wall{}, false, "wall-unknown"},
		{"a login keeps its class", login, true, "login-required"},
		{"rows of another cause keep their class", other, true, "wall-unknown"},
	}
	for _, c := range cases {
		if got := c.wall.StopReasonFor(c.timed); got != c.want {
			t.Errorf("%s: StopReasonFor(%v)=%q, want %q", c.name, c.timed, got, c.want)
		}
	}
}
