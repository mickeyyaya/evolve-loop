package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasetargets"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	releasePromoteUsage   = "usage: evolve release-promote <tag> [--rerun]"
	releasePromoteTimeout = 30 * time.Minute
	releaseWorkflowFile   = "release.yml"
	runFields             = "databaseId,status,conclusion,workflowName,headBranch,name"
)

type workflowRun struct {
	ID         int64  `json:"databaseId"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

func runReleasePromote(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	tag, rerun, ok := parseReleasePromoteArgs(args)
	if !ok {
		fmt.Fprintf(stderr, "release-promote: bad arguments\n%s\n", releasePromoteUsage)
		return 2
	}
	goreleaserPath := filepath.Join(sourceRoot(), ".goreleaser.yml")
	cfg, err := releasetargets.ParseConfig(goreleaserPath)
	if err != nil || cfg.RepoOwner == "" || cfg.RepoName == "" {
		fmt.Fprintf(stderr, "release-promote: release matrix %s unusable: %v\n", goreleaserPath, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), releasePromoteTimeout)
	defer cancel()
	return promoteRelease(ctx, sysexec.DefaultRunner, cfg, tag, rerun, stdout, stderr)
}

func parseReleasePromoteArgs(args []string) (tag string, rerun, ok bool) {
	for _, a := range args {
		switch {
		case a == "--rerun":
			rerun = true
		case a == "" || a[0] == '-' || tag != "":
			return "", false, false
		default:
			tag = a
		}
	}
	return tag, rerun, tag != ""
}

type ghFunc func(args ...string) (string, error)

func ghJSON(gh ghFunc, v any, args ...string) error {
	out, err := gh(args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(out), v); err != nil {
		return fmt.Errorf("decode gh %s: %w", args[0], err)
	}
	return nil
}

func promoteRelease(ctx context.Context, run sysexec.RunFunc, cfg releasetargets.Config, tag string, rerun bool, stdout, stderr io.Writer) int {
	repo := cfg.RepoOwner + "/" + cfg.RepoName
	gh := func(args ...string) (string, error) {
		if args[0] == "run" {
			args = append(args[:len(args):len(args)], "-R", repo)
		}
		return sysexec.Output(ctx, run, "", "gh", args...)
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "release-promote: %v\n", err)
		return 2
	}

	var runs []workflowRun
	if err := ghJSON(gh, &runs, "run", "list", "--workflow", releaseWorkflowFile, "--branch", tag, "--limit", "1", "--json", runFields); err != nil {
		return fail(err)
	}
	if len(runs) == 0 {
		return fail(fmt.Errorf("no %s workflow run found for %s", releaseWorkflowFile, tag))
	}
	id := fmt.Sprint(runs[0].ID)

	if rerun {
		for _, step := range [][]string{{"run", "rerun", id, "--failed"}, {"run", "watch", id}} {
			if _, err := gh(step...); err != nil {
				return fail(err)
			}
		}
	}

	var current workflowRun
	if err := ghJSON(gh, &current, "run", "view", id, "--json", runFields); err != nil {
		return fail(err)
	}
	if current.Conclusion != "success" {
		fmt.Fprintf(stderr, "release-promote: refusing: workflow run %s status=%q conclusion=%q is not success; release %s left as prerelease\n", id, current.Status, current.Conclusion, tag)
		return 1
	}

	relID, code := verifiedReleaseID(gh, cfg, tag, stdout, stderr)
	if code != 0 {
		return code
	}

	if _, err := gh("api", "-X", "PATCH", fmt.Sprintf("repos/%s/releases/%d", repo, relID), "-F", "prerelease=false"); err != nil {
		return fail(err)
	}
	fmt.Fprintf(stdout, "release-promote: %s promoted (prerelease cleared)\n", tag)
	return 0
}

func verifiedReleaseID(gh ghFunc, cfg releasetargets.Config, tag string, stdout, stderr io.Writer) (int64, int) {
	var rel struct {
		ID     int64 `json:"id"`
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := ghJSON(gh, &rel, "api", "repos/"+cfg.RepoOwner+"/"+cfg.RepoName+"/releases/tags/"+tag); err != nil {
		fmt.Fprintf(stderr, "release-promote: %v\n", err)
		return 0, 2
	}
	names := make([]string, 0, len(rel.Assets))
	for _, a := range rel.Assets {
		names = append(names, a.Name)
	}
	if reportBinaryVerification(cfg, tag, func(_, _, _ string) ([]string, error) { return names, nil }, stdout, stderr) != 0 {
		fmt.Fprintf(stderr, "release-promote: refusing: release %s left as prerelease\n", tag)
		return 0, 1
	}
	return rel.ID, 0
}
