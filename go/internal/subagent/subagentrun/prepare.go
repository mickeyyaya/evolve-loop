package subagentrun

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (d *Dispatcher) prepare(ctx context.Context, req Request, id identity, p plan) (provenance, error) {
	prov := provenance{artifactPath: artifactPathFor(req, id, p.profile.OutputArtifact)}
	if err := os.MkdirAll(filepath.Dir(prov.artifactPath), 0o755); err != nil {
		d.warn(id, req.Cycle, CodePrepareFailed, "mkdir artifact dir: "+err.Error(),
			map[string]string{"step": "artifact_dir", "artifact": prov.artifactPath})
		return provenance{}, fmt.Errorf("subagent/run: mkdir artifact dir: %w", err)
	}
	token, err := d.token(req)
	if err != nil {
		d.warn(id, req.Cycle, CodePrepareFailed, "token: "+err.Error(),
			map[string]string{"step": "token", "artifact": prov.artifactPath})
		return provenance{}, fmt.Errorf("subagent/run: token: %w", err)
	}
	prov.token = token
	prov.gitHead, prov.treeDiff = d.gitState(ctx, req, id)
	body, err := io.ReadAll(req.Prompt)
	if err != nil {
		d.warn(id, req.Cycle, CodePrepareFailed, "read prompt: "+err.Error(),
			map[string]string{"step": "prompt", "artifact": prov.artifactPath})
		return provenance{}, fmt.Errorf("subagent/run: read prompt: %w", err)
	}
	prov.prompt = composePrompt(req.Agent, req.Cycle, req.WorkspacePath, prov.artifactPath, token, filepath.Base(p.profilePath), string(body))
	if id.role == "auditor" && req.AdversarialAudit {
		prov.prompt += adversarialAuditFraming()
	}
	return prov, nil
}

func artifactPathFor(req Request, id identity, template string) string {
	if id.worker != "" {
		return filepath.Join(req.WorkspacePath, "workers", req.Agent+".md")
	}
	return ResolveArtifactPath(template, req.Cycle, req.ProjectRoot)
}

func (d *Dispatcher) token(req Request) (string, error) {
	if req.ChallengeTokenOverride != "" {
		return req.ChallengeTokenOverride, nil
	}
	return MintToken(d.rand)
}

func (d *Dispatcher) gitState(ctx context.Context, req Request, id identity) (head, tree string) {
	head, tree, err := d.deps.GitState(ctx, req.ProjectRoot)
	var causes []string
	if err != nil {
		causes = append(causes, err.Error())
	}
	if head == "" {
		head, causes = "unknown", append(causes, "empty head")
	}
	if tree == "" {
		tree, causes = "unknown", append(causes, "empty tree diff")
	}
	if len(causes) > 0 {
		d.warn(id, req.Cycle, CodeGitStateUnknown, "git state unavailable ("+strings.Join(causes, "; ")+`); ledger stamps "unknown"`,
			map[string]string{"step": "provenance", "head": head, "tree_state": tree, "project_root": req.ProjectRoot})
	}
	return head, tree
}

func MintToken(rng func([]byte) (int, error)) (string, error) {
	buf := make([]byte, ChallengeTokenBytes)
	n, err := rng(buf)
	if err != nil {
		return "", err
	}
	if n != ChallengeTokenBytes {
		return "", fmt.Errorf("rand returned %d bytes, want %d", n, ChallengeTokenBytes)
	}
	return hex.EncodeToString(buf), nil
}

func ResolveArtifactPath(template string, cycle int, projectRoot string) string {
	if template == "" {
		return ""
	}
	expanded := strings.ReplaceAll(template, "{cycle}", strconv.Itoa(cycle))
	if filepath.IsAbs(expanded) {
		return expanded
	}
	return filepath.Join(projectRoot, expanded)
}
