package defectledger

import (
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const blockerSeverity = "HIGH"

var leadingFindingID = regexp.MustCompile(`^\W*([A-Z])(\d+)(?:[\s:*)\].]|$)`)

var leadingSeverityTag = regexp.MustCompile(`^\W*(?:\[(CRITICAL|HIGH|MEDIUM|LOW)\]|(CRITICAL|HIGH|MEDIUM|LOW):)(?:[\s:*]|$)`)

var severityByIDPrefix = map[string]string{"C": "CRITICAL", "H": "HIGH", "M": "MEDIUM", "L": "LOW"}

func isBlocker(severity string) bool {
	return severity != "" && reportdoc.SeverityRank(severity) <= reportdoc.SeverityRank(blockerSeverity)
}

func blocksDeferral(e Entry) bool {
	return isBlocker(e.Severity) && !strings.Contains(e.Text, overflowClause)
}

func severityOf(defect string, findings []reportdoc.Finding) string {
	if tag := leadingSeverityTag.FindStringSubmatch(defect); tag != nil {
		return tag[1] + tag[2]
	}
	m := leadingFindingID.FindStringSubmatch(defect)
	if m == nil {
		return ""
	}
	id := m[1] + m[2]
	for _, f := range findings {
		if f.ID == id {
			return f.Severity
		}
	}
	return severityByIDPrefix[m[1]]
}
