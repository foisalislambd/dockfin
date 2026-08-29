package httpapi

import (
	"net/http"
	"testing"
)

func TestAuditFromPath(t *testing.T) {
	action, rtype, rid := auditFromPath(http.MethodPost, "/api/v1/services/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/deploy")
	if rtype != "services" || rid != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" || action != "deploy" {
		t.Fatalf("got %q %q %q", action, rtype, rid)
	}
}
