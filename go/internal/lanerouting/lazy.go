package lanerouting

import (
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeRoutingFallback signalcenter.Code = "INBOX_ROUTING_FALLBACK"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleInbox, CodeRoutingFallback, "the lane-routing predicate could not load the build profile (fields.profile, fields.project_root; the reason is the load error), so routing judges protected surface only and a sandbox-denied item can reach a lane; emitted once per predicate, on its first routing question")
}

func LazyForbidden(projectRoot, writerProfile string, emit signalcenter.Listener) func(string) bool {
	var once sync.Once
	judge := guards.IsProtectedScope
	return func(path string) bool {
		once.Do(func() {
			forbidden, err := Forbidden(projectRoot, writerProfile)
			if err != nil {
				emit(fallbackEvent(projectRoot, writerProfile, err))
				return
			}
			judge = forbidden
		})
		return judge(path)
	}
}

func fallbackEvent(projectRoot, writerProfile string, err error) signalcenter.Event {
	return signalcenter.Event{
		Module:   signalcenter.ModuleInbox,
		Origin:   "lanerouting.LazyForbidden",
		Kind:     signalcenter.KindInboxWarning,
		Code:     CodeRoutingFallback,
		Severity: signalcenter.SeverityWarn,
		Reason:   err.Error() + "; routing judges protected surface only",
		Fields:   map[string]string{"profile": writerProfile, "project_root": projectRoot},
	}
}
