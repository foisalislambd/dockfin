package dns

import "testing"

func TestZoneFromHost(t *testing.T) {
	cands := candidateZones("app.example.com")
	if len(cands) < 2 || cands[0] != "app.example.com" || cands[1] != "example.com" {
		t.Fatalf("got %#v", cands)
	}
	uk := candidateZones("foo.bar.co.uk")
	if len(uk) < 3 || uk[len(uk)-1] != "co.uk" || uk[len(uk)-2] != "bar.co.uk" {
		t.Fatalf("got %#v", uk)
	}
}
