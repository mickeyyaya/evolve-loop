package committedset

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const HandoffFile = "no-work-handoff.json"

type Handoff struct {
	Cycle  int      `json:"cycle"`
	Routed []string `json:"routed"`
}

func WriteHandoff(workspace string, h Handoff) error {
	return atomicwrite.JSON(filepath.Join(workspace, HandoffFile), h)
}

func HandedOff(workspace string) bool {
	var h Handoff
	return readJSON(workspace, HandoffFile, &h) && len(h.Routed) > 0
}
