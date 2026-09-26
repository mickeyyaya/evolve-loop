package guards

import (
	"context"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// DocDelete denies Bash rm and mv commands that would remove content from docs/ or knowledge-base/.
type DocDelete struct {
	allow bool
}

// NewDocDelete returns a DocDelete guard; allow (workflow.allow_doc_delete) disables it.
func NewDocDelete(allow bool) *DocDelete { return &DocDelete{allow: allow} }

// Name reports "docdelete".
func (d *DocDelete) Name() string { return "docdelete" }

var (
	rmDocsRe = regexp.MustCompile(`(?m)\brm\b[^\n]*\b(docs|knowledge-base)/`)
	mvDocsRe = regexp.MustCompile(`(?m)\bmv\b[ \t]+([^\s]+)[ \t]+([^\s]+)`)
)

// Decide denies an rm that names docs/ or knowledge-base/, and an mv that moves doc content outside docs/.
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
			Reason: "rm against docs/ or knowledge-base/ is forbidden — archive instead (mv to docs/private/research/archived-YYYY-MM-DD/<file>); set workflow.allow_doc_delete=true to bypass",
		}
	}
	for _, m := range mvDocsRe.FindAllStringSubmatch(cmd, -1) {
		src, dst := m[1], m[2]
		if isDocPath(src) && !isDocsDest(dst) {
			return core.GuardDecision{
				Allow:  false,
				Reason: "mv out of docs//knowledge-base must land under docs/ (archive home: docs/private/research/archived-YYYY-MM-DD/) — moving content OUT of the doc root is a deletion",
			}
		}
	}
	return core.GuardDecision{Allow: true}
}

func isDocPath(p string) bool {
	return regexp.MustCompile(`(^|/)(docs|knowledge-base)/`).MatchString(p)
}

// isDocsDest reports whether an mv destination stays under docs/, the single documentation root.
// knowledge-base/ is not a destination: its research/ subtree is retired and cycles/ is runtime state.
func isDocsDest(p string) bool {
	return regexp.MustCompile(`(^|/)docs/`).MatchString(p)
}
