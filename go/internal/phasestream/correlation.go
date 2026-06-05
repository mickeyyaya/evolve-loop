package phasestream

import "encoding/json"

// breadcrumb is the driver's structured stderr marker (a later task emits it).
type breadcrumb struct {
	Channel string `json:"evolve_channel"`
	CorrID  string `json:"corr_id"`
}

// corrMark is the internal pre-envelope form the classifier converts to a
// KindCorrelation Envelope. sub ∈ {request, response_complete}.
type corrMark struct {
	sub      string
	corrID   string
	atSeq    int64 // request
	startSeq int64 // response_complete
	endSeq   int64 // response_complete
}

// correlator tracks the open inject per corr_id so an idle_reached can be
// bracketed into a span. One per phase (the classifier owns it). Not safe for
// concurrent use — the classifier drives it from a single goroutine.
type correlator struct {
	openAtSeq map[string]int64
}

func newCorrelator() *correlator {
	return &correlator{openAtSeq: map[string]int64{}}
}

// observe parses one stderr line. currentSeq is the classifier's seq counter at
// this line. Returns nil for non-breadcrumb lines.
func (c *correlator) observe(line []byte, currentSeq int64) []corrMark {
	var b breadcrumb
	if err := json.Unmarshal(line, &b); err != nil || b.Channel == "" || b.CorrID == "" {
		return nil
	}
	switch b.Channel {
	case "inject_applied":
		c.openAtSeq[b.CorrID] = currentSeq
		return []corrMark{{sub: "request", corrID: b.CorrID, atSeq: currentSeq}}
	case "idle_reached":
		at, ok := c.openAtSeq[b.CorrID]
		if !ok {
			return nil // no matching open inject (dup / out-of-order) → ignore
		}
		delete(c.openAtSeq, b.CorrID)
		return []corrMark{{sub: "response_complete", corrID: b.CorrID, startSeq: at + 1, endSeq: currentSeq}}
	default:
		return nil
	}
}
