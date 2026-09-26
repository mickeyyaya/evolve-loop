package policy_test

import (
	"os"
	"path/filepath"
	"testing"
)

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
