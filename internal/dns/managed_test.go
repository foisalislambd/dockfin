package dns

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMatchZoneLongestAndAmbiguous(t *testing.T) {
	token := uuid.New()
	zones := []Zone{
		{ID: uuid.New(), TokenID: token, ProviderZoneID: "z1", Name: "example.com"},
		{ID: uuid.New(), TokenID: token, ProviderZoneID: "z2", Name: "app.example.com"},
	}
	got, ok, amb := MatchZone("api.app.example.com", zones)
	if !ok || amb || got.ProviderZoneID != "z2" {
		t.Fatalf("longest: ok=%v amb=%v zone=%s", ok, amb, got.ProviderZoneID)
	}
	zones = append(zones, Zone{ID: uuid.New(), TokenID: uuid.New(), ProviderZoneID: "z3", Name: "app.example.com"})
	_, ok, amb = MatchZone("api.app.example.com", zones)
	if ok || !amb {
		t.Fatalf("ambiguous: ok=%v amb=%v", ok, amb)
	}
}

func TestPublicRecord(t *testing.T) {
	if c, typ, ok := PublicRecord("1.2.3.4"); !ok || c != "1.2.3.4" || typ != "A" {
		t.Fatalf("v4: %s %s %v", c, typ, ok)
	}
	if c, typ, ok := PublicRecord("2001:db8::1"); !ok || typ != "AAAA" || c == "" {
		t.Fatalf("v6: %s %s %v", c, typ, ok)
	}
	for _, ip := range []string{"10.1.2.3", "127.0.0.1", "169.254.1.1", "", "not-an-ip"} {
		if _, _, ok := PublicRecord(ip); ok {
			t.Fatalf("accepted %s", ip)
		}
	}
}

type memLedger struct {
	zones   []Zone
	records map[string]Record
	refs    map[uuid.UUID]map[string]struct{}
}

func newMem(zones []Zone) *memLedger {
	return &memLedger{zones: zones, records: map[string]Record{}, refs: map[uuid.UUID]map[string]struct{}{}}
}

func (m *memLedger) ListDNSZones(context.Context, uuid.UUID) ([]Zone, error) {
	return append([]Zone(nil), m.zones...), nil
}

func (m *memLedger) DNSRecordByName(_ context.Context, _ uuid.UUID, name string) (*Record, error) {
	rec, ok := m.records[name]
	if !ok {
		return nil, nil
	}
	cp := rec
	return &cp, nil
}

func (m *memLedger) SaveDNSRecord(_ context.Context, rec Record) (Record, error) {
	if existing, ok := m.records[rec.Name]; ok {
		rec.ID = existing.ID
	}
	m.records[rec.Name] = rec
	return rec, nil
}

func (m *memLedger) AddDNSRef(_ context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	if m.refs[recordID] == nil {
		m.refs[recordID] = map[string]struct{}{}
	}
	m.refs[recordID][resourceType+":"+resourceID.String()] = struct{}{}
	return nil
}

