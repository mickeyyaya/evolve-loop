package incidentkb

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

func RenderRecord(root, id string) error {
	if !ValidID(id) {
		return fmt.Errorf("incident id %q must look like yyyy-mm-dd-slug", id)
	}
	path := RecordPath(root, id)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rendered, err := Render(raw)
	if err != nil {
		return err
	}
	return atomicwrite.Durable(path, rendered, 0o644)
}
