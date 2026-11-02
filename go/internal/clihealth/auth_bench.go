package clihealth

import (
	"fmt"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeAuthBenchCleared signalcenter.Code = "BRIDGE_AUTH_BENCH_CLEARED"

type ClearSource string

const (
	SourceLaunch     ClearSource = "launch"
	SourceSmoke      ClearSource = "smoke"
	SourceUsageProbe ClearSource = "usage_probe"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeAuthBenchCleared, "a launch, live smoke or usage read of a CLI family succeeded, so the cli-health store removed the auth bench (auth_recheck) of that family; quota benches stay; fields family, reason, source (clihealth.ClearSource: launch, smoke or usage_probe), key")
}

func AuthBench(reason string) bool { return reason == CredentialPattern }

func (s *Store) ClearAuthBench(family string, source ClearSource, signals *signalcenter.Center) ([]Entry, error) {
	if benches, _ := s.Load(); len(authBenchKeys(benches, family)) == 0 {
		return nil, nil
	}
	var cleared []Entry
	err := s.withLock(func() error {
		benches, _ := s.Load()
		for _, key := range authBenchKeys(benches, family) {
			cleared = append(cleared, benches[key])
			delete(benches, key)
		}
		return s.write(benches)
	})
	if err != nil {
		return nil, err
	}
	for _, e := range cleared {
		signals.Emit(authClearedEvent(e, source))
	}
	return cleared, nil
}

func authBenchKeys(benches map[string]Entry, family string) []string {
	var keys []string
	for key, e := range benches {
		if e.Family == family && AuthBench(e.Reason) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func authClearedEvent(e Entry, source ClearSource) signalcenter.Event {
	return signalcenter.Event{
		Module: signalcenter.ModuleBridge, Origin: "Store.ClearAuthBench", Kind: signalcenter.KindBridgeWarning,
		Severity: signalcenter.SeverityInfo, Code: CodeAuthBenchCleared,
		Reason: fmt.Sprintf("%s: a %s succeeded, so the %s bench is cleared", e.Key(), source, e.Reason),
		Fields: map[string]string{"family": e.Family, "reason": e.Reason, "source": string(source), "key": e.Key()},
	}
}
