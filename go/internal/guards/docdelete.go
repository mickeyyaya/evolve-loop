package guards

import (
	"context"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// DocDelete denies rm/mv operations that would remove docs/** or
// knowledge-base/** content. Port of scripts/hooks/doc-deletion-guard.sh.
type DocDelete struct {
	allow bool
}

func NewDocDelete(allow bool) *DocDelete { return &DocDelete{allow: allow} }

func (d *DocDelete) Name() string { return "docdelete" }

// rmDocsRe matches an `rm` invocation (with optional flags) that
// references docs/ or knowledge-base/ as a path component.
var (
	rmDocsRe = regexp.MustCompile(`(?m)\brm\b[^\n]*\b(docs|knowledge-base)/`)
	mvDocsRe = regexp.MustCompile(`(?m)\bmv\b[ \t]+([^\s]+)[ \t]+([^\s]+)`)
)

func (d *DocDelete) Decide(_ context.Context, in core.GuardInput) core.GuardDecision {
	if d.allow {
		return core.GuardDecision{Allow: true}
	}
	if in.ToolName != "Bash" {
		return core.GuardDecision{Allow: true}
	}
	cmd := cmdString(in)
	if cmd == "" {
		return core.GuardDecision{Allow: true}
	}
	if rmDocsRe.MatchString(cmd) {
		return core.GuardDecision{
			Allow:  false,
			Reason: "rm against docs/ or knowledge-base/ is forbidden — archive instead (mv to knowledge-base/research/archived-YYYY-MM-DD/<file>); set workflow.allow_doc_delete=true to bypass",
		}
	}
	for _, m := range mvDocsRe.FindAllStringSubmatch(cmd, -1) {
		src, dst := m[1], m[2]
		if isDocPath(src) && !isArchiveDest(dst) {
			return core.GuardDecision{
				Allow:  false,
				Reason: "mv from docs/ must target knowledge-base/research/archived-YYYY-MM-DD/",
			}
		}
	}
	return core.GuardDecision{Allow: true}
}

var archiveDestRe = regexp.MustCompile(`knowledge-base/research/archived-\d{4}-\d{2}-\d{2}/`)

func isDocPath(p string) bool {
	return regexp.MustCompile(`(^|/)(docs|knowledge-base)/`).MatchString(p)
}

func isArchiveDest(p string) bool {
	return archiveDestRe.MatchString(p)
}
