package deploy

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// TransferImage streams docker save on buildHost into docker load on deployHost.
func TransferImage(buildHost, deployHost *ssh.Client, image string) error {
	buildSess, err := buildHost.NewSession()
	if err != nil {
		return fmt.Errorf("build session: %w", err)
	}
	defer buildSess.Close()

	deploySess, err := deployHost.NewSession()
	if err != nil {
		return fmt.Errorf("deploy session: %w", err)
	}
	defer deploySess.Close()

	buildOut, err := buildSess.StdoutPipe()
	if err != nil {
		return err
	}
	deployIn, err := deploySess.StdinPipe()
	if err != nil {
		return err
	}

	var deployErr bytes.Buffer
	deploySess.Stderr = &deployErr

	if err := deploySess.Start("docker load"); err != nil {
		return fmt.Errorf("docker load start: %w", err)
	}

	if err := buildSess.Start("docker save " + image); err != nil {
		_ = deploySess.Close()
		return fmt.Errorf("docker save start: %w", err)
	}

	_, copyErr := io.Copy(deployIn, buildOut)
	_ = deployIn.Close()
	buildWait := buildSess.Wait()
	deployWait := deploySess.Wait()

	if copyErr != nil {
		return fmt.Errorf("image stream: %w", copyErr)
	}
	if buildWait != nil {
		return fmt.Errorf("docker save: %w", buildWait)
	}
	if deployWait != nil {
		msg := deployErr.String()
		if msg != "" {
			return fmt.Errorf("docker load: %w (%s)", deployWait, msg)
		}
		return fmt.Errorf("docker load: %w", deployWait)
	}
	return nil
}
