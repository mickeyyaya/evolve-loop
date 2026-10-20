package ciparity

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDarwinPackages_SelectsEveryPackageWhoseCodeOrTestsDifferOnMacOS(t *testing.T) {
	t.Parallel()
	root := writeModule(t, map[string]string{
		"go.mod":                          "module m\n\ngo 1.23\n",
		"plain/a.go":                      "package plain\n",
		"plain/a_test.go":                 "package plain\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n",
		"suffix/a.go":                     "package suffix\n",
		"suffix/a_darwin.go":              "package suffix\n",
		"darwintest/a.go":                 "package darwintest\n",
		"darwintest/a_darwin_test.go":     "package darwintest\n",
		"linuxonly/a_linux.go":            "package linuxonly\n",
		"notdarwin/a.go":                  "//go:build !darwin\n\npackage notdarwin\n",
		"notdarwin/b.go":                  "package notdarwin\n",
		"linuxonlypkg/a.go":               "//go:build !darwin\n\npackage linuxonlypkg\n",
		"unix/a.go":                       "//go:build darwin || linux\n\npackage unix\n",
		"integration/a.go":                "package integration\n",
		"integration/a_test.go":           "//go:build integration && darwin\n\npackage integration\n",
		"notwindows/a.go":                 "//go:build !windows\n\npackage notwindows\n",
		"goosbranch/a.go":                 "package goosbranch\nimport \"runtime\"\nvar Mac = runtime.GOOS == \"darwin\"\n",
		"goosswitch/a_test.go":            "package goosswitch\nimport \"runtime\"\nfunc max() int {\n\tswitch runtime.GOOS {\n\tcase \"linux\":\n\t\treturn 1\n\t}\n\treturn 0\n}\n",
		"goosvalue/a.go":                  "package goosvalue\nimport \"runtime\"\nvar Name = runtime.GOOS\n",
		"linuxcheck/a.go":                 "package linuxcheck\nimport \"runtime\"\nvar Linux = \"linux\" != runtime.GOOS\n",
		"windowsonly/a.go":                "package windowsonly\nimport \"runtime\"\nvar Win = runtime.GOOS == \"windows\" || \"plan9\" != runtime.GOOS\n",
		"othergoos/a.go":                  "package othergoos\ntype t struct{ GOOS string }\nvar v t\nvar Name = v.GOOS == \"darwin\"\n",
		"sandbox/a.go":                    "package sandbox\nconst Bin = \"/usr/bin/sandbox-exec\"\n",
		"sandboxtest/a.go":                "package sandboxtest\n",
		"sandboxtest/a_test.go":           "package sandboxtest\nconst bin = \"sandbox-exec\"\n",
		"nogo/README.md":                  "x\n",
		"plain/testdata/a_darwin.go":      "package testdata\n",
		"vendor/v/a_darwin.go":            "package v\n",
		".hidden/a_darwin.go":             "package hidden\n",
		"_skip/a_darwin.go":               "package skip\n",
		"acs/a/a_darwin.go":               "package a\n",
		"nested/go.mod":                   "module n\n",
		"nested/a_darwin.go":              "package nested\n",
		"deep/inner/pkg/a_darwin.go":      "package pkg\n",
		"deep/inner/pkg/plain/a.go":       "package plain\n",
		"generatedheader/a.go":            "// Package generatedheader names darwin only in prose.\n//\n//go:build darwin\n\npackage generatedheader\n",
		"comment/a.go":                    "// runs on darwin\npackage comment\n",
		"constraintafterpackage/a.go":     "package constraintafterpackage\n\n//go:build darwin\n",
		"plain/sub/a.go":                  "package sub\n",
		"cgo/a.go":                        "package cgo\n",
		"cgo/b.go":                        "//go:build cgo && darwin\n\npackage cgo\n",
		"testdata/top/a_darwin.go":        "package top\n",
		"suffix/testdata/inner_darwin.go": "package inner\n",
	})

	got, err := DarwinPackages(root)

	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./cgo", "./darwintest", "./deep/inner/pkg", "./generatedheader", "./goosbranch", "./goosswitch", "./goosvalue", "./integration", "./linuxcheck", "./notdarwin", "./sandbox", "./suffix", "./unix"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DarwinPackages =\n%v\nwant\n%v", got, want)
	}
}

func TestDarwinPackages_FailsLoudlyOnAPackageItCannotRead(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"two packages in one dir":         {"p/a.go": "package a\n", "p/b.go": "package b\n"},
		"a linux file of another package": {"p/a.go": "package a\n", "p/b_linux.go": "package b\n"},
		"a body that does not parse":      {"p/a.go": "package a\nfunc {\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got, err := DarwinPackages(writeModule(t, files)); err == nil {
				t.Errorf("DarwinPackages = %v, nil; want an error", got)
			}
		})
	}
	if _, err := DarwinPackages(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("a missing module dir gave no error")
	}
}

func TestDarwinPackages_FailsLoudlyWhenAFileCannotBeRead(t *testing.T) {
	t.Parallel()
	root := writeModule(t, map[string]string{"p/a.go": "package a\n"})
	deny := func(string) ([]byte, error) { return nil, os.ErrPermission }

	if _, err := darwinPackages(root, deny); !errors.Is(err, os.ErrPermission) {
		t.Errorf("DarwinPackages error = %v, want the read error", err)
	}
}

func TestDarwinSetFile_MatchesTheSetDerivedFromTheTree(t *testing.T) {
	t.Parallel()
	moduleDir := filepath.Join(workflowRepoRoot(t), "go")
	pkgs, err := DarwinPackages(moduleDir)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join(moduleDir, DarwinSetFile))
	if err != nil {
		t.Fatal(err)
	}
	if want := DarwinSetText(pkgs); string(committed) != want {
		t.Errorf("go/%s is stale: a package gained or lost macOS-specific code. Run `make -C go darwin-set` and commit the file.\ncommitted:\n%s\nderived:\n%s", DarwinSetFile, committed, want)
	}
}

func TestDarwinMain_PrintsTheSetFileAndFailsOnAnEmptyOrUnreadableTree(t *testing.T) {
	t.Parallel()
	mac := writeModule(t, map[string]string{"go.mod": "module m\n", "p/a_darwin.go": "package p\n"})
	var out, errOut bytes.Buffer
	if code := DarwinMain([]string{"-dir", mac}, &out, &errOut); code != 0 || out.String() != DarwinSetText([]string{"./p"}) {
		t.Errorf("DarwinMain = %d, %q (%s); want 0 and the set file", code, out.String(), errOut.String())
	}
	if !strings.HasPrefix(out.String(), "# ") || !strings.HasSuffix(out.String(), "\n./p\n") {
		t.Errorf("the set file must open with its comment header and end with one package per line: %q", out.String())
	}
	for name, tc := range map[string]struct {
		args []string
		code int
		want string
	}{
		"no darwin package": {[]string{"-dir", writeModule(t, map[string]string{"p/a.go": "package p\n"})}, 1, "no darwin-sensitive package"},
		"an unreadable dir": {[]string{"-dir", filepath.Join(t.TempDir(), "missing")}, 1, "darwinset:"},
		"an unknown flag":   {[]string{"-nope"}, 2, "-nope"},
	} {
		var stdout, stderr bytes.Buffer
		if code := DarwinMain(tc.args, &stdout, &stderr); code != tc.code || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("%s: DarwinMain = %d, stdout %q, stderr %q; want %d and %q", name, code, stdout.String(), stderr.String(), tc.code, tc.want)
		}
	}
}
