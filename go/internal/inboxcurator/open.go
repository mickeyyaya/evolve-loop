package inboxcurator

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func OpenItems(inboxDir string) ([]inboxbatch.Item, []inboxbatch.LoadWarning, error) {
	var items []inboxbatch.Item
	var warnings []inboxbatch.LoadWarning
	for _, dir := range append([]string{inboxDir}, inboxbatch.ProcessingCycleDirs(inboxDir)...) {
		scan, err := inboxbatch.ScanDir(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("inboxcurator: read the open items in %s: %w", dir, err)
		}
		items = append(items, scan.Items...)
		warnings = append(warnings, scan.Warnings...)
	}
	return items, warnings, nil
}
