package fleet

import "testing"

func TestGroupByFiles_TodosSharingEveryFileFormOneGroup(t *testing.T) {
	t.Parallel()
	todos := []Todo{
		{ID: "a", Files: []string{"x.go", "y.go"}},
		{ID: "b", Files: []string{"x.go", "y.go"}},
	}

	groups := groupByFiles(todos)

	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("groups = %v, want one group holding both todos", groups)
	}
}

func TestGroupByFiles_BridgingTodoMergesGroupsWhicheverFileItListsFirst(t *testing.T) {
	t.Parallel()
	for attempt := 0; attempt < 200; attempt++ {
		todos := []Todo{
			{ID: "a", Files: []string{"x.go"}},
			{ID: "b", Files: []string{"y.go"}},
			{ID: "c", Files: []string{"x.go", "y.go"}},
		}

		groups := groupByFiles(todos)

		if len(groups) != 1 || len(groups[0]) != 3 {
			t.Fatalf("attempt %d: groups = %v, want one group of a, b and c", attempt, groups)
		}
	}
}
