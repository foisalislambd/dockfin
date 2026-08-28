package services

import "testing"

func TestClassifyHTTPReady(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code int
		want httpReadyKind
	}{
		{200, httpReadyUp},
		{301, httpReadyUp},
		{401, httpReadyUp},
		{403, httpReadyUp},
		{404, httpReadyMissing},
		{500, httpReadyStarting},
		{502, httpReadyStarting},
		{503, httpReadyStarting},
		{504, httpReadyStarting},
		{0, httpReadyStarting},
	}
	for _, tc := range cases {
		if got := classifyHTTPReady(tc.code); got != tc.want {
			t.Fatalf("code %d: got %v want %v", tc.code, got, tc.want)
		}
	}
}
