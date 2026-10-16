package phasestream

import "encoding/json"

type breadcrumb struct {
	Channel string `json:"evolve_channel"`
	CorrID  string `json:"corr_id"`
}

type corrMark struct {
	sub      string
	corrID   string
	atSeq    int64
	startSeq int64
	endSeq   int64
}

type correlator struct {
	openAtSeq map[string]int64
}

func newCorrelator() *correlator {
	return &correlator{openAtSeq: map[string]int64{}}
}

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
			return nil
		}
		delete(c.openAtSeq, b.CorrID)
		return []corrMark{{sub: "response_complete", corrID: b.CorrID, startSeq: at + 1, endSeq: currentSeq}}
	default:
		return nil
	}
}
