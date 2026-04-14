package cron

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Schedule represents a parsed cron expression with five fields:
// minute, hour, day-of-month, month, day-of-week.
type Schedule struct {
	minutes    []int
	hours      []int
	daysOfMonth []int
	months     []int
	daysOfWeek []int
}

// Parse parses a standard 5-field cron expression.
// Supported syntax: *, lists (1,3,5), ranges (1-5), steps (*/15, 1-5/2).
func Parse(expr string) (Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return Schedule{}, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return Schedule{}, fmt.Errorf("minute: %w", err)
	}

	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return Schedule{}, fmt.Errorf("hour: %w", err)
	}

	daysOfMonth, err := parseField(fields[2], 1, 31)
	if err != nil {
		return Schedule{}, fmt.Errorf("day-of-month: %w", err)
	}

	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return Schedule{}, fmt.Errorf("month: %w", err)
	}

	daysOfWeek, err := parseField(fields[4], 0, 6)
	if err != nil {
		return Schedule{}, fmt.Errorf("day-of-week: %w", err)
	}

	return Schedule{
		minutes:     minutes,
		hours:       hours,
		daysOfMonth: daysOfMonth,
		months:      months,
		daysOfWeek:  daysOfWeek,
	}, nil
}

// Next returns the next time after from that matches the schedule.
func (s Schedule) Next(from time.Time) time.Time {
	// Start from the next minute boundary.
	t := from.Truncate(time.Minute).Add(time.Minute)

	// Safety limit: don't search more than 4 years ahead.
	limit := from.Add(4 * 365 * 24 * time.Hour)

	for t.Before(limit) {
		if !contains(s.months, int(t.Month())) {
			// Advance to first day of next matching month.
			t = advanceMonth(t, s.months)
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
			continue
		}

		if !contains(s.daysOfMonth, t.Day()) || !contains(s.daysOfWeek, int(t.Weekday())) {
			t = t.AddDate(0, 0, 1)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			continue
		}

		if !contains(s.hours, t.Hour()) {
			t = t.Add(time.Hour)
			t = t.Truncate(time.Hour)
			continue
		}

		if !contains(s.minutes, t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	// Should not happen for valid schedules within 4 years.
	return limit
}

func advanceMonth(t time.Time, months []int) time.Time {
	cur := int(t.Month())
	year := t.Year()

	for _, m := range months {
		if m > cur {
			return time.Date(year, time.Month(m), 1, 0, 0, 0, 0, t.Location())
		}
	}
	// Wrap to next year, first matching month.
	return time.Date(year+1, time.Month(months[0]), 1, 0, 0, 0, 0, t.Location())
}

func contains(vals []int, v int) bool {
	for _, val := range vals {
		if val == v {
			return true
		}
	}
	return false
}

func parseField(field string, min, max int) ([]int, error) {
	var result []int
	parts := strings.Split(field, ",")

	for _, part := range parts {
		vals, err := parsePart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, vals...)
	}

	sort.Ints(result)
	// Deduplicate.
	deduped := result[:0]
	seen := make(map[int]bool)
	for _, v := range result {
		if !seen[v] {
			seen[v] = true
			deduped = append(deduped, v)
		}
	}

	if len(deduped) == 0 {
		return nil, fmt.Errorf("empty field")
	}

	return deduped, nil
}

func parsePart(part string, min, max int) ([]int, error) {
	// Check for step: */n or range/n.
	stepStr := ""
	if idx := strings.Index(part, "/"); idx != -1 {
		stepStr = part[idx+1:]
		part = part[:idx]
	}

	var rangeStart, rangeEnd int

	if part == "*" {
		rangeStart = min
		rangeEnd = max
	} else if idx := strings.Index(part, "-"); idx != -1 {
		var err error
		rangeStart, err = strconv.Atoi(part[:idx])
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", part[:idx])
		}
		rangeEnd, err = strconv.Atoi(part[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", part[idx+1:])
		}
		if rangeStart > rangeEnd {
			return nil, fmt.Errorf("invalid range %d-%d", rangeStart, rangeEnd)
		}
	} else {
		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", part)
		}
		rangeStart = val
		rangeEnd = val
	}

	if rangeStart < min || rangeEnd > max {
		return nil, fmt.Errorf("value out of range [%d, %d]", min, max)
	}

	step := 1
	if stepStr != "" {
		var err error
		step, err = strconv.Atoi(stepStr)
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step %q", stepStr)
		}
	}

	var vals []int
	for i := rangeStart; i <= rangeEnd; i += step {
		vals = append(vals, i)
	}

	return vals, nil
}
