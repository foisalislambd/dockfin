package tlscerts

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"
)

func TestLeafIsSignedByCA(t *testing.T) {
	ca, err := GenerateCA("Dockfin CA", 10*365*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := GenerateLeaf(ca, "db-1", nil, []net.IP{net.ParseIP("1.2.3.4")}, 365*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	caCert := parseCert(t, ca.CertPEM)
	if !caCert.IsCA {
		t.Fatal("CA bit not set")
	}
	got := parseCert(t, leaf.CertPEM)
	if err := got.CheckSignatureFrom(caCert); err != nil {
		t.Fatal(err)
	}
	if got.Subject.CommonName != "db-1" {
		t.Fatalf("cn %s", got.Subject.CommonName)
	}
	if len(got.DNSNames) != 1 || got.DNSNames[0] != "db-1" {
		t.Fatalf("dns %#v", got.DNSNames)
	}
	if len(got.IPAddresses) != 1 || !got.IPAddresses[0].Equal(net.ParseIP("1.2.3.4")) {
		t.Fatalf("ips %#v", got.IPAddresses)
	}
	if leaf.KeyPEM == "" || leaf.KeyPEM == ca.KeyPEM {
		t.Fatal("leaf key missing or reused the CA key")
	}
	if err := LeafCovers(leaf.CertPEM, ca.CertPEM, []string{"db-1"}, []net.IP{net.ParseIP("1.2.3.4")}); err != nil {
		t.Fatal(err)
	}
	other, err := GenerateCA("Other", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := LeafCovers(leaf.CertPEM, other.CertPEM, []string{"db-1"}, nil); err == nil {
		t.Fatal("leaf signed by another CA was accepted")
	}
	if err := LeafCovers(leaf.CertPEM, ca.CertPEM, []string{"db-1", "dockfin-db-1"}, nil); err == nil {
		t.Fatal("missing DNS SAN was accepted")
	}
	if err := LeafCovers(leaf.CertPEM, ca.CertPEM, []string{"db-1"}, []net.IP{net.ParseIP("9.9.9.9")}); err == nil {
		t.Fatal("missing IP SAN was accepted")
	}
}

func parseCert(t *testing.T, pemText string) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		t.Fatal("pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}
