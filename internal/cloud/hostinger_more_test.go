package cloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHostingerListsAndPurchase(t *testing.T) {
	var purchase map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("auth %s", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/vps/v1/virtual-machines":
			_, _ = w.Write([]byte(`[{"id":7,"state":"running","ipv4":[{"address":"203.0.113.8"}]}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/vps/v1/data-centers":
			_, _ = w.Write([]byte(`[{"id":1,"name":"us"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/vps/v1/templates":
			_, _ = w.Write([]byte(`[{"id":2,"name":"ubuntu"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/vps/v1/virtual-machines":
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &purchase); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"id":"ord_9","status":"ok","virtual_machine":{"id":42,"state":"starting","ipv4":[{"address":"203.0.113.9"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	h := Hostinger{Token: "tok", BaseURL: srv.URL}
	vms, err := h.VirtualMachines(context.Background())
	if err != nil || len(vms) != 1 || vms[0].ID != 7 || vms[0].State != "running" || vms[0].PublicIP() != "203.0.113.8" {
		t.Fatalf("%+v %v", vms, err)
	}
	centers, err := h.DataCenters(context.Background())
	if err != nil || !strings.Contains(string(centers), `"us"`) {
		t.Fatalf("%s %v", centers, err)
	}
	templates, err := h.Templates(context.Background())
	if err != nil || !strings.Contains(string(templates), `"ubuntu"`) {
		t.Fatalf("%s %v", templates, err)
	}
	vm, err := h.Purchase(context.Background(), PurchaseOrder{
		ItemID: "hostingercom-vps-kvm2", DataCenterID: 1, TemplateID: 2, Hostname: "app.example.com",
		PublicKey: "ssh-ed25519 AAAA", KeyName: "dockfin",
	})
	if err != nil || vm.ID != 42 || vm.PublicIP() != "203.0.113.9" {
		t.Fatalf("%+v %v", vm, err)
	}
	if purchase["item_id"] != "hostingercom-vps-kvm2" {
		t.Fatalf("purchase %#v", purchase)
	}
	setup, _ := purchase["setup"].(map[string]any)
	if setup["hostname"] != "app.example.com" || setup["data_center_id"] != float64(1) {
		t.Fatalf("setup %#v", setup)
	}
}

func TestHostingerRejectsBadCalls(t *testing.T) {
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(limited.Close)
	if _, err := (Hostinger{Token: "tok", BaseURL: limited.URL}).Catalog(context.Background()); err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatal(err)
	}
	if _, err := (Hostinger{Token: "tok", BaseURL: "https://evil.example"}).VirtualMachines(context.Background()); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatal(err)
	}
	if _, err := (Hostinger{Token: "tok"}).Purchase(context.Background(), PurchaseOrder{}); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatal(err)
	}
	v6 := VirtualMachine{IPv6: []struct {
		Address string `json:"address"`
	}{{Address: "2001:db8::1"}}}
	if v6.PublicIP() != "2001:db8::1" {
		t.Fatal(v6.PublicIP())
	}
}
