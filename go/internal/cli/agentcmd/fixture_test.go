package agentcmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const passReport = "## Threat Model\nA diff that adds one function.\n## Findings\nNone.\n## Verdict\nPASS\n"

type fakeBridge struct {
	artifact string
	err      error
	cost     float64
	prompts  []string
	requests []core.BridgeRequest
	onLaunch func(core.BridgeRequest)
}

func (b *fakeBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.prompts = append(b.prompts, req.Prompt)
	b.requests = append(b.requests, req)
	if b.artifact != "" {
		if err := os.WriteFile(req.ArtifactPath, []byte(b.artifact), 0o644); err != nil {
			return core.BridgeResponse{}, err
		}
	}
	if b.onLaunch != nil {
		b.onLaunch(req)
	}
	resp := core.BridgeResponse{CostUSD: b.cost, Tokens: core.TokenUsage{Input: 1200, Output: 300}}
	if b.err != nil {
		resp.ExitCode = 1
	}
	return resp, b.err
}

func (b *fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

var errBridgeDown = errors.New("bridge down")

var fixtureFiles = []string{
	"docs/architecture/phase-registry.json",
	".evolve/phases/adversarial-review/phase.json",
	".evolve/profiles/adversarial-review.json",
	"agents/evolve-adversarial-review.md",
	".evolve/phases/test-amplification/phase.json",
	".evolve/profiles/test-amplification.json",
}

func project(t *testing.T) string {
	t.Helper()
	repo := filepath.Join("..", "..", "..", "..")
	root := t.TempDir()
	for _, rel := range fixtureFiles {
		raw, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			out[rel+"/"] = "dir"
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func changedPaths(before, after map[string]string) []string {
	var changed []string
	for path, sum := range after {
		if before[path] != sum {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	return changed
}

func fixedNow() time.Time { return time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC) }

func noEnv(string) string { return "" }

func depsWith(b *fakeBridge) Deps {
	return Deps{Bridge: func(string) core.Bridge { return b }, Getenv: noEnv, Now: fixedNow}
}

func runArgs(root string, extra ...string) []string {
	return append([]string{"run", "adversarial-review", "--project-root", root}, extra...)
}

func isLedgerFile(path string) bool {
	return strings.HasPrefix(path, filepath.Join(".evolve", "ledger"))
}
