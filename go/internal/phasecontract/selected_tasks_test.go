package phasecontract

import (
	"reflect"
	"testing"
)

func TestSelectedTasks_IsTheScoutReportsTasksSection(t *testing.T) {
	want := Section{Canonical: "## Selected Tasks", Accepted: []string{"## Selected Tasks", "## Proposed Tasks"}}
	if !reflect.DeepEqual(SelectedTasks, want) {
		t.Errorf("SelectedTasks = %+v, want %+v", SelectedTasks, want)
	}
	if !sectionsHave(Scout.Sections, SelectedTasks.Canonical) {
		t.Errorf("Scout.Sections must include SelectedTasks (%q); got %+v", SelectedTasks.Canonical, Scout.Sections)
	}
	if !SelectedTasks.Present("# Scout\n## Proposed Tasks\n### alpha\n") {
		t.Error("SelectedTasks must accept the legacy ## Proposed Tasks heading")
	}
}
