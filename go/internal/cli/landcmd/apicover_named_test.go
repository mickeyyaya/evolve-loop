package landcmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestApicover_NamesEveryExport(t *testing.T) {
	t.Run("Deps and RunPR", func(t *testing.T) {
		deps := testDeps()
		if deps.ResolveRoot == nil || deps.PrintVerbJSON == nil || deps.NewSignalCenter == nil ||
			deps.DarwinSet == nil || deps.RefreshWorktree == nil {
			t.Fatalf("a Deps field is nil: %+v", deps)
		}
		var out, errb bytes.Buffer
		if rc := RunPR(nil, strings.NewReader(""), &out, &errb, deps); rc != exitUsage || !strings.Contains(errb.String(), "usage: evolve pr merge") {
			t.Fatalf("RunPR with no verb: rc %d stdout %q stderr %q", rc, out.String(), errb.String())
		}
	})
	t.Run("RunCI", func(t *testing.T) {
		var out, errb bytes.Buffer
		if rc := RunCI([]string{"watch", "--help"}, strings.NewReader(""), &out, &errb, testDeps()); rc != exitOK || !strings.Contains(out.String(), "usage: evolve ci watch") {
			t.Fatalf("RunCI watch --help: rc %d stdout %q stderr %q", rc, out.String(), errb.String())
		}
	})
}
