package sshx

import "testing"

func TestPoolKeySeparatesCredentialsAndHostKeys(t *testing.T) {
	p := NewPool()
	base := Target{Host: "1.2.3.4", Port: 22, User: "root", ExpectedFingerprint: "SHA256:abc"}
	a := base
	a.PrivateKey = []byte("team-a-key")
	b := base
	b.PrivateKey = []byte("team-b-key")
	c := base
	c.PrivateKey = []byte("team-a-key")
	c.ExpectedFingerprint = "SHA256:other"
	same := base
	same.PrivateKey = []byte("team-a-key")

	ka, kb, kc, ks := p.key(a), p.key(b), p.key(c), p.key(same)
	if ka == kb {
		t.Fatal("different private keys shared a pool slot")
	}
	if ka == kc {
		t.Fatal("different host key pins shared a pool slot")
	}
	if ka != ks {
		t.Fatal("identical targets must share a pool slot")
	}
}