func (m *memLedger) RemoveDNSRef(_ context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (int, error) {
	key := resourceType + ":" + resourceID.String()
	if set, ok := m.refs[recordID]; ok {
		delete(set, key)
		return len(set), nil
	}
	return 0, nil
}

func (m *memLedger) OtherDNSRefs(_ context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (int, error) {
	key := resourceType + ":" + resourceID.String()
	n := 0
	for ref := range m.refs[recordID] {
		if ref != key {
			n++
		}
	}
	return n, nil
}

func (m *memLedger) DeleteDNSRecord(_ context.Context, id uuid.UUID) error {
	for name, rec := range m.records {
		if rec.ID == id {
			delete(m.records, name)
		}
	}
	delete(m.refs, id)
	return nil
}

func (m *memLedger) DNSHostsForResource(_ context.Context, _ uuid.UUID, resourceType string, resourceID uuid.UUID) ([]string, error) {
	key := resourceType + ":" + resourceID.String()
	var hosts []string
	for name, rec := range m.records {
		if _, ok := m.refs[rec.ID][key]; ok {
			hosts = append(hosts, name)
		}
	}
	return hosts, nil
}

type fakeRec struct {
	zone string
	rec  RemoteRecord
}

type fakeAPI struct {
	records []fakeRec
	listed  []string
	updated []string
	deleted []string
}

func (f *fakeAPI) ListAddressRecords(zoneID, hostname string) ([]RemoteRecord, error) {
	f.listed = append(f.listed, zoneID)
	var out []RemoteRecord
	for _, rec := range f.records {
		if rec.zone == zoneID && rec.rec.Name == hostname {
			out = append(out, rec.rec)
		}
	}
	return out, nil
}

func (f *fakeAPI) CreateRecord(zoneID string, rec RemoteRecord) (string, error) {
	rec.ID = "cf-" + rec.Name
	f.records = append(f.records, fakeRec{zone: zoneID, rec: rec})
	return rec.ID, nil
}

func (f *fakeAPI) UpdateContent(_, recordID, content string) error {
	for i := range f.records {
		if f.records[i].rec.ID == recordID {
			f.records[i].rec.Content = content
			f.updated = append(f.updated, recordID)
			return nil
		}
	}
	return errors.New("missing")
}

func (f *fakeAPI) GetRecord(_, recordID string) (RemoteRecord, error) {
	for _, rec := range f.records {
		if rec.rec.ID == recordID {
			return rec.rec, nil
		}
	}
	return RemoteRecord{}, errors.New("cloudflare API HTTP 404")
}

func (f *fakeAPI) DeleteRecord(_, recordID string) error {
	var next []fakeRec
	for _, rec := range f.records {
		if rec.rec.ID == recordID {
			f.deleted = append(f.deleted, recordID)
			continue
		}
		next = append(next, rec)
	}
	f.records = next
	return nil
}

func TestSyncCreateAdoptConflictAndRelease(t *testing.T) {
	ctx := context.Background()
	team := uuid.New()
	token := uuid.New()
	zone := Zone{ID: uuid.New(), TokenID: token, ProviderZoneID: "zone", Name: "example.com"}
	ledger := newMem([]Zone{zone})
	api := &fakeAPI{}
	app := uuid.New()
	in := SyncInput{
		TeamID: team, ResourceType: "application", ResourceID: app,
		Hosts: []string{"app.example.com", "x.sslip.io"}, Content: "1.2.3.4", RecordType: "A",
		ClientFor: func(uuid.UUID) (RecordAPI, error) { return api, nil },
	}
	if w := SyncHostnames(ctx, ledger, in); len(w) != 0 {
		t.Fatalf("create warnings: %v", w)
	}
	rec := ledger.records["app.example.com"]
	if !rec.Owned || len(api.records) != 1 || api.records[0].rec.Comment != OwnershipComment(rec.ID) || api.records[0].rec.Content != "1.2.3.4" {
		t.Fatalf("created %#v remote %#v", rec, api.records)
	}
	if _, ok := ledger.records["x.sslip.io"]; ok {
		t.Fatal("synced magic host")
	}

	in.Content = "5.6.7.8"
	if w := SyncHostnames(ctx, ledger, in); len(w) != 0 {
		t.Fatalf("update warnings: %v", w)
	}
	if api.records[0].rec.Content != "5.6.7.8" || len(api.updated) != 1 {
		t.Fatalf("content not updated: %#v", api.records)
	}

	api.records[0].rec.Comment = "edited"
	in.Content = "9.9.9.9"
	w := SyncHostnames(ctx, ledger, in)
	if len(w) != 1 || !strings.Contains(w[0], "no longer managed") || api.records[0].rec.Content != "5.6.7.8" {
		t.Fatalf("overwrite: warnings=%v content=%s", w, api.records[0].rec.Content)
	}

	other := uuid.New()
	ledger2 := newMem([]Zone{zone})
	api2 := &fakeAPI{records: []fakeRec{{zone: "zone", rec: RemoteRecord{ID: "ext", Type: "A", Name: "app.example.com", Content: "1.2.3.4"}}}}
	in2 := SyncInput{
		TeamID: team, ResourceType: "application", ResourceID: other,
		Hosts: []string{"app.example.com"}, Content: "1.2.3.4", RecordType: "A",
		ClientFor: func(uuid.UUID) (RecordAPI, error) { return api2, nil },
	}
	if w := SyncHostnames(ctx, ledger2, in2); len(w) != 0 {
		t.Fatalf("adopt: %v", w)
	}
	if ledger2.records["app.example.com"].Owned {
		t.Fatal("adopted record marked owned")
	}
	in2.Hosts = nil
	if w := SyncHostnames(ctx, ledger2, in2); len(w) != 0 {
		t.Fatalf("release adopt: %v", w)
	}
	if len(api2.deleted) != 0 || len(api2.records) != 1 {
		t.Fatal("deleted a record Dockfin does not own")
	}
	if _, ok := ledger2.records["app.example.com"]; ok {
		t.Fatal("local adopt row remains")
	}

	api3 := &fakeAPI{records: []fakeRec{{zone: "zone", rec: RemoteRecord{ID: "ext", Type: "A", Name: "app.example.com", Content: "8.8.8.8"}}}}
	ledger3 := newMem([]Zone{zone})
	in3 := in2
	in3.ClientFor = func(uuid.UUID) (RecordAPI, error) { return api3, nil }
	in3.Hosts = []string{"app.example.com"}
	in3.Content = "1.2.3.4"
	w = SyncHostnames(ctx, ledger3, in3)
	if len(w) != 1 || api3.records[0].rec.Content != "8.8.8.8" || len(ledger3.records) != 0 {
		t.Fatalf("conflict: %v remote=%s saved=%d", w, api3.records[0].rec.Content, len(ledger3.records))
	}
}

func TestSyncKeepsRecordWhileAnotherResourceReferencesIt(t *testing.T) {
	ctx := context.Background()
	team := uuid.New()
	zone := Zone{ID: uuid.New(), TokenID: uuid.New(), ProviderZoneID: "zone", Name: "example.com"}
	ledger := newMem([]Zone{zone})
	api := &fakeAPI{}
	a := uuid.New()
	b := uuid.New()
	base := SyncInput{
		TeamID: team, ResourceType: "service", Hosts: []string{"app.example.com"},
		Content: "1.2.3.4", RecordType: "A",
		ClientFor: func(uuid.UUID) (RecordAPI, error) { return api, nil },
	}
	base.ResourceID = a
	if w := SyncHostnames(ctx, ledger, base); len(w) != 0 {
		t.Fatal(w)
	}
	base.ResourceID = b
	if w := SyncHostnames(ctx, ledger, base); len(w) != 0 {
		t.Fatal(w)
	}
	base.ResourceID = a
	base.Hosts = nil
	if w := SyncHostnames(ctx, ledger, base); len(w) != 0 || len(api.deleted) != 0 {
		t.Fatalf("shared delete: %v deleted=%v", w, api.deleted)
	}
	base.ResourceID = b
	if w := SyncHostnames(ctx, ledger, base); len(w) != 0 || len(api.deleted) != 1 {
		t.Fatalf("last ref: %v deleted=%v", w, api.deleted)
	}
}

func TestSyncDeletesOnlyWhenCommentStillMatches(t *testing.T) {
	ctx := context.Background()
	team := uuid.New()
	zone := Zone{ID: uuid.New(), TokenID: uuid.New(), ProviderZoneID: "zone", Name: "example.com"}
	ledger := newMem([]Zone{zone})
	api := &fakeAPI{}
	id := uuid.New()
	in := SyncInput{
		TeamID: team, ResourceType: "application", ResourceID: id,
		Hosts: []string{"app.example.com"}, Content: "1.2.3.4", RecordType: "A",
		ClientFor: func(uuid.UUID) (RecordAPI, error) { return api, nil },
	}
	if w := SyncHostnames(ctx, ledger, in); len(w) != 0 {
		t.Fatal(w)
	}
	api.records[0].rec.Comment = "someone else"
	in.Hosts = nil
	w := SyncHostnames(ctx, ledger, in)
	if len(api.deleted) != 0 || len(w) != 1 || !strings.Contains(w[0], "left the Cloudflare record") {
		t.Fatalf("comment guard: deleted=%v warnings=%v", api.deleted, w)
	}
	if _, ok := ledger.records["app.example.com"]; ok {
		t.Fatal("local row kept after giving up ownership")
	}
}

func TestSyncDoesNotStealHostnameFromAnotherResource(t *testing.T) {
	ctx := context.Background()
	team := uuid.New()
	zone := Zone{ID: uuid.New(), TokenID: uuid.New(), ProviderZoneID: "zone", Name: "example.com"}
	ledger := newMem([]Zone{zone})
	api := &fakeAPI{}
	first := uuid.New()
	second := uuid.New()
	base := SyncInput{
		TeamID: team, ResourceType: "application", ResourceID: first,
		Hosts: []string{"app.example.com"}, Content: "1.2.3.4", RecordType: "A",
		ClientFor: func(uuid.UUID) (RecordAPI, error) { return api, nil },
	}
	if w := SyncHostnames(ctx, ledger, base); len(w) != 0 {
		t.Fatal(w)
	}
	base.ResourceID = second
	base.Content = "5.6.7.8"
	w := SyncHostnames(ctx, ledger, base)
	if len(w) != 1 || !strings.Contains(w[0], "another resource") {
		t.Fatalf("warnings: %v", w)
	}
	if api.records[0].rec.Content != "1.2.3.4" || len(api.updated) != 0 {
		t.Fatalf("record changed: %#v updates=%v", api.records, api.updated)
	}
	if _, ok := ledger.refs[ledger.records["app.example.com"].ID]["application:"+second.String()]; ok {
		t.Fatal("second resource was attached to the record")
	}
}

func TestSyncKeepsRecordInItsOriginalZone(t *testing.T) {
	ctx := context.Background()
	team := uuid.New()
	parent := Zone{ID: uuid.New(), TokenID: uuid.New(), ProviderZoneID: "parent", Name: "example.com"}
	ledger := newMem([]Zone{parent})
	api := &fakeAPI{}
	in := SyncInput{
		TeamID: team, ResourceType: "application", ResourceID: uuid.New(),
		Hosts: []string{"api.app.example.com"}, Content: "1.2.3.4", RecordType: "A",
		ClientFor: func(uuid.UUID) (RecordAPI, error) { return api, nil },
	}
	if w := SyncHostnames(ctx, ledger, in); len(w) != 0 {
		t.Fatal(w)
	}
	ledger.zones = append(ledger.zones, Zone{ID: uuid.New(), TokenID: uuid.New(), ProviderZoneID: "child", Name: "app.example.com"})
	in.Content = "1.2.3.4"
	if w := SyncHostnames(ctx, ledger, in); len(w) != 0 {
		t.Fatal(w)
	}
	if len(api.records) != 1 || api.records[0].zone != "parent" {
		t.Fatalf("records: %#v", api.records)
	}
	if api.listed[len(api.listed)-1] != "parent" {
		t.Fatalf("listed zones: %v", api.listed)
	}
}

func TestListZonesPaginates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"z2","name":"Other.COM"}],"result_info":{"total_pages":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"z1","name":"example.com"}],"result_info":{"total_pages":2}}`))
	}))
	defer srv.Close()
	c := NewCloudflare("tok")
	c.Base = srv.URL
	c.HTTP = srv.Client()
	zones, err := c.ListZones()
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 2 || zones[1].Name != "other.com" {
		t.Fatalf("%#v", zones)
	}
}
