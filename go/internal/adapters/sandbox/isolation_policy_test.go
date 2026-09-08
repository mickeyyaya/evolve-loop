package sandbox

import (
	"strings"
	"testing"
)

func TestBwrapWriteDenialsOverrideBroadWriteAllow(t *testing.T) {
	cfg := Config{WritePaths: []string{"/repo"}, DenyPaths: []string{"/repo/evals"}}
	args := strings.Join(BwrapPrefix(cfg), " ")
	allow := strings.Index(args, "--bind /repo /repo")
	deny := strings.Index(args, "--ro-bind /repo/evals /repo/evals")
	if allow < 0 || deny < allow {
		t.Fatalf("write deny must overlay broad allow: %s", args)
	}
}
