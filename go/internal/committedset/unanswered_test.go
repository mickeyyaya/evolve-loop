package committedset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnanswered_IsThePinLessWhatTheDecisionAnswered(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		decision any
		want     string
	}{
		{"an escalation and a reasoned drop answer; the rest is owed", map[string]any{
			"top_n":          []any{},
			"escalate_block": []map[string]string{{"task_id": "a", "reason": "protected-surface: x"}},
			"dropped":        []map[string]string{{"id": "b", "reason": "premise landed"}},
		}, "c"},
		{"a deferral is not an answer", map[string]any{"top_n": []any{}, "deferred": ids("a", "b")}, "a,b,c"},
		{"a drop without a reason answers for nothing", map[string]any{"dropped": ids("a")}, "a,b,c"},
		{"an answer outside the pin changes nothing", map[string]any{"escalate_block": []map[string]string{{"task_id": "alias", "reason": "r"}}}, "a,b,c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			write(t, ws, LanePinFile, map[string]any{"todo_ids": []string{"a", "b", "c"}})
			write(t, ws, DecisionFile, tc.decision)

			if got := strings.Join(Unanswered(ws), ","); got != tc.want {
				t.Errorf("Unanswered = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnanswered_WithoutADecisionTheWholePinIsOwed(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	write(t, ws, LanePinFile, map[string]any{"todo_ids": []string{"a", " ", "b"}})

	if got := strings.Join(Unanswered(ws), ","); got != "a,b" {
		t.Errorf("absent decision: Unanswered = %q, want the non-blank pin", got)
	}
	if err := os.WriteFile(filepath.Join(ws, DecisionFile), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(Unanswered(ws), ","); got != "a,b" {
		t.Errorf("malformed decision answers for nothing: Unanswered = %q", got)
	}
}

func TestUnanswered_NoPinOwesNothing(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	write(t, ws, DecisionFile, map[string]any{"top_n": []any{}})

	if got := Unanswered(ws); len(got) != 0 {
		t.Errorf("Unanswered = %v; a cycle without a pin has no owed pin", got)
	}
}
