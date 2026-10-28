package store

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DNSZone is a Cloudflare zone synced for a team token.
type DNSZone struct {
	ID             uuid.UUID
	TeamID         uuid.UUID
	TokenID        uuid.UUID
	ProviderZoneID string
	Name           string
}

// DNSZoneInput is one zone returned by Cloudflare.
type DNSZoneInput struct {
	ProviderZoneID string
	Name           string
}

// DNSRecord is a hostname Dockfin tracks in Cloudflare.
type DNSRecord struct {
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

// ReplaceDNSZones upserts the token's zones and drops zones that no longer exist
// and have no managed records. Retained lists zone names that still have records.
func (s *Store) ReplaceDNSZones(ctx context.Context, teamID, tokenID uuid.UUID, zones []DNSZoneInput) (kept []DNSZone, retained []string, err error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	ids := make([]string, 0, len(zones))
	for _, z := range zones {
		name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(z.Name)), ".")
		providerID := strings.TrimSpace(z.ProviderZoneID)
		if name == "" || providerID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO dns_zones (team_id, cloud_token_id, provider_zone_id, name)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (cloud_token_id, provider_zone_id) DO UPDATE
			SET name=EXCLUDED.name, updated_at=NOW()
		`, teamID, tokenID, providerID, name); err != nil {
			return nil, nil, err
		}
		ids = append(ids, providerID)
	}
	rows, err := tx.Query(ctx, `
		SELECT name FROM dns_zones z
		WHERE z.cloud_token_id=$1
		  AND NOT (z.provider_zone_id = ANY($2::text[]))
		  AND EXISTS (SELECT 1 FROM managed_dns_records r WHERE r.zone_id=z.id)
	`, tokenID, ids)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, nil, err
		}
		retained = append(retained, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM dns_zones z
		WHERE z.cloud_token_id=$1
		  AND NOT (z.provider_zone_id = ANY($2::text[]))
		  AND NOT EXISTS (SELECT 1 FROM managed_dns_records r WHERE r.zone_id=z.id)
	`, tokenID, ids); err != nil {
		return nil, nil, err
	}
	listed, err := tx.Query(ctx, `
		SELECT id, team_id, cloud_token_id, provider_zone_id, name
		FROM dns_zones WHERE cloud_token_id=$1 AND team_id=$2 ORDER BY name
	`, tokenID, teamID)
	if err != nil {
		return nil, nil, err
	}
	defer listed.Close()
	for listed.Next() {
		var z DNSZone
		if err := listed.Scan(&z.ID, &z.TeamID, &z.TokenID, &z.ProviderZoneID, &z.Name); err != nil {
			return nil, nil, err
		}
		kept = append(kept, z)
	}
	if err := listed.Err(); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return kept, retained, nil
}

// ListDNSZones returns every synced zone for the team.
func (s *Store) ListDNSZones(ctx context.Context, teamID uuid.UUID) ([]DNSZone, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, cloud_token_id, provider_zone_id, name
		FROM dns_zones WHERE team_id=$1 ORDER BY length(name) DESC, name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DNSZone
	for rows.Next() {
		var z DNSZone
		if err := rows.Scan(&z.ID, &z.TeamID, &z.TokenID, &z.ProviderZoneID, &z.Name); err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

// DNSRecordByName loads the managed record for a hostname, if any.
func (s *Store) DNSRecordByName(ctx context.Context, teamID uuid.UUID, name string) (*DNSRecord, error) {
	var rec DNSRecord
	err := s.Pool.QueryRow(ctx, `
		SELECT r.id, r.team_id, r.zone_id, r.cloud_token_id, z.provider_zone_id,
		       r.provider_record_id, r.type, r.name, r.content, r.owned, r.comment
		FROM managed_dns_records r
		JOIN dns_zones z ON z.id = r.zone_id
		WHERE r.team_id=$1 AND r.name=$2
	`, teamID, name).Scan(
		&rec.ID, &rec.TeamID, &rec.ZoneID, &rec.TokenID, &rec.ProviderZoneID,
		&rec.ProviderRecordID, &rec.Type, &rec.Name, &rec.Content, &rec.Owned, &rec.Comment,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// SaveDNSRecord inserts or updates the hostname row. The stored id is returned.
func (s *Store) SaveDNSRecord(ctx context.Context, rec DNSRecord) (DNSRecord, error) {
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO managed_dns_records (
			id, team_id, zone_id, cloud_token_id, provider_record_id, type, name, content, owned, comment
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (team_id, name) DO UPDATE SET
			zone_id=EXCLUDED.zone_id,
			cloud_token_id=EXCLUDED.cloud_token_id,
			provider_record_id=EXCLUDED.provider_record_id,
			type=EXCLUDED.type,
			content=EXCLUDED.content,
			owned=EXCLUDED.owned,
			comment=CASE
				WHEN managed_dns_records.id = EXCLUDED.id THEN EXCLUDED.comment
				WHEN EXCLUDED.owned THEN 'managed-by: dockfin ' || managed_dns_records.id::text
				ELSE EXCLUDED.comment
			END,
			updated_at=NOW()
		RETURNING id, team_id, zone_id, cloud_token_id, provider_record_id, type, name, content, owned, comment
	`, rec.ID, rec.TeamID, rec.ZoneID, rec.TokenID, rec.ProviderRecordID, rec.Type, rec.Name, rec.Content, rec.Owned, rec.Comment).Scan(
		&rec.ID, &rec.TeamID, &rec.ZoneID, &rec.TokenID, &rec.ProviderRecordID, &rec.Type, &rec.Name, &rec.Content, &rec.Owned, &rec.Comment,
	)
	if err != nil {
		return DNSRecord{}, err
	}
	rec.ProviderZoneID = ""
	if row, err := s.DNSRecordByName(ctx, rec.TeamID, rec.Name); err == nil && row != nil {
		return *row, nil
	}
	return rec, nil
}

// AddDNSRef records that a resource uses this hostname.
func (s *Store) AddDNSRef(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO managed_dns_refs (record_id, resource_type, resource_id)
		VALUES ($1,$2,$3)
		ON CONFLICT DO NOTHING
	`, recordID, resourceType, resourceID)
	return err
}

// OtherDNSRefs counts references other than this resource.
func (s *Store) OtherDNSRefs(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM managed_dns_refs
		WHERE record_id=$1 AND NOT (resource_type=$2 AND resource_id=$3)
	`, recordID, resourceType, resourceID).Scan(&n)
	return n, err
}

// RemoveDNSRef drops one resource reference and returns how many remain.
func (s *Store) RemoveDNSRef(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (int, error) {
	if _, err := s.Pool.Exec(ctx, `
		DELETE FROM managed_dns_refs
		WHERE record_id=$1 AND resource_type=$2 AND resource_id=$3
	`, recordID, resourceType, resourceID); err != nil {
		return 0, err
	}
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM managed_dns_refs WHERE record_id=$1`, recordID).Scan(&n)
	return n, err
}

// DeleteDNSRecord removes the local row. References cascade.
func (s *Store) DeleteDNSRecord(ctx context.Context, id uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM managed_dns_records WHERE id=$1`, id)
	return err
}

// DNSHostsForResource lists hostnames currently referenced by a resource.
func (s *Store) DNSHostsForResource(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.name
		FROM managed_dns_records r
		JOIN managed_dns_refs f ON f.record_id = r.id
		WHERE r.team_id=$1 AND f.resource_type=$2 AND f.resource_id=$3
	`, teamID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
