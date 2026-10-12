package gotooltest

import (
	"fmt"
	"os"
	"strings"
)

func LimitProcess() error {
	return applyLimits(os.Setenv, os.Environ())
}

func applyLimits(setenv func(key, value string) error, environ []string) error {
	for _, entry := range Env(environ) {
		key, value, _ := strings.Cut(entry, "=")
		if key != "GOFLAGS" && key != "GOMAXPROCS" {
			continue
		}
		if err := setenv(key, value); err != nil {
			return fmt.Errorf("gotooltest: limit %s for the nested go runs: %w", key, err)
		}
	}
	return nil
}
