package policy_test

// Black-box (package policy_test) helpers shared by the parameter test suite.
// Black-box deliberately restricts these tests to the EXPORTED policy API
// (Load, the *Config accessors, the *Policy types) so the suite documents and
// locks the public contract callers actually use — and never reaches into
// unexported internals or the system environment.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempPolicy writes body to <tempdir>/policy.json and returns its path,
// for feeding to policy.Load. No environment variables are involved — the path
// is an explicit input parameter, mirroring the env-agnostic design goal.
func writeTempPolicy(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	return p
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// derefBool fails the test if p is nil (the accessors guarantee non-nil pointer
// fields; callers such as cmd_fanout_dispatch dereference them directly).
func derefBool(t *testing.T, name string, p *bool) bool {
	t.Helper()
	if p == nil {
		t.Fatalf("%s pointer is nil — accessor must never return a nil pointer field", name)
	}
	return *p
}

func derefInt(t *testing.T, name string, p *int) int {
	t.Helper()
	if p == nil {
		t.Fatalf("%s pointer is nil — accessor must never return a nil pointer field", name)
	}
	return *p
}
