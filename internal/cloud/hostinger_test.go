package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostingerCatalogFiltersKVM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("auth %s", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`[{"id":"hostingercom-vps-kvm1","name":"KVM 1"},{"id":"game-panel","name":"Game"}]`))
	}))
	defer srv.Close()
	items, err := (Hostinger{Token: "tok", BaseURL: srv.URL}).Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["id"] != "hostingercom-vps-kvm1" {
		t.Fatalf("%v", items)
	}
}

func TestHostingerPurchaseRequiresPayment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"ord_1","status":"pending","message":"waiting for payment"}`))
	}))
	defer srv.Close()
	_, err := (Hostinger{Token: "tok", BaseURL: srv.URL}).Purchase(context.Background(), PurchaseOrder{
		ItemID: "hostingercom-vps-kvm1", DataCenterID: 1, TemplateID: 2, PublicKey: "ssh-ed25519 AAAA",
	})
	if err == nil || !stringsContains(err.Error(), "waiting for payment") {
		t.Fatal(err)
	}
}

func TestVirtualMachinePublicIP(t *testing.T) {
	vm := VirtualMachine{IPv4: []struct {
		Address string `json:"address"`
	}{{Address: "203.0.113.5"}}}
	if vm.PublicIP() != "203.0.113.5" {
		t.Fatal(vm.PublicIP())
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && contains(s, sub)))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
