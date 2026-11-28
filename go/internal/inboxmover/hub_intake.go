package inboxmover

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeHubFilesUnnamed signalcenter.Code = "INBOX_HUB_FILES_UNNAMED"

var hubDirectoryFamilies = map[string]string{
	"go/cmd/evolve":    "cmd_<verb>.go",
	"go/internal/core": "<feature>.go",
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeHubFilesUnnamed, "an inbox item declared a whole hub directory instead of naming the files it changes; fields.task_id, hub, family, stage=shadow, step=intake")
}

func WarnUnnamedHubFiles(item inboxbatch.Item, signals *signalcenter.Center) []string {
	var matched []string
	seen := map[string]bool{}
	for _, p := range item.DeclaredPaths() {
		clean := path.Clean(strings.TrimSuffix(filepath.ToSlash(p), "/"))
		family, isHub := hubDirectoryFamilies[clean]
		if !isHub || seen[clean] {
			continue
		}
		seen[clean] = true
		matched = append(matched, clean)
		if signals != nil {
			signals.Emit(signalcenter.Event{
				Module:   signalcenter.ModuleInbox,
				Origin:   "inboxmover.WarnUnnamedHubFiles",
				Kind:     signalcenter.KindInboxWarning,
				Severity: signalcenter.SeverityWarn,
				Code:     CodeHubFilesUnnamed,
				Reason:   fmt.Sprintf("item %s declares whole hub directory %s in files; name specific %s files instead", item.ID, clean, family),
				Fields: map[string]string{
					"task_id": item.ID,
					"hub":     clean,
					"family":  family,
					"stage":   "shadow",
					"step":    "intake",
				},
			})
		}
	}
	return matched
}
