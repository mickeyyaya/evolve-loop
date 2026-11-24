package bridge

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

func TestGitSSHCommand_SshResolvesNoAgentAndOnlyTheNullIdentity(t *testing.T) {
	t.Parallel()
	testhost.Tool(t, "ssh")
	parts := strings.Fields(ipcenv.GitSSHCommand)
	args := append(slices.Clone(parts[1:]), "-G", "cell.example.test")
	cmd := exec.Command(parts[0], args...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "SSH_AUTH_SOCK=/host/agent.sock", "HOME=" + t.TempDir()}
	out, _ := cmd.Output()
	got := map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		k, v, _ := strings.Cut(line, " ")
		got[k] = append(got[k], v)
	}
	want := map[string][]string{"batchmode": {"yes"}, "identitiesonly": {"yes"}, "identityagent": {"none"}, "identityfile": {"/dev/null"}}
	for k, v := range want {
		if !slices.Equal(got[k], v) {
			t.Errorf("ssh -G %s = %q, want %q", k, got[k], v)
		}
	}
}
