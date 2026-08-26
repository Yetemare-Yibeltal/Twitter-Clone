// backend/internal/utils/time.go
package utils

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// Standard time formats
	FormatISO         = "2006-01-02T15:04:05Z07:00"
	FormatISO8601     = "2006-01-02T15:04:05-07:00"
	FormatDate        = "2006-01-02"
	FormatTime        = "15:04:05"
	FormatDateTime    = "2006-01-02 15:04:05"
	FormatRFC3339     = time.RFC3339
	FormatRFC1123     = time.RFC1123
	FormatRFC822      = time.RFC822
	FormatRFC850      = time.RFC850
	FormatANSIC       = time.ANSIC
	FormatUnixDate    = time.UnixDate
	FormatRubyDate    = time.RubyDate
	FormatKitchen     = "3:04PM"
	FormatTimestamp   = "20060102150405"
	FormatShortDate   = "02/01/2006"
	FormatShortTime   = "15:04"
	FormatMonthDay    = "Jan 2"
	FormatMonthYear   = "Jan 2006"
	FormatDayMonth    = "2 Jan"
	FormatDayMonthYear = "2 Jan 2006"
)

const (
	DaysInWeek     = 7
	HoursInDay     = 24
	MinutesInHour  = 60
	SecondsInMinute = 60
	MillisecondsInSecond = 1000
)

var (
	ErrInvalidTimeFormat = errors.New("invalid time format")
	ErrInvalidTimezone   = errors.New("invalid timezone")
	ErrTimeOutOfRange    = errors.New("time out of range")
	ErrInvalidDuration   = errors.New("invalid duration")
)

// ======================================================================
// Timezone Management
// ======================================================================

// GetTimezone returns a time.Location for the given timezone string.
func GetTimezone(timezone string) (*time.Location, error) {
	if timezone == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTimezone, timezone)
	}
	return loc, nil
}

// GetLocalTimezone returns the local timezone.
func GetLocalTimezone() *time.Location {
	return time.Local
}

// GetUTCTimezone returns UTC timezone.
func GetUTCTimezone() *time.Location {
	return time.UTC
}

// ListTimezones returns a list of common timezones.
func ListTimezones() []string {
	return []string{
		"UTC",
		"America/New_York",
		"America/Chicago",
		"America/Denver",
		"America/Los_Angeles",
		"America/Toronto",
		"America/Vancouver",
		"Europe/London",
		"Europe/Paris",
		"Europe/Berlin",
		"Europe/Moscow",
		"Asia/Dubai",
		"Asia/Kolkata",
		"Asia/Singapore",
		"Asia/Tokyo",
		"Asia/Shanghai",
		"Australia/Sydney",
		"Australia/Melbourne",
		"Pacific/Auckland",
		"Pacific/Honolulu",
	}
}

// ======================================================================
// Parsing Functions
// ======================================================================

// ParseTime parses a time string with the given format.
func ParseTime(value, format string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("time value is empty")
	}
	t, err := time.Parse(format, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s", ErrInvalidTimeFormat, err.Error())
	}
	return t, nil
}

// ParseISO parses an ISO 8601 time string.
func ParseISO(value string) (time.Time, error) {
	return ParseTime(value, FormatISO)
}

// ParseDate parses a date string in YYYY-MM-DD format.
func ParseDate(value string) (time.Time, error) {
	return ParseTime(value, FormatDate)
}

// ParseDateTime parses a date time string in YYYY-MM-DD HH:MM:SS format.
func ParseDateTime(value string) (time.Time, error) {
	return ParseTime(value, FormatDateTime)
}

// ParseRFC3339 parses an RFC3339 time string.
func ParseRFC3339(value string) (time.Time, error) {
	return ParseTime(value, FormatRFC3339)
}

// ParseTimestamp parses a timestamp string in YYYYMMDDHHMMSS format.
func ParseTimestamp(value string) (time.Time, error) {
	return ParseTime(value, FormatTimestamp)
}

