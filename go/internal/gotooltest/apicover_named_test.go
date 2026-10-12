package gotooltest

import "testing"

func TestExecRunner_IsARunner(t *testing.T) {
	var run Runner = ExecRunner
	var cache *Cache = NewCache(run)

	if cache == nil {
		t.Fatal("NewCache returned nil")
	}
}
