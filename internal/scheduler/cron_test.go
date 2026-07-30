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
		{"5 12 * * 7", false}, // 7=Sunday, today is Thursday
		{"0 0 * * *", false},
		{"5-10 12 * * *", true},
		{"1,5,9 12 * * *", true},
		{"bad", false},
		{"* * *", false},
	}
	for _, c := range cases {
		if got := Matches(c.expr, tm); got != c.want {
			t.Fatalf("%q => %v want %v", c.expr, got, c.want)
		}
	}

	sun, _ := time.Parse(time.RFC3339, "2026-07-26T00:00:00Z") // Sunday
	if !Matches("0 0 * * 0", sun) || !Matches("0 0 * * 7", sun) {
		t.Fatal("Sunday dow 0/7 should match")
	}
}
