package guards

import "testing"

func TestIsHealingConstitution_IsTheMarkedPartOfTheManifest(t *testing.T) {
	cases := map[string]bool{
		"go/internal/gateclass/gateclass.go":     true,
		"go/internal/policy/healing.go":          true,
		"go/internal/phasecontract/diagnosis.go": true,
		"go/internal/guards/role.go":             false,
		"go/internal/widget/widget.go":           false,
		"":                                       false,
	}
	for path, want := range cases {
		if got := IsHealingConstitution(path); got != want {
			t.Errorf("IsHealingConstitution(%q) = %v, want %v", path, got, want)
		}
		if want && !IsProtectedSurface(path) {
			t.Errorf("%s is constitution but not protected", path)
		}
	}
}
