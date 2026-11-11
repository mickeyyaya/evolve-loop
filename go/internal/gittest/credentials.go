package gittest

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
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

	creds := ipcenv.CredentialIsolation(dir, os.DevNull)
	keys := append(slices.Sorted(maps.Keys(creds.Set)), creds.Unset...)

	backups := make([]envBackup, len(keys))
	for i, key := range keys {
		val, set := os.LookupEnv(key)
		backups[i] = envBackup{key: key, val: val, set: set}
	}

	for k, v := range creds.Set {
		_ = os.Setenv(k, v)
	}
	for _, k := range creds.Unset {
		_ = os.Unsetenv(k)
	}

	restore := func() error {
		for _, b := range backups {
			if b.set {
				_ = os.Setenv(b.key, b.val)
			} else {
				_ = os.Unsetenv(b.key)
			}
		}
		if rmErr := removeAll(dir); rmErr != nil {
			return fmt.Errorf("isolate host credentials restore: %w", rmErr)
		}
		return nil
	}

	return restore, nil
}