// ParseRelativeTime parses a relative time string like "5m", "2h", "3d", "1w", "2M", "1y".
func ParseRelativeTime(value string) (time.Duration, error) {
	if value == "" {
		return 0, errors.New("relative time value is empty")
	}
	// Convert to lowercase
	value = strings.ToLower(strings.TrimSpace(value))
	// Parse the duration
	if strings.HasSuffix(value, "s") {
		return time.ParseDuration(value)
	}
	// Handle longer durations
	var num int64
	var unit string
	fmt.Sscanf(value, "%d%s", &num, &unit)
	switch unit {
	case "m":
		if len(value) > 1 && value[len(value)-2:] == "ms" {
			return time.Duration(num) * time.Millisecond, nil
		}
		return time.Duration(num) * time.Minute, nil
	case "h":
		return time.Duration(num) * time.Hour, nil
	case "d":
		return time.Duration(num) * 24 * time.Hour, nil
	case "w":
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	case "M":
		return time.Duration(num) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported relative time unit: %s", unit)
	}
}

// ======================================================================
= Formatting Functions
// ======================================================================

// FormatTime formats a time using the given format.
func FormatTime(t time.Time, format string) string {
	return t.Format(format)
}

// FormatISO formats a time as ISO 8601.
func FormatISO(t time.Time) string {
	return t.Format(FormatISO)
}

// FormatDate formats a time as YYYY-MM-DD.
func FormatDate(t time.Time) string {
	return t.Format(FormatDate)
}

// FormatDateTime formats a time as YYYY-MM-DD HH:MM:SS.
func FormatDateTime(t time.Time) string {
	return t.Format(FormatDateTime)
}

// FormatRFC3339 formats a time as RFC3339.
func FormatRFC3339(t time.Time) string {
	return t.Format(FormatRFC3339)
}

// FormatTimestamp formats a time as YYYYMMDDHHMMSS.
func FormatTimestamp(t time.Time) string {
	return t.Format(FormatTimestamp)
}

// FormatHumanReadable formats a time in a human-readable way.
func FormatHumanReadable(t time.Time) string {
	return t.Format("Jan 2, 2006 at 3:04 PM")
}

// FormatShortDate formats a time as DD/MM/YYYY.
func FormatShortDate(t time.Time) string {
	return t.Format(FormatShortDate)
}

// FormatShortTime formats a time as HH:MM.
func FormatShortTime(t time.Time) string {
	return t.Format(FormatShortTime)
}

// FormatRelativeTime formats a time relative to now (e.g., "2 hours ago").
func FormatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	if diff < 0 {
		diff = -diff
	}
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Minute*5:
		return "a few minutes ago"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < time.Hour*2:
		return "an hour ago"
	case diff < time.Hour*24:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hours ago", hours)
	case diff < time.Hour*48:
		return "yesterday"
	case diff < time.Hour*24*7:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	case diff < time.Hour*24*14:
		return "last week"
	case diff < time.Hour*24*30:
		weeks := int(diff.Hours() / (24 * 7))
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < time.Hour*24*60:
		months := int(diff.Hours() / (24 * 30))
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / (24 * 365))
		if years <= 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// FormatTimeAgo is an alias for FormatRelativeTime.
func FormatTimeAgo(t time.Time) string {
	return FormatRelativeTime(t)
}

// FormatTimeUntil formats a time until the given time (e.g., "in 2 hours").
func FormatTimeUntil(t time.Time) string {
	now := time.Now()
	diff := t.Sub(now)
	if diff < 0 {
		diff = -diff
	}
	switch {
	case diff < time.Minute:
		return "in a few seconds"
	case diff < time.Minute*5:
		return "in a few minutes"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("in %d minutes", mins)
	case diff < time.Hour*2:
		return "in an hour"
	case diff < time.Hour*24:
		hours := int(diff.Hours())
		return fmt.Sprintf("in %d hours", hours)
	case diff < time.Hour*48:
		return "tomorrow"
	case diff < time.Hour*24*7:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("in %d days", days)
	case diff < time.Hour*24*14:
		return "next week"
	case diff < time.Hour*24*30:
		weeks := int(diff.Hours() / (24 * 7))
		return fmt.Sprintf("in %d weeks", weeks)
	default:
		months := int(diff.Hours() / (24 * 30))
		return fmt.Sprintf("in %d months", months)
	}
}

