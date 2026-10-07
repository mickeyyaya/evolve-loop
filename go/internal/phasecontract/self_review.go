package phasecontract

import (
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

var SelfReview = Section{Canonical: "## Self-Review", Accepted: []string{"## Self-Review", "## Self-review"}}

const selfReviewScoresField = "Scores"

var decimalScore = regexp.MustCompile(`[0-9]\.[0-9]`)

func SelfReviewRecorded(report string) bool {
	for _, heading := range SelfReview.Accepted {
		body, found, duplicate := reportdoc.Section(report, strings.TrimPrefix(heading, "## "))
		if found && (duplicate != nil || scoresRecorded(body)) {
			return true
		}
	}
	return false
}

func scoresRecorded(body string) bool {
	fields, duplicate := reportdoc.Fields(body, selfReviewScoresField)
	return duplicate != nil || decimalScore.MatchString(fields[strings.ToLower(selfReviewScoresField)])
}
