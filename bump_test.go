package semfrag

import "testing"

func TestParseVersionAcceptsPlainVPrefixedAndPrerelease(t *testing.T) {
	for _, test := range []struct {
		in   string
		want ParsedVersion
	}{
		{"1.2.3", ParsedVersion{Major: 1, Minor: 2, Patch: 3}},
		{"v1.2.3", ParsedVersion{Major: 1, Minor: 2, Patch: 3}},
		{"1.2.3-beta.1", ParsedVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta.1"}},
	} {
		got, err := ParseVersion(test.in)
		if err != nil {
			t.Fatalf("ParseVersion(%q) returned error: %v", test.in, err)
		}
		assertEqual(t, got, test.want)
	}
}

func TestParseVersionRejectsInvalidVersions(t *testing.T) {
	for _, version := range []string{
		"1.2",
		"not-a-version",
		"01.2.3",
		"1.02.3",
		"1.2.03",
		"1.2.3-alpha..1",
		"1.2.3-alpha.01",
		"1.2.3+build..1",
		"9007199254740992.0.0",
	} {
		_, err := ParseVersion(version)
		assertErrorContains(t, err, "Invalid semantic version")
	}
	parsed, err := ParseVersion("1.2.3+build.01")
	if err != nil {
		t.Fatalf("ParseVersion returned error: %v", err)
	}
	assertEqual(t, parsed.Build, "build.01")
}

func TestCustomDottedPrereleaseChannels(t *testing.T) {
	got, err := NextPrerelease("1.2.3", "preview.test", []string{"1.2.3-preview.test.1", "1.2.3-preview.test.3"})
	if err != nil {
		t.Fatalf("NextPrerelease returned error: %v", err)
	}
	assertEqual(t, got, "1.2.3-preview.test.4")

	for _, channel := range []string{"", "bad channel", "alpha..test", "alpha+build", "01"} {
		_, err := NextPrerelease("1.2.3", channel, []string{})
		assertErrorContains(t, err, "Invalid")
	}

	_, err = ApplyBump("9007199254740991.0.0", "MAJOR")
	assertErrorContains(t, err, "Invalid semantic version")
}

func TestApplyBump(t *testing.T) {
	for _, test := range []struct {
		version string
		level   BumpLevel
		want    string
	}{
		{"1.2.3", BumpPatch, "1.2.4"},
		{"1.2.3", BumpMinor, "1.3.0"},
		{"1.2.3", BumpMajor, "2.0.0"},
		{"1.2.3-beta.1", BumpPatch, "1.2.4"},
	} {
		got, err := ApplyBump(test.version, test.level)
		if err != nil {
			t.Fatalf("ApplyBump(%q, %q) returned error: %v", test.version, test.level, err)
		}
		assertEqual(t, got, test.want)
	}
}

func TestIsBumpLevel(t *testing.T) {
	assertEqual(t, IsBumpLevel("MAJOR"), true)
	assertEqual(t, IsBumpLevel("major"), false)
	assertEqual(t, IsBumpLevel("HUGE"), false)
}

func TestHighestBump(t *testing.T) {
	got, ok := HighestBump([]BumpLevel{BumpPatch, BumpMinor, BumpPatch})
	assertEqual(t, ok, true)
	assertEqual(t, got, BumpMinor)

	got, ok = HighestBump([]BumpLevel{BumpPatch, BumpMajor, BumpMinor})
	assertEqual(t, ok, true)
	assertEqual(t, got, BumpMajor)

	_, ok = HighestBump([]BumpLevel{})
	assertEqual(t, ok, false)
}

func TestNextVersion(t *testing.T) {
	got, err := NextVersion("1.2.3", bumpPtr(BumpMinor))
	if err != nil {
		t.Fatalf("NextVersion returned error: %v", err)
	}
	assertEqual(t, got, "1.3.0")

	got, err = NextVersion("1.2.3", nil)
	if err != nil {
		t.Fatalf("NextVersion returned error: %v", err)
	}
	assertEqual(t, got, "1.2.3")
}

func TestBaseVersion(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"1.2.3", "1.2.3"},
		{"1.2.3-alpha.4", "1.2.3"},
		{"1.2.3-rc.1+build.7", "1.2.3"},
	} {
		got, err := BaseVersion(test.in)
		if err != nil {
			t.Fatalf("BaseVersion(%q) returned error: %v", test.in, err)
		}
		assertEqual(t, got, test.want)
	}
}

func TestPrereleaseOfAndIsPrerelease(t *testing.T) {
	assertEqual(t, PrereleaseOf("1.2.3-alpha.4"), "alpha.4")
	assertEqual(t, PrereleaseOf("1.2.3"), "")
	assertEqual(t, PrereleaseOf("not-a-version"), "")
	assertEqual(t, IsPrerelease("1.2.3-beta.1"), true)
	assertEqual(t, IsPrerelease("1.2.3"), false)
}

func TestPrereleaseVersion(t *testing.T) {
	got, err := PrereleaseVersion("1.2.3", "alpha", 1)
	if err != nil {
		t.Fatalf("PrereleaseVersion returned error: %v", err)
	}
	assertEqual(t, got, "1.2.3-alpha.1")

	got, err = PrereleaseVersion("1.2.3-beta.1", "rc", 2)
	if err != nil {
		t.Fatalf("PrereleaseVersion returned error: %v", err)
	}
	assertEqual(t, got, "1.2.3-rc.2")
}

func TestNextPrerelease(t *testing.T) {
	versions := []string{"1.2.3-alpha.1", "1.2.3-alpha.2", "1.2.2", "1.3.0-alpha.1"}
	for _, test := range []struct {
		base    string
		channel string
		want    string
	}{
		{"1.2.3", "alpha", "1.2.3-alpha.3"},
		{"1.2.3", "beta", "1.2.3-beta.1"},
		{"1.2.4", "alpha", "1.2.4-alpha.1"},
	} {
		got, err := NextPrerelease(test.base, test.channel, versions)
		if err != nil {
			t.Fatalf("NextPrerelease returned error: %v", err)
		}
		assertEqual(t, got, test.want)
	}
}

func TestCompareBaseAndHighestBase(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.2.3-alpha.1", "1.2.3", 0},
	} {
		got, err := CompareBase(test.a, test.b)
		if err != nil {
			t.Fatalf("CompareBase returned error: %v", err)
		}
		assertEqual(t, got, test.want)
	}

	got, err := HighestBase("1.0.1", "1.0.0", "1.1.0-alpha.1")
	if err != nil {
		t.Fatalf("HighestBase returned error: %v", err)
	}
	assertEqual(t, got, "1.1.0")
}
