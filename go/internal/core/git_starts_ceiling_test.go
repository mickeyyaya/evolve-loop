package core

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitstarts"
)

func TestCoreTestGitStartsStayInsideTheRecordedCount(t *testing.T) {
	t.Parallel()
	if err := gitstarts.Verify(".", filepath.Join("testdata", "gitstarts.json")); err != nil {
		t.Fatal(err)
	}
}
