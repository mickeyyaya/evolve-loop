package looppreflight

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/diskfloor"
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
			Message: fmt.Sprintf("%d MiB free under %s, below the %d MiB floor (policy %s)", free>>20, dir, minFreeBytes>>20, diskfloor.KeyFloorGB),
			Detail:  "free disk space with `evolve gc`, then restart the loop; a full disk fails lanes as code FAILs (ADR-0072 system halt)",
		}
	}
	return CheckResult{Name: name, Level: LevelPass, Message: fmt.Sprintf("%d MiB free (floor %d MiB)", free>>20, minFreeBytes>>20)}
}

func checkDiskSpace(o resolved, minFreeBytes uint64, diskFloorErr error) CheckResult {
	if diskFloorErr != nil {
		return CheckResult{
			Name:    "disk-space",
			Level:   LevelHalt,
			Message: diskFloorErr.Error(),
			Detail:  fmt.Sprintf("set only %s in .evolve/policy.json, then restart the loop", diskfloor.KeyFloorGB),
		}
	}
	if minFreeBytes == 0 {
		minFreeBytes = policy.Policy{}.PreflightConfig().MinFreeBytes()
	}
	return CheckDiskSpace(o.evolveDir, minFreeBytes, o.diskFreeBytes)
}
