package storetests

import (
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/traffic"
	"github.com/google/uuid"
)

func TestTrafficRejectsEmptyRowsAndReportsTheRest(t *testing.T) {
	w := newWorld(t, "traffic@example.com", "traffic")
	server, err := w.st.CreateServer(t.Context(), w.team.ID, nil, "edge", "", "203.0.113.10", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	err = w.st.AddTraffic(t.Context(), server.ID, now, []traffic.Counted{
		{Hit: traffic.Hit{Host: "", Path: "/", Method: "GET", Class: "2xx", Client: "203.0.113.8"}, N: 4},
		{Hit: traffic.Hit{Host: "app.example.com", Path: "/../secret", Method: "GET", Class: "2xx", Client: "203.0.113.8"}, N: 0},
		{Hit: traffic.Hit{Host: "app.example.com", Path: "/health", Method: "GET", Class: "2xx", Client: "203.0.113.8", At: now}, N: 3},
		{Hit: traffic.Hit{Host: "app.example.com", Path: "/health", Method: "GET", Class: "2xx", Client: "203.0.113.8", At: now}, N: 2},
		{Hit: traffic.Hit{Host: "other.example.com", Path: "/admin", Method: "POST", Class: "4xx", Client: "198.51.100.4", At: now.Add(-time.Hour)}, N: 1},
	}, "full", 7)
	if err != nil {
		t.Fatal(err)
	}
	report, err := w.st.TrafficReport(t.Context(), server.ID, now.Add(-24*time.Hour), nil, "hour")
	if err != nil {
		t.Fatal(err)
	}
	if report.Requests != 6 {
		t.Fatalf("requests %d", report.Requests)
	}
	filtered, err := w.st.TrafficReport(t.Context(), server.ID, now.Add(-24*time.Hour), []string{"app.example.com"}, "day")
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Requests != 5 {
		t.Fatalf("filtered %d", filtered.Requests)
	}
	none, err := w.st.TrafficReport(t.Context(), server.ID, now.Add(-24*time.Hour), []string{}, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if none.Requests != 0 {
		t.Fatalf("empty host list %d", none.Requests)
	}
	if err := w.st.ClearTrafficClients(t.Context(), server.ID); err != nil {
		t.Fatal(err)
	}
	cleared, err := w.st.TrafficReport(t.Context(), server.ID, now.Add(-24*time.Hour), nil, "hour")
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.Clients) != 0 {
		t.Fatalf("clients left %v", cleared.Clients)
	}
	if cleared.Requests != 6 {
		t.Fatalf("requests after client wipe %d", cleared.Requests)
	}
	missing, err := w.st.TrafficReport(t.Context(), uuid.New(), now.Add(-time.Hour), nil, "hour")
	if err != nil || missing.Requests != 0 {
		t.Fatalf("missing server %v %+v", err, missing)
	}
}
