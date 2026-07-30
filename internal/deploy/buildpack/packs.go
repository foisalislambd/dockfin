package buildpack

// Names of supported application build packs.
const (
	Dockerfile    = "dockerfile"
	DockerCompose = "dockercompose"
	DockerImage   = "dockerimage"
	Nixpacks      = "nixpacks"
	Static        = "static"
	Railpack      = "railpack"
)

func Supported(name string) bool {
	switch name {
	case Dockerfile, DockerCompose, DockerImage, Nixpacks, Static, Railpack:
		return true
	default:
		return false
	}
}
