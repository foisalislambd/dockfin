package services

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeUnit is one container service inside a compose stack.
type ComposeUnit struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// ParseComposeUnits lists services from a compose YAML document.
func ParseComposeUnits(compose string) []ComposeUnit {
	compose = strings.TrimSpace(compose)
	if compose == "" {
		return nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(compose), &doc); err != nil {
		return nil
	}
	svcs, _ := doc["services"].(map[string]any)
	if svcs == nil {
		return nil
	}
	names := make([]string, 0, len(svcs))
	for name := range svcs {
		names = append(names, name)
	}
	// stable-ish: prefer web-like first then alpha
	sortComposeNames(names)
	out := make([]ComposeUnit, 0, len(names))
	for _, name := range names {
		svc, _ := svcs[name].(map[string]any)
		image := ""
		if svc != nil {
			image, _ = svc["image"].(string)
		}
		out = append(out, ComposeUnit{Name: name, Image: image})
	}
	return out
}

func sortComposeNames(names []string) {
	preferred := []string{"wordpress", "ghost", "n8n", "umami", "frontend", "nginx", "app", "web"}
	rank := func(n string) int {
		lower := strings.ToLower(n)
		for i, p := range preferred {
			if lower == p || strings.HasPrefix(lower, p+"-") {
				return i
			}
		}
		return 100 + len(n)
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			ri, rj := rank(names[i]), rank(names[j])
			if rj < ri || (rj == ri && names[j] < names[i]) {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
}

// ComposeVolume is a bind or named volume mount from a compose service.
type ComposeVolume struct {
	Service   string `json:"service"`
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	HostPath  string `json:"host_path,omitempty"`
	Type      string `json:"type"` // named | bind | anonymous
}

// ParseComposeVolumes extracts volume mounts from compose services.
func ParseComposeVolumes(compose string) []ComposeVolume {
	compose = strings.TrimSpace(compose)
	if compose == "" {
		return nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(compose), &doc); err != nil {
		return nil
	}
	svcs, _ := doc["services"].(map[string]any)
	if svcs == nil {
		return nil
	}
	names := make([]string, 0, len(svcs))
	for name := range svcs {
		names = append(names, name)
	}
	sortComposeNames(names)
	var out []ComposeVolume
	for _, svcName := range names {
		svc, _ := svcs[svcName].(map[string]any)
		if svc == nil {
			continue
		}
		raw, ok := svc["volumes"]
		if !ok {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			switch v := item.(type) {
			case string:
				parts := strings.SplitN(v, ":", 3)
				if len(parts) < 2 {
					out = append(out, ComposeVolume{
						Service: svcName, Name: "", MountPath: parts[0], Type: "anonymous",
					})
					continue
				}
				src, dest := parts[0], parts[1]
				typ := "named"
				host := ""
				if strings.HasPrefix(src, "/") || strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../") || src == "." {
					typ = "bind"
					host = src
				} else {
					host = ""
				}
				out = append(out, ComposeVolume{
					Service: svcName, Name: src, MountPath: dest, HostPath: host, Type: typ,
				})
			case map[string]any:
				src, _ := v["source"].(string)
				dest, _ := v["target"].(string)
				typ, _ := v["type"].(string)
				if typ == "" {
					typ = "volume"
				}
				if typ == "volume" {
					typ = "named"
				}
				host := ""
				if typ == "bind" {
					host = src
				}
				out = append(out, ComposeVolume{
					Service: svcName, Name: src, MountPath: dest, HostPath: host, Type: typ,
				})
			}
		}
	}
	return out
}
