package cron_test

import (
	"testing"
	"time"

	"github.com/narrowcastdev/cronguard/internal/cron"
)

func TestParseValid(t *testing.T) {
	cases := []string{
		"* * * * *",
		"0 3 * * *",
		"*/15 * * * *",
		"0 0 1 * *",
		"30 4 1-7 * 1",
		"0 0 * * 0,6",
		"0 9-17 * * 1-5",
		"0 */6 * * *",
		"5 4 * * 0",
		"0 0 1,15 * *",
	}
	for _, expr := range cases {
		_, err := cron.Parse(expr)
		if err != nil {
			t.Errorf("Parse(%q) returned error: %v", expr, err)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []string{
		"",
		"* * *",
		"* * * * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * 32 * *",
		"* * * 0 *",
		"* * * 13 *",
		"* * * * 7",
		"abc * * * *",
		"1-0 * * * *",
	}
	for _, expr := range cases {
		_, err := cron.Parse(expr)
		if err == nil {
			t.Errorf("Parse(%q) should have returned error", expr)
		}
	}
}

func TestNextEveryMinute(t *testing.T) {
	s, _ := cron.Parse("* * * * *")
	from := time.Date(2026, 4, 13, 10, 30, 0, 0, time.UTC)
	next := s.Next(from)
	expected := time.Date(2026, 4, 13, 10, 31, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextDailyAt3AM(t *testing.T) {
	s, _ := cron.Parse("0 3 * * *")
	from := time.Date(2026, 4, 13, 3, 0, 0, 0, time.UTC)
	next := s.Next(from)
	expected := time.Date(2026, 4, 14, 3, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextEvery15Min(t *testing.T) {
	s, _ := cron.Parse("*/15 * * * *")
	from := time.Date(2026, 4, 13, 10, 16, 0, 0, time.UTC)
	next := s.Next(from)
	expected := time.Date(2026, 4, 13, 10, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextMonthlyFirstDay(t *testing.T) {
	s, _ := cron.Parse("0 0 1 * *")
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	next := s.Next(from)
	expected := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextWeekdayOnly(t *testing.T) {
	s, _ := cron.Parse("0 9 * * 1-5")
	// Sunday April 12, 2026
	from := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	next := s.Next(from)
	// Next weekday is Monday April 13
	expected := time.Date(2026, 4, 13, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextYearRollover(t *testing.T) {
	s, _ := cron.Parse("0 0 1 1 *")
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next := s.Next(from)
	expected := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextWeekend(t *testing.T) {
	s, _ := cron.Parse("0 0 * * 0,6")
	// Monday April 13, 2026
	from := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	next := s.Next(from)
	// Next Saturday is April 18
	expected := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}
