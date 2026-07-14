package releasepipeline

// classify.go — one-binary S5 release classification. A release is a
// "binary-release" when it changes the compiled artifact (any go/** or
// .goreleaser.yml change since the previous tag) → a NEW macOS fingerprint that
// needs a fresh corporate approval. Otherwise it is a "config-release": the
// binary fingerprint is byte-identical to a prior version, so no new approval is
// needed. The classification is stamped into the GitHub release notes so a
// corporate operator can tell at a glance whether adopting the release requires
// an approval request (see docs/operations/corporate-deployment.md).

import (
	"fmt"
	"os/exec"
	"strings"
)

// ReleaseClass is the fingerprint-impact classification of a release.
type ReleaseClass string

const (
	// BinaryRelease changed the compiled binary → a new fingerprint, approval needed.
	BinaryRelease ReleaseClass = "binary-release"
	// ConfigRelease left the binary byte-identical to a prior version → no approval.
	ConfigRelease ReleaseClass = "config-release"
)

// ReleaseClassification is the result of classifying a release.
type ReleaseClassification struct {
	Class ReleaseClass
	// SinceVersion, for a config-release, is the prior version whose binary
	// fingerprint this release shares (the last binary-release). It is the target
	// version for a binary-release (it IS the new fingerprint).
	SinceVersion string
}

// goChangedProbe reports whether any fingerprint-affecting path (go/** or
// .goreleaser.yml) changed between two git refs. Injected so classifyRelease is
// unit-testable without a real repo.
type goChangedProbe func(fromRef, toRef string) (bool, error)

// classifyRelease decides whether the release at HEAD (version target) is a
// binary- or config-release relative to prevTag, walking older tags to find the
// version a config-release's fingerprint traces back to. Pure: all git access is
// behind `changed`.
func classifyRelease(target, prevTag string, olderTags []string, changed goChangedProbe) (ReleaseClassification, error) {
	if prevTag == "" {
		// No prior tag — the first release always introduces the fingerprint.
		return ReleaseClassification{Class: BinaryRelease, SinceVersion: target}, nil
	}
	ch, err := changed(prevTag, "HEAD")
	if err != nil {
		return ReleaseClassification{}, err
	}
	if ch {
		return ReleaseClassification{Class: BinaryRelease, SinceVersion: target}, nil
	}
	// config-release: this fingerprint == prevTag's. Walk back through the tag
	// chain to the newest tag that actually changed the binary (the last
	// binary-release) — the version this fingerprint traces to.
	tags := append([]string{prevTag}, olderTags...)
	for i := 0; i+1 < len(tags); i++ {
		ch, err := changed(tags[i+1], tags[i])
		if err != nil {
			return ReleaseClassification{}, err
		}
		if ch {
			return ReleaseClassification{Class: ConfigRelease, SinceVersion: tags[i]}, nil
		}
	}
	// Every known tag left the binary unchanged: trace to the earliest one.
	return ReleaseClassification{Class: ConfigRelease, SinceVersion: tags[len(tags)-1]}, nil
}

// releaseClassBanner classifies the target release and renders the markdown
// banner prepended to the GitHub release notes. It always returns a non-empty
// banner so the operator's "read the first line" rule is always actionable; on a
// git error it returns an "unavailable" banner that FAILS CLOSED — treat as a
// binary-release (assume a new fingerprint needing approval) — plus the error,
// so the release pipeline can log it rather than silently drop the classification.
func releaseClassBanner(repoRoot, target, prevTag string) (string, error) {
	probe := func(fromRef, toRef string) (bool, error) {
		return gitPathsChanged(repoRoot, fromRef, toRef, "go", ".goreleaser.yml")
	}
	res, err := classifyRelease(target, prevTag, gitOlderTags(repoRoot, prevTag), probe)
	if err != nil {
		return bannerUnavailable, err
	}
	return bannerFor(res.Class, res.SinceVersion), nil
}

// bannerUnavailable is the fail-closed banner emitted when classification could
// not run: it steers the operator to the safe default (assume a new fingerprint).
const bannerUnavailable = "**Release class: unavailable** — could not determine the fingerprint impact " +
	"(git classification failed); treat this as a binary-release — assume a NEW fingerprint that needs a " +
	"corporate approval — and verify the artifact against checksums.txt manually."

// bannerFor renders the operator-facing release-class banner. Pure text (no
// git), so the approval-relevant wording is unit-testable.
func bannerFor(class ReleaseClass, sinceVersion string) string {
	if class == ConfigRelease {
		return fmt.Sprintf("**Release class: config-release** — the binary fingerprint is unchanged "+
			"since v%s (no go/ or .goreleaser.yml changes), so no new corporate approval is required to "+
			"adopt this release.", strings.TrimPrefix(sinceVersion, "v"))
	}
	return "**Release class: binary-release** — this release changes the compiled binary, so it has a " +
		"NEW macOS fingerprint that needs a corporate approval request (see the Fingerprints section below)."
}

// gitPathsChanged reports whether any of paths changed between fromRef and toRef.
func gitPathsChanged(repoRoot, fromRef, toRef string, paths ...string) (bool, error) {
	args := append([]string{"-C", repoRoot, "diff", "--name-only", fromRef + ".." + toRef, "--"}, paths...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return false, fmt.Errorf("git diff %s..%s: %w", fromRef, toRef, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// gitOlderTags lists version tags strictly older than prevTag, newest first, so
// classifyRelease can walk the fingerprint chain back. Best-effort: returns nil
// on error (classifyRelease then traces to prevTag).
func gitOlderTags(repoRoot, prevTag string) []string {
	out, err := exec.Command("git", "-C", repoRoot, "tag", "-l", "v*", "--sort=-version:refname").Output()
	if err != nil {
		return nil
	}
	var older []string
	seenPrev := false
	for _, t := range strings.Fields(string(out)) {
		if t == prevTag {
			seenPrev = true
			continue
		}
		if seenPrev {
			older = append(older, t)
		}
	}
	return older
}
