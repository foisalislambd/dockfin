package store

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestDedupeDestinationIDs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	got, err := DedupeDestinationIDs([]uuid.UUID{a, uuid.Nil, a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("%v", got)
	}
	var many []uuid.UUID
	for i := 0; i < 9; i++ {
		many = append(many, uuid.New())
	}
	if _, err := DedupeDestinationIDs(many); !errors.Is(err, ErrTooManyS3Destinations) {
		t.Fatalf("err %v", err)
	}
}

func TestKeepPrimary(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	kept := KeepPrimary(&a, []uuid.UUID{b, a})
	if kept == nil || *kept != a {
		t.Fatalf("kept %v", kept)
	}
	first := KeepPrimary(&a, []uuid.UUID{b})
	if first == nil || *first != b {
		t.Fatalf("first %v", first)
	}
	if KeepPrimary(nil, nil) != nil {
		t.Fatal("expected no primary")
	}
}

func TestDestinationIDsFallback(t *testing.T) {
	a := uuid.New()
	got := DestinationIDs(&a, nil)
	if len(got) != 1 || got[0] != a {
		t.Fatalf("%v", got)
	}
	got = DestinationIDs(&a, []uuid.UUID{})
	if len(got) != 1 || got[0] != a {
		t.Fatalf("empty slice should use the primary, got %v", got)
	}
}
