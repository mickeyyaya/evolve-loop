package agentcmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
)

func TestAgentCapabilities_JSONListsEachVerbOfTheTableWithDefaultClassD(t *testing.T) {
	t.Parallel()
	deps := Deps{VerbSchema: func(path string) string {
		if path == "status" {
			return "evolve.status/1"
		}
		return ""
	}}
	var stdout, stderr bytes.Buffer
	code := New(deps)([]string{"capabilities", "--json", "--project-root", t.TempDir()}, nil, &stdout, &stderr)
	var got agentclass.Capabilities
	if code != 0 || json.Unmarshal(stdout.Bytes(), &got) != nil {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if got.DefaultClass != agentclass.Denied || got.Cell != nil || len(got.Verbs) != len(agentclass.Verbs()) {
		t.Fatalf("capabilities = %+v; want default D, cell null and each verb of the table", got)
	}
	if got.Verbs[0].Verb != "status" || got.Verbs[0].Output != "evolve.status/1" {
		t.Errorf("the first row must carry the envelope schema of status, got %+v", got.Verbs[0])
	}
}

func TestAgentCapabilities_TextNamesEachClassAndAStrayWordIsUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := New(Deps{})([]string{"capabilities"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr.String())
	}
	for _, want := range []string{"phase verify\tA\n", "inbox add\tR\tinbox-add\n", "wave next\tD\n", "default\tD\n"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("text output has no %q:\n%s", want, stdout.String())
		}
	}
	for _, args := range [][]string{{"capabilities", "extra"}, {"capabilities", "--bogus"}} {
		if code := New(Deps{})(args, nil, &bytes.Buffer{}, &bytes.Buffer{}); code != exitUsage {
			t.Errorf("evolve agent %v = %d, want %d", args, code, exitUsage)
		}
	}
}
