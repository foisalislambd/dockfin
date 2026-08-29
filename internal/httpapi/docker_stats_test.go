package httpapi

import "testing"

func TestParseDockerStatsJSON(t *testing.T) {
	out := `{"BlockIO":"1.2MB / 3.4MB","CPUPerc":"5.70%","Container":"abc","ID":"abc","MemPerc":"1.56%","MemUsage":"125.5MiB / 7.76GiB","Name":"dockfin-svc-7b8d804e-duplicati-1","NetIO":"1kB / 2kB","PIDs":12}
{"Name":"other","CPUPerc":"0.00%","MemUsage":"10MiB / 512MiB","MemPerc":"2.00%","NetIO":"0B / 0B","BlockIO":"0B / 0B"}`

	rows := parseDockerStatsJSON(out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	if rows[0].Name != "dockfin-svc-7b8d804e-duplicati-1" {
		t.Fatalf("name: %q", rows[0].Name)
	}
	if rows[0].CPUPerc != "5.70" {
		t.Fatalf("cpu: %q", rows[0].CPUPerc)
	}
	if rows[0].BlockIO != "1.2MB / 3.4MB" {
		t.Fatalf("block: %q", rows[0].BlockIO)
	}
	if rows[1].Name != "other" {
		t.Fatalf("second name: %q", rows[1].Name)
	}
}

func TestParseDockerStatsJSONNameFallback(t *testing.T) {
	rows := parseDockerStatsJSON(`{"Container":"only-id","CPUPerc":"1%"}`)
	if len(rows) != 1 || rows[0].Name != "only-id" {
		t.Fatalf("%+v", rows)
	}
}
