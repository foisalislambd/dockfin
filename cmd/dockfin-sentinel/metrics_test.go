package main

import (
	"testing"
)

func TestEnvAndURLTrim(t *testing.T) {
	t.Setenv("DOCKFIN_TOKEN", "")
	if env("DOCKFIN_TOKEN", "fallback") != "fallback" {
		t.Fatal(env("DOCKFIN_TOKEN", "fallback"))
	}
	t.Setenv("DOCKFIN_TOKEN", "abc")
	if env("DOCKFIN_TOKEN", "fallback") != "abc" {
		t.Fatal("override")
	}
	if stringsTrim("http://example.com///") != "http://example.com" {
		t.Fatal(stringsTrim("http://example.com///"))
	}
	if stringsTrim("http://example.com") != "http://example.com" || stringsTrim("") != "" {
		t.Fatal("plain trim")
	}
}

func TestHostMetricParsers(t *testing.T) {
	used, total := readCPU()
	if total == 0 || used > total {
		t.Fatalf("cpu used=%d total=%d", used, total)
	}
	memTotal, memUsed := readMem()
	if memTotal <= 0 || memUsed < 0 || memUsed > memTotal {
		t.Fatalf("mem used=%d total=%d", memUsed, memTotal)
	}
	diskTotal, diskUsed := readDisk("/")
	if diskTotal <= 0 || diskUsed < 0 || diskUsed > diskTotal {
		t.Fatalf("disk used=%d total=%d", diskUsed, diskTotal)
	}
	if total, used := readDisk("/no/such/dockfin-path"); total != 0 || used != 0 {
		t.Fatalf("missing path returned %d %d", total, used)
	}
	cpu, mu, mt, du, dt := sampleHostMetrics()
	if cpu < 0 || cpu > 100 || mt <= 0 || dt <= 0 || mu < 0 || du < 0 || mu > mt || du > dt {
		t.Fatalf("sample cpu=%v mem=%d/%d disk=%d/%d", cpu, mu, mt, du, dt)
	}
}
