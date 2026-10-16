// Package docsfloor mechanizes operating-policy §3 rule 2's documentation
// half: an architecture-labeled change must touch documentation.
// See docs/architecture/packages/internal-docsfloor.md.
package docsfloor

import (
	"fmt"
	"strings"
)

const (
	StatusPass = "PASS"
	StatusWarn = "WARN"
	StatusSkip = "SKIP"
)

type Config struct {
	Stage string
}

type Input struct {
	ArchitectureLabeled bool
	ChangedFiles        []string
}

type Verdict struct {
	Status string
	Reason string
}

var docPrefixes = []string{"docs/"}

var architectureSurfaces = []string{
	"go/internal/core/",
	"go/internal/policy/",
	"go/internal/config/",
	"go/internal/phasecontract/",
	"go/internal/router/",
	"docs/architecture/phase-registry.json",
}

func LabelArchitecture(changedFiles []string) bool {
	for _, f := range changedFiles {
		p := normalizePath(f)
		for _, s := range architectureSurfaces {
			if strings.HasPrefix(p, s) {
				return true
			}
		}
	}
	return false
}

var strictArchitectureSurfaces = append(append([]string{}, architectureSurfaces...),
	"go/internal/guards/",
	"go/internal/ship/",
	"go/internal/fleet/",
	"go/internal/phases/ship/",
)

func IsArchitectureClass(changedFiles []string) bool {
	for _, f := range changedFiles {
		p := normalizePath(f)
		if p == "" || strings.HasSuffix(p, "_test.go") {
			continue
		}
		for _, s := range strictArchitectureSurfaces {
			if strings.HasPrefix(p, s) {
				return true
			}
		}
		if isPackageNamesake(p) {
			return true
		}
		if strings.HasPrefix(p, "phases/") && strings.HasSuffix(p, ".json") {
			return true
		}
	}
	return false
}

var DocsRoots = []string{"docs/architecture/", "docs/operations/runtime-reference.md"}

func HasDocsDelta(changedFiles []string) bool {
	for _, f := range changedFiles {
		p := normalizePath(f)
		for _, root := range DocsRoots {
			if strings.HasPrefix(p, root) {
				return true
			}
		}
	}
	return false
}

func isPackageNamesake(p string) bool {
	const root = "go/internal/"
	if !strings.HasPrefix(p, root) || !strings.HasSuffix(p, ".go") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(p, root), "/")
	if len(parts) < 2 {
		return false
	}
	return parts[len(parts)-1] == parts[len(parts)-2]+".go"
}

func Evaluate(cfg Config, in Input) Verdict {
	stage := strings.ToLower(strings.TrimSpace(cfg.Stage))
	if stage == "off" {
		return Verdict{Status: StatusSkip, Reason: "docs floor stage is off"}
	}
	if !in.ArchitectureLabeled {
		return Verdict{Status: StatusSkip, Reason: "change is not architecture-labeled"}
	}
	if len(in.ChangedFiles) == 0 {
		return Verdict{Status: StatusSkip, Reason: "empty change set — nothing to judge"}
	}
	for _, f := range in.ChangedFiles {
		p := normalizePath(f)
		for _, prefix := range docPrefixes {
			if strings.HasPrefix(p, prefix) {
				return Verdict{
					Status: StatusPass,
					Reason: fmt.Sprintf("architecture change documented by %s", p),
				}
			}
		}
	}
	return Verdict{
		Status: StatusWarn,
		Reason: fmt.Sprintf(
			"architecture-labeled change touches %d file(s) but no %s file — add the ADR or design doc that explains the change (operating-policy §3.2)",
			len(in.ChangedFiles), strings.Join(docPrefixes, "/")),
	}
}

func normalizePath(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return strings.TrimPrefix(p, "/")
}
