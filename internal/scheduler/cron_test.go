package scheduler

import (
	"testing"
	"time"
)

func TestMatches(t *testing.T) {
	tm, err := time.Parse(time.RFC3339, "2026-07-30T12:05:00Z")
	if err != nil {
		t.Fatal(err)
	}
	// Thursday = 4
	cases := []struct {
		expr string
		want bool
	}{
		{"5 12 * * *", true},
		{"0 12 * * *", false},
		{"*/5 12 * * *", true},
		{"* * * * *", true},
		{"5 12 * * 4", true},
		{"5 12 * * 1", false},
		{"bad", false},
	}
	for _, c := range cases {
		if got := Matches(c.expr, tm); got != c.want {
			t.Fatalf("%q => %v want %v", c.expr, got, c.want)
		}
	}
}
