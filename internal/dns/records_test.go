package dns

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestOwnershipCommentRoundTrip(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	comment := OwnershipComment(id)
	if !CommentMatches("  "+comment+"  ", id) || CommentMatches(comment, uuid.New()) {
		t.Fatal("comment match")
	}
	got, ok := RecordIDFromComment(comment)
	if !ok || got != id {
		t.Fatalf("%s %v", got, ok)
	}
	for _, bad := range []string{"", "managed-by: someone", "managed-by: dockfin not-a-uuid", comment + " extra"} {
		if _, ok := RecordIDFromComment(bad); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestSyncSkipsUnpublishableAndMissingZone(t *testing.T) {
	team := uuid.New()
	res := uuid.New()
	led := newMem(nil)
	warns := SyncHostnames(context.Background(), led, SyncInput{
		TeamID: team, ResourceID: res, ResourceType: "application",
		Hosts:      []string{"app.1.2.3.4.sslip.io", "localhost", "not a host", ""},
		Content:    "1.2.3.4",
		RecordType: "A",
	})
	if len(warns) != 0 || len(led.records) != 0 {
		t.Fatalf("warns=%v records=%v", warns, led.records)
	}
	if SyncHostnames(context.Background(), nil, SyncInput{TeamID: team, ResourceID: res}) != nil {
		t.Fatal("nil ledger")
	}
	led = newMem([]Zone{{ID: uuid.New(), TokenID: uuid.New(), ProviderZoneID: "z", Name: "other.com"}})
	warns = SyncHostnames(context.Background(), led, SyncInput{
		TeamID: team, ResourceID: res, ResourceType: "application",
		Hosts: []string{"app.example.com"}, Content: "1.2.3.4", RecordType: "A",
	})
	if len(warns) != 1 || !strings.Contains(warns[0], "no synced Cloudflare zone") {
		t.Fatalf("%v", warns)
	}
	warns = SyncHostnames(context.Background(), led, SyncInput{
		TeamID: team, ResourceID: res, ResourceType: "application",
		Hosts: []string{"app.other.com"}, Content: "", RecordType: "CNAME",
	})
	if len(warns) != 1 || !strings.Contains(warns[0], "no public address") {
		t.Fatalf("%v", warns)
	}
}

func TestCloudflareUpsertAndRecordCRUD(t *testing.T) {
	var finds int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "denied", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/dns_records"):
			finds++
			if finds == 1 {
				_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"rec1","type":"A","name":"app.example.com"}]}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/zones") && r.URL.Query().Get("name") != "":
			name := r.URL.Query().Get("name")
			if name != "example.com" {
				_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zid","name":"Example.COM"}]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/dns_records"):
			if !strings.Contains(string(body), `"content":"1.2.3.4"`) || !strings.Contains(string(body), `"proxied":false`) {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"created"}}`))
		case r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rec1"}}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.String(), http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewCloudflare("  tok  ")
	c.Base = srv.URL
	c.HTTP = srv.Client()

	created, err := c.UpsertA("App.Example.com.", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if created.Action != "created" || created.RecordID != "created" || created.Zone != "example.com" || created.Type != "A" {
		t.Fatalf("%+v", created)
	}
	updated, err := c.UpsertA("app.example.com", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Action != "updated" || updated.RecordID != "rec1" {
		t.Fatalf("%+v", updated)
	}
	if _, err := c.UpsertA("", "1.2.3.4"); err == nil {
		t.Fatal("empty hostname")
	}
	if _, err := NewCloudflare("").UpsertA("app.example.com", "1.2.3.4"); err == nil {
		t.Fatal("empty token")
	}
	if _, err := NewCloudflare("").ListZones(); err == nil {
		t.Fatal("empty token zones")
	}
}

func TestCloudflareRecordFiltersAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/dns_records") && !strings.Contains(r.URL.Path, "/rec"):
			_, _ = w.Write([]byte(`{"success":true,"result":[
				{"id":"a","type":"A","name":"App.Example.com.","content":"1.2.3.4","comment":"c"},
				{"id":"aaaa","type":"aaaa","name":"app.example.com","content":"2001:db8::1"},
				{"id":"cname","type":"CNAME","name":"app.example.com","content":"origin.example.com"},
				{"id":"txt","type":"TXT","name":"app.example.com","content":"v"},
				{"id":"other","type":"A","name":"other.example.com","content":"9.9.9.9"}
			]}`))
		case r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"success":true,"result":{}}`))
		case r.Method == http.MethodPatch:
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"record locked"}]}`))
		case r.Method == http.MethodDelete:
			http.Error(w, "missing", http.StatusNotFound)
		case strings.Contains(r.URL.Path, "/rec1"):
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rec1","type":"a","name":"App.Example.com.","content":"1.2.3.4","comment":"managed"}}`))
		default:
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"zone missing"}]}`))
		}
	}))
	t.Cleanup(srv.Close)
	c := NewCloudflare("tok")
	c.Base = srv.URL
	c.HTTP = srv.Client()

	recs, err := c.ListAddressRecords("zid", "App.Example.com.")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("%#v", recs)
	}
	for _, rec := range recs {
		if rec.Name != "app.example.com" || (rec.Type != "A" && rec.Type != "AAAA" && rec.Type != "CNAME") {
			t.Fatalf("%#v", rec)
		}
	}
	if _, err := c.CreateRecord("zid", RemoteRecord{Type: "A", Name: "app.example.com", Content: "1.2.3.4"}); err == nil {
		t.Fatal("expected missing id")
	}
	if err := c.UpdateContent("zid", "rec1", "1.2.3.5"); err == nil || !strings.Contains(err.Error(), "record locked") {
		t.Fatalf("%v", err)
	}
	got, err := c.GetRecord("zid", "rec1")
	if err != nil || got.Type != "A" || got.Name != "app.example.com" || got.Comment != "managed" {
		t.Fatalf("%+v %v", got, err)
	}
	if err := c.DeleteRecord("zid", "gone"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("%v", err)
	}
	if _, err := c.UpsertA("nope.invalid", "1.2.3.4"); err == nil || !strings.Contains(err.Error(), "zone") {
		t.Fatalf("%v", err)
	}
}
