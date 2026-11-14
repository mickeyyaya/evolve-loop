package qualitypanel

import "math"

const xmrD2 = 1.128

type XmRChart struct{ Center, MRBar, LCL, UCL float64 }

func NewXmRChart(baseline []float64) (XmRChart, bool) {
	if len(baseline) < 2 {
		return XmRChart{}, false
	}
	var sum, ranges float64
	for i, v := range baseline {
		sum += v
		if i > 0 {
			ranges += math.Abs(v - baseline[i-1])
		}
	}
	center := sum / float64(len(baseline))
	mrBar := ranges / float64(len(baseline)-1)
	span := 3 * mrBar / xmrD2
	return XmRChart{Center: center, MRBar: mrBar, LCL: center - span, UCL: center + span}, true
}

const XmRRule1Field = "xmr_rule1"

func (c XmRChart) Outside(v float64) bool { return v < c.LCL || v > c.UCL }

func (c XmRChart) Rule1(v float64) string {
	if c.Outside(v) {
		return "outside"
	}
	return "in_control"
}
