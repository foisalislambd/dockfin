package imagescan

import (
	"strings"

	"github.com/dockfin/dockfin/internal/store"
)

// ApplicationImage is the image a build pack leaves on the deploy server.
// Compose applications have no single image; use ComposeImages on the compose file.
func ApplicationImage(app *store.Application) string {
	if app == nil {
		return ""
	}
	switch app.BuildPack {
	case "dockerimage":
		name := strings.TrimSpace(app.DockerRegistryImageName)
		if name == "" {
			return ""
		}
		tag := strings.TrimSpace(app.DockerRegistryImageTag)
		if tag == "" {
			tag = "latest"
		}
		ref := name + ":" + tag
		if !ValidImageRef(ref) {
			return ""
		}
		return ref
	case "dockerfile", "static", "railpack":
		return "dockfin/" + app.ID.String() + ":latest"
	default:
		return ""
	}
}

// ServiceImages reads image references from a service compose file.
func ServiceImages(svc *store.Service) []string {
	if svc == nil {
		return nil
	}
	raw := svc.DockerCompose
	if strings.TrimSpace(raw) == "" {
		raw = svc.DockerComposeRaw
	}
	return ComposeImages(raw)
}
