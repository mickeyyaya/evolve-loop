package proctree

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestNewLister_ReadsTheTableOfTheGivenUIDAndAttachesArgumentsAndEnvironment(t *testing.T) {
	t.Parallel()
	var askedUID int
	table := func(uid int) ([]Process, error) {
		askedUID = uid
		return []Process{{Pid: 100, Ppid: 1, Comm: "node"}, {Pid: 101, Ppid: 1, Comm: "zsh"}}, nil
	}
	var read argsReader = func(pid int) ([]string, map[string]string, error) {
		if pid == 101 {
			return nil, nil, errors.New("operation not permitted")
		}
		return []string{"node", "mcp.js"}, map[string]string{"EVOLVE_DISPATCH_ID": "r/1/a/p1n1"}, nil
	}

	got, err := newLister(table, read, 501)(context.Background())

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if askedUID != 501 {
		t.Errorf("the table was read for uid %d, want 501", askedUID)
	}
	if len(got) != 2 || !reflect.DeepEqual(got[0].Args, []string{"node", "mcp.js"}) || got[0].Env["EVOLVE_DISPATCH_ID"] != "r/1/a/p1n1" {
		t.Fatalf("got %+v, want pid 100 with its arguments and environment", got)
	}
	if got[1].Args != nil || len(got[1].Env) != 0 || got[1].Comm != "zsh" {
		t.Errorf("pid 101 = %+v, want the row kept with no arguments and no environment: an unreadable process has no proof", got[1])
	}
}

func TestNewLister_ReturnsTheReadTableError(t *testing.T) {
	t.Parallel()
	cause := errors.New("sysctl refused")
	table := func(int) ([]Process, error) { return []Process{{Pid: 1}}, cause }

	got, err := newLister(table, nil, 501)(context.Background())

	if !errors.Is(err, cause) || got != nil {
		t.Errorf("list = %v, %v, want no rows and the table error", got, err)
	}
}
