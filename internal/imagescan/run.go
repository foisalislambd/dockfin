package imagescan

import (
	"fmt"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

const (
	SyftImage  = "anchore/syft:v1.18.1"
	GrypeImage = "anchore/grype:v0.87.0"
)

// Run executes Syft and Grype on the server that has the image.
// The SBOM stays on that server and is deleted after Grype reads it.
func Run(client *ssh.Client, imageRef string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("ssh client required")
	}
	imageRef = strings.TrimSpace(imageRef)
	if !ValidImageRef(imageRef) {
		return nil, fmt.Errorf("invalid image reference")
	}
	sbom := "/tmp/dockfin-sbom-" + strings.ReplaceAll(imageRef, "/", "_")
	sbom = strings.Map(func(r rune) rune {
		switch r {
		case ':', '@', '.':
			return '-'
		default:
			if r > 127 {
				return -1
			}
			return r
		}
	}, sbom)
	if len(sbom) > 80 {
		sbom = sbom[:80]
	}
	sbom += ".json"
	defer func() { _, _, _ = sshx.RunArgs(client, "rm", "-f", sbom) }()

	_, errOut, err := sshx.RunArgs(client,
		"docker", "run", "--rm",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", "/tmp:/tmp",
		SyftImage, imageRef, "-o", "syft-json="+sbom,
	)
	if err != nil {
		return nil, fmt.Errorf("syft: %v %s", err, strings.TrimSpace(errOut))
	}
	stdout, errOut, err := sshx.RunArgs(client,
		"docker", "run", "--rm",
		"-v", sbom+":/sbom.json:ro",
		GrypeImage, "sbom:/sbom.json", "-o", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("grype: %v %s", err, strings.TrimSpace(errOut))
	}
	if strings.TrimSpace(stdout) == "" {
		return nil, fmt.Errorf("grype returned an empty report")
	}
	return []byte(stdout), nil
}
