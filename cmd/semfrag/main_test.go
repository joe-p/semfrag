package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestParseArgsRejectsValuesOnBooleanFlags(t *testing.T) {
	for _, arg := range []string{
		"--dry-run=false", "--no-clear=true", "--help=", "--version=garbage",
		"-vgarbage", "-h=false", "-vh",
	} {
		t.Run(arg, func(t *testing.T) {
			_, err := parseArgs([]string{arg})
			if err == nil || !strings.Contains(err.Error(), "does not take a value") {
				t.Fatalf("expected invalid boolean flag error, got %v", err)
			}
		})
	}
}

func TestResolveVersionPrefersInjectedVersion(t *testing.T) {
	original := version
	defer func() { version = original }()

	version = "1.2.3"
	if got := resolveVersion(); got != "1.2.3" {
		t.Fatalf("got %q, want %q", got, "1.2.3")
	}
}

func TestVersionFromBuildInfo(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"tagged install", &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true, "1.2.3"},
		{"devel build", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, "0.0.0"},
		{"empty version", &debug.BuildInfo{Main: debug.Module{Version: ""}}, true, "0.0.0"},
		{"missing info", nil, false, "0.0.0"},
	}
	for _, test := range cases {
		if got := versionFromBuildInfo(test.info, test.ok); got != test.want {
			t.Fatalf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}

func TestResolveVersionFallsBackForLocalBuilds(t *testing.T) {
	original := version
	defer func() { version = original }()

	version = ""
	got := resolveVersion()
	if got == "" {
		t.Fatal("expected a non-empty fallback version")
	}
	if strings.HasPrefix(got, "v") {
		t.Fatalf("fallback version should not keep a v prefix: %q", got)
	}
}
