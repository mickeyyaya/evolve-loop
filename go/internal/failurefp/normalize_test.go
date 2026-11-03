package failurefp_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurefp"
)

func TestNormalize_FoldsEachPerCycleToken(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"audit verdict conflict narrative=PASS", "audit verdict conflict narrative=<verdict>"},
		{"phase=scout cycle=2071 driver=claude-tmux", "phase=scout cycle=N driver=claude-tmux"},
		{"recycle=12 stays", "recycle=12 stays"},
		{"docs/explain/builds/cycle-2049-01m5xtvnwk94g9ryfgzhqre9m0.md:8", "docs/explain/builds/cycle-N-<ulid>.md:8"},
		{"failed in cycle 1946 and runs/cycle-12-3/x", "failed in cycle-N and runs/cycle-N/x"},
		{"the STE lint: 2 finding(s) in 1 file(s)", "the STE lint: <n> finding(s) in 1 file(s)"},
		{"12 findings and 1 finding", "<n> findings and <n> finding"},
		{"2 deterministic check failure(s)", "2 deterministic check failure(s)"},
		{"attempt 2/3 and retry 1 of 4", "attempt <n>/<n> and retry <n>/<n>"},
		{"ok 0.31s with -timeout 300s", "ok <dur> with -timeout 300s"},
		{"  padded reason \n", "padded reason"},
	} {
		if got := failurefp.Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalize_DropsTheDetailSpan(t *testing.T) {
	t.Parallel()

	in := cyclestate.WithDetail("floor red", "run 1 log")
	if got, plain := failurefp.Normalize(in), failurefp.Normalize("floor red"); got != plain {
		t.Errorf("Normalize(%q) = %q, want %q", in, got, plain)
	}
}
