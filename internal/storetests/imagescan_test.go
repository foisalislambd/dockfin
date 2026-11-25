package storetests

import (
	"errors"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/store"
)

func TestImageScans(t *testing.T) {
	w := newWorld(t, "scan@example.com", "scans")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID

	if err := st.SetImageScanPolicy(ctx, true, "nope"); err == nil {
		t.Fatal("invalid gate accepted")
	}
	if err := st.SetImageScanPolicy(ctx, true, "block"); err != nil {
		t.Fatal(err)
	}
	policy, err := st.ImageScanPolicy(ctx)
	if err != nil || !policy.Enabled || policy.Gate != "block" {
		t.Fatalf("policy %+v %v", policy, err)
	}

	app := w.app(t, "img", "dockerimage")
	scan, err := st.InsertImageScan(ctx, store.ImageScan{
		TeamID: teamID, ResourceType: "application", ResourceID: app.ID,
		ImageRef: "nginx:1.27", Status: "completed", High: 2,
		Findings: []byte(`[{"id":"CVE-2024-1","severity":"high"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if scan.ID.String() == "" || scan.High != 2 || scan.FinishedAt == nil {
		t.Fatalf("inserted %+v", scan)
	}
	if err := st.SetImageScanCandidates(ctx, teamID, scan.ID, []byte(`{"tags":["1.28"]}`)); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetImageScan(ctx, teamID, scan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ImageRef != "nginx:1.27" || got.High != 2 || !strings.Contains(string(got.Findings), "CVE-2024-1") || !strings.Contains(string(got.Candidates), "1.28") {
		t.Fatalf("scan %+v", got)
	}
	listed, err := st.ListImageScans(ctx, teamID, 10)
	if err != nil || len(listed) != 1 || listed[0].ID != scan.ID {
		t.Fatalf("list %+v %v", listed, err)
	}
	if err := st.SetImageScanCandidates(ctx, teamID, scan.ID, nil); err != nil {
		t.Fatal(err)
	}
	cleared, err := st.GetImageScan(ctx, teamID, scan.ID)
	if err != nil || string(cleared.Candidates) == "" || strings.Contains(string(cleared.Candidates), "1.28") {
		t.Fatalf("cleared candidates %s %v", cleared.Candidates, err)
	}

	if err := st.SetApplicationImageTag(ctx, teamID, app.ID, "bad tag"); err == nil {
		t.Fatal("invalid tag accepted")
	}
	if err := st.SetApplicationImageTag(ctx, teamID, app.ID, "1.28.0"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := st.GetApplication(ctx, teamID, app.ID)
	if err != nil || reloaded.DockerRegistryImageTag != "1.28.0" || reloaded.BuildPack != "dockerimage" {
		t.Fatalf("tag %+v %v", reloaded, err)
	}
	other := w.app(t, "src", "dockerfile")
	if err := st.SetApplicationImageTag(ctx, teamID, other.ID, "1.2.3"); err == nil {
		t.Fatal("dockerfile app accepted a tag change")
	}

	ignores, err := st.ImageScanIgnores(ctx, teamID)
	if err != nil || len(ignores) != 0 {
		t.Fatalf("ignores %#v %v", ignores, err)
	}
	if err := st.IgnoreImageVuln(ctx, teamID, "CVE-2024-1", "accepted"); err != nil {
		t.Fatal(err)
	}
	if err := st.IgnoreImageVuln(ctx, teamID, "CVE-2024-1", "still accepted"); err != nil {
		t.Fatal(err)
	}
	ignores, err = st.ImageScanIgnores(ctx, teamID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ignores["CVE-2024-1"]; !ok || len(ignores) != 1 {
		t.Fatalf("ignores %#v", ignores)
	}
	if err := st.DeleteImageVulnIgnore(ctx, teamID, "CVE-2024-1"); err != nil {
		t.Fatal(err)
	}
	ignores, err = st.ImageScanIgnores(ctx, teamID)
	if err != nil || len(ignores) != 0 {
		t.Fatalf("after delete %#v %v", ignores, err)
	}
	if err := st.DeleteImageVulnIgnore(ctx, teamID, "CVE-2024-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete %v", err)
	}
}
