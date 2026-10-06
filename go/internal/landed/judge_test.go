package landed

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestJudgePath_AGitlinkOrAnUnmergedPathIsRefusedEvenWhenEqualToMain(t *testing.T) {
	t.Parallel()
	same := Blob{Data: []byte("0123456789abcdef0123456789abcdef01234567"), Present: true}
	cases := []change{
		{oldMode: "100644", newMode: gitModeGitlink, status: "T", path: "vendor/sub"},
		{oldMode: gitModeGitlink, newMode: gitModeGitlink, status: "M", path: "vendor/sub"},
		{oldMode: "100644", newMode: "100644", status: "U", path: "conflicted.txt"},
	}
	for _, ch := range cases {
		v, err := judgePath(context.Background(), gitexec.Default(t.TempDir()), ch, pathVersions{base: same, main: same, tree: same, mainMode: ch.newMode})
		if err != nil || v.Landed || !strings.Contains(v.Reason, ch.path) {
			t.Errorf("%+v: judgePath = %+v (err=%v), want refused naming %s: the proof does not judge submodules or unmerged paths", ch, v, err, ch.path)
		}
	}
}
