package hostpressure

import (
	"errors"
	"strings"
	"testing"
)

var errBoom = errors.New("boom")

type host struct {
	files   map[string]string
	outputs map[string]string
	failRun map[string]bool
}

func (h host) readFile(path string) ([]byte, error) {
	if v, ok := h.files[path]; ok {
		return []byte(v), nil
	}
	return nil, errBoom
}

func (h host) run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if h.failRun[key] {
		return nil, errBoom
	}
	if v, ok := h.outputs[key]; ok {
		return []byte(v), nil
	}
	return nil, errBoom
}

func (h host) reader(cpus int) Reader {
	return Reader{ReadFile: h.readFile, Run: h.run, CPUs: func() int { return cpus }}
}

const (
	keyLoad   = "sysctl -n vm.loadavg"
	keyLevel  = "sysctl -n kern.memorystatus_level"
	keySwap   = "sysctl -n vm.swapusage"
	keyMemsz  = "sysctl -n hw.memsize"
	keyVMStat = "vm_stat "
)

func linuxHost() host {
	return host{files: map[string]string{"/proc/loadavg": "2.50 1.00 0.50 1/100 123\n", "/proc/meminfo": meminfoFixture}}
}

func darwinHost(t *testing.T) host {
	t.Helper()
	return host{outputs: map[string]string{
		keyLoad: "{ 3.00 2.00 1.00 }\n", keyLevel: "93\n", keySwap: fixture(t, "swapusage.txt"),
		keyVMStat: fixture(t, "vm_stat.txt"), keyMemsz: "17179869184\n",
	}}
}

func TestReader_LinuxParsesLoadavgAndMeminfoAvailable(t *testing.T) {
	got, err := linuxHost().reader(8).Read()
	if err != nil {
		t.Fatal(err)
	}
	want := Reading{Load1: 2.5, CPUs: 8, MemAvailablePct: 25, SwapUsedMB: 1024}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReader_LinuxFailuresReturnAnError(t *testing.T) {
	badLoad := linuxHost()
	badLoad.files["/proc/loadavg"] = "x y z"
	noMem := linuxHost()
	delete(noMem.files, "/proc/meminfo")
	badMem := linuxHost()
	badMem.files["/proc/meminfo"] = "MemTotal: 1 kB\n"
	cases := map[string]struct {
		h    host
		want error
	}{"bad loadavg": {badLoad, nil}, "no meminfo": {noMem, errBoom}, "bad meminfo": {badMem, ErrMissingField}}
	for name, c := range cases {
		_, err := c.h.reader(8).Read()
		if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}

func TestReader_DarwinUsesMemorystatusLevelWhenItWorks(t *testing.T) {
	got, err := darwinHost(t).reader(14).Read()
	if err != nil {
		t.Fatal(err)
	}
	want := Reading{Load1: 3, CPUs: 14, MemAvailablePct: 93, SwapUsedMB: 0}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReader_DarwinFallsBackToVMStatWhenLevelFails(t *testing.T) {
	for name, mutate := range map[string]func(host){
		"exec fails": func(h host) { h.failRun[keyLevel] = true },
		"unparsable": func(h host) { h.outputs[keyLevel] = "junk" },
	} {
		h := darwinHost(t)
		h.failRun = map[string]bool{}
		mutate(h)
		got, err := h.reader(14).Read()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		usedMB := float64(1225522+212195) * 16384 / (1 << 20)
		want := 100 - usedMB/(17179869184.0/(1<<20))*100
		if got.MemAvailablePct != want {
			t.Errorf("%s: MemAvailablePct = %v, want %v", name, got.MemAvailablePct, want)
		}
	}
}

func TestReader_DarwinFailuresReturnAnError(t *testing.T) {
	cases := map[string]func(host){
		"load exec fails":      func(h host) { h.failRun[keyLoad] = true },
		"load unparsable":      func(h host) { h.outputs[keyLoad] = "{ }" },
		"swap exec fails":      func(h host) { h.failRun[keySwap] = true },
		"swap unparsable":      func(h host) { h.outputs[keySwap] = "junk" },
		"fallback vm_stat":     func(h host) { h.failRun[keyLevel] = true; h.failRun[keyVMStat] = true },
		"fallback vm_stat bad": func(h host) { h.failRun[keyLevel] = true; h.outputs[keyVMStat] = "junk" },
		"fallback memsize":     func(h host) { h.failRun[keyLevel] = true; h.failRun[keyMemsz] = true },
		"fallback memsize bad": func(h host) { h.failRun[keyLevel] = true; h.outputs[keyMemsz] = "0" },
	}
	for name, mutate := range cases {
		h := darwinHost(t)
		h.failRun = map[string]bool{}
		mutate(h)
		if _, err := h.reader(14).Read(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestReader_LoadNotNumberIsErrMissingField(t *testing.T) {
	h := darwinHost(t)
	h.outputs[keyLoad] = "{ }"
	if _, err := h.reader(14).Read(); !errors.Is(err, ErrMissingField) {
		t.Fatalf("err = %v, want ErrMissingField", err)
	}
}
