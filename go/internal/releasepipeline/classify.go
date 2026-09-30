package releasepipeline

import (
	"fmt"
	"os/exec"
	"strings"
)

type ReleaseClass string

const (
	BinaryRelease ReleaseClass = "binary-release"
	ConfigRelease ReleaseClass = "config-release"
)

type ReleaseClassification struct {
	Class        ReleaseClass
	SinceVersion string
}

type goChangedProbe func(fromRef, toRef string) (bool, error)

func classifyRelease(target, prevTag string, olderTags []string, changed goChangedProbe) (ReleaseClassification, error) {
	isFirstRelease := prevTag == ""
	if isFirstRelease {
		return ReleaseClassification{Class: BinaryRelease, SinceVersion: target}, nil
	}
	binaryChanged, err := changed(prevTag, "HEAD")
	if err != nil {
		return ReleaseClassification{}, err
	}
	if binaryChanged {
		return ReleaseClassification{Class: BinaryRelease, SinceVersion: target}, nil
	}
	return configReleaseSinceLastBinaryRelease(append([]string{prevTag}, olderTags...), changed)
}

func configReleaseSinceLastBinaryRelease(newestFirst []string, changed goChangedProbe) (ReleaseClassification, error) {
	for i := 0; i+1 < len(newestFirst); i++ {
		newer, older := newestFirst[i], newestFirst[i+1]
		binaryChanged, err := changed(older, newer)
		if err != nil {
			return ReleaseClassification{}, err
		}
		if binaryChanged {
			return ReleaseClassification{Class: ConfigRelease, SinceVersion: newer}, nil
		}
	}
	earliest := newestFirst[len(newestFirst)-1]
	return ReleaseClassification{Class: ConfigRelease, SinceVersion: earliest}, nil
}

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

const bannerUnavailable = "**Release class: unavailable** — could not determine the fingerprint impact " +
	"(git classification failed); treat this as a binary-release — assume a NEW fingerprint that needs a " +
	"corporate approval — and verify the artifact against checksums.txt manually."

func bannerFor(class ReleaseClass, sinceVersion string) string {
	if class == ConfigRelease {
		return fmt.Sprintf("**Release class: config-release** — the binary fingerprint is unchanged "+
			"since v%s (no go/ or .goreleaser.yml changes), so no new corporate approval is required to "+
			"adopt this release.", strings.TrimPrefix(sinceVersion, "v"))
	}
	return "**Release class: binary-release** — this release changes the compiled binary, so it has a " +
		"NEW macOS fingerprint that needs a corporate approval request (see the Fingerprints section below)."
}

func gitPathsChanged(repoRoot, fromRef, toRef string, paths ...string) (bool, error) {
	args := append([]string{"-C", repoRoot, "diff", "--name-only", fromRef + ".." + toRef, "--"}, paths...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return false, fmt.Errorf("git diff %s..%s: %w", fromRef, toRef, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

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
