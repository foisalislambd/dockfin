package version

// Version is injected at release build time via:
//
//	-ldflags "-X github.com/dockfin/dockfin/internal/version.Version=X.Y.Z"
//
// GitHub Actions Release workflow sets this to match the git tag / Docker tag.
var Version = "dev"
