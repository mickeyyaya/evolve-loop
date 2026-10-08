package proctree

import (
	"reflect"
	"testing"
)

func TestParseProcArgs2_KeepsArgumentsAndEnvironmentApart(t *testing.T) {
	t.Parallel()
	raw := []byte("\x03\x00\x00\x00/usr/bin/grep\x00\x00\x00\x00/usr/bin/grep\x00EVOLVE_DISPATCH_ID=r/1/build/p9n1\x00log.txt\x00" +
		"HOME=/Users/x\x00EVOLVE_PROJECT_ROOT=/hub/runtime\x00EVOLVE_TMUX_SOCKET=evolve-bridge-p42\x00\x00\x00junk")

	args, env, err := parseProcArgs2(raw)

	if err != nil {
		t.Fatalf("parseProcArgs2: %v", err)
	}
	if want := []string{"/usr/bin/grep", "EVOLVE_DISPATCH_ID=r/1/build/p9n1", "log.txt"}; !reflect.DeepEqual(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
	want := map[string]string{"EVOLVE_PROJECT_ROOT": "/hub/runtime", "EVOLVE_TMUX_SOCKET": "evolve-bridge-p42"}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("env = %v, want %v (an argument that looks like a tag is not environment; only EVOLVE_ keys are kept)", env, want)
	}
}

func TestParseProcArgs2_AHiddenEnvironmentIsEmpty(t *testing.T) {
	t.Parallel()
	raw := []byte("\x02\x00\x00\x00/usr/bin/tail\x00\x00\x00/usr/bin/tail\x00-F\x00")

	args, env, err := parseProcArgs2(raw)

	if err != nil || !reflect.DeepEqual(args, []string{"/usr/bin/tail", "-F"}) || len(env) != 0 {
		t.Errorf("got args=%q env=%v err=%v, want the two arguments and no environment", args, env, err)
	}
}

func TestParseProcArgs2_RefusesATruncatedBuffer(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{nil, []byte("\x05\x00"), []byte("\x05\x00\x00\x00/bin/x\x00\x00a\x00"), []byte("\x03\x00\x00\x00/bin/x\x00\x00a\x00b")} {
		if _, _, err := parseProcArgs2(raw); err == nil {
			t.Errorf("parseProcArgs2(%q) = nil error, want a refusal: an argument count that the buffer cannot hold", raw)
		}
	}
}

func TestParseProcFiles_SplitsTheLinuxCmdlineAndEnviron(t *testing.T) {
	t.Parallel()
	args, env := parseProcFiles([]byte("node\x00server.js\x00"), []byte("PATH=/bin\x00EVOLVE_DISPATCH_ID=r/2/audit/p1n2\x00EVOLVE_X\x00"))

	if !reflect.DeepEqual(args, []string{"node", "server.js"}) {
		t.Errorf("args = %q", args)
	}
	if want := map[string]string{"EVOLVE_DISPATCH_ID": "r/2/audit/p1n2"}; !reflect.DeepEqual(env, want) {
		t.Errorf("env = %v, want %v (an entry without = is not a variable)", env, want)
	}
}
