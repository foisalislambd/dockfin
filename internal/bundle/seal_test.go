package bundle

import "testing"

func TestSealRoundTrip(t *testing.T) {
	secret := []byte(`{"private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\nabc"}`)
	sealed, err := Seal("correct horse battery", secret)
	if err != nil {
		t.Fatal(err)
	}
	if sealed == string(secret) || contains(sealed, "OPENSSH") {
		t.Fatal("bundle leaked plaintext")
	}
	got, err := Open("correct horse battery", sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(secret) {
		t.Fatalf("got %s", got)
	}
	if _, err := Open("wrong passphrase", sealed); err == nil {
		t.Fatal("expected passphrase failure")
	}
	if _, err := Seal("short", secret); err == nil {
		t.Fatal("expected short passphrase to fail")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (stringIndex(s, sub) >= 0)))
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
