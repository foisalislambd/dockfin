package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestApplyRemoteSecrets(t *testing.T) {
	s := &Store{
		ResolveRemoteSecrets: func(context.Context, uuid.UUID, string, uuid.UUID) (map[string]string, error) {
			return map[string]string{"DB": "p@ss"}, nil
		},
	}
	team := uuid.New()
	id := uuid.New()
	values := map[string]string{
		"URL": "x={{vault.DB}}",
		"LIT": "{{vault.DB}}",
	}
	literal := map[string]bool{"LIT": true}
	if err := s.ApplyRemoteSecrets(context.Background(), team, "application", id, values, literal); err != nil {
		t.Fatal(err)
	}
	if values["URL"] != "x=p@ss" || values["LIT"] != "{{vault.DB}}" {
		t.Fatalf("values: %#v", values)
	}

	missing := map[string]string{"URL": "{{vault.OTHER}}"}
	if err := s.ApplyRemoteSecrets(context.Background(), team, "application", id, missing, nil); err == nil {
		t.Fatal("expected missing key to fail")
	}
	if missing["URL"] != "{{vault.OTHER}}" {
		t.Fatalf("missing key was substituted: %#v", missing)
	}

	s.ResolveRemoteSecrets = func(context.Context, uuid.UUID, string, uuid.UUID) (map[string]string, error) {
		return nil, ErrNotFound
	}
	if err := s.ApplyRemoteSecrets(context.Background(), team, "application", id, map[string]string{"URL": "{{vault.DB}}"}, nil); err == nil {
		t.Fatal("expected missing link to fail")
	}
}
