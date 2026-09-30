package looppreflight

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func CheckDiskSpace(dir string, minFreeBytes uint64, diskFree func(string) (uint64, error)) CheckResult {
	const name = "disk-space"
	free, err := diskFree(dir)
	if err != nil {
		return CheckResult{Name: name, Level: LevelWarn, Message: "free disk space unmeasurable", Detail: fmt.Sprintf("%s: %v", dir, err)}
	}
	if free < minFreeBytes {
		return CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("%d MiB free under %s, below the %d MiB floor (policy preflight.min_free_gib)", free>>20, dir, minFreeBytes>>20),
			Detail:  "free disk space with `evolve gc`, then restart the loop; a full disk fails lanes as code FAILs (ADR-0072 system halt)",
		}
	}
	return CheckResult{Name: name, Level: LevelPass, Message: fmt.Sprintf("%d MiB free (floor %d MiB)", free>>20, minFreeBytes>>20)}
}

func checkDiskSpace(o resolved, minFreeBytes uint64) CheckResult {
	if minFreeBytes == 0 {
		minFreeBytes = policy.Policy{}.PreflightConfig().MinFreeBytes()
	}
	return CheckDiskSpace(o.evolveDir, minFreeBytes, o.diskFreeBytes)
}
