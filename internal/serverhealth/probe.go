package serverhealth

import (
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

// Notice is one operator alert produced by a health probe.
type Notice struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// Probe talks to one server and returns the snapshot plus alerts that newly apply.
// A nil client counts as an unreachable check.
func Probe(client *ssh.Client, prev store.ServerHealth, serverName string, threshold int, proxyType string, checkPatches bool, dialErr error, now time.Time) (store.ServerHealth, []Notice) {
	if threshold <= 0 {
		threshold = 80
	}
	next := prev
	next.LastCheckedAt = &now
	if next.DiskMount == "" {
		next.DiskMount = "/"
	}
	if next.PatchNotifiedCount == 0 && prev.LastCheckedAt == nil && prev.PatchCount == 0 {
		next.PatchNotifiedCount = -1
	}
	var notices []Notice
	if client == nil {
		next.Reachable = false
		next.ConsecutiveFailures++
		if dialErr != nil {
			next.LastError = trim(dialErr.Error(), 500)
		} else {
			next.LastError = "ssh dial failed"
		}
		if next.ConsecutiveFailures >= 2 && !next.UnreachableNotified {
			next.UnreachableNotified = true
			notices = append(notices, Notice{
				Type:    "server_unreachable",
				Title:   "Server unreachable",
				Message: serverName + " did not answer two checks. " + next.LastError,
			})
		}
		return next, notices
	}
	wasUnreachable := next.UnreachableNotified
	next.Reachable = true
	next.ConsecutiveFailures = 0
	next.LastError = ""
	next.LastReachableAt = &now
	next.UnreachableNotified = false
	if wasUnreachable {
		notices = append(notices, Notice{
			Type:    "server_reachable",
			Title:   "Server reachable",
			Message: serverName + " is accepting SSH again.",
		})
	}

	diskOut, _, _ := sshx.Run(client, DiskCommand)
	if pct, ok := ParseDiskPercent(diskOut); ok {
		next.DiskUsedPercent = &pct
		if pct >= threshold && !next.DiskNotified {
			next.DiskNotified = true
			notices = append(notices, Notice{
				Type:    "server_disk_usage",
				Title:   "Disk usage warning",
				Message: serverName + " disk on / is at " + itoa(pct) + "%, above the " + itoa(threshold) + "% threshold.",
			})
		}
		if pct < threshold {
			next.DiskNotified = false
		}
	}

	if strings.EqualFold(proxyType, "traefik") || proxyType == "" {
		imgOut, _, _ := sshx.Run(client, TraefikImageCommand)
		img := strings.TrimSpace(imgOut)
		next.TraefikImage = img
		next.TraefikOutdated = img != "" && TraefikOutdated(img, proxy.ExpectedTraefikImage)
		if next.TraefikOutdated && next.TraefikNotifiedImage != img {
			next.TraefikNotifiedImage = img
			notices = append(notices, Notice{
				Type:    "traefik_outdated",
				Title:   "Traefik proxy outdated",
				Message: serverName + " is running " + img + ". Dockfin starts " + proxy.ExpectedTraefikImage + ". Restart the proxy to pick up the current image.",
			})
		}
		if !next.TraefikOutdated {
			next.TraefikNotifiedImage = ""
		}
	} else {
		next.TraefikImage = ""
		next.TraefikOutdated = false
		next.TraefikNotifiedImage = ""
	}

	if checkPatches {
		patchOut, _, _ := sshx.Run(client, PatchCommand)
		next.PatchCount = CountPackageLines(patchOut)
		next.LastPatchCheckAt = &now
		if next.PatchCount > 0 && next.PatchCount != next.PatchNotifiedCount {
			next.PatchNotifiedCount = next.PatchCount
			notices = append(notices, Notice{
				Type:    "server_patch",
				Title:   "Server patching",
				Message: serverName + " has " + itoa(next.PatchCount) + " pending package update(s).",
			})
		}
		if next.PatchCount == 0 {
			next.PatchNotifiedCount = 0
		}
	}
	return next, notices
}

func trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
