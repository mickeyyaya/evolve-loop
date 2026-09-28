package cyclestate

import "strings"

const (
	detailOpen  = " [[detail: "
	detailClose = " ]]"
)

func WithDetail(reason, detail string) string {
	if detail == "" {
		return reason
	}
	return reason + detailOpen + detail + detailClose
}

func WithoutDetail(reason string) string {
	for {
		open := strings.Index(reason, detailOpen)
		if open < 0 {
			return reason
		}
		span := strings.Index(reason[open:], detailClose)
		if span < 0 {
			return reason[:open]
		}
		reason = reason[:open] + reason[open+span+len(detailClose):]
	}
}
