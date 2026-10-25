package looppreflight

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

var versionCaptureTimeout = 5 * time.Second

var execVersion = func(bin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionCaptureTimeout)
	defer cancel()
	var out strings.Builder
	code, err := bridge.LaunchLLM(ctx, bridge.Deps{}, bridge.LLMExec{Purpose: bridge.PurposeInventory, Binary: bin, Args: []string{"--version"}, Stdout: &out, Stderr: io.Discard})
	if err == nil && code != 0 {
		err = fmt.Errorf("exit %d", code)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

var versionTokenRE = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

func CLIVersion(bin string) (string, error) {
	raw, err := execVersion(bin)
	if err != nil {
		return "", fmt.Errorf("%s --version: %w", bin, err)
	}
	token := versionTokenRE.FindString(raw)
	if token == "" {
		return "", fmt.Errorf("%s --version printed no version token: %q", bin, raw)
	}
	return token, nil
}

func captureVersionInventory(bins []string) map[string]string {
	inv := make(map[string]string, len(bins))
	for _, b := range bins {
		if v, err := CLIVersion(b); err == nil {
			inv[b] = v
		}
	}
	return inv
}

func loadVersionCache(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func saveVersionCache(path string, versions map[string]string) error {
	return atomicwrite.JSON(path, versions)
}
