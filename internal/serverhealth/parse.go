package serverhealth

import (
	"strconv"
	"strings"
)

// ParseDiskPercent reads the use percentage from `df -P` output.
// It accepts either the full table or a single integer line.
func ParseDiskPercent(out string) (int, bool) {
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(out, "%")); err == nil && n >= 0 && n <= 100 {
		return n, true
	}
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		fields := strings.Fields(lines[i])
		if len(fields) < 5 {
			continue
		}
		cap := strings.TrimSuffix(fields[4], "%")
		n, err := strconv.Atoi(cap)
		if err != nil || n < 0 || n > 100 {
			continue
		}
		return n, true
	}
	return 0, false
}

// ImageTag returns the tag of a docker image reference.
func ImageTag(image string) string {
	image = strings.TrimSpace(image)
	if i := strings.LastIndex(image, "@"); i >= 0 {
		image = image[:i]
	}
	slash := strings.LastIndex(image, "/")
	colon := strings.LastIndex(image, ":")
	if colon < 0 || colon < slash {
		return ""
	}
	return image[colon+1:]
}

// TraefikOutdated reports whether the running image is older than the expected tag.
// Non-semver tags such as "latest" are not reported as outdated.
func TraefikOutdated(runningImage, expectedImage string) bool {
	run := parseVersion(ImageTag(runningImage))
	want := parseVersion(ImageTag(expectedImage))
	if run == nil || want == nil {
		return false
	}
	if run[0] != want[0] {
		return run[0] < want[0]
	}
	if run[1] != want[1] {
		return run[1] < want[1]
	}
	return run[2] < want[2]
}

func parseVersion(tag string) []int {
	tag = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(tag)), "v")
	if tag == "" || tag == "latest" {
		return nil
	}
	parts := strings.Split(tag, ".")
	if len(parts) < 2 {
		return nil
	}
	out := []int{0, 0, 0}
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

// CountPackageLines counts pending package lines. Empty output and the
// "no updates" sentinels count as zero.
func CountPackageLines(out string) int {
	out = strings.TrimSpace(out)
	if out == "" || strings.HasPrefix(out, "No pending") || strings.HasPrefix(out, "no supported package manager") {
		return 0
	}
	n := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Listing...") {
			continue
		}
		n++
	}
	return n
}

const (
	DiskCommand = `df -P /`
	// TraefikImageCommand prints the proxy image, or nothing when it is absent.
	TraefikImageCommand = `docker inspect -f '{{.Config.Image}}' dockfin-proxy 2>/dev/null || true`
	PatchCommand        = `if command -v apt-get >/dev/null 2>&1; then apt-get update -qq >/dev/null 2>&1; apt list --upgradable 2>/dev/null | tail -n +2; elif command -v dnf >/dev/null 2>&1; then dnf -q check-update || true; elif command -v yum >/dev/null 2>&1; then yum -q check-update || true; elif command -v apk >/dev/null 2>&1; then apk update >/dev/null 2>&1; apk list --upgradable; else echo "no supported package manager found"; fi`
)
