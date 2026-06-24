// cmd_release_verify_binaries.go implements `evolve release-verify-binaries
// <tag>`: the release-flow gate that proves every prebuilt binary the release is
// supposed to publish is actually present as an asset on the GitHub release.
//
// Why this exists: `evolve release` proves only LOCAL binary consistency, and
// "all binaries published" lived only as prose in skills/publish/SKILL.md, which
// is non-deterministic — v21.1.0 reported success yet published zero assets. This
// command makes that closing check deterministic Go.
//
// Single source of truth: the expected asset set is DERIVED from .goreleaser.yml
// (the only place the build matrix must exist inline). Add a target there and
// this gate automatically requires its archive published — no second list.
//
// Determinism: no live LLM. The one effect — listing a release's assets — is
// injected via releaseAssetLister so the orchestration (coverage, no early
// return) is unit-testable with pure stubs; defaultReleaseAssetLister wires the
// real GitHub REST call used by the release workflow.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasetargets"
)

// binVerify is one row of the binary-asset verification matrix: an expected
// asset name and whether it was found on the release, with a human Detail.
type binVerify struct {
	Asset  string
	OK     bool
	Detail string
}

// releaseAssetLister returns the asset names attached to the GitHub release for
// tag in owner/repo. Injected so the orchestration is unit-testable; an error
// (e.g. release not found) means no binaries were published — a hard failure.
type releaseAssetLister func(owner, repo, tag string) ([]string, error)

// verifyReleaseBinaries checks that every expected asset — one archive per
// goreleaser target plus the checksums file — is present on the release. It
// never returns early: every missing asset is reported so the operator sees the
// complete picture, not just the first gap.
func verifyReleaseBinaries(cfg releasetargets.Config, tag string, list releaseAssetLister) ([]binVerify, error) {
	assets, err := list(cfg.RepoOwner, cfg.RepoName, tag)
	if err != nil {
		return nil, fmt.Errorf("list release assets for %s: %w", tag, err)
	}
	present := make(map[string]bool, len(assets))
	for _, a := range assets {
		present[a] = true
	}

	rows := make([]binVerify, 0, len(cfg.Targets)+1)
	for _, tg := range cfg.Targets {
		name, err := cfg.AssetName(tg)
		if err != nil {
			rows = append(rows, binVerify{Asset: tg.String(), OK: false, Detail: err.Error()})
			continue
		}
		rows = append(rows, assetRow(name, present[name]))
	}
	rows = append(rows, assetRow(cfg.ChecksumsName, present[cfg.ChecksumsName]))
	return rows, nil
}

// assetRow builds one row from whether the named asset was found.
func assetRow(name string, ok bool) binVerify {
	if ok {
		return binVerify{Asset: name, OK: true, Detail: "present in release"}
	}
	return binVerify{Asset: name, OK: false, Detail: "MISSING from release"}
}

// defaultReleaseAssetLister lists a release's assets via the GitHub REST API.
// It uses GITHUB_TOKEN when set (CI / private rate limits) but works
// unauthenticated for the public repo. A 404 is surfaced as "release not found"
// so a tag that published nothing fails the gate loudly.
func defaultReleaseAssetLister(owner, repo, tag string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	// Explicit client-level timeout as a backstop in addition to the request
	// context — a release gate must never hang on a stalled connection.
	client := &http.Client{Timeout: 35 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %s not found in %s/%s (no binaries published)", tag, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API %s returned %s: %s", url, resp.Status, string(body))
	}

	var payload struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode release JSON: %w", err)
	}
	names := make([]string, 0, len(payload.Assets))
	for _, a := range payload.Assets {
		names = append(names, a.Name)
	}
	return names, nil
}

// runReleaseVerifyBinaries is the `evolve release-verify-binaries <tag>` handler:
// it parses the release matrix from .goreleaser.yml, lists the GitHub release's
// assets, prints a table, and exits non-zero if any expected asset is missing.
func runReleaseVerifyBinaries(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintln(stderr, "usage: evolve release-verify-binaries <tag>")
		fmt.Fprintln(stderr, "  <tag> is the release tag to verify, e.g. v21.1.1")
		return 1
	}
	tag := args[0]

	goreleaserPath := filepath.Join(sourceRoot(), ".goreleaser.yml")
	cfg, err := releasetargets.ParseConfig(goreleaserPath)
	if err != nil {
		fmt.Fprintf(stderr, "read release matrix: %v\n", err)
		return 1
	}
	if cfg.RepoOwner == "" || cfg.RepoName == "" {
		fmt.Fprintf(stderr, "release matrix %s has no release.github.owner/name\n", goreleaserPath)
		return 1
	}

	return reportBinaryVerification(cfg, tag, defaultReleaseAssetLister, stdout, stderr)
}

// reportBinaryVerification runs the matrix with the given lister, prints the
// result table, and returns the process exit code (0 = all assets present).
// Split from the handler so the success/partial-failure/lister-error paths are
// unit-testable with a stubbed lister, no filesystem or network.
func reportBinaryVerification(cfg releasetargets.Config, tag string, list releaseAssetLister, stdout, stderr io.Writer) int {
	rows, err := verifyReleaseBinaries(cfg, tag, list)
	if err != nil {
		fmt.Fprintf(stderr, "release-verify-binaries: %v\n", err)
		return 1
	}

	allOK := true
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ASSET\tSTATUS\tDETAIL")
	for _, r := range rows {
		status := "OK"
		if !r.OK {
			status = "FAIL"
			allOK = false
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Asset, status, r.Detail)
	}
	_ = tw.Flush()

	if !allOK {
		fmt.Fprintf(stderr, "release-verify-binaries: one or more expected binaries are missing from release %s\n", tag)
		return 1
	}
	fmt.Fprintf(stdout, "release-verify-binaries: all %d prebuilt binaries + checksums present on release %s\n", len(cfg.Targets), tag)
	return 0
}
