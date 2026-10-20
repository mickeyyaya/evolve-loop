package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	shipPrRefusedExit = 10
	shipPrGhFailExit  = 2
	shipPrBaseBranch  = "main"
)

type shipPrRequest struct {
	enabled  bool
	bodyFile string
}

type openPullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

func (r shipPrRequest) refusal(class string, pushOnly bool) string {
	switch {
	case !r.enabled && r.bodyFile != "":
		return "--pr-body-file requires --pr"
	case !r.enabled:
		return ""
	case class != string(ship.ClassManual):
		return "--pr is only valid with --class manual"
	case pushOnly:
		return "--pr cannot be combined with --push-only"
	}
	return ""
}

func shipPrBranch(ctx context.Context, projectRoot string) (string, error) {
	out, err := sysexec.Command(ctx, "git", "-C", projectRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func ghOutput(ctx context.Context, projectRoot string, args ...string) (string, error) {
	cmd := sysexec.Command(ctx, "gh", args...)
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return "", fmt.Errorf("gh %s: %w: %s", args[0]+" "+args[1], err, strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("gh %s: %w", args[0]+" "+args[1], err)
	}
	return strings.TrimSpace(string(out)), nil
}

func findOpenPullRequest(ctx context.Context, projectRoot, branch string) (openPullRequest, bool, error) {
	out, err := ghOutput(ctx, projectRoot, "pr", "list", "--head", branch, "--state", "open", "--json", "number,url")
	if err != nil {
		return openPullRequest{}, false, err
	}
	var open []openPullRequest
	if err := json.Unmarshal([]byte(out), &open); err != nil {
		return openPullRequest{}, false, fmt.Errorf("gh pr list output: %w", err)
	}
	if len(open) == 0 {
		return openPullRequest{}, false, nil
	}
	return open[0], true, nil
}

func pullRequestNumber(url string) string {
	return url[strings.LastIndex(url, "/")+1:]
}

func ensurePullRequest(ctx context.Context, projectRoot, branch, title, bodyFile string, stdout io.Writer) error {
	existing, found, err := findOpenPullRequest(ctx, projectRoot, branch)
	if err != nil {
		return err
	}
	if found {
		fmt.Fprintf(stdout, "PR #%d already open: %s\n", existing.Number, existing.URL)
		return nil
	}
	args := []string{"pr", "create", "--base", shipPrBaseBranch, "--head", branch, "--title", title}
	if bodyFile != "" {
		args = append(args, "--body-file", bodyFile)
	} else {
		args = append(args, "--body", "")
	}
	url, err := ghOutput(ctx, projectRoot, args...)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "PR #%s created: %s\n", pullRequestNumber(url), url)
	return nil
}

func shipPrTitle(message string) string {
	first, _, _ := strings.Cut(message, "\n")
	return strings.TrimSpace(first)
}

func resolveShipPrBranch(pr shipPrRequest, class string, pushOnly bool, projectRoot string, stderr io.Writer) (string, int) {
	if reason := pr.refusal(class, pushOnly); reason != "" {
		fmt.Fprintf(stderr, "evolve ship: %s\n", reason)
		return "", shipPrRefusedExit
	}
	if !pr.enabled {
		return "", 0
	}
	branch, err := shipPrBranch(context.Background(), projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "evolve ship: %v\n", err)
		return "", int(ship.ExitFailure)
	}
	if branch == shipPrBaseBranch {
		fmt.Fprintf(stderr, "evolve ship: --pr needs a feature branch, not %s\n", shipPrBaseBranch)
		return "", shipPrRefusedExit
	}
	return branch, 0
}

func openShipPr(pr shipPrRequest, projectRoot, branch, message string, stdout, stderr io.Writer) int {
	if err := ensurePullRequest(context.Background(), projectRoot, branch, shipPrTitle(message), pr.bodyFile, stdout); err != nil {
		fmt.Fprintf(stderr, "evolve ship: pull request: %v\n", err)
		return shipPrGhFailExit
	}
	return int(ship.ExitOK)
}
