package policy

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func GCFor(projectRoot string) gcpolicy.Policy {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil || pol.GC == nil {
		return gcpolicy.Policy{}
	}
	return *pol.GC
}
