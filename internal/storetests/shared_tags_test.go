package storetests

import (
	"testing"

	"github.com/google/uuid"
)

func TestSharedEnvAndTagsStayOnTheTeam(t *testing.T) {
	w := newWorld(t, "shared@example.com", "shared")
	other := newWorld(t, "other-shared@example.com", "other")
	v, err := w.st.UpsertSharedEnv(t.Context(), w.team.ID, "team", nil, "API_TOKEN", "super-secret", true)
	if err != nil {
		t.Fatal(err)
	}
	again, err := w.st.UpsertSharedEnv(t.Context(), w.team.ID, "team", nil, "API_TOKEN", "rotated", true)
	if err != nil || again.ID != v.ID {
		t.Fatal(err, again)
	}
	list, err := w.st.ListSharedEnv(t.Context(), w.team.ID, "team", nil, true)
	if err != nil || len(list) != 1 || list[0].Value != "rotated" {
		t.Fatalf("%v %+v", err, list)
	}
	hidden, err := w.st.ListSharedEnv(t.Context(), w.team.ID, "team", nil, false)
	if err != nil || len(hidden) != 1 || hidden[0].Value != "" {
		t.Fatalf("hidden %+v %v", hidden, err)
	}
	theirs, err := other.st.ListSharedEnv(t.Context(), other.team.ID, "team", nil, true)
	if err != nil || len(theirs) != 0 {
		t.Fatal(theirs, err)
	}

	tag, err := w.st.CreateTag(t.Context(), w.team.ID, " prod ", "")
	if err != nil || tag.Name != "prod" || tag.Color == "" {
		t.Fatal(tag, err)
	}
	app := w.app(t, "web", "dockerfile")
	if err := w.st.AttachTag(t.Context(), w.team.ID, tag.ID, "application", app.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.st.AttachTag(t.Context(), w.team.ID, tag.ID, "volume", app.ID); err == nil {
		t.Fatal("attached an unsupported type")
	}
	if err := other.st.AttachTag(t.Context(), other.team.ID, tag.ID, "application", app.ID); err == nil {
		t.Fatal("other team attached our tag")
	}
	got, err := w.st.ListResourceTags(t.Context(), w.team.ID, "application", app.ID)
	if err != nil || len(got) != 1 || got[0].ID != tag.ID {
		t.Fatal(got, err)
	}
	if err := w.st.DetachTag(t.Context(), w.team.ID, tag.ID, "application", app.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.st.DeleteTag(t.Context(), w.team.ID, tag.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.st.DeleteTag(t.Context(), w.team.ID, uuid.New()); err == nil {
		t.Fatal("deleted a missing tag")
	}
}
