// Package acsverdict names the acs-verdict.json artifact and the harness-red ids both of its writers use.
package acsverdict

import (
	"fmt"
	"path/filepath"
)

const Filename = "acs-verdict.json"

const SyntheticRedPrefix = "egps/"

const NoPredicatesID = SyntheticRedPrefix + "no-predicates"

func Path(evolveDir string, cycle int) (string, error) {
	if !filepath.IsAbs(evolveDir) {
		return "", fmt.Errorf("acsverdict: evolve dir %q is not absolute; resolve it against the project root before writing the verdict", evolveDir)
	}
	return filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle), Filename), nil
}
