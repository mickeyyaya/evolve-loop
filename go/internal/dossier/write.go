package dossier

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolveloop/go/internal/atomicwrite"
)

// Write persists d to dir as cycle-N.json and cycle-N.md using atomic
// temp+rename via atomicwrite.Bytes. The commit parameter is reserved for a
// future slice that will git-commit the two files; pass false for now.
func Write(d *Dossier, dir string, commit bool) error {
	if dir == "" {
		return fmt.Errorf("dossier: Write: dir must not be blank")
	}
	base := fmt.Sprintf("cycle-%d", d.Cycle)

	jsonBytes, err := RenderJSON(d)
	if err != nil {
		return fmt.Errorf("dossier: Write: %w", err)
	}
	if err := atomicwrite.Bytes(filepath.Join(dir, base+".json"), jsonBytes); err != nil {
		return fmt.Errorf("dossier: Write JSON: %w", err)
	}

	mdBytes, err := RenderMarkdown(d)
	if err != nil {
		return fmt.Errorf("dossier: Write: %w", err)
	}
	if err := atomicwrite.Bytes(filepath.Join(dir, base+".md"), mdBytes); err != nil {
		return fmt.Errorf("dossier: Write markdown: %w", err)
	}
	return nil
}
