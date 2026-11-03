package gittest

import (
	"fmt"
	"os"
)

var (
	mkdirTemp = os.MkdirTemp
	removeAll = os.RemoveAll
)

type envBackup struct {
	key string
	val string
	set bool
}

func IsolateHostCredentials() (func() error, error) {
	dir, err := mkdirTemp("", "evghconfig")
	if err != nil {
		return nil, fmt.Errorf("isolate host credentials: %w", err)
	}

	keys := []string{
		"GH_CONFIG_DIR",
		"GIT_CONFIG_GLOBAL",
		"GIT_CONFIG_NOSYSTEM",
		"GIT_TERMINAL_PROMPT",
		"GIT_SSH_COMMAND",
		"GH_TOKEN",
		"GITHUB_TOKEN",
	}

	backups := make([]envBackup, len(keys))
	for i, key := range keys {
		val, set := os.LookupEnv(key)
		backups[i] = envBackup{key: key, val: val, set: set}
	}

	os.Setenv("GH_CONFIG_DIR", dir)
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	os.Setenv("GIT_SSH_COMMAND", "ssh -o BatchMode=yes")
	os.Unsetenv("GH_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")

	restore := func() error {
		for _, b := range backups {
			if b.set {
				os.Setenv(b.key, b.val)
			} else {
				os.Unsetenv(b.key)
			}
		}
		if rmErr := removeAll(dir); rmErr != nil {
			return fmt.Errorf("isolate host credentials restore: %w", rmErr)
		}
		return nil
	}

	return restore, nil
}
