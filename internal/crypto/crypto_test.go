package crypto_test

import (
	"testing"

	"github.com/dockfin/dockfin/internal/crypto"
)

func TestBoxRoundTrip(t *testing.T) {
	box, err := crypto.NewBox("this-is-a-32-byte-test-master-key!")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := box.EncryptString("super-secret")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.DecryptString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "super-secret" {
		t.Fatalf("got %q", plain)
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := crypto.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !crypto.VerifyPassword(hash, "password123") {
		t.Fatal("verify failed")
	}
	if crypto.VerifyPassword(hash, "wrong") {
		t.Fatal("should not verify")
	}
}
