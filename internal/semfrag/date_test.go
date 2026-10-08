package semfrag

import (
	"testing"
	"time"
)

func TestFormatReleaseDate(t *testing.T) {
	assertEqual(t, FormatReleaseDate(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)), "January 1st, 2026")
	assertEqual(t, FormatReleaseDate(time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)), "December 31st, 2026")
}

func TestFormatReleaseDateOrdinalSuffix(t *testing.T) {
	cases := []struct {
		day  int
		want string
	}{
		{1, "1st"},
		{2, "2nd"},
		{3, "3rd"},
		{4, "4th"},
		{11, "11th"},
		{12, "12th"},
		{13, "13th"},
		{21, "21st"},
		{22, "22nd"},
		{23, "23rd"},
	}
	for _, test := range cases {
		date := time.Date(2026, time.June, test.day, 0, 0, 0, 0, time.UTC)
		assertEqual(t, FormatReleaseDate(date), "June "+test.want+", 2026")
	}
}
