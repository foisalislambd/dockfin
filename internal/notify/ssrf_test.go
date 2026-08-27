package notify

import "testing"

func TestURLPolicyBlocksPrivate(t *testing.T) {
	p := URLPolicy{}
	if err := p.ValidateHTTPURL("http://127.0.0.1/hook"); err == nil {
		t.Fatal("expected localhost block")
	}
	if err := p.ValidateHTTPURL("http://10.0.0.5/hook"); err == nil {
		t.Fatal("expected private block")
	}
	p.AllowLocalhost = true
	if err := p.ValidateHTTPURL("http://127.0.0.1/hook"); err != nil {
		t.Fatalf("allow localhost: %v", err)
	}
	p = URLPolicy{AllowedHosts: []string{"10.0.0.0/8"}}
	if err := p.ValidateHTTPURL("http://10.1.2.3/hook"); err != nil {
		t.Fatalf("allowlisted CIDR: %v", err)
	}
}

func TestURLPolicyRejectsUnresolvedHost(t *testing.T) {
	p := URLPolicy{}
	if err := p.ValidateHTTPURL("http://this-host-should-not-resolve.invalid/hook"); err == nil {
		t.Fatal("expected unresolved host to be rejected")
	}
}
