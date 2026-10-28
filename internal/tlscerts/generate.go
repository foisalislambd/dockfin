// Package tlscerts builds the CA and server certificates Dockfin mounts into databases.
package tlscerts

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// Material is a PEM certificate and its matching private key.
type Material struct {
	CertPEM    string
	KeyPEM     string
	CommonName string
	NotAfter   time.Time
}

// GenerateCA creates a self-signed ECDSA P-256 CA.
func GenerateCA(commonName string, validFor time.Duration) (Material, error) {
	if commonName == "" {
		return Material{}, fmt.Errorf("common name required")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Material{}, err
	}
	serial, err := randomSerial()
	if err != nil {
		return Material{}, err
	}
	now := time.Now().UTC()
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, Organization: []string{"Dockfin"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(validFor),
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return Material{}, err
	}
	return materialFrom(commonName, der, key, tpl.NotAfter)
}

// GenerateLeaf signs a server certificate with the CA.
// DNS names and IPs are written as subject alternative names. The common name is also a DNS SAN.
func GenerateLeaf(ca Material, commonName string, dnsNames []string, ips []net.IP, validFor time.Duration) (Material, error) {
	if commonName == "" {
		return Material{}, fmt.Errorf("common name required")
	}
	caCert, caKey, err := parseCA(ca)
	if err != nil {
		return Material{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Material{}, err
	}
	serial, err := randomSerial()
	if err != nil {
		return Material{}, err
	}
	names := []string{commonName}
	seen := map[string]bool{commonName: true}
	for _, name := range dnsNames {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	now := time.Now().UTC()
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName, Organization: []string{"Dockfin"}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(validFor),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     names,
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return Material{}, err
	}
	return materialFrom(commonName, der, key, tpl.NotAfter)
}

// LeafCovers reports whether leafPEM was signed by caPEM and includes every required DNS name and IP.
func LeafCovers(leafPEM, caPEM string, dnsNames []string, ips []net.IP) error {
	leafBlock, _ := pem.Decode([]byte(leafPEM))
	if leafBlock == nil {
		return fmt.Errorf("leaf certificate is not PEM")
	}
	leaf, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		return err
	}
	caBlock, _ := pem.Decode([]byte(caPEM))
	if caBlock == nil {
		return fmt.Errorf("ca certificate is not PEM")
	}
	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}
	if err := leaf.CheckSignatureFrom(ca); err != nil {
		return fmt.Errorf("certificate was not signed by the current CA")
	}
	have := map[string]bool{}
	for _, name := range leaf.DNSNames {
		have[name] = true
	}
	for _, name := range dnsNames {
		if name == "" {
			continue
		}
		if !have[name] {
			return fmt.Errorf("certificate is missing DNS SAN %s", name)
		}
	}
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		found := false
		for _, got := range leaf.IPAddresses {
			if got.Equal(ip) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("certificate is missing IP SAN %s", ip)
		}
	}
	return nil
}

func parseCA(ca Material) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(ca.CertPEM))
	if block == nil {
		return nil, nil, fmt.Errorf("ca certificate is not PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	keyBlock, _ := pem.Decode([]byte(ca.KeyPEM))
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("ca key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("ca key is not ECDSA")
	}
	return cert, key, nil
}

func materialFrom(cn string, der []byte, key *ecdsa.PrivateKey, notAfter time.Time) (Material, error) {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return Material{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return Material{CertPEM: string(certPEM), KeyPEM: string(keyPEM), CommonName: cn, NotAfter: notAfter}, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}