// ======================================================================
= Time Calculation Helpers
// ======================================================================

// StartOfDay returns the start of the day (00:00:00) for the given time.
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDay returns the end of the day (23:59:59.999999999) for the given time.
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// StartOfWeek returns the start of the week (Monday) for the given time.
func StartOfWeek(t time.Time) time.Time {
	offset := int(t.Weekday() - time.Monday)
	if offset < 0 {
		offset = 6
	}
	return StartOfDay(t.AddDate(0, 0, -offset))
}

// EndOfWeek returns the end of the week (Sunday) for the given time.
func EndOfWeek(t time.Time) time.Time {
	return EndOfDay(StartOfWeek(t).AddDate(0, 0, 6))
}

// StartOfMonth returns the start of the month for the given time.
func StartOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// EndOfMonth returns the end of the month for the given time.
func EndOfMonth(t time.Time) time.Time {
	return StartOfMonth(t).AddDate(0, 1, 0).Add(-time.Nanosecond)
}

// StartOfYear returns the start of the year for the given time.
func StartOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
}

// EndOfYear returns the end of the year for the given time.
func EndOfYear(t time.Time) time.Time {
	return StartOfYear(t).AddDate(1, 0, 0).Add(-time.Nanosecond)
}

// DaysInMonth returns the number of days in a month.
func DaysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// DaysInYear returns the number of days in a year (365 or 366).
func DaysInYear(year int) int {
	if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
		return 366
	}
	return 365
}

// IsLeapYear checks if a year is a leap year.
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// IsWeekend checks if a time falls on a weekend (Saturday or Sunday).
func IsWeekend(t time.Time) bool {
	weekday := t.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// IsWeekday checks if a time falls on a weekday (Monday to Friday).
func IsWeekday(t time.Time) bool {
	return !IsWeekend(t)
}

// IsToday checks if a time is today.
func IsToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day()
}

// IsYesterday checks if a time is yesterday.
func IsYesterday(t time.Time) bool {
	yesterday := time.Now().AddDate(0, 0, -1)
	return t.Year() == yesterday.Year() && t.Month() == yesterday.Month() && t.Day() == yesterday.Day()
}

// IsTomorrow checks if a time is tomorrow.
func IsTomorrow(t time.Time) bool {
	tomorrow := time.Now().AddDate(0, 0, 1)
	return t.Year() == tomorrow.Year() && t.Month() == tomorrow.Month() && t.Day() == tomorrow.Day()
}

// IsSameDay checks if two times are on the same day.
func IsSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

// IsSameWeek checks if two times are in the same week (Monday to Sunday).
func IsSameWeek(t1, t2 time.Time) bool {
	start1 := StartOfWeek(t1)
	start2 := StartOfWeek(t2)
	return start1.Equal(start2)
}

// IsSameMonth checks if two times are in the same month.
func IsSameMonth(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month()
}

// ======================================================================
= Time Difference Helpers
// ======================================================================

// DiffDays returns the number of days between two times.
func DiffDays(t1, t2 time.Time) int {
	// Use start of day to avoid timezone issues
	d1 := StartOfDay(t1)
	d2 := StartOfDay(t2)
	diff := d1.Sub(d2)
	return int(diff.Hours() / 24)
}

// DiffHours returns the number of hours between two times.
func DiffHours(t1, t2 time.Time) float64 {
	return t1.Sub(t2).Hours()
}

// DiffMinutes returns the number of minutes between two times.
func DiffMinutes(t1, t2 time.Time) float64 {
	return t1.Sub(t2).Minutes()
}

// DiffSeconds returns the number of seconds between two times.
func DiffSeconds(t1, t2 time.Time) float64 {
	return t1.Sub(t2).Seconds()
}

// Age returns the age of a time in days.
func Age(t time.Time) int {
	return DiffDays(time.Now(), t)
}

// ======================================================================
= Timezone Conversion
// ======================================================================

// ConvertTimezone converts a time to a different timezone.
func ConvertTimezone(t time.Time, timezone string) (time.Time, error) {
	loc, err := GetTimezone(timezone)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(loc), nil
}

// ConvertToUTC converts a time to UTC.
func ConvertToUTC(t time.Time) time.Time {
	return t.UTC()
}

