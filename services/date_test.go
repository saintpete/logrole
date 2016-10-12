package services

import (
	"testing"
	"time"
)

func mustParse(layout, s string) time.Time {
	t, err := time.Parse(layout, s)
	if err != nil {
		panic(err)
	}
	return t
}

const layout = "2006-01-02 15:04:05.999999999 -0700 MST"

var now = mustParse(layout, "2016-10-12 19:54:00 -0800 PST").UTC()

var dateTests = []struct {
	in       time.Time
	expected string
}{
	{mustParse(layout, "2016-10-12 18:30:00 -0800 PST"), "6:30pm"},
	{mustParse(layout, "2016-10-12 6:30:00 -0800 PST"), "6:30am"},
	{mustParse(layout, "2016-10-12 11:30:00 -0800 PST"), "11:30am"},
	{mustParse(layout, "2016-10-11 23:59:59 -0800 PST"), "Yesterday, 11:59pm"},
	{mustParse(layout, "2016-10-11 00:00:00 -0800 PST"), "Yesterday, 12:00am"},
	{mustParse(layout, "2016-10-10 00:00:00 -0800 PST"), "12:00am, October 10"},
	{mustParse(layout, "2015-10-10 00:00:00 -0800 PST"), "12:00am, October 10, 2015"},
}

func TestFriendlyDate(t *testing.T) {
	for _, tt := range dateTests {
		out := friendlyDate(tt.in, now)
		if out != tt.expected {
			t.Errorf("FriendlyDate(%v): got %s, want %s", tt.in, out, tt.expected)
		}
	}
}
