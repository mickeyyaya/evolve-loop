package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const landingWitnessGitTimeout = 10 * time.Second

func lostLandingVerdict() string { return VerdictWARN }

type landingEvidence struct {
	Workspace    string
	ProjectRoot  string
	PreCycleHEAD string
	Cycle        int
	Shipped      bool
	FinalVerdict string
}

func detectLostLanding(ev landingEvidence) *SystemFailureSignal {
	if ev.Workspace == "" || !IsShippingVerdict(ev.FinalVerdict) || ev.Shipped {
		return nil
	}
	if bindingWitnessesLanding(ev) {
		return nil
	}
	code, class, msg, ok := hostShipError(ev.ProjectRoot, ev.Cycle)
	if !ok {
		code, class, msg, ok = readShipError(ev.Workspace)
	}
	if !ok {
		return nil
	}
	return &SystemFailureSignal{
		Category: "landing-lost",
		Level:    "system",
		Halt:     false,
		Evidence: fmt.Sprintf(
			"cycle reported %s but ship produced no commit: ship-error %s (%s) with no ship-binding.json that git confirms in %s — the work was completed and then discarded. %s",
			ev.FinalVerdict, code, class, ev.Workspace, msg),
	}
}

func bindingWitnessesLanding(ev landingEvidence) bool {
	if ev.ProjectRoot == "" {
		return false
	}
	body, err := os.ReadFile(filepath.Join(ev.Workspace, dossier.ShipBindingFile))
	if err != nil {
		return false
	}
	var binding dossier.ShipBinding
	if json.Unmarshal(body, &binding) != nil || binding.Cycle != ev.Cycle {
		return false
	}
	commit := strings.TrimSpace(binding.CommitSHA)
	claimedTree := strings.TrimSpace(binding.TreeSHACommitted)
	if claimedTree == "" {
		claimedTree = strings.TrimSpace(binding.AuditBoundTreeSHA)
	}
	if !objectIDRe.MatchString(commit) || claimedTree == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), landingWitnessGitTimeout)
	defer cancel()
	committedTree := commit + "^{commit}^{tree}"
	if tree, err := gitStdout(ctx, gitCapture, ev.ProjectRoot, "rev-parse", "--verify", "--quiet", committedTree); err != nil || tree != claimedTree {
		return false
	}
	onHead, headErr := isAncestor(ctx, ev.ProjectRoot, commit, "HEAD")
	beforeCycle, preErr := isAncestor(ctx, ev.ProjectRoot, commit, ev.PreCycleHEAD)
	return headErr == nil && preErr == nil && onHead && !beforeCycle
}

func isAncestor(ctx context.Context, dir, commit, ref string) (bool, error) {
	_, code, err := gitCapture(ctx, dir, "merge-base", "--is-ancestor", commit, ref)
	if err != nil || code > 1 {
		return false, fmt.Errorf("git merge-base --is-ancestor %s %s: exit %d: %v", commit, ref, code, err)
	}
	return code == 0, nil
}

func readShipError(workspace string) (code, class, msg string, ok bool) {
	b, err := os.ReadFile(filepath.Join(workspace, "ship-error.json"))
	if err != nil {
		return "", "", "", false
	}
	var se struct {
		Code    string `json:"code"`
		Class   string `json:"class"`
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &se) != nil || se.Code == "" {
		return "", "", "", false
	}
	return se.Code, se.Class, se.Message, true
}
