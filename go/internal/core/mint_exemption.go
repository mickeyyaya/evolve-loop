package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

var mintNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

const maxMintSpecBytes = 1 << 20

func isActiveMintPhasePath(mints map[string]bool, p string) bool {
	const pfx = ".evolve/phases/"
	if len(mints) == 0 || !strings.HasPrefix(p, pfx) {
		return false
	}
	rest := strings.TrimPrefix(p, pfx)
	name, sub, nested := strings.Cut(rest, "/")
	if nested && sub != "phase.json" {
		return false
	}
	return name != "" && mints[name]
}

func verifiedActiveMints(projectRoot string, mints map[string]bool) map[string]bool {
	verified := make(map[string]bool, len(mints))
	for name := range mints {
		if !mintNameRE.MatchString(name) {
			continue
		}
		specPath := filepath.Join(projectRoot, ".evolve", "phases", name, "phase.json")
		fi, err := os.Lstat(specPath)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxMintSpecBytes {
			continue
		}
		raw, err := os.ReadFile(specPath)
		if err != nil {
			continue
		}
		var spec phasespec.PhaseSpec
		if json.Unmarshal(raw, &spec) != nil {
			continue
		}
		if spec.Name != name || !spec.Optional || len(phasespec.ValidateUserSpec(spec)) > 0 {
			continue
		}
		verified[name] = true
	}
	return verified
}
