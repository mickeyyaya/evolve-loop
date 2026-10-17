//go:build linux

package main

import "testing"

func TestNewSource_ReadsRealProc(t *testing.T) {
	read, err := newSource()
	if err != nil {
		t.Fatal(err)
	}
	got, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasCPU || got.CPUTotal == 0 || !got.HasRunQueue || !got.HasMem || !got.HasSwap || got.MemUsedMB <= 0 {
		t.Fatalf("got %+v", got)
	}
}
