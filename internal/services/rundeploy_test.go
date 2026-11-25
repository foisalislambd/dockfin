package services

import "testing"

func TestPreferURLFromMagicEnv(t *testing.T) {
	base, host := PreferURLFromMagicEnv(map[string]string{
		"SERVICE_URL_A": "http://skip.127.0.0.1.sslip.io",
		"SERVICE_URL_B": "https://app.example.com/health",
		"SERVICE_URL_C": "",
	})
	if base != "https://app.example.com/health" || host != "app.example.com" {
		t.Fatalf("url %q host %q", base, host)
	}
	base, host = PreferURLFromMagicEnv(map[string]string{
		"SERVICE_FQDN_WEB": "app.example.com",
		"SERVICE_FQDN_BAD": "http://nope.127.0.0.1.sslip.io",
	})
	if base != "https://app.example.com" || host != "app.example.com" {
		t.Fatalf("fqdn %q host %q", base, host)
	}
	base, host = PreferURLFromMagicEnv(map[string]string{"OTHER": "https://app.example.com"})
	if base != "" || host != "" {
		t.Fatalf("ignored %q %q", base, host)
	}
}

func TestLooksLikeUnpreparedCompose(t *testing.T) {
	raw := "services:\n  app:\n    image: nginx\n    volumes:\n      - data:/var/lib\n"
	if !looksLikeUnpreparedCompose(raw, "") {
		t.Fatal("empty prepared should need prepare")
	}
	if !looksLikeUnpreparedCompose(raw, raw) {
		t.Fatal("unchanged compose should need prepare")
	}
	if !looksLikeUnpreparedCompose(raw, "services:\n  app:\n    image: nginx\n    environment:\n      - SERVICE_URL_APP\n") {
		t.Fatal("magic env placeholder should need prepare")
	}
	if !looksLikeUnpreparedCompose(raw, "services:\n  app:\n    image: nginx:$SERVICE_TAG\n") {
		t.Fatal("unexpanded magic value should need prepare")
	}
	prepared := "services:\n  app:\n    image: nginx\n    volumes:\n      - data:/var/lib\nvolumes:\n  data:\n"
	if looksLikeUnpreparedCompose(raw, prepared) {
		t.Fatal("named volume with a top-level volumes block is prepared")
	}
	if !hasNamedVolumeMounts(raw) || !hasTopLevelVolumes(prepared) || hasTopLevelVolumes(raw) {
		t.Fatal("volume helpers mismatch")
	}
}

func TestDetectPublicIPNilClient(t *testing.T) {
	if got := detectPublicIP(nil); got != "" {
		t.Fatalf("nil client returned %q", got)
	}
}
