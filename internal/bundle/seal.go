package bundle

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	formatVersion = 1
	minPassphrase = 12
	argonTime     = 1
	argonMemory   = 64 * 1024
	argonThreads  = 4
	argonKeyLen   = 32
	saltLen       = 16
)

// Envelope is the file an operator moves between Dockfin instances.
// The payload is encrypted with a passphrase so the source master key never leaves the source.
type Envelope struct {
	V    int    `json:"v"`
	KDF  string `json:"kdf"`
	Salt string `json:"salt"`
	Data string `json:"data"`
}

// Seal encrypts plaintext with the passphrase. The result is JSON, not the source master key.
func Seal(passphrase string, plaintext []byte) (string, error) {
	if err := checkPassphrase(passphrase); err != nil {
		return "", err
	}
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	raw, err := json.Marshal(Envelope{
		V:    formatVersion,
		KDF:  "argon2id",
		Salt: base64.RawStdEncoding.EncodeToString(salt),
		Data: base64.RawStdEncoding.EncodeToString(sealed),
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Open decrypts a bundle produced by Seal.
func Open(passphrase, envelope string) ([]byte, error) {
	if err := checkPassphrase(passphrase); err != nil {
		return nil, err
	}
	var env Envelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(envelope)), &env); err != nil {
		return nil, fmt.Errorf("bundle is not a Dockfin secret bundle")
	}
	if env.V != formatVersion || env.KDF != "argon2id" {
		return nil, fmt.Errorf("unsupported bundle format")
	}
	salt, err := base64.RawStdEncoding.DecodeString(env.Salt)
	if err != nil || len(salt) != saltLen {
		return nil, fmt.Errorf("bundle salt is invalid")
	}
	sealed, err := base64.RawStdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("bundle data is invalid")
	}
	key := argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(sealed) < ns {
		return nil, fmt.Errorf("bundle data is invalid")
	}
	plain, err := gcm.Open(nil, sealed[:ns], sealed[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("wrong passphrase or damaged bundle")
	}
	return plain, nil
}

func checkPassphrase(passphrase string) error {
	if len(strings.TrimSpace(passphrase)) < minPassphrase {
		return fmt.Errorf("passphrase must be at least %d characters", minPassphrase)
	}
	return nil
}
