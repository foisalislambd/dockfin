package scan

import "strings"

// Result is the build pack a repository scan suggests.
type Result struct {
	BuildPack        string   `json:"build_pack"`
	Dockerfile       string   `json:"dockerfile,omitempty"`
	ComposeFile      string   `json:"compose_file,omitempty"`
	PublishDirectory string   `json:"publish_directory,omitempty"`
	Reasons          []string `json:"reasons"`
}

// Detect picks a Dockfin build pack from repository paths.
// Railpack is the build pack Dockfin uses where Coolify would pick Nixpacks.
func Detect(paths []string) Result {
	var dockerfiles, composes, manifests []string
	hasIndex := false
	for _, raw := range paths {
		p := strings.Trim(strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")), "/")
		if p == "" || strings.Contains(p, "..") {
			continue
		}
		base := p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			base = p[i+1:]
		}
		depth := strings.Count(p, "/")
		switch strings.ToLower(base) {
		case "dockerfile":
			if depth <= 1 {
				dockerfiles = append(dockerfiles, p)
			}
		case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
			if depth == 0 {
				composes = append(composes, p)
			}
		case "package.json", "go.mod", "requirements.txt", "pyproject.toml", "gemfile", "composer.json", "cargo.toml", "mix.exs", "pom.xml", "build.gradle", "build.gradle.kts":
			if depth == 0 {
				manifests = append(manifests, p)
			}
		case "index.html":
			if depth == 0 {
				hasIndex = true
			}
		}
	}
	if len(composes) > 0 {
		return Result{
			BuildPack:   "dockercompose",
			ComposeFile: composes[0],
			Reasons:     []string{"Found " + composes[0]},
		}
	}
	if len(dockerfiles) > 0 {
		file := dockerfiles[0]
		for _, f := range dockerfiles {
			if strings.EqualFold(f, "Dockerfile") {
				file = f
			}
		}
		return Result{
			BuildPack:  "dockerfile",
			Dockerfile: file,
			Reasons:    []string{"Found " + file},
		}
	}
	if len(manifests) > 0 {
		return Result{
			BuildPack: "railpack",
			Reasons:   []string{"Found " + manifests[0] + ". Railpack builds it the way Nixpacks would in Coolify."},
		}
	}
	if hasIndex {
		return Result{
			BuildPack:        "static",
			PublishDirectory: "/",
			Reasons:          []string{"Found index.html and no Dockerfile or application manifest."},
		}
	}
	return Result{
		BuildPack: "dockerfile",
		Reasons:   []string{"No Dockerfile, Compose file, or application manifest was found. Dockerfile is the fallback."},
	}
}
