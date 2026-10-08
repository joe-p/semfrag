package semfrag

import (
	"fmt"
	"time"
)

var months = [...]string{
	"January",
	"February",
	"March",
	"April",
	"May",
	"June",
	"July",
	"August",
	"September",
	"October",
	"November",
	"December",
}

func ordinal(day int) string {
	if day >= 11 && day <= 13 {
		return "th"
	}
	switch day % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

func FormatReleaseDate(date time.Time) string {
	day := date.Day()
	return fmt.Sprintf("%s %d%s, %d", months[date.Month()-1], day, ordinal(day), date.Year())
}
