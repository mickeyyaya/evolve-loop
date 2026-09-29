package ciparity

import (
	"reflect"
	"testing"
)

func TestCIEnv_KeepsOnlyTheAllowlistInItsOrder(t *testing.T) {
	t.Parallel()
	environ := []string{"GOFLAGS=-mod=mod", "EVOLVE_WORKTREE_ROOT=/lane/wt", "PATH=/bin", "BRIDGE_SESSION=pane", "HOME=/home/x", "NOEQUALS"}

	got := CIEnv(environ)

	if want := []string{"PATH=/bin", "HOME=/home/x", "GOFLAGS=-mod=mod"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CIEnv = %v, want %v: the allowlist in its order, never the lane's runtime state", got, want)
	}
}

func TestCIEnv_PassesEveryAllowlistedKeyThrough(t *testing.T) {
	t.Parallel()
	var environ, want []string
	for _, k := range CIEnvAllowlist {
		environ = append(environ, k+"=v-"+k)
		want = append(want, k+"=v-"+k)
	}

	if got := CIEnv(append(environ, "EVOLVE_FLEET=1")); !reflect.DeepEqual(got, want) {
		t.Errorf("CIEnv = %v, want every allowlisted key in order and nothing else", got)
	}
}
