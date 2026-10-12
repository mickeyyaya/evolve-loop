package triagedecision

import (
	"strings"
	"testing"
)

func TestDerive_AnUnreadableDroppedSectionFailsTheStrictReadingButNotTheLenientOne(t *testing.T) {
	report := "# Triage\n\nphase_skip: []\n\n## top_n (commit to THIS cycle)\n- good-task: do the thing — files=a.go\n\n" +
		"## deferred (carry)\n(none)\n\n## dropped (rejected with reason)\nThis section holds prose instead of cards.\n"

	out, err := Derive([]byte(report), 7, nil)

	if err == nil || !strings.Contains(err.Error(), "`## dropped` holds prose") || out != nil {
		t.Fatalf("strict: out=%s err=%v", out, err)
	}
	lenient, err := Project(report, 7)
	if err != nil || !strings.Contains(string(lenient), `"good-task"`) {
		t.Fatalf("lenient: %s err=%v", lenient, err)
	}
}
