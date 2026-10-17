//go:build darwin

package main

import "testing"

func TestNewSource_ReadsRealHost(t *testing.T) {
	read, err := newSource()
	if err != nil {
		t.Fatal(err)
	}
	got, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasMem || got.MemUsedMB <= 0 || !got.HasSwap || got.HasCPU || got.HasRunQueue {
		t.Fatalf("got %+v", got)
	}
}
