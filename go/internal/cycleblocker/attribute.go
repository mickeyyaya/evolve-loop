package cycleblocker

import "github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"

const (
	None         = "none"
	Waiting      = "waiting"
	CheckPrefix  = "check:"
	ClassPrefix  = "class:"
	percentScale = 100
)

type Input struct {
	Shipped     bool
	CheckTags   []string
	Class       string
	Verdict     cyclespend.Verdict
	WallMinutes float64
	Paused      bool
}

type Attribution struct {
	Blocker string
	Minutes float64
}

func Attribute(in Input, waitSharePct int) Attribution {
	if in.Shipped {
		return attributeShipped(in, waitSharePct)
	}
	minutes := in.Verdict.LLMMinutesToVerdict
	switch {
	case len(in.CheckTags) > 0:
		return Attribution{Blocker: CheckPrefix + in.CheckTags[0], Minutes: minutes}
	case in.Class != "":
		return Attribution{Blocker: ClassPrefix + in.Class, Minutes: minutes}
	}
	return Attribution{Blocker: None}
}

func attributeShipped(in Input, waitSharePct int) Attribution {
	waiting := in.WallMinutes - in.Verdict.LLMMinutesToVerdict - in.Verdict.HostMinutesToVerdict
	if in.Paused || in.WallMinutes <= 0 || waiting <= 0 || waiting*percentScale < float64(waitSharePct)*in.WallMinutes {
		return Attribution{Blocker: None}
	}
	return Attribution{Blocker: Waiting, Minutes: waiting}
}
