package httpapi

import "testing"

func TestDeleteOptionsDefaults(t *testing.T) {
	var unset DeleteOptions
	if !unset.volumes() || !unset.configurations() || !unset.networks() || !unset.dockerCleanup() {
		t.Fatalf("nil flags should default to true")
	}

	f := false
	tr := true
	off := DeleteOptions{
		DeleteVolumes:        &f,
		DeleteConfigurations: &f,
		DeleteNetworks:       &f,
		DockerCleanup:        &f,
	}
	if off.volumes() || off.configurations() || off.networks() || off.dockerCleanup() {
		t.Fatalf("explicit false should stay false")
	}
	on := DeleteOptions{
		DeleteVolumes:        &tr,
		DeleteConfigurations: &tr,
		DeleteNetworks:       &tr,
		DockerCleanup:        &tr,
	}
	if !on.volumes() || !on.configurations() || !on.networks() || !on.dockerCleanup() {
		t.Fatalf("explicit true should stay true")
	}
}

func TestProtectedNetworks(t *testing.T) {
	for _, name := range []string{"bridge", "host", "none", "goolify", "coolify"} {
		if _, ok := protectedNetworks[name]; !ok {
			t.Fatalf("expected %q to be protected", name)
		}
	}
	if _, ok := protectedNetworks["aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"]; ok {
		t.Fatalf("resource UUID networks must not be protected")
	}
}
