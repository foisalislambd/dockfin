package dns

import (
	"context"
	"net"
	"strings"

	"github.com/google/uuid"
)

const commentPrefix = "managed-by: dockfin "

// RemoteRecord is one Cloudflare A, AAAA, or CNAME record.
type RemoteRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Comment string `json:"comment"`
}

// RecordAPI is the slice of Cloudflare used by managed DNS sync.
type RecordAPI interface {
	ListAddressRecords(zoneID, hostname string) ([]RemoteRecord, error)
	CreateRecord(zoneID string, rec RemoteRecord) (string, error)
	UpdateContent(zoneID, recordID, content string) error
	GetRecord(zoneID, recordID string) (RemoteRecord, error)
	DeleteRecord(zoneID, recordID string) error
}

// Zone is a synced Cloudflare zone.
type Zone struct {
	ID             uuid.UUID
	TokenID        uuid.UUID
	ProviderZoneID string
	Name           string
}

// Record is a hostname Dockfin is tracking.
type Record struct {
	ID               uuid.UUID
	TeamID           uuid.UUID
	ZoneID           uuid.UUID
	TokenID          uuid.UUID
	ProviderZoneID   string
	ProviderRecordID string
	Type             string
	Name             string
	Content          string
	Owned            bool
	Comment          string
}

