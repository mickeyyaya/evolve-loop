package log

import (
	"os"
	"strings"
	"testing"
)

func TestConsole_Warnf_WritesToErrNotOut(t *testing.T) {
	t.Parallel()
	var out, errb strings.Builder
	c := Console{Out: &out, Err: &errb}
	c.Warnf("danger %d", 7)
	if out.String() != "" {
		t.Errorf("Warnf wrote to Out = %q, want empty", out.String())
	}
	if got := errb.String(); got != "danger 7" {
		t.Errorf("Warnf -> Err = %q, want %q", got, "danger 7")
	}
}

func TestConsole_Infof_WritesToOutNotErr(t *testing.T) {
	t.Parallel()
	var out, errb strings.Builder
	c := Console{Out: &out, Err: &errb}
	c.Infof("hello %s", "world")
	if got := out.String(); got != "hello world" {
		t.Errorf("Infof -> Out = %q, want %q", got, "hello world")
	}
	if errb.String() != "" {
		t.Errorf("Infof wrote to Err = %q, want empty", errb.String())
	}
}

func TestConsole_Quiet_SuppressesInfoNotWarn(t *testing.T) {
	t.Parallel()
	var out, errb strings.Builder
	c := Console{Out: &out, Err: &errb, Quiet: true}
	c.Infof("info")
	c.Warnf("warn")
	if out.String() != "" {
		t.Errorf("Quiet Infof wrote %q, want empty", out.String())
	}
	if errb.String() != "warn" {
		t.Errorf("Quiet Warnf = %q, want \"warn\" (warnings survive quiet)", errb.String())
	}
}

func TestConsole_Errorf_WritesToErr(t *testing.T) {
	t.Parallel()
	var out, errb strings.Builder
	c := Console{Out: &out, Err: &errb}
	c.Errorf("boom %v", 1)
	if errb.String() != "boom 1" || out.String() != "" {
		t.Errorf("Errorf out=%q err=%q, want out empty / err \"boom 1\"", out.String(), errb.String())
	}
}

func TestConsole_NilSinks_NoPanic(t *testing.T) {
	t.Parallel()
	c := Console{}
	c.Infof("x")
	c.Warnf("y")
	c.Errorf("z")
}

func TestDefault_WiresStdStreams(t *testing.T) {
	t.Parallel()
	c := Default()
	if c.Out != os.Stdout || c.Err != os.Stderr {
		t.Errorf("Default() = {Out:%v Err:%v}, want os.Stdout/os.Stderr", c.Out, c.Err)
	}
}

func TestDiag_SendsInfoToStderr(t *testing.T) {
	t.Parallel()
	c := Diag()
	if c.Out != os.Stderr || c.Err != os.Stderr {
		t.Errorf("Diag() = {Out:%v Err:%v}, want both os.Stderr", c.Out, c.Err)
	}
}