// ConvertToLocal converts a time to local timezone.
func ConvertToLocal(t time.Time) time.Time {
	return t.Local()
}

// ======================================================================
= Time Helpers for Database
// ======================================================================

// NowUTC returns the current time in UTC.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// NowLocal returns the current time in local timezone.
func NowLocal() time.Time {
	return time.Now()
}

// ZeroTime returns a zero time (time.Time{}).
func ZeroTime() time.Time {
	return time.Time{}
}

// IsZero checks if a time is zero.
func IsZero(t time.Time) bool {
	return t.IsZero()
}

// IsValid checks if a time is valid (non-zero).
func IsValid(t time.Time) bool {
	return !t.IsZero()
}

// TruncateToDay truncates a time to the start of the day.
func TruncateToDay(t time.Time) time.Time {
	return StartOfDay(t)
}

// TruncateToHour truncates a time to the start of the hour.
func TruncateToHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

// ======================================================================
= Slice Operations
// ======================================================================

// TimeRange returns a slice of times between start and end with the given interval.
func TimeRange(start, end time.Time, interval time.Duration) []time.Time {
	if start.After(end) {
		return []time.Time{}
	}
	if interval <= 0 {
		interval = time.Hour
	}
	times := []time.Time{}
	for t := start; !t.After(end); t = t.Add(interval) {
		times = append(times, t)
	}
	return times
}

// DateRange returns a slice of dates between start and end.
func DateRange(start, end time.Time) []time.Time {
	return TimeRange(StartOfDay(start), StartOfDay(end), 24*time.Hour)
}

// ======================================================================
= Common Time Constants
// ======================================================================

// Time constants for common durations.
var (
	Day   = 24 * time.Hour
	Week  = 7 * Day
	Month = 30 * Day
	Year  = 365 * Day
)

// ======================================================================
= Current Time Helpers
// ======================================================================

// CurrentTimestamp returns the current Unix timestamp.
func CurrentTimestamp() int64 {
	return time.Now().Unix()
}

// CurrentTimestampMilli returns the current Unix timestamp in milliseconds.
func CurrentTimestampMilli() int64 {
	return time.Now().UnixMilli()
}

// CurrentTimestampNano returns the current Unix timestamp in nanoseconds.
func CurrentTimestampNano() int64 {
	return time.Now().UnixNano()
}

// CurrentDate returns the current date as a string (YYYY-MM-DD).
func CurrentDate() string {
	return FormatDate(time.Now())
}

// CurrentDateTime returns the current date and time as a string (YYYY-MM-DD HH:MM:SS).
func CurrentDateTime() string {
	return FormatDateTime(time.Now())
}

// ======================================================================
= Testing Helpers
// ======================================================================

// MockTime returns a fixed time for testing.
func MockTime(year int, month time.Month, day, hour, min, sec, nsec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, nsec, time.UTC)
}

// MockDate returns a fixed date (midnight) for testing.
func MockDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// MockTimeWithLocation returns a fixed time with a location for testing.
func MockTimeWithLocation(year int, month time.Month, day, hour, min, sec, nsec int, loc *time.Location) time.Time {
	return time.Date(year, month, day, hour, min, sec, nsec, loc)
}

// ======================================================================
= Duration Helpers
// ======================================================================

// HumanDuration returns a human-readable representation of a duration.
func HumanDuration(d time.Duration) string {
	if d < time.Second {
		return "less than a second"
	}
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	if d < 7*24*time.Hour {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days", days)
	}
	if d < 30*24*time.Hour {
		weeks := int(d.Hours() / (24 * 7))
		return fmt.Sprintf("%d weeks", weeks)
	}
	if d < 365*24*time.Hour {
		months := int(d.Hours() / (24 * 30))
		return fmt.Sprintf("%d months", months)
	}
	years := int(d.Hours() / (24 * 365))
	return fmt.Sprintf("%d years", years)
}

// MustParseDuration parses a duration or panics.
func MustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// MustParseRelativeTime parses a relative time or panics.
func MustParseRelativeTime(s string) time.Duration {
	d, err := ParseRelativeTime(s)
	if err != nil {
		panic(err)
	}
	return d
}