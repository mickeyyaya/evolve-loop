package policy_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_TheDeadlineBasisDefaultsToAll(t *testing.T) {
	for name, raw := range map[string]string{"no policy block": `{}`, "an orchestrator block without the key": `{"fleet":{"orchestrator":{"stage":"enforce"}}}`} {
		p, err := policy.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := p.FleetConfig().Orchestrator.DeadlineBasis; got != "all" {
			t.Errorf("%s: DeadlineBasis = %q, want all", name, got)
		}
	}
}

func TestFleetConfig_TheDeadlineBasisKeyOverridesTheDefault(t *testing.T) {
	for _, basis := range []string{"all", "shipped"} {
		p, err := policy.Parse([]byte(`{"fleet":{"orchestrator":{"deadline_basis":"` + basis + `"}}}`))
		if err != nil {
			t.Fatalf("%s: %v", basis, err)
		}
		if got := p.FleetConfig().Orchestrator.DeadlineBasis; got != basis {
			t.Errorf("DeadlineBasis = %q, want %q", got, basis)
		}
	}
}

func TestParse_RefusesADeadlineBasisOtherThanAllOrShipped(t *testing.T) {
	cases := map[string]string{
		"an unknown basis":       `"p99"`,
		"a wrong case":           `"Shipped"`,
		"a padded value":         `" shipped"`,
		"a list in one string":   `"all,shipped"`,
		"a legacy synonym":       `"passed"`,
		"a negation of the name": `"unshipped"`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := policy.Parse([]byte(`{"fleet":{"orchestrator":{"deadline_basis":` + raw + `}}}`))
			if !errors.Is(err, policy.ErrDeadlineBasis) {
				t.Fatalf("err = %v, want policy.ErrDeadlineBasis", err)
			}
		})
	}
}

func TestParse_RefusesADeadlineBasisThatIsNotAString(t *testing.T) {
	for _, raw := range []string{`5`, `true`} {
		t.Run(raw, func(t *testing.T) {
			_, err := policy.Parse([]byte(`{"fleet":{"orchestrator":{"deadline_basis":` + raw + `}}}`))
			var typeErr *json.UnmarshalTypeError
			if !errors.As(err, &typeErr) || typeErr.Field != "deadline_basis" {
				t.Fatalf("err = %v, want a json type error on the field deadline_basis", err)
			}
		})
	}
}
