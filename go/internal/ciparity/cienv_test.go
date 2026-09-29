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
