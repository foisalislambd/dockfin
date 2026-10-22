package services

import (
	"sort"
	"strings"

	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/store"
)

// PublishedServiceDomains is the host list Traefik should serve.
// Custom domains replace magic domains on the same router, because a mixed
// list disables Let's Encrypt. Magic hosts are used only when no custom host exists.
// Order is oldest first so the first saved domain stays the primary host.
func PublishedServiceDomains(domains []store.ServiceDomain) []store.ServiceDomain {
	custom := make([]store.ServiceDomain, 0, len(domains))
	magic := make([]store.ServiceDomain, 0, len(domains))
	for _, d := range domains {
		if strings.TrimSpace(d.Host) == "" {
			continue
		}
		if proxy.IsMagicDomainHost(d.Host) {
			magic = append(magic, d)
			continue
		}
		custom = append(custom, d)
	}
	chosen := custom
	if len(chosen) == 0 {
		chosen = magic
	}
	sort.SliceStable(chosen, func(i, j int) bool {
		return chosen[i].CreatedAt.Before(chosen[j].CreatedAt)
	})
	return chosen
}
