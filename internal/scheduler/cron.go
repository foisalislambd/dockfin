package scheduler

import (
	"strconv"
	"strings"
	"time"
)

// Matches reports whether a 5-field cron expression (min hour dom mon dow) matches t.
// Supports *, N, N-M, */N, and comma lists. Dow: 0=Sunday … 6=Saturday (also 7=Sunday).
func Matches(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	min, hour, dom, mon, dow := t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())
	return matchField(fields[0], min, 0, 59) &&
		matchField(fields[1], hour, 0, 23) &&
		matchField(fields[2], dom, 1, 31) &&
		matchField(fields[3], mon, 1, 12) &&
		matchDow(fields[4], dow)
}

func matchDow(field string, dow int) bool {
	if field == "*" {
		return true
	}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(part[2:])
			if err != nil || step <= 0 {
				return false
			}
			if dow%step == 0 {
				return true
			}
			continue
		}
		if strings.Contains(part, "-") {
			ab := strings.SplitN(part, "-", 2)
			if len(ab) != 2 {
				return false
			}
			a, ea := strconv.Atoi(ab[0])
			b, eb := strconv.Atoi(ab[1])
			if ea != nil || eb != nil {
				return false
			}
			if a == 7 {
				a = 0
			}
			if b == 7 {
				b = 0
			}
			if a <= b {
				if dow >= a && dow <= b {
					return true
				}
			} else if dow >= a || dow <= b {
				return true
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return false
		}
		if n == 7 {
			n = 0
		}
		if n == dow {
			return true
		}
	}
	return false
}

func matchField(field string, value, min, max int) bool {
	if field == "*" {
		return true
	}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(part[2:])
			if err != nil || step <= 0 {
				return false
			}
			for v := min; v <= max; v += step {
				if v == value {
					return true
				}
			}
			continue
		}
		if strings.Contains(part, "-") {
			ab := strings.SplitN(part, "-", 2)
			if len(ab) != 2 {
				return false
			}
			a, ea := strconv.Atoi(ab[0])
			b, eb := strconv.Atoi(ab[1])
			if ea != nil || eb != nil || a < min || b > max || a > b {
				return false
			}
			if value >= a && value <= b {
				return true
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < min || n > max {
			return false
		}
		if n == value {
			return true
		}
	}
	return false
}