// Ledger stores zones, records, and which resources reference a hostname.
type Ledger interface {
	ListDNSZones(ctx context.Context, teamID uuid.UUID) ([]Zone, error)
	DNSRecordByName(ctx context.Context, teamID uuid.UUID, name string) (*Record, error)
	SaveDNSRecord(ctx context.Context, rec Record) (Record, error)
	AddDNSRef(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) error
	RemoveDNSRef(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (remaining int, err error)
	OtherDNSRefs(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (int, error)
	DeleteDNSRecord(ctx context.Context, id uuid.UUID) error
	DNSHostsForResource(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]string, error)
}

// ClientFor opens the Cloudflare client that owns a token.
type ClientFor func(tokenID uuid.UUID) (RecordAPI, error)

// SyncInput publishes the desired hostnames for one resource.
type SyncInput struct {
	TeamID       uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	Hosts        []string
	Content      string
	RecordType   string
	ClientFor    ClientFor
}

// OwnershipComment is stored on records Dockfin creates.
func OwnershipComment(id uuid.UUID) string {
	return commentPrefix + id.String()
}

// CommentMatches reports whether the remote comment is still the one Dockfin wrote.
func CommentMatches(comment string, id uuid.UUID) bool {
	return strings.TrimSpace(comment) == OwnershipComment(id)
}

// RecordIDFromComment reads the record UUID from a Dockfin ownership comment.
func RecordIDFromComment(comment string) (uuid.UUID, bool) {
	comment = strings.TrimSpace(comment)
	if !strings.HasPrefix(comment, commentPrefix) {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimPrefix(comment, commentPrefix))
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// PublicRecord returns an A or AAAA target for a globally routable address.
func PublicRecord(ip string) (content, recType string, ok bool) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil || !parsed.IsGlobalUnicast() || parsed.IsPrivate() {
		return "", "", false
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String(), "A", true
	}
	return parsed.String(), "AAAA", true
}

// MatchZone picks the longest zone suffix for host.
// ambiguous is true when two zones share that length.
func MatchZone(host string, zones []Zone) (Zone, bool, bool) {
	host = normalizeHost(host)
	var best Zone
	bestLen := -1
	ties := 0
	for _, z := range zones {
		name := normalizeHost(z.Name)
		if name == "" {
			continue
		}
		if host != name && !strings.HasSuffix(host, "."+name) {
			continue
		}
		if len(name) > bestLen {
			best = z
			bestLen = len(name)
			ties = 1
			continue
		}
		if len(name) == bestLen {
			ties++
		}
	}
	if bestLen < 0 {
		return Zone{}, false, false
	}
	if ties > 1 {
		return Zone{}, false, true
	}
	return best, true, false
}

// SyncHostnames creates, adopts, or releases Cloudflare records for one resource.
// Domain saves stay successful: every problem is returned as a warning.
func SyncHostnames(ctx context.Context, ledger Ledger, in SyncInput) []string {
	if ledger == nil || in.TeamID == uuid.Nil || in.ResourceID == uuid.Nil {
		return nil
	}
	desired := map[string]struct{}{}
	var warnings []string
	for _, host := range in.Hosts {
		host = normalizeHost(host)
		if host == "" || !publishableHost(host) {
			continue
		}
		desired[host] = struct{}{}
	}
	prev, err := ledger.DNSHostsForResource(ctx, in.TeamID, in.ResourceType, in.ResourceID)
	if err != nil {
		return []string{"dns: could not read managed records"}
	}
	for _, host := range prev {
		host = normalizeHost(host)
		if _, keep := desired[host]; keep {
			continue
		}
		warnings = append(warnings, releaseHost(ctx, ledger, in, host)...)
	}
	recType := strings.ToUpper(strings.TrimSpace(in.RecordType))
	content := strings.TrimSpace(in.Content)
	for host := range desired {
		warnings = append(warnings, ensureHost(ctx, ledger, in, host, content, recType)...)
	}
	return warnings
}

func ensureHost(ctx context.Context, ledger Ledger, in SyncInput, host, content, recType string) []string {
	local, err := ledger.DNSRecordByName(ctx, in.TeamID, host)
	if err != nil {
		return []string{host + ": could not read managed record"}
	}
	// Keep managing a record in the zone where it was created. A longer zone synced
	// later must not create a second copy and leave the original behind.
	var zone Zone
	var ok, amb bool
	if local != nil && local.ProviderZoneID != "" && local.TokenID != uuid.Nil {
		zone = Zone{ID: local.ZoneID, TokenID: local.TokenID, ProviderZoneID: local.ProviderZoneID}
		ok = true
	} else {
		zones, err := ledger.ListDNSZones(ctx, in.TeamID)
		if err != nil {
			return []string{"dns: could not read zones"}
		}
		zone, ok, amb = MatchZone(host, zones)
		if amb {
			return []string{host + ": more than one Cloudflare zone matches"}
		}
		if !ok {
			if len(zones) > 0 {
				return []string{host + ": no synced Cloudflare zone matches"}
			}
			return nil
		}
	}
	if content == "" || (recType != "A" && recType != "AAAA") {
		return []string{host + ": server has no public address for DNS"}
	}
	if in.ClientFor == nil {
		return []string{host + ": cloudflare token unavailable"}
	}
	api, err := in.ClientFor(zone.TokenID)
	if err != nil || api == nil {
		return []string{host + ": cloudflare token unavailable"}
	}
	remote, err := api.ListAddressRecords(zone.ProviderZoneID, host)
	if err != nil {
		return []string{host + ": cloudflare list failed"}
	}
	if local != nil && local.Owned {
		return ensureOwned(ctx, ledger, in, api, zone, *local, remote, host, content, recType)
	}
	if local != nil {
		if len(remote) == 1 && remote[0].ID == local.ProviderRecordID && remote[0].Type == recType && sameContent(remote[0].Content, content) {
			if err := ledger.AddDNSRef(ctx, local.ID, in.ResourceType, in.ResourceID); err != nil {
				return []string{host + ": could not record DNS reference"}
			}
			return nil
		}
		return []string{host + ": existing DNS record is not managed by Dockfin"}
	}
	if len(remote) == 1 {
		if id, ok := RecordIDFromComment(remote[0].Comment); ok {
			return reclaim(ctx, ledger, in, api, zone, remote[0], id, host, content, recType)
		}
		if remote[0].Type == recType && sameContent(remote[0].Content, content) {
			saved, err := ledger.SaveDNSRecord(ctx, Record{
				ID:               uuid.New(),
				TeamID:           in.TeamID,
				ZoneID:           zone.ID,
				TokenID:          zone.TokenID,
				ProviderZoneID:   zone.ProviderZoneID,
				ProviderRecordID: remote[0].ID,
				Type:             recType,
				Name:             host,
				Content:          content,
				Owned:            false,
			})
			if err != nil {
				return []string{host + ": could not save DNS record"}
			}
			if err := ledger.AddDNSRef(ctx, saved.ID, in.ResourceType, in.ResourceID); err != nil {
				return []string{host + ": could not record DNS reference"}
			}
			return nil
		}
	}
	if len(remote) > 0 {
		return []string{host + ": DNS record already exists and was left unchanged"}
	}
	id := uuid.New()
	comment := OwnershipComment(id)
	providerID, err := api.CreateRecord(zone.ProviderZoneID, RemoteRecord{
		Type: recType, Name: host, Content: content, Comment: comment,
	})
	if err != nil {
		return []string{host + ": cloudflare create failed"}
	}
	saved, err := ledger.SaveDNSRecord(ctx, Record{
		ID:               id,
		TeamID:           in.TeamID,
		ZoneID:           zone.ID,
		TokenID:          zone.TokenID,
		ProviderZoneID:   zone.ProviderZoneID,
		ProviderRecordID: providerID,
		Type:             recType,
		Name:             host,
		Content:          content,
		Owned:            true,
		Comment:          comment,
	})
	if err != nil {
		return []string{host + ": record was created at Cloudflare but could not be saved"}
	}
	if err := ledger.AddDNSRef(ctx, saved.ID, in.ResourceType, in.ResourceID); err != nil {
		return []string{host + ": could not record DNS reference"}
	}
	return nil
}

func ensureOwned(ctx context.Context, ledger Ledger, in SyncInput, api RecordAPI, zone Zone, local Record, remote []RemoteRecord, host, content, recType string) []string {
	var mine *RemoteRecord
	for i := range remote {
		if remote[i].ID == local.ProviderRecordID {
			mine = &remote[i]
			break
		}
	}
	if mine == nil && len(remote) == 0 {
		if !sameContent(local.Content, content) {
			others, err := ledger.OtherDNSRefs(ctx, local.ID, in.ResourceType, in.ResourceID)
			if err != nil {
				return []string{host + ": could not read DNS references"}
			}
			if others > 0 {
				return []string{host + ": hostname is already managed for another resource"}
			}
		}
		providerID, err := api.CreateRecord(zone.ProviderZoneID, RemoteRecord{
			Type: recType, Name: host, Content: content, Comment: OwnershipComment(local.ID),
		})
		if err != nil {
			return []string{host + ": cloudflare create failed"}
		}
		local.ProviderZoneID = zone.ProviderZoneID
		local.ZoneID = zone.ID
		local.TokenID = zone.TokenID
		local.ProviderRecordID = providerID
		local.Type = recType
		local.Content = content
		local.Comment = OwnershipComment(local.ID)
		if _, err := ledger.SaveDNSRecord(ctx, local); err != nil {
			return []string{host + ": record was created at Cloudflare but could not be saved"}
		}
		if err := ledger.AddDNSRef(ctx, local.ID, in.ResourceType, in.ResourceID); err != nil {
			return []string{host + ": could not record DNS reference"}
		}
		return nil
	}
	if mine == nil || len(remote) != 1 || !CommentMatches(mine.Comment, local.ID) {
		if mine != nil && len(remote) != 1 {
			return []string{host + ": conflicting DNS records were left unchanged"}
		}
		return []string{host + ": DNS record is no longer managed by Dockfin"}
	}
	if mine.Type != recType {
		return []string{host + ": DNS record type differs from the server address"}
	}
	if !sameContent(mine.Content, content) {
		others, err := ledger.OtherDNSRefs(ctx, local.ID, in.ResourceType, in.ResourceID)
		if err != nil {
			return []string{host + ": could not read DNS references"}
		}
		if others > 0 {
			return []string{host + ": hostname is already managed for another resource"}
		}
		if err := api.UpdateContent(zone.ProviderZoneID, mine.ID, content); err != nil {
			return []string{host + ": cloudflare update failed"}
		}
		local.Content = content
		local.Type = recType
		local.ZoneID = zone.ID
		local.TokenID = zone.TokenID
		local.ProviderZoneID = zone.ProviderZoneID
		local.Comment = OwnershipComment(local.ID)
		if _, err := ledger.SaveDNSRecord(ctx, local); err != nil {
			return []string{host + ": could not save DNS record"}
		}
	}
	if err := ledger.AddDNSRef(ctx, local.ID, in.ResourceType, in.ResourceID); err != nil {
		return []string{host + ": could not record DNS reference"}
	}
	return nil
}

func reclaim(ctx context.Context, ledger Ledger, in SyncInput, api RecordAPI, zone Zone, remote RemoteRecord, id uuid.UUID, host, content, recType string) []string {
	if remote.Type != recType {
		return []string{host + ": DNS record type differs from the server address"}
	}
	saved, err := ledger.SaveDNSRecord(ctx, Record{
		ID:               id,
		TeamID:           in.TeamID,
		ZoneID:           zone.ID,
		TokenID:          zone.TokenID,
		ProviderZoneID:   zone.ProviderZoneID,
		ProviderRecordID: remote.ID,
		Type:             recType,
		Name:             host,
		Content:          content,
		Owned:            true,
		Comment:          OwnershipComment(id),
	})
	if err != nil {
		return []string{host + ": could not save DNS record"}
	}
	if !sameContent(remote.Content, content) {
		if err := api.UpdateContent(zone.ProviderZoneID, remote.ID, content); err != nil {
			return []string{host + ": cloudflare update failed"}
		}
	}
	if err := ledger.AddDNSRef(ctx, saved.ID, in.ResourceType, in.ResourceID); err != nil {
		return []string{host + ": could not record DNS reference"}
	}
	return nil
}

func releaseHost(ctx context.Context, ledger Ledger, in SyncInput, host string) []string {
	local, err := ledger.DNSRecordByName(ctx, in.TeamID, host)
	if err != nil || local == nil {
		return nil
	}
	remaining, err := ledger.RemoveDNSRef(ctx, local.ID, in.ResourceType, in.ResourceID)
	if err != nil {
		return []string{host + ": could not release DNS reference"}
	}
	if remaining > 0 {
		return nil
	}
	if !local.Owned {
		if err := ledger.DeleteDNSRecord(ctx, local.ID); err != nil {
			return []string{host + ": could not forget unmanaged DNS record"}
		}
		return nil
	}
	if in.ClientFor == nil {
		return []string{host + ": cloudflare token unavailable"}
	}
	api, err := in.ClientFor(local.TokenID)
	if err != nil || api == nil {
		return []string{host + ": cloudflare token unavailable"}
	}
	remote, err := api.GetRecord(local.ProviderZoneID, local.ProviderRecordID)
	if err != nil {
		if isNotFound(err) {
			_ = ledger.DeleteDNSRecord(ctx, local.ID)
			return nil
		}
		return []string{host + ": cloudflare lookup failed"}
	}
	if !CommentMatches(remote.Comment, local.ID) {
		_ = ledger.DeleteDNSRecord(ctx, local.ID)
		return []string{host + ": DNS record comment changed, left the Cloudflare record in place"}
	}
	if err := api.DeleteRecord(local.ProviderZoneID, local.ProviderRecordID); err != nil && !isNotFound(err) {
		return []string{host + ": cloudflare delete failed"}
	}
	if err := ledger.DeleteDNSRecord(ctx, local.ID); err != nil {
		return []string{host + ": cloudflare record was deleted but the local row remains"}
	}
	return nil
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 404")
}

func sameContent(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if strings.EqualFold(a, b) {
		return true
	}
	left, right := net.ParseIP(a), net.ParseIP(b)
	return left != nil && right != nil && left.Equal(right)
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func publishableHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}
	if strings.HasSuffix(host, ".sslip.io") || host == "sslip.io" || strings.HasSuffix(host, ".nip.io") || host == "nip.io" {
		return false
	}
	check := host
	if strings.HasPrefix(host, "*.") {
		check = host[2:]
	}
	if strings.ContainsAny(check, " /\\*?") || !strings.Contains(check, ".") {
		return false
	}
	return true
}
