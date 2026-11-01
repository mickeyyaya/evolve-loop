package llmroute

type Binding struct{ driver string }

func Bind(driver string) Binding { return Binding{driver: driver} }

func (b Binding) String() string { return b.driver }

func (b Binding) IsZero() bool { return b.driver == "" }

func EscalationTarget(candidates []string, failed Binding, allowed func(driver string) bool) (Binding, bool) {
	family := Family(failed.driver)
	for _, c := range candidates {
		if c != "" && Family(c) != family && allowed(c) {
			return Bind(c), true
		}
	}
	return Binding{}, false
}
