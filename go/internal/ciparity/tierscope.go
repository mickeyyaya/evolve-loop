package ciparity

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const TierLockWait = 5 * time.Minute

func TierLockPath(projectRoot string) string {
	return filepath.Join(paths.EvolveDirOf(projectRoot), "locks", "integration-tier.lock")
}

func TierScope(changed []string) (scoped, envExclusive []string, wholeModule bool) {
	scoped = make([]string, 0, len(changed))
	for _, p := range changed {
		if p == "./..." {
			return nil, nil, true
		}
		if strings.Contains(p, "/acs/") {
			continue
		}
		if EnvExclusivePkg(p) {
			envExclusive = append(envExclusive, p)
			continue
		}
		scoped = append(scoped, p)
	}
	return scoped, envExclusive, false
}
