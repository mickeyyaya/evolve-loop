package committedset

import (
	"strings"
	"testing"
)

func TestProtectedSurfaceReasonPrefix_MarksTheReasonTriageWritesForAProtectedCard(t *testing.T) {
	t.Parallel()
	body := []byte(`{"escalate_block":[{"task_id":"a","reason":"protected-surface: go/internal/guards/role.go"}],"dropped":[{"id":"b","reason":"duplicate"}]}`)
	var marked []string
	for _, d := range DispositionsFrom(body) {
		if strings.HasPrefix(d.Reason, ProtectedSurfaceReasonPrefix) {
			marked = append(marked, d.ID)
		}
	}
	if len(marked) != 1 || marked[0] != "a" {
		t.Errorf("marked = %v, want [a]", marked)
	}
}
