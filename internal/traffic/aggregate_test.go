package traffic

import (
	"strings"
	"testing"
	"time"
)

func TestParseLineUsesThePeerAndDropsQuery(t *testing.T) {
	line := `{"ClientHost":"203.0.113.9:52314","RequestMethod":"GET","RequestPath":"/hello?x=1","RequestHost":"App.Example.com:443","DownstreamStatus":201,"StartUTC":"2026-10-28T16:07:45.5Z","RequestHeaders":{"Cf-Connecting-Ip":"198.51.100.4","X-Forwarded-For":"203.0.113.50"}}`
	hit, ok := ParseLine(line)
	if !ok {
		t.Fatal("line was rejected")
	}
	if hit.Host != "app.example.com" || hit.Path != "/hello" || hit.Method != "GET" || hit.Class != "2xx" {
		t.Fatalf("%+v", hit)
	}
	if hit.Client != "203.0.113.9" {
		t.Fatalf("client %s", hit.Client)
	}
	if hit.At.Minute() != 7 || hit.At.Second() != 45 {
		t.Fatalf("time %s", hit.At)
	}
	if _, ok := ParseLine("not json"); ok {
		t.Fatal("plain text was accepted")
	}
	if _, ok := ParseLine(`{"RequestPath":"/x","DownstreamStatus":200}`); ok {
		t.Fatal("a line without a host was accepted")
	}
}

func TestSplitHostsReadsEveryDomain(t *testing.T) {
	got := SplitHosts("https://App.Example.com, https://www.example.com:8443/docs")
	if len(got) != 2 || got[0] != "app.example.com" || got[1] != "www.example.com" {
		t.Fatalf("%v", got)
	}
}

func TestApplyIPMode(t *testing.T) {
	if got := ApplyIPMode("203.0.113.9", "anonymized"); got != "203.0.113.0/24" {
		t.Fatalf("ipv4 %s", got)
	}
	if got := ApplyIPMode("2001:db8:1:2::5", "anonymized"); !strings.HasSuffix(got, "/48") || strings.Contains(got, ":5") {
		t.Fatalf("ipv6 %s", got)
	}
	if ApplyIPMode("203.0.113.9", "off") != "" {
		t.Fatal("off mode kept an address")
	}
	if ApplyIPMode("203.0.113.9", "full") != "203.0.113.9" {
		t.Fatal("full mode changed the address")
	}
}

func TestSampleKeepsTheStartThenScales(t *testing.T) {
	var hits []Hit
	for i := 0; i < 15; i++ {
		hits = append(hits, Hit{Host: "a.example", Path: "/", Method: "GET", Class: "2xx"})
	}
	got := Sample(hits, 10)
	exact := 0
	scaled := 0
	for _, row := range got {
		if row.N == 1 {
			exact++
		}
		if row.N == 10 {
			scaled++
		}
	}
	if exact != 10 || scaled == 0 {
		t.Fatalf("exact %d scaled %d", exact, scaled)
	}
	if len(Sample(hits, 0)) != len(hits) {
		t.Fatal("threshold 0 dropped requests")
	}
}

func TestBucketTruncatesToTheMinute(t *testing.T) {
	got := Bucket(time.Date(2026, 10, 28, 16, 7, 45, 0, time.UTC))
	if got.Second() != 0 || got.Minute() != 7 {
		t.Fatalf("%s", got)
	}
}
