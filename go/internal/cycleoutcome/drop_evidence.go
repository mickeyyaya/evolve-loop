package cycleoutcome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const codeStaleDropRepeated signalcenter.Code = "INBOX_STALE_DROP_REPEATED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, codeStaleDropRepeated, "the same inbox item was stale-dropped a second time after a console re-queue; the item stays console-manual and no third lane is dispatched; fields.task_id, stale_drop_count")
}

func stampDisposition(in NoWorkInputs, d committedset.Disposition) error {
	return errors.Join(stampProtectedPath(in, d), stampDropVerification(in, d))
}

func stampDropVerification(in NoWorkInputs, d committedset.Disposition) error {
	if d.Bucket != "dropped" {
		return nil
	}
	tag, token := inboxmover.DropReasonToken(d.Reason)
	if tag != "stale" && tag != "absent" && tag != "invalid" {
		return nil
	}
	loc, err := inboxmover.Locate(filepath.Join(in.ProjectRoot, ".evolve", "inbox"), d.ID)
	if err != nil {
		return nil
	}
	if tag == "stale" {
		verified := verifyStaleEvidence(in.ProjectRoot, token)
		var count int
		if err := inboxmover.UpdateItemJSON(loc.Path, func(item map[string]json.RawMessage) {
			item[inboxbatch.EvidenceVerifiedField] = jsonBool(verified)
			if raw, ok := item[inboxbatch.StaleDropCountField]; ok {
				_ = json.Unmarshal(raw, &count)
			}
			count++
			item[inboxbatch.StaleDropCountField] = json.RawMessage(strconv.Itoa(count))
		}); err != nil {
			return err
		}
		if count >= 2 && in.Signals != nil {
			in.Signals.Emit(signalcenter.Event{
				Cycle:    in.Cycle,
				Module:   signalcenter.ModuleInbox,
				Origin:   "cycleoutcome.ApplyNoWork",
				Kind:     signalcenter.KindInboxWarning,
				Severity: signalcenter.SeverityWarn,
				Code:     codeStaleDropRepeated,
				Reason:   fmt.Sprintf("item %q was stale-dropped %d times; escalating to console-manual", d.ID, count),
				Fields:   map[string]string{"task_id": d.ID, "stale_drop_count": strconv.Itoa(count)},
			})
		}
		return nil
	}
	verified := verifyScoutCitation(in.Workspace, d.ID, d.Reason)
	return inboxmover.UpdateItemJSON(loc.Path, func(item map[string]json.RawMessage) {
		item[inboxbatch.EvidenceVerifiedField] = jsonBool(verified)
	})
}

func jsonBool(v bool) json.RawMessage {
	if v {
		return json.RawMessage("true")
	}
	return json.RawMessage("false")
}

var lineSuffixRE = regexp.MustCompile(`:\d+(?:-\d+)?$`)

func verifyStaleEvidence(projectRoot, token string) bool {
	if projectRoot == "" || token == "" {
		return false
	}
	if isHexSHA(token) {
		_, _, code, err := gitexec.Default(projectRoot).Capture(context.Background(), "merge-base", "--is-ancestor", token, "HEAD")
		return err == nil && code == 0
	}
	p := strings.TrimSuffix(lineSuffixRE.ReplaceAllString(token, ""), "/")
	if !looksLikeRepoPath(p) {
		return false
	}
	_, _, code, err := gitexec.Default(projectRoot).Capture(context.Background(), "cat-file", "-e", "HEAD:"+p)
	return err == nil && code == 0
}

func isHexSHA(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func looksLikeRepoPath(p string) bool {
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") {
		return false
	}
	return strings.Contains(p, "/") || strings.Contains(p, ".")
}

type lineRange struct{ start, end int }

var scoutCitationRE = regexp.MustCompile(`scout-report\.md:(\d+(?:-\d+)?(?:,\d+(?:-\d+)?)*)`)

func verifyScoutCitation(workspace, id, reason string) bool {
	if workspace == "" || id == "" {
		return false
	}
	body, err := os.ReadFile(filepath.Join(workspace, "scout-report.md"))
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	occ := idOccurrenceLines(string(body), id)
	if len(occ) == 0 {
		return true
	}
	ranges := parseScoutCitations(reason)
	for _, line := range occ {
		if !lineInRanges(line, ranges) {
			return false
		}
	}
	return true
}

func idOccurrenceLines(text, id string) []int {
	re := regexp.MustCompile(`(^|[^a-z0-9])` + regexp.QuoteMeta(id) + `([^a-z0-9-]|$)`)
	var lines []int
	for i, line := range strings.Split(text, "\n") {
		if re.MatchString(line) {
			lines = append(lines, i+1)
		}
	}
	return lines
}

func parseScoutCitations(reason string) []lineRange {
	var ranges []lineRange
	for _, m := range scoutCitationRE.FindAllStringSubmatch(reason, -1) {
		for _, part := range strings.Split(m[1], ",") {
			if a, b, ok := strings.Cut(part, "-"); ok {
				start, errA := strconv.Atoi(a)
				end, errB := strconv.Atoi(b)
				if errA == nil && errB == nil && start > 0 && end >= start {
					ranges = append(ranges, lineRange{start: start, end: end})
				}
				continue
			}
			if n, err := strconv.Atoi(part); err == nil && n > 0 {
				ranges = append(ranges, lineRange{start: n, end: n})
			}
		}
	}
	return ranges
}

func lineInRanges(line int, ranges []lineRange) bool {
	for _, r := range ranges {
		if line >= r.start && line <= r.end {
			return true
		}
	}
	return false
}
