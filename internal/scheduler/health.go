package scheduler

import (
	"context"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/notify"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/serverhealth"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func (r *Runner) runServerHealth(ctx context.Context) {
	due, err := r.Store.ListServersDueForHealth(ctx, 5*time.Minute, 15)
	if err != nil {
		r.Logger.Error("list server health", "err", err)
		return
	}
	for _, c := range due {
		r.probeServer(ctx, c)
	}
}

func (r *Runner) probeServer(ctx context.Context, c store.ServerHealthCandidate) {
	prev, err := r.Store.GetServerHealth(ctx, c.TeamID, c.ServerID)
	if err != nil {
		r.Logger.Error("get server health", "server", c.ServerID, "err", err)
		return
	}
	checkPatches := prev.LastPatchCheckAt == nil || time.Since(*prev.LastPatchCheckAt) > 24*time.Hour
	client, dialErr := r.dialServer(ctx, c.TeamID, c.ServerID)
	next, notes := serverhealth.Probe(client, *prev, c.Name, c.DiskThreshold, c.ProxyType, checkPatches, dialErr, time.Now().UTC())
	next.ServerID = c.ServerID
	next.TeamID = c.TeamID
	if err := r.Store.SaveServerHealth(ctx, next); err != nil {
		r.Logger.Error("save server health", "server", c.ServerID, "err", err)
	}
	if client != nil && (c.ProxyType == "" || strings.EqualFold(c.ProxyType, "traefik")) {
		if err := r.syncProxyEdge(ctx, client, c.TeamID, c.ServerID); err != nil {
			r.Logger.Error("sync proxy edge", "server", c.ServerID, "err", err)
		}
	}
	for _, n := range notes {
		(&notify.Dispatcher{Store: r.Store}).Send(ctx, c.TeamID, notify.Event{
			Type:    n.Type,
			Title:   n.Title,
			Message: n.Message,
		})
	}
	_ = r.Store.UpdateServerReachability(ctx, c.ServerID, next.Reachable)
}

func (r *Runner) syncProxyEdge(ctx context.Context, client *ssh.Client, teamID, serverID uuid.UUID) error {
	targets, err := r.Store.ListServerMaintenance(ctx, teamID, serverID)
	if err != nil {
		return err
	}
	var hosts []proxy.MaintenanceHost
	for _, t := range targets {
		hosts = append(hosts, maintenanceHosts(t.ResourceID.String(), t.FQDN, t.Message, t.ForceHTTPS)...)
		if t.ResourceType != "service" {
			continue
		}
		domains, err := r.Store.ListServiceDomains(ctx, teamID, t.ResourceID)
		if err != nil {
			continue
		}
		for _, d := range domains {
			hosts = append(hosts, maintenanceHosts(t.ResourceID.String(), d.Host, t.Message, d.ForceHTTPS)...)
		}
	}
	cfg, err := proxy.BuildMaintenanceConfig(hosts)
	if err != nil {
		return err
	}
	network := "dockfin"
	if dests, err := r.Store.ListDestinations(ctx, teamID, &serverID); err == nil && len(dests) > 0 && dests[0].Network != "" {
		network = dests[0].Network
	}
	if err := proxy.SyncMaintenance(client, network, cfg); err != nil {
		return err
	}
	ops, err := r.Store.GetServerOpsSettings(ctx, teamID, serverID)
	if err != nil {
		return err
	}
	raw, err := proxy.BuildCatchallYAML(proxy.CatchallConfig{
		Enabled:   ops.ProxyCatchallEnabled,
		URL:       ops.ProxyCatchallURL,
		Permanent: ops.ProxyCatchallPermanent,
	})
	if err != nil {
		return err
	}
	return proxy.SyncCatchall(client, raw)
}

func maintenanceHosts(id, raw, message string, https bool) []proxy.MaintenanceHost {
	var out []proxy.MaintenanceHost
	for _, h := range proxy.HostsFromDomainList(raw) {
		out = append(out, proxy.MaintenanceHost{ID: id, Host: h, Message: message, HTTPS: https})
	}
	if len(out) == 0 {
		if h := proxy.HostFromDomainEntry(raw); h != "" {
			out = append(out, proxy.MaintenanceHost{ID: id, Host: h, Message: message, HTTPS: https})
		}
	}
	return out
}
