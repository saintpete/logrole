package services

import "time"

// FriendlyDate returns a friendlier version of the date.
func FriendlyDate(t time.Time) string {
	return friendlyDate(t, time.Now().UTC())
}

func friendlyDate(t time.Time, utcnow time.Time) string {
	now := utcnow.In(t.Location())
	y, m, d := now.Date()
	if d == t.Day() && m == t.Month() && y == t.Year() {
		return t.Format("3:04pm")
	}
	y1, m1, d1 := now.Add(-24 * time.Hour).Date()
	if d1 == t.Day() && m1 == t.Month() && y1 == t.Year() {
		return t.Format("Yesterday, 3:04pm")
	}
	// if the same year, return the day
	if y == t.Year() {
		return t.Format("3:04pm, January 2")
	}
	return t.Format("3:04pm, January 2, 2006")
}
