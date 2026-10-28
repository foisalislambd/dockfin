// Package githubrunner starts ephemeral GitHub Actions runners on a Dockfin server.
// Each job gets its own container. The host Docker socket is never mounted.
package githubrunner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultImage = "ghcr.io/actions/actions-runner:latest"
	DindImage    = "docker:29.8-dind"
	SecretsDir   = "/data/dockfin/github-runners"
	HookPath     = "/home/runner/dockfin-job-started.sh"
	runnerUID    = "1001"
	dockerGID    = "123"
	pidsLimit    = "4096"
	maxAttempts  = 3
)

// MaxAttempts is how many times a job is started before it is marked failed.
const MaxAttempts = maxAttempts

var (
	labelRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
	imageRE  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,200}$`)
	cpuRE    = regexp.MustCompile(`^(?:[1-9][0-9]{0,2}|0)(?:\.[0-9]{1,2})?$`)
	memoryRE = regexp.MustCompile(`^[1-9][0-9]{0,6}[bkmg]$`)
)

// PullRequestEvents are workflow events a runner refuses unless the server allows them.
var PullRequestEvents = []string{
	"pull_request",
	"pull_request_target",
	"pull_request_review",
	"pull_request_review_comment",
}

// NormalizeLabels parses a comma-separated label list. self-hosted and linux are
// registered automatically and are not accepted as the custom label.
func NormalizeLabels(raw string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		label := strings.ToLower(strings.TrimSpace(part))
		if label == "" || label == "self-hosted" || label == "linux" || seen[label] {
			continue
		}
		if !labelRE.MatchString(label) {
			return nil, fmt.Errorf("label %q must be lowercase letters, digits, dots, underscores, or hyphens", label)
		}
		seen[label] = true
		out = append(out, label)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("add a custom label other than self-hosted or linux")
	}
	if len(out) > 20 {
		return nil, fmt.Errorf("use at most 20 labels")
	}
	return out, nil
}

// RegisteredLabels is the label set sent to GitHub.
func RegisteredLabels(custom []string) []string {
	out := []string{"self-hosted", "linux"}
	seen := map[string]bool{"self-hosted": true, "linux": true}
	for _, label := range custom {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}

// Matches reports whether a job's requested labels belong to this runner.
// The job must ask for at least one custom label, and every requested label must be registered.
func Matches(custom, requested []string) bool {
	customSet := map[string]bool{}
	for _, label := range custom {
		label = strings.ToLower(strings.TrimSpace(label))
		if label != "" {
			customSet[label] = true
		}
	}
	registered := map[string]bool{}
	for _, label := range RegisteredLabels(custom) {
		registered[label] = true
	}
	hit := false
	for _, label := range requested {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" {
			continue
		}
		if !registered[label] {
			return false
		}
		if customSet[label] {
			hit = true
		}
	}
	return hit
}

// IsPullRequestEvent reports whether a workflow run event is a pull request event.
func IsPullRequestEvent(event string) bool {
	event = strings.ToLower(strings.TrimSpace(event))
	for _, name := range PullRequestEvents {
		if event == name {
			return true
		}
	}
	return false
}

// RunnerName is the Docker container name for one execution.
func RunnerName(id uuid.UUID) string {
	return "dockfin-runner-" + id.String()
}

// ValidateImage accepts an empty value (the default image) or a registry reference.
func ValidateImage(image string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil
	}
	if strings.Contains(image, "..") || !imageRE.MatchString(image) {
		return fmt.Errorf("runner image must be a registry reference, without spaces")
	}
	return nil
}

// ValidateCPU accepts an empty value or a CPU count such as 2 or 0.5.
func ValidateCPU(cpu string) error {
	cpu = strings.TrimSpace(cpu)
	if cpu == "" || cpu == "0" || cpuRE.MatchString(cpu) {
		if cpu == "0" {
			return fmt.Errorf("cpu limit must be greater than 0")
		}
		return nil
	}
	return fmt.Errorf("cpu limit must be a number such as 2 or 0.5")
}

// ValidateMemory accepts an empty value or a Docker size such as 4g.
func ValidateMemory(memory string) error {
	memory = strings.ToLower(strings.TrimSpace(memory))
	if memory == "" || memoryRE.MatchString(memory) {
		return nil
	}
	return fmt.Errorf("memory limit must look like 4g or 512m")
}

func ImageOrDefault(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return DefaultImage
	}
	return image
}

// HookScript fails pull request jobs before any step runs.
func HookScript() string {
	events := strings.Join(PullRequestEvents, "|")
	return "#!/bin/bash\n" +
		"case \"${GITHUB_EVENT_NAME:-}\" in\n" +
		"  " + events + "|\"\")\n" +
		"    echo \"::error::This Dockfin runner does not run pull request jobs.\"\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"esac\n" +
		"exit 0\n"
}

// PrepareArgs builds the docker commands that create the network and, in dind mode, the sidecar.
func PrepareArgs(id uuid.UUID, image, mode, cpu, memory string) [][]string {
	name := RunnerName(id)
	labels := labelArgs(id)
	cmds := [][]string{
		append([]string{"docker", "network", "create"}, append(labels, name)...),
		{"docker", "pull", ImageOrDefault(image)},
	}
	if mode != "dind" {
		return cmds
	}
	for _, suffix := range []string{"sock", "work", "externals"} {
		cmds = append(cmds, append([]string{"docker", "volume", "create"}, append(labels, name+"-"+suffix)...))
	}
	cmds = append(cmds, []string{"docker", "pull", DindImage})
	run := []string{"docker", "run", "-d", "--privileged", "--name", name + "-dind", "--network", name}
	run = append(run, labels...)
	run = append(run, limitArgs(cpu, memory)...)
	run = append(run, volumeArgs(name)...)
	run = append(run, DindImage, "dockerd", "--host=unix:///var/run/docker.sock", "--group="+dockerGID)
	cmds = append(cmds, run)
	return cmds
}

// StartArgs builds the docker run command for the runner. The JIT config is read from envFile.
func StartArgs(id uuid.UUID, image, mode, envFile, hookFile, cpu, memory string) []string {
	name := RunnerName(id)
	args := []string{"docker", "run", "-d", "--name", name, "--network", name}
	args = append(args, labelArgs(id)...)
	args = append(args, limitArgs(cpu, memory)...)
	args = append(args, "--env-file", envFile)
	if hookFile != "" {
		args = append(args, "-v", hookFile+":"+HookPath+":ro", "-e", "ACTIONS_RUNNER_HOOK_JOB_STARTED="+HookPath)
	}
	if mode == "dind" {
		args = append(args, "-e", "DOCKER_HOST=unix:///var/run/docker.sock", "-e", "RUNNER_WAIT_FOR_DOCKER_IN_SECONDS=120")
		args = append(args, volumeArgs(name)...)
	}
	args = append(args, ImageOrDefault(image), "/home/runner/run.sh")
	return args
}

// CleanupArgs removes the containers, volumes, network, and secret files of one runner.
func CleanupArgs(id uuid.UUID) [][]string {
	name := RunnerName(id)
	return [][]string{
		{"docker", "rm", "-f", "-v", name, name + "-dind"},
		{"docker", "volume", "rm", name + "-sock", name + "-work", name + "-externals"},
		{"docker", "network", "rm", name},
		{"rm", "-f", EnvPath(id), HookFile(id)},
	}
}

func EnvPath(id uuid.UUID) string {
	return SecretsDir + "/" + id.String() + ".env"
}

func HookFile(id uuid.UUID) string {
	return SecretsDir + "/" + id.String() + "-job-started.sh"
}

func labelArgs(id uuid.UUID) []string {
	return []string{
		"--label", "dockfin.githubRunner=true",
		"--label", "dockfin.githubRunnerExecution=" + id.String(),
	}
}

func limitArgs(cpu, memory string) []string {
	args := []string{"--pids-limit", pidsLimit}
	if cpu = strings.TrimSpace(cpu); cpu != "" {
		args = append(args, "--cpus", cpu)
	}
	if memory = strings.TrimSpace(memory); memory != "" {
		args = append(args, "--memory", memory, "--memory-swap", memory)
	}
	return args
}

func volumeArgs(name string) []string {
	return []string{
		"-v", name + "-sock:/var/run",
		"-v", name + "-work:/home/runner/_work",
		"-v", name + "-externals:/home/runner/externals",
	}
}

// DockerReadyArgs asks the sidecar daemon for its version.
func DockerReadyArgs(id uuid.UUID) []string {
	return []string{"docker", "exec", RunnerName(id) + "-dind", "docker", "version", "--format", "{{.Server.Version}}"}
}

// ChownWorkArgs makes the job directory writable by the runner user inside the sidecar.
func ChownWorkArgs(id uuid.UUID) []string {
	return []string{"docker", "exec", RunnerName(id) + "-dind", "chown", runnerUID + ":" + dockerGID, "/home/runner/_work"}
}
