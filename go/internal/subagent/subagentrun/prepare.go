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

// prepare is steps 8-11: the artifact path (workers override the profile
// template) and its directory, the challenge token (the parent-dictated
// override or a fresh mint), the git state the ledger stamps, the prompt
// read and its composition. Each failure is ONE BRIDGE_SUBAGENT_PREPARE_FAILED
// naming its step; the git fallback is BRIDGE_SUBAGENT_GIT_STATE_UNKNOWN.
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

// artifactPathFor is step 8's placement rule: a worker writes under
// <workspace>/workers/<agent>.md; everything else expands the profile's
// template (an empty template yields "" and the run proceeds — quirk Q1).
func artifactPathFor(req Request, id identity, template string) string {
	if id.worker != "" {
		return filepath.Join(req.WorkspacePath, "workers", req.Agent+".md")
	}
	return ResolveArtifactPath(template, req.Cycle, req.ProjectRoot)
}

// token is step 9's first half: the parent-dictated override verbatim, else
// a fresh mint from the dispatcher's entropy.
func (d *Dispatcher) token(req Request) (string, error) {
	if req.ChallengeTokenOverride != "" {
		return req.ChallengeTokenOverride, nil
	}
	return MintToken(d.rand)
}

// gitState is step 9's second half: the port's head and tree-diff sha with
// every empty value stamped "unknown" (the chain-hashed line is unchanged).
// An error or a substitution is ONE BRIDGE_SUBAGENT_GIT_STATE_UNKNOWN naming
// what was unavailable — the silent discard before unit 16.
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

// MintToken returns 16 lowercase hex chars (ChallengeTokenBytes random bytes
// encoded) from rng; a short read is an error. The dispatcher's default rng
// is crypto/rand — a nil rng is a programming error.
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

// ResolveArtifactPath expands {cycle} in the profile's output_artifact
// template and returns an absolute path under projectRoot. Returns "" when
// the template is empty (profile has no defined artifact).
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
