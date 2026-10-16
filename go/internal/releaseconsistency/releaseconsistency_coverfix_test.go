package releaseconsistency

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_RequiredContentHeadingPrecedesTheChangelogAndFollowsTheVersionMarkers(t *testing.T) {
	t.Parallel()
	d := makeRepo(t, "11.8.2")
	var buf bytes.Buffer
	if _, err := Run(Options{ProjectRoot: d, Target: "11.8.2", Stderr: &buf}); err != nil {
		t.Fatal(err)
	}
	log := buf.String()
	heading := strings.Index(log, "--- Required content ---")
	if heading < 0 {
		t.Fatalf("no heading in log:\n%s", log)
	}
	if readme := strings.LastIndex(log[:heading], "README.md"); readme < 0 {
		t.Errorf("the README version marker must print before the heading:\n%s", log)
	}
	if !strings.Contains(log[heading:], "CHANGELOG.md") {
		t.Errorf("the CHANGELOG check must print after the heading:\n%s", log)
	}
	if strings.Contains(log[:heading], "CHANGELOG.md") {
		t.Errorf("the CHANGELOG check printed before the heading:\n%s", log)
	}
}
