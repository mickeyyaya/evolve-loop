package ciparitygate

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestTierLogName_IsAToolOutputFileThatACyclePassDeletes(t *testing.T) {
	name := filepath.Base((&tierLog{workspace: "/ws"}).target())
	if !slices.Contains(gcpolicy.ToolOutputFiles(), name) {
		t.Errorf("gcpolicy.ToolOutputFiles() = %v does not name %q, so a PASS seal keeps the tier log", gcpolicy.ToolOutputFiles(), name)
	}
}
