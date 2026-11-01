package store

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNormalizeFreezeReason(t *testing.T) {
	got, err := NormalizeFreezeReason("  holiday \n freeze\t ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "holiday freeze" {
		t.Fatalf("got %q", got)
	}
	if _, err := NormalizeFreezeReason(strings.Repeat("a", 241)); !errors.Is(err, ErrFreezeReasonTooLong) {
		t.Fatal(err)
	}
}

func TestFrozenDeployError(t *testing.T) {
	err := FrozenDeployError("shipping window")
	if !errors.Is(err, ErrDeployFrozen) {
		t.Fatal(err)
	}
	if err.Error() != "deployments are frozen for this environment: shipping window" {
		t.Fatalf("%q", err.Error())
	}
	if FrozenDeployError("  ").Error() != ErrDeployFrozen.Error() {
		t.Fatal(FrozenDeployError(" "))
	}
}

func TestSecondsBetween(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Second)
	got := SecondsBetween(&start, &end)
	if got == nil || *got != 90 {
		t.Fatalf("%v", got)
	}
	if SecondsBetween(nil, &end) != nil {
		t.Fatal("nil start")
	}
	if SecondsBetween(&end, &start) != nil {
		t.Fatal("negative")
	}
}

func TestPrepareAttentionPrefersFailedDeploy(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	other := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	items := []AttentionItem{
		{Kind: "unhealthy", ResourceType: "application", ResourceID: id, Message: "Status is failed", At: now},
		{Kind: "deploy_failed", ResourceType: "application", ResourceID: id, Message: "build failed", At: now.Add(time.Minute)},
		{Kind: "backup_failed", ResourceType: "database", ResourceID: other, Message: "s3 timeout", At: now.Add(2 * time.Minute)},
		{Kind: "backup_failed", ResourceType: "database", ResourceID: other, Message: "s3 timeout again", At: now},
	}
	out := PrepareAttention(items)
	if len(out) != 3 {
		t.Fatalf("len %d %#v", len(out), out)
	}
	var deploy, backups int
	for _, item := range out {
		if item.ResourceID == id {
			if item.Kind != "deploy_failed" {
				t.Fatalf("kept %s", item.Kind)
			}
			deploy++
		}
		if item.Kind == "backup_failed" {
			backups++
		}
	}
	if deploy != 1 || backups != 2 {
		t.Fatalf("deploy %d backups %d", deploy, backups)
	}
}

func TestPrepareAttentionCapsAndEmpty(t *testing.T) {
	if len(PrepareAttention(nil)) != 0 {
		t.Fatal("nil")
	}
	items := make([]AttentionItem, 0, 20)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		items = append(items, AttentionItem{
			Kind:         "backup_failed",
			ResourceType: "database",
			ResourceID:   uuid.New(),
			At:           base.Add(time.Duration(i) * time.Minute),
		})
	}
	out := PrepareAttention(items)
	if len(out) != 12 {
		t.Fatalf("cap %d", len(out))
	}
	if !out[0].At.After(out[len(out)-1].At) {
		t.Fatal("expected newest first")
	}
}

func TestClampListLimit(t *testing.T) {
	if clampListLimit(0, 50, 100) != 50 {
		t.Fatal("default")
	}
	if clampListLimit(10, 50, 100) != 10 {
		t.Fatal("keep")
	}
	if clampListLimit(500, 50, 100) != 100 {
		t.Fatal("cap")
	}
}
